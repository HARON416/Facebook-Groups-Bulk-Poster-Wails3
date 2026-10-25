import test from 'node:test';
import assert from 'node:assert/strict';
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { writeDownloadFixture, writeVersionFixture } from './release-test-fixtures.mjs';
import { flattenRelease } from './flatten-release.mjs';
import { generateWebsiteRelease } from './website-release.mjs';

test('website links follow the verified release version and preserve release notes on retry', t => {
  const root = mkdtempSync(join(tmpdir(), 'postika-site-release-'));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  writeVersionFixture(root, '0.1.4');
  const download = join(root, 'bin/releases/ci-1');
  writeDownloadFixture(download, '0.1.4');
  flattenRelease(root, download, '0.1.4');
  const file = generateWebsiteRelease(root);
  const release = JSON.parse(readFileSync(file, 'utf8'));
  assert.equal(release.version, '0.1.4');
  assert.equal(release.downloads.length, 8);
  for (const item of release.downloads) {
    assert.ok(item.url.startsWith('https://postika-desktop-application.b-cdn.net/downloads/0.1.4/postika-0.1.4-'));
  }
  release.date = '2026-10-25';
  release.notes = ['Use text Close buttons in dialogs.'];
  writeFileSync(file, JSON.stringify(release));
  generateWebsiteRelease(root);
  assert.deepEqual(JSON.parse(readFileSync(join(root, 'website/releases/current.json'), 'utf8')), release);
  writeFileSync(join(root, 'bin/releases/0.1.4/postika-0.1.4-windows-amd64.exe'), 'corrupt');
  assert.throws(() => generateWebsiteRelease(root), /checksum/);
  assert.deepEqual(JSON.parse(readFileSync(file, 'utf8')), release);
});
