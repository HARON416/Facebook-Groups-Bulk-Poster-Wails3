import assert from 'node:assert/strict';
import test from 'node:test';
import { downloadPrefix, feedPackages, generateFeeds, validateFeed } from './update-feeds.mjs';
import { generateKeyPairSync } from 'node:crypto';
import { spawnSync } from 'node:child_process';
import { existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { tmpdir } from 'node:os';
import { writeDownloadFixture, writeVersionFixture } from './release-test-fixtures.mjs';
import { flattenRelease } from './flatten-release.mjs';

test('feeds select install-compatible payloads and exclude first-install wrappers', () => {
  const feeds = feedPackages('0.1.2');
  assert.deepEqual(feeds['stable.json'], [
    'postika-0.1.2-windows-amd64.exe', 'postika-0.1.2-darwin-amd64.app.zip',
    'postika-0.1.2-darwin-arm64.app.zip', 'postika-0.1.2-linux-amd64.AppImage',
  ]);
  assert.equal(Object.keys(feeds).length, 5);
  assert.equal(new Set(Object.values(feeds).flat()).size, 8);
  for (const [feed, suffix] of [['deb', '.deb'], ['rpm', '.rpm'], ['arch', '.pkg.tar.zst'], ['binary', '-binary']]) {
    assert.deepEqual(feeds[`stable-${feed}.json`], [`postika-0.1.2-linux-amd64${suffix}`]);
  }
  assert.throws(() => feedPackages('../bad'));
});

const project = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const cli = join(project, '.tools/bin', process.platform === 'win32' ? 'wails3.exe' : 'wails3');
test('pinned Wails signs and verifies all five feeds; retry reuses signatures and tampering is rejected', { skip: !existsSync(cli) }, t => {
  const root = mkdtempSync(join(tmpdir(), 'postika-signed-feeds-'));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  writeVersionFixture(root, '0.1.4');
  const download = join(root, 'bin/releases/ci-1');
  writeDownloadFixture(download, '0.1.4');
  flattenRelease(root, download, '0.1.4');
  const { privateKey, publicKey } = generateKeyPairSync('ed25519', {
    privateKeyEncoding: { type: 'pkcs8', format: 'pem' }, publicKeyEncoding: { type: 'spki', format: 'pem' },
  });
  mkdirSync(join(root, '.update-keys'));
  mkdirSync(join(root, 'internal/updateauth'), { recursive: true });
  writeFileSync(join(root, '.update-keys/updater.key'), privateKey, { mode: 0o600 });
  writeFileSync(join(root, 'internal/updateauth/public-key.pem'), publicKey);
  const run = args => {
    const result = spawnSync(cli, args, { encoding: 'utf8' });
    if (result.status !== 0) throw new Error('Wails rejected fixture manifest');
  };
  const directory = generateFeeds(root, run);
  const path = join(directory, 'stable.json');
  const original = readFileSync(path, 'utf8');
  assert.equal(generateFeeds(root, run), directory);
  assert.equal(readFileSync(path, 'utf8'), original);
  const manifest = JSON.parse(original);
  assert.equal(manifest.artifacts.length, 4);
  assert.ok(manifest.artifacts.every(artifact => artifact.url.includes('/downloads/0.1.4/')));
  manifest.artifacts[0].signature = 'AAAA';
  writeFileSync(path, JSON.stringify(manifest));
  assert.throws(() => generateFeeds(root, run), /Wails rejected/);
});

test('manifest guard rejects wrong URLs, platform, missing signatures and duplicated artifacts', () => {
  const names = feedPackages('0.1.2')['stable.json'];
  const manifest = {
    version: '0.1.2', channel: 'stable',
    artifacts: names.map(name => {
      const [, platform, arch] = /-(windows|darwin|linux)-(amd64|arm64)/.exec(name);
      return { url: downloadPrefix('0.1.2') + name, platform, arch, digestAlgo: 'sha512', signatureAlgo: 'ed25519ph', signature: 'verified separately by Wails' };
    }),
  };
  validateFeed(manifest, '0.1.2', names);
  for (const mutation of [
    m => m.version = '0.1.1',
    m => m.channel = 'beta',
    m => m.artifacts.pop(),
    m => m.artifacts[0].url = 'https://other.example/app.exe',
    m => m.artifacts[0].platform = 'linux',
    m => m.artifacts[0].arch = 'arm64',
    m => m.artifacts[0].signature = '',
    m => m.artifacts[0].digestAlgo = 'sha256',
    m => m.artifacts[1] = m.artifacts[0],
  ]) {
    const copy = structuredClone(manifest);
    mutation(copy);
    assert.throws(() => validateFeed(copy, '0.1.2', names));
  }
});
