import test from 'node:test';
import assert from 'node:assert/strict';
import { mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { syncPublishedMetadata, publishExisting, previousBuild } from './remote-publish.mjs';
import { websiteRelease } from './release-layout.mjs';
import { writeVersionFixture } from './release-test-fixtures.mjs';
import { targets } from './package-release.mjs';

function fixture(t) {
  const root = mkdtempSync(join(tmpdir(), 'postika-remote-'));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  writeVersionFixture(root, '0.1.8');
  mkdirSync(join(root, 'internal/updateauth'), { recursive: true });
  writeFileSync(join(root, 'internal/updateauth/public-key.pem'), 'fixture public key');
  const calls = [], sha = 'a'.repeat(40);
  const options = { root, runID: '42', request: 'fixture-request', sleep: async () => {}, log: () => {}, sync: async () => 'metadata-synced', run(name, args) {
    calls.push([name, ...args]);
    if (name === 'git') return args[0] === 'rev-parse' ? sha : args[0] === 'branch' ? 'main' : args[1].endsWith('public-key.pem') ? 'fixture public key' : '  version: "0.1.8"';
    if (args[0] === 'secret') return JSON.stringify(['BUNNY_STORAGE_PASSWORD', 'POSTIKA_UPDATE_PRIVATE_KEY'].map(name => ({ name })));
    if (args[0] === 'variable') return JSON.stringify(['BUNNY_STORAGE_ZONE', 'BUNNY_STORAGE_HOST'].map(name => ({ name })));
    if (args[0] === 'api') return JSON.stringify(args[1].includes('/jobs?') ? { jobs: Object.keys(targets).map(name => ({ name, conclusion: 'success' })) } : { path: '.github/workflows/distribution.yml', event: 'workflow_dispatch', head_sha: sha });
    if (args[0] === 'run' && args[1] === 'list') return JSON.stringify([{ databaseId: 99, displayTitle: 'Postika publish 42 fixture-request', headSha: sha, url: 'https://example.invalid/run/99' }]);
    return '';
  } };
  return { root, options, calls };
}

test('remote publishing dispatches a publish-only workflow with no local artifact download', async t => {
  const f = fixture(t);
  assert.equal(await publishExisting(f.options), 'metadata-synced');
  assert.ok(f.calls.some(call => call[0] === 'gh' && call[1] === 'workflow' && call.includes('publish.yml')));
  assert.ok(!f.calls.some(call => call.includes('download') || call.includes('commit') || call.includes('push')));
  assert.equal(previousBuild(f.root).publishRunID, 99);
});

test('an interrupted publishing watch retains the source run for retry without bumping', async t => {
  const f = fixture(t), run = f.options.run;
  f.options.run = (name, args) => { if (args[0] === 'run' && args[1] === 'watch') throw new Error('connection reset'); return run(name, args); };
  await assert.rejects(publishExisting(f.options), /make publish-release RUN_ID=42/);
  assert.equal(previousBuild(f.root).runID, '42');
});

test('retry reconnects to a completed publish run instead of dispatching duplicate uploads', async t => {
  const f = fixture(t), run = f.options.run;
  let failWatch = true;
  f.options.run = (name, args) => {
    if (args[0] === 'run' && args[1] === 'watch' && failWatch) { failWatch = false; throw new Error('connection reset'); }
    if (args[0] === 'api' && args[1].endsWith('/99')) return JSON.stringify({ path: '.github/workflows/publish.yml', status: 'completed', conclusion: 'success' });
    return run(name, args);
  };
  await assert.rejects(publishExisting(f.options));
  assert.equal(await publishExisting(f.options), 'metadata-synced');
  assert.equal(f.calls.filter(call => call[0] === 'gh' && call[1] === 'workflow').length, 1);
});

test('only a small, validated metadata document is fetched to update local website links', async t => {
  const { root } = fixture(t);
  const metadata = websiteRelease('0.1.8', { date: '2026-10-26' });
  const urls = [];
  const file = await syncPublishedMetadata(root, '0.1.8', { request: async url => { urls.push(url); return new Response(JSON.stringify(metadata)); } });
  assert.equal(urls.length, 1);
  assert.equal(urls[0], 'https://postika-desktop-application.b-cdn.net/downloads/0.1.8/release.json');
  assert.deepEqual(JSON.parse(readFileSync(file, 'utf8')), metadata);
  metadata.downloads[0].url = 'https://untrusted.example/installer.exe';
  await assert.rejects(syncPublishedMetadata(root, '0.1.8', { sleep: async () => {}, request: async () => new Response(JSON.stringify(metadata)) }), /expected release download URLs/);
  assert.ok(!readFileSync(file, 'utf8').includes('untrusted.example'));
});
