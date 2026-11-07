import test from 'node:test';
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { bunnyClient, bunnyConfig, pruneRemoteReleases, publishRelease } from './publish-release.mjs';
import { websiteRelease } from './release-layout.mjs';
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
    async getStorage(path) {
      events.push(['GET', path]);
      if (path === 'downloads/') return JSON.stringify([...new Set([...objects.keys()].map(path => /^downloads\/([^/]+)\//.exec(path)?.[1]).filter(Boolean))].map(ObjectName => ({ ObjectName, IsDirectory: true })));
      return objects.get(path) ?? null;
    },
    async upload(file, path) { events.push(['PUT', path]); objects.set(path, readFileSync(file, 'utf8')); },
    async verifyPublic(file, path, feed) { events.push(['VERIFY', path, feed]); assert.equal(objects.get(path), readFileSync(file, 'utf8')); },
    async deleteRelease(version) {
      events.push(['DELETE', `downloads/${version}/`]);
      for (const path of objects.keys()) if (path.startsWith(`downloads/${version}/`)) objects.delete(path);
    },
  };
  return { root, feeds, client, events, objects, options: { root, client, prepareFeeds: () => feeds, log: () => {} } };
}

function seedRemoteRelease(objects, version) {
  const metadata = websiteRelease(version);
  objects.set(`downloads/${version}/release.json`, JSON.stringify(metadata));
  const names = metadata.downloads.map(item => item.url.split('/').at(-1));
  names.push('LICENSE.txt');
  objects.set(`downloads/${version}/SHA256SUMS`, names.map(name => `${'a'.repeat(64)}  ${name}`).join('\n') + '\n');
}

test('Bunny retention keeps current and previous releases, deleting older folders after every feed is verified', async t => {
  const f = fixture(t);
  for (const version of ['0.1.1', '0.1.2', '0.1.3']) seedRemoteRelease(f.objects, version);
  f.objects.set('downloads/legacy/postika.exe', 'keep');
  f.objects.set('downloads/postika-0.1.2.exe', 'legacy flat file');
  f.objects.set('updates/metadata.json', 'keep');
  await publishRelease(f.options);
  assert.deepEqual(f.events.filter(([method]) => method === 'DELETE').map(([, path]) => path), ['downloads/0.1.2/', 'downloads/0.1.1/']);
  for (const path of ['downloads/0.1.3/release.json', 'downloads/0.1.4/release.json', 'downloads/legacy/postika.exe', 'downloads/postika-0.1.2.exe', 'updates/metadata.json']) assert.ok(f.objects.has(path), path);
  const firstDelete = f.events.findIndex(([method]) => method === 'DELETE');
  assert.equal(f.events.slice(0, firstDelete).filter(([method, path]) => method === 'VERIFY' && path.startsWith('updates/')).length, 5);
  f.events.length = 0;
  await publishRelease(f.options);
  assert.equal(f.events.some(([method]) => method === 'DELETE'), false, 'retry does not delete retained releases');
});

test('incomplete remote folders do not displace the previous complete release', async t => {
  const f = fixture(t);
  seedRemoteRelease(f.objects, '0.1.1');
  seedRemoteRelease(f.objects, '0.1.2');
  f.objects.set('downloads/0.1.3/release.json', '{"version":"0.1.3"}');
  await publishRelease(f.options);
  assert.deepEqual(f.events.filter(([method]) => method === 'DELETE').map(([, path]) => path), ['downloads/0.1.1/']);
  assert.ok(f.objects.has('downloads/0.1.2/release.json'));
  assert.ok(f.objects.has('downloads/0.1.3/release.json'));
});

test('failed publication never deletes Bunny releases', async t => {
  const f = fixture(t);
  for (const version of ['0.1.1', '0.1.2', '0.1.3']) seedRemoteRelease(f.objects, version);
  f.client.verifyPublic = async (_file, path) => { if (path.startsWith('updates/')) throw new Error('feed unavailable'); };
  await assert.rejects(publishRelease(f.options), /feed unavailable/);
  assert.equal(f.events.some(([method]) => method === 'DELETE'), false);
});

test('changed feeds or a newer remote build prevent remote cleanup', async t => {
  const f = fixture(t);
  for (const version of ['0.1.1', '0.1.2', '0.1.3', '0.1.4']) seedRemoteRelease(f.objects, version);
  await assert.rejects(pruneRemoteReleases(f.client, '0.1.4', f.feeds), /feeds changed/);
  seedRemoteRelease(f.objects, '0.1.5');
  await assert.rejects(pruneRemoteReleases(f.client, '0.1.4', f.feeds), /inventory changed/);
  assert.equal(f.events.some(([method]) => method === 'DELETE'), false);
});

test('cleanup failure does not turn a verified publication into a failed release', async t => {
  const f = fixture(t);
  for (const version of ['0.1.1', '0.1.2', '0.1.3']) seedRemoteRelease(f.objects, version);
  const logs = [];
  f.client.deleteRelease = async () => { throw new Error('cleanup denied'); };
  assert.equal(await publishRelease({ ...f.options, log: message => logs.push(message) }), '0.1.4');
  assert.ok(logs.some(message => message.includes('Bunny cleanup could not finish')));
  assert.ok(f.objects.has('downloads/0.1.1/release.json'));
});

test('Bunny DELETE is restricted to validated version folders and treats missing folders as already removed', async () => {
  const calls = [];
  const client = bunnyClient({ host: 'storage.bunnycdn.com', zone: 'fixture', password: 'secret' }, {
    request: async (url, options) => { calls.push([url, options]); return new Response(null, { status: 404 }); },
  });
  await client.deleteRelease('0.1.2');
  assert.equal(calls[0][0], 'https://storage.bunnycdn.com/fixture/downloads/0.1.2/');
  assert.equal(calls[0][1].method, 'DELETE');
  assert.equal(calls[0][1].headers.AccessKey, 'secret');
  for (const invalid of ['../updates', '', '0.1.2/../../', '01.2.3']) await assert.rejects(client.deleteRelease(invalid));
  assert.equal(calls.length, 1);
});

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
  await assert.rejects(publishRelease(f.options), /Retry this version/);
  assert.ok(!f.events.some(([method, path]) => method === 'PUT' && path.startsWith('updates/')));
  assert.ok(!existsSync(join(f.root, 'bin/.publish.lock')));
});

test('successful publication removes older local packages and duplicate staging, keeping retry files', async t => {
  const f = fixture(t);
  const older = join(f.root, 'bin/releases/0.1.3');
  mkdirSync(older);
  writeFileSync(join(older, 'postika-0.1.3-linux-amd64.deb'), 'old package');
  const oldStage = join(f.root, 'bin/releases/ci-local-0.1.3');
  const currentStage = join(f.root, 'bin/releases/ci-local-0.1.4');
  const newerStage = join(f.root, 'bin/releases/ci-local-0.1.5');
  writeDownloadFixture(oldStage, '0.1.3');
  writeDownloadFixture(currentStage, '0.1.4');
  writeDownloadFixture(newerStage, '0.1.5');
  const notes = join(f.root, 'bin/releases/ci-local-0.1.2');
  mkdirSync(notes);
  writeFileSync(join(notes, 'notes.txt'), 'Keep unknown files');
  const verify = f.client.verifyPublic;
  f.client.verifyPublic = async (...args) => {
    assert.ok(existsSync(older), 'cleanup must wait for all CDN verification');
    return verify(...args);
  };
  await publishRelease(f.options);
  for (const path of [older, oldStage, currentStage]) assert.equal(existsSync(path), false);
  for (const path of [newerStage, notes, f.feeds, join(f.root, 'bin/releases/0.1.4')]) assert.ok(existsSync(path));
  f.client.verifyPublic = verify;
  assert.equal(await publishRelease(f.options), '0.1.4', 'current release can still be republished');
});

test('failed publication keeps previous packages and staging for recovery', async t => {
  const f = fixture(t);
  const older = join(f.root, 'bin/releases/0.1.3');
  mkdirSync(older);
  writeFileSync(join(older, 'postika-0.1.3-linux-amd64.deb'), 'old package');
  const staging = join(f.root, 'bin/releases/ci-local-0.1.4');
  writeDownloadFixture(staging, '0.1.4');
  f.client.verifyPublic = async (_file, path) => {
    if (path.startsWith('updates/')) throw new Error('Feed verification failed');
  };
  await assert.rejects(publishRelease(f.options), /Feed verification failed/);
  assert.ok(existsSync(older));
  assert.ok(existsSync(staging));
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

test('upload retries network/server failures using fresh streams with all bytes', async t => {
  const directory = mkdtempSync(join(tmpdir(), 'postika-upload-retry-'));
  t.after(() => rmSync(directory, { recursive: true, force: true }));
  const file = join(directory, 'fixture.bin');
  const payload = Buffer.from('complete fixture upload');
  writeFileSync(file, payload);
  let attempts = 0;
  const delays = [];
  const bodies = new Set();
  const client = bunnyClient({ host: 'storage.bunnycdn.com', zone: 'fixture', password: 'fixture-secret' }, {
    sleep: async delay => delays.push(delay),
    request: async (url, options) => {
      attempts++;
      assert.ok(!bodies.has(options.body));
      bodies.add(options.body);
      const chunks = [];
      for await (const chunk of options.body) chunks.push(chunk);
      assert.deepEqual(Buffer.concat(chunks), payload);
      if (attempts === 1) throw new TypeError('fixture-secret', { cause: Object.assign(new Error(), { code: 'ECONNRESET' }) });
      return new Response('', { status: attempts === 2 ? 503 : 201 });
    },
  });
  await client.upload(file, 'downloads/0.1.5/fixture.bin');
  assert.equal(attempts, 3);
  assert.deepEqual(delays, [1000, 2000]);
});

test('connection errors name the storage check and safe nested codes after bounded retries', async () => {
  let attempts = 0;
  const client = bunnyClient({ host: 'storage.bunnycdn.com', zone: 'fixture', password: 'fixture-secret' }, {
    sleep: async () => {},
    request: async () => {
      attempts++;
      throw new TypeError('fixture-secret', { cause: new AggregateError([
        Object.assign(new Error('fixture-secret'), { code: 'ETIMEDOUT' }),
        Object.assign(new Error('fixture-secret'), { code: 'ENETUNREACH' }),
      ]) });
    },
  });
  await assert.rejects(client.getStorage(''), error => {
    assert.match(error.message, /Storage Zone connection \(storage.bunnycdn.com\)/);
    assert.match(error.message, /ETIMEDOUT, ENETUNREACH/);
    assert.ok(!error.message.includes('fixture-secret'));
    return true;
  });
  assert.equal(attempts, 3);
});

test('public checksum verification retries interrupted bodies with a fresh hash', async t => {
  const root = mkdtempSync(join(tmpdir(), 'postika-body-retry-'));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  const file = join(root, 'fixture.bin');
  const payload = Buffer.from('complete payload');
  writeFileSync(file, payload);
  for (const recover of [true, false]) {
    let attempts = 0;
    const client = bunnyClient({ host: 'storage.bunnycdn.com', zone: 'fixture', password: 'fixture-secret' }, {
      sleep: async () => {}, request: async () => {
        attempts++;
        if (recover && attempts > 1) return new Response(payload);
        return new Response(new ReadableStream({
          start(controller) { controller.enqueue(payload.subarray(0, 5)); },
          pull(controller) { controller.error(Object.assign(new Error('fixture-secret'), { code: 'ECONNRESET' })); },
        }));
      },
    });
    if (recover) {
      await client.verifyPublic(file, 'downloads/0.1.7/fixture.bin');
      assert.equal(attempts, 2);
    } else {
      await assert.rejects(client.verifyPublic(file, 'downloads/0.1.7/fixture.bin'), error => {
        assert.match(error.message, /fixture.bin.*interrupted download \(ECONNRESET\)/);
        assert.ok(!error.message.includes('fixture-secret'));
        return true;
      });
      assert.equal(attempts, 5);
    }
  }
});

test('response cleanup failures do not abort an accepted upload', async t => {
  const root = mkdtempSync(join(tmpdir(), 'postika-cleanup-'));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  const file = join(root, 'fixture.bin');
  writeFileSync(file, 'payload');
  const client = bunnyClient({ host: 'storage.bunnycdn.com', zone: 'fixture', password: 'fixture-secret' }, {
    request: async (url, options) => {
      for await (const chunk of options.body) { /* Consume the request. */ }
      return { ok: true, status: 201, body: { cancel: async () => { throw new Error('terminated'); } } };
    },
  });
  await client.upload(file, 'downloads/0.1.7/fixture.bin');
});
