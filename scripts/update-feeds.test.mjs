import assert from 'node:assert/strict';
import test from 'node:test';
import { downloadPrefix, feedPackages, validateFeed } from './update-feeds.mjs';

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

test('manifest guard rejects wrong URLs, platform, missing signatures and duplicated artifacts', () => {
  const names = feedPackages('0.1.2')['stable.json'];
  const manifest = {
    version: '0.1.2', channel: 'stable',
    artifacts: names.map(name => {
      const [, platform, arch] = /-(windows|darwin|linux)-(amd64|arm64)/.exec(name);
      return { url: downloadPrefix + name, platform, arch, digestAlgo: 'sha512', signatureAlgo: 'ed25519ph', signature: 'verified separately by Wails' };
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
