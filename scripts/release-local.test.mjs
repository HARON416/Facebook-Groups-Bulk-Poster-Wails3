import test from 'node:test';
import assert from 'node:assert/strict';
import { existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { releaseLocal } from './release-local.mjs';
import { stageRelease, targets, releaseVersion } from './package-release.mjs';
import { verifyFlatRelease } from './flatten-release.mjs';
import { generateWebsiteRelease, saveWebsiteRelease } from './website-release.mjs';
import { feedPackages } from './update-feeds.mjs';
import { writeVersionFixture } from './release-test-fixtures.mjs';

function fixture(t) {
  const root = mkdtempSync(join(tmpdir(), 'postika-local-release-'));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  writeVersionFixture(root);
  mkdirSync(join(root, 'bin'), { recursive: true });
  writeFileSync(join(root, 'LICENSE'), 'Fixture license\n');
  const calls = [];
  return { root, calls, options: { root, preflight() {}, validate() {}, fingerprint: () => 'source-1', log() {},
    build(root, target, options) {
      calls.push(target);
      assert.equal(options.cross, true);
      for (const [name, suffix] of targets[target].files) {
        if (suffix !== '.dmg') writeFileSync(join(root, 'bin', name), `payload ${target}${suffix}`);
      }
      return stageRelease(root, target, releaseVersion(root), { ...options, macZipOnly: true });
    },
  } };
}

test('local release produces a complete ZIP-based release, website links and unchanged updater URLs', async t => {
  const { root, calls, options } = fixture(t);
  const output = await releaseLocal(options);
  assert.deepEqual(calls, Object.keys(targets));
  assert.equal(releaseVersion(root), '0.1.1');
  verifyFlatRelease(output, '0.1.1');
  assert.equal(existsSync(join(output, 'postika-0.1.1-darwin-arm64.dmg')), false);
  const metadata = JSON.parse(readFileSync(generateWebsiteRelease(root), 'utf8'));
  assert.equal(metadata.downloads.length, 8);
  assert.ok(metadata.downloads.filter(item => item.os === 'macos').every(item => item.url.endsWith('.app.zip')));
  saveWebsiteRelease(root, metadata);
  for (const names of Object.values(feedPackages('0.1.1'))) {
    for (const name of names) assert.ok(existsSync(join(output, name)), name);
  }
  await releaseLocal({ ...options, change: 'none' });
  assert.equal(calls.length, 4, 'completed release is verified and reused');
});

test('failed target resumes prior verified packages without a second version bump', async t => {
  const { root, calls, options } = fixture(t);
  await assert.rejects(releaseLocal({ ...options, build(root, target, config) {
    if (target === 'darwin-arm64') throw new Error('compiler failed');
    return options.build(root, target, config);
  } }), /compiler failed/);
  assert.equal(existsSync(join(root, 'bin/releases/0.1.1')), false);
  await releaseLocal({ ...options, change: 'none' });
  assert.deepEqual(calls, Object.keys(targets));
});

test('changed source and corrupt staged packages cannot be resumed or published', async t => {
  const { root, options } = fixture(t);
  await assert.rejects(releaseLocal({ ...options, build(root, target, config) {
    if (target === 'windows-amd64') throw new Error('interrupted');
    return options.build(root, target, config);
  } }));
  await assert.rejects(releaseLocal({ ...options, change: 'none', fingerprint: () => 'different-source' }), /Source changed/);
  writeFileSync(join(root, 'bin/releases/ci-local-0.1.1/linux-amd64/postika-0.1.1-linux-amd64.deb'), 'corrupt');
  await assert.rejects(releaseLocal({ ...options, change: 'none' }), /checksum mismatch/);
});

test('missing tools fail before changing version; publishing preflight also runs before builds', async t => {
  const { root, options } = fixture(t);
  await assert.rejects(releaseLocal({ ...options, preflight() { throw new Error('missing Docker'); } }), /missing Docker/);
  await assert.rejects(releaseLocal({ ...options, publish: true, preparePublish() { throw new Error('missing signing key'); } }), /missing signing key/);
  assert.equal(releaseVersion(root), '0.1.0');
  assert.equal(existsSync(join(root, 'bin/.release-local.lock')), false);
});

test('only a complete verified local release reaches Bunny publication', async t => {
  const { root, options } = fixture(t);
  const client = {};
  let published = false;
  await releaseLocal({ ...options, publish: true, preparePublish: async () => client,
    async publishPackages(args) {
      assert.equal(args.client, client);
      verifyFlatRelease(join(root, 'bin/releases/0.1.1'), '0.1.1');
      published = true;
    },
  });
  assert.equal(published, true);
});

test('partial DMG sets remain invalid', async t => {
  const { options } = fixture(t);
  const output = await releaseLocal(options);
  writeFileSync(join(output, 'postika-0.1.1-darwin-amd64.dmg'), 'unexpected');
  assert.throws(() => verifyFlatRelease(output, '0.1.1'), /incomplete or contains unexpected/);
});

test('edits during a build stop the release before publication', async t => {
  const { root, options } = fixture(t);
  let source = 'source-1';
  await assert.rejects(releaseLocal({ ...options, fingerprint: () => source,
    build(root, target, config) {
      const output = options.build(root, target, config);
      source = 'changed';
      return output;
    },
  }), /Source changed during the build/);
  assert.equal(existsSync(join(root, 'bin/releases/0.1.1')), false);
});
