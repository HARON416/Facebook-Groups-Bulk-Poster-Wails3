import test from 'node:test';
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { bunnyClient, bunnyConfig, publishRelease } from './publish-release.mjs';
import { writeDownloadFixture, writeVersionFixture } from './release-test-fixtures.mjs';
import { flattenRelease } from './flatten-release.mjs';
import { feedPackages } from './update-feeds.mjs';

function fixture(t) {
  const root = mkdtempSync(join(tmpdir(), 'postika-publish-'));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  writeVersionFixture(root, '0.1.4');
  const download = join(root, 'bin/releases/ci-1');
  writeDownloadFixture(download, '0.1.4');
  flattenRelease(root, download, '0.1.4');
  const feeds = join(root, 'bin/updates/0.1.4');
  mkdirSync(feeds, { recursive: true });
  for (const [name, packages] of Object.entries(feedPackages('0.1.4'))) {
    writeFileSync(join(feeds, name), JSON.stringify({ version: '0.1.4', artifacts: packages.map(name => ({ url: name })) }));
  }
  const events = [];
  const objects = new Map([['', '[]']]);
  const client = {
    async getStorage(path) { events.push(['GET', path]); return objects.get(path) ?? null; },
    async upload(file, path) { events.push(['PUT', path]); objects.set(path, readFileSync(file, 'utf8')); },
    async verifyPublic(file, path, feed) { events.push(['VERIFY', path, feed]); assert.equal(objects.get(path), readFileSync(file, 'utf8')); },
  };
  return { root, feeds, client, events, objects, options: { root, client, prepareFeeds: () => feeds, log: () => {} } };
}

test('configuration requires the storage password and constrains credential destinations', () => {
  const env = { BUNNY_STORAGE_ZONE: 'postika-desktop-application', BUNNY_STORAGE_HOST: 'ny.storage.bunnycdn.com', BUNNY_STORAGE_PASSWORD: 'fixture-secret' };
  assert.equal(bunnyConfig(env).zone, env.BUNNY_STORAGE_ZONE);
  assert.throws(() => bunnyConfig({ ...env, BUNNY_STORAGE_PASSWORD: '' }), /BUNNY_STORAGE_PASSWORD/);
  assert.throws(() => bunnyConfig({ ...env, BUNNY_STORAGE_HOST: 'evil.example' }), /regional Bunny/);
  assert.throws(() => bunnyConfig({ ...env, BUNNY_STORAGE_ZONE: '../elsewhere' }), /Invalid/);
});

test('all packages are publicly verified before any stable feed, and retries preserve version', async t => {
  const f = fixture(t);
  assert.equal(await publishRelease(f.options), '0.1.4');
  const firstFeed = f.events.findIndex(([method, path]) => method === 'PUT' && path.startsWith('updates/'));
  assert.equal(f.events.slice(0, firstFeed).filter(([method, path]) => method === 'VERIFY' && path.startsWith('downloads/0.1.4/')).length, 15);
  assert.equal(f.events.filter(([method, path]) => method === 'PUT' && path.startsWith('updates/')).length, 5);
  assert.ok(!existsSync(join(f.root, 'bin/.publish.lock')));
  assert.equal(await publishRelease(f.options), '0.1.4');
});

test('failed public package verification leaves all stable feeds untouched', async t => {
  const f = fixture(t);
  f.client.verifyPublic = async () => { throw new Error('CDN checksum mismatch'); };
  await assert.rejects(publishRelease(f.options), /make publish-release/);
  assert.ok(!f.events.some(([method, path]) => method === 'PUT' && path.startsWith('updates/')));
  assert.ok(!existsSync(join(f.root, 'bin/.publish.lock')));
});

test('failed authentication or signing makes no uploads', async t => {
  for (const failure of ['auth', 'sign']) {
    const f = fixture(t);
    if (failure === 'auth') f.client.getStorage = async () => { throw new Error('HTTP 401'); };
    else f.options.prepareFeeds = () => { throw new Error('Signature verification failed'); };
    await assert.rejects(publishRelease(f.options));
    assert.ok(!f.events.some(([method]) => method === 'PUT'));
  }
});

test('remote newer feeds, conflicting same-version payloads or checksums cannot be overwritten', async t => {
  for (const kind of ['newer', 'same', 'checksums']) {
    const f = fixture(t);
    if (kind === 'checksums') f.objects.set('downloads/0.1.4/SHA256SUMS', 'different');
    else f.objects.set('updates/stable.json', JSON.stringify({ version: kind === 'newer' ? '0.2.0' : '0.1.4', artifacts: [] }));
    await assert.rejects(publishRelease(f.options), /refusing|different|Different/);
    assert.ok(!f.events.some(([method]) => method === 'PUT'));
  }
});

test('a partial feed failure can be resumed without bumping or replacing different payloads', async t => {
  const f = fixture(t);
  const upload = f.client.upload;
  let failed = false;
  f.client.upload = async (file, path) => {
    if (!failed && path === 'updates/stable-deb.json') { failed = true; throw new Error('Network failure'); }
    await upload(file, path);
  };
  await assert.rejects(publishRelease(f.options), /Retry this version/);
  assert.equal(await publishRelease(f.options), '0.1.4');
});

test('storage transport sends uppercase checksums, streams raw bytes, and never sends credentials to CDN', async t => {
  const file = join(mkdtempSync(join(tmpdir(), 'postika-transport-')), 'fixture.json');
  t.after(() => rmSync(join(file, '..'), { recursive: true, force: true }));
  const payload = Buffer.from('{"version":"0.1.4"}');
  writeFileSync(file, payload);
  const calls = [];
  const client = bunnyClient({ host: 'ny.storage.bunnycdn.com', zone: 'fixture-zone', password: 'fixture-secret' }, {
    sleep: async () => {},
    request: async (url, options) => {
      calls.push({ url, options });
      assert.equal(options.redirect, 'error');
      if (options.method === 'PUT') {
        assert.equal(options.headers.AccessKey, 'fixture-secret');
        assert.equal(options.headers.Checksum, createHash('sha256').update(payload).digest('hex').toUpperCase());
        const chunks = [];
        for await (const chunk of options.body) chunks.push(chunk);
        assert.deepEqual(Buffer.concat(chunks), payload);
        return new Response('', { status: 201 });
      }
      assert.equal(options.headers.AccessKey, undefined);
      return new Response(payload, { headers: { 'Content-Type': 'application/json', 'Cache-Control': 'no-cache, no-store' } });
    },
  });
  await client.upload(file, 'updates/stable.json');
  await client.verifyPublic(file, 'updates/stable.json', true);
  assert.equal(calls[0].url, 'https://ny.storage.bunnycdn.com/fixture-zone/updates/stable.json');
});

test('public verification rejects stale bytes and cacheable JSON without logging a password', async t => {
  const root = mkdtempSync(join(tmpdir(), 'postika-cdn-'));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  const file = join(root, 'feed.json');
  writeFileSync(file, '{}');
  for (const mode of ['stale', 'cached']) {
    const client = bunnyClient({ host: 'storage.bunnycdn.com', zone: 'fixture', password: 'fixture-secret' }, {
      sleep: async () => {}, request: async () => new Response(mode === 'stale' ? 'old' : '{}', { headers: { 'Content-Type': 'application/json', 'Cache-Control': mode === 'cached' ? 'public, max-age=2592000' : 'no-cache' } }),
    });
    await assert.rejects(client.verifyPublic(file, 'updates/stable.json', true), error => !error.message.includes('fixture-secret'));
  }
});
