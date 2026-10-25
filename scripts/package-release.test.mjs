import test from 'node:test';
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { mkdtempSync, mkdirSync, writeFileSync, readFileSync, existsSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { targets, packageRelease, releaseVersion, stageRelease } from './package-release.mjs';
import { writeVersionFixture } from './release-test-fixtures.mjs';

function fixture(t) {
  const root = mkdtempSync(join(tmpdir(), 'postika-release-'));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  writeVersionFixture(root);
  mkdirSync(join(root, 'bin'), { recursive: true });
  writeFileSync(join(root, 'LICENSE'), 'Fixture license\n');
  return root;
}

for (const [target, plan] of Object.entries(targets)) {
  test(`${target}: complete artifact set, architecture names, license, and checksums`, t => {
    const root = fixture(t);
    for (const [name] of plan.files) writeFileSync(join(root, 'bin', name), `Fixture payload ${name}`);
    if (target.startsWith('darwin-')) {
      mkdirSync(join(root, 'bin/postika.app/Contents/MacOS'), { recursive: true });
      writeFileSync(join(root, 'bin/postika.app/Contents/MacOS/postika'), 'Fixture app');
    }
    const output = stageRelease(root, target, releaseVersion(root));
    const sums = readFileSync(join(output, 'SHA256SUMS'), 'utf8').trim().split('\n');
    assert.equal(sums.length, plan.files.length + 1);
    for (const row of sums) {
      const [hash, name] = row.split('  ');
      assert.equal(hash, createHash('sha256').update(readFileSync(join(output, name))).digest('hex'));
      assert.ok(name === 'LICENSE.txt' || name.startsWith(`postika-0.1.0-${target}`));
    }
    assert.throws(() => stageRelease(root, target, '0.1.0'), /already staged/);
    if (target.startsWith('darwin-')) assert.ok(existsSync(join(output, `postika-0.1.0-${target}.app/Contents/MacOS/postika`)));
  });
}

test('missing artifacts do not create a partial release', t => {
  const root = fixture(t);
  assert.throws(() => stageRelease(root, 'linux-amd64', '0.1.0'), /Missing package/);
  assert.equal(existsSync(join(root, 'bin/releases')), false);
});

test('failed packaging never stages old bin outputs', t => {
  const root = fixture(t);
  assert.throws(() => packageRelease(root, 'linux-amd64', { platform: 'linux', run: () => ({ status: 1 }) }), /failed/);
  assert.equal(existsSync(join(root, 'bin/releases')), false);
});

test('unsupported hosts fail before running the packaging command', t => {
  const root = fixture(t);
  assert.throws(() => packageRelease(root, 'darwin-arm64', { platform: 'linux', run: () => assert.fail('command executed') }), /native darwin runner/);
});

test('target task and architecture are passed explicitly', t => {
  const root = fixture(t);
  assert.throws(() => packageRelease(root, 'windows-amd64', {
    platform: 'win32',
    run: (_command, args) => { assert.deepEqual(args, ['task', 'windows:package:all', 'ARCH=amd64']); return { status: 1 }; },
  }), /failed/);
});
