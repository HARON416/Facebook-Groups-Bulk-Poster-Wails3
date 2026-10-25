import test from 'node:test';
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { existsSync, mkdirSync, mkdtempSync, readFileSync, readdirSync, rmSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { flattenRelease, prunePreviousReleases } from './flatten-release.mjs';
import { writeDownloadFixture } from './release-test-fixtures.mjs';

function fixture(t) {
  const root = mkdtempSync(join(tmpdir(), 'postika-flatten-'));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  const download = join(root, 'bin/releases/ci-42');
  const directories = writeDownloadFixture(download);
  return { root, download, directories, output: join(root, 'bin/releases/0.1.0') };
}

test('all twelve packages flatten with one normalized license and verified combined checksums', t => {
  const { root, download, output } = fixture(t);
  assert.equal(flattenRelease(root, download, '0.1.0'), output);
  assert.equal(existsSync(download), false);
  assert.equal(readdirSync(output).length, 14);
  assert.equal(readFileSync(join(output, 'LICENSE.txt'), 'utf8'), 'Fixture license\nSecond line\n');
  const sums = readFileSync(join(output, 'SHA256SUMS'), 'utf8').trim().split('\n');
  assert.equal(sums.length, 13);
  for (const row of sums) {
    const [hash, name] = row.split('  ');
    assert.equal(hash, createHash('sha256').update(readFileSync(join(output, name))).digest('hex'));
  }
});

test('identical repeated downloads reuse the existing release', t => {
  const { root, download, output } = fixture(t);
  flattenRelease(root, download, '0.1.0');
  const sums = readFileSync(join(output, 'SHA256SUMS'), 'utf8');
  writeDownloadFixture(download);
  flattenRelease(root, download, '0.1.0');
  assert.equal(readFileSync(join(output, 'SHA256SUMS'), 'utf8'), sums);
  assert.equal(existsSync(download), false);
});

test('checksum corruption preserves the original download and creates no release', t => {
  const { root, download, output, directories } = fixture(t);
  writeFileSync(join(directories['linux-amd64'], 'postika-0.1.0-linux-amd64.deb'), 'Corrupt');
  assert.throws(() => flattenRelease(root, download, '0.1.0'), /Checksum mismatch/);
  assert.equal(existsSync(download), true);
  assert.equal(existsSync(output), false);
});

test('incomplete target sets cannot become an upload-ready release', t => {
  const { root, download, directories } = fixture(t);
  rmSync(directories['darwin-arm64'], { recursive: true });
  assert.throws(() => flattenRelease(root, download, '0.1.0'), /all four release targets/);
  assert.equal(existsSync(download), true);
});

test('different existing releases are never overwritten', t => {
  const { root, download, output } = fixture(t);
  mkdirSync(output);
  writeFileSync(join(output, 'old.txt'), 'Existing release');
  assert.throws(() => flattenRelease(root, download, '0.1.0'), /different release already exists/);
  assert.equal(readFileSync(join(output, 'old.txt'), 'utf8'), 'Existing release');
  assert.equal(existsSync(download), true);
});

test('unexpected files and checksum traversal entries preserve downloads', t => {
  const { root, download, directories } = fixture(t);
  writeFileSync(join(download, 'extra.txt'), 'Keep this');
  assert.throws(() => flattenRelease(root, download, '0.1.0'), /Unexpected files/);
  rmSync(join(download, 'extra.txt'));
  const manifest = join(directories['linux-amd64'], 'SHA256SUMS');
  writeFileSync(manifest, `${'a'.repeat(64)}  ../../outside\n`);
  assert.throws(() => flattenRelease(root, download, '0.1.0'), /Invalid checksum entry/);
  assert.equal(existsSync(download), true);
});

test('genuinely different licenses are rejected even when their checksums are valid', t => {
  const { root, download, directories } = fixture(t);
  const directory = directories['windows-amd64'];
  const license = 'Different license\n';
  const hash = createHash('sha256').update(license).digest('hex');
  writeFileSync(join(directory, 'LICENSE.txt'), license);
  const manifest = join(directory, 'SHA256SUMS');
  writeFileSync(manifest, readFileSync(manifest, 'utf8').replace(/^[a-f0-9]{64}  LICENSE.txt$/m, `${hash}  LICENSE.txt`));
  assert.throws(() => flattenRelease(root, download, '0.1.0'), /Conflicting release licenses/);
  assert.equal(existsSync(download), true);
});

test('successful releases remove older versions and stale CI caches, preserving unrelated folders', t => {
  const { root, download, output } = fixture(t);
  const previous = join(root, 'bin/releases/ci-40');
  writeDownloadFixture(previous, '0.0.9');
  flattenRelease(root, previous, '0.0.9');
  const stale = join(root, 'bin/releases/ci-41');
  writeDownloadFixture(stale, '0.0.9');
  const notes = join(root, 'bin/releases/notes');
  mkdirSync(notes);
  writeFileSync(join(notes, 'readme.txt'), 'Keep this');
  flattenRelease(root, download, '0.1.0');
  assert.equal(existsSync(join(root, 'bin/releases/0.0.9')), false);
  assert.equal(existsSync(stale), false);
  assert.equal(existsSync(output), true);
  assert.equal(readFileSync(join(notes, 'readme.txt'), 'utf8'), 'Keep this');
});

test('failed new downloads retain the previous complete release', t => {
  const { root, download, directories } = fixture(t);
  const previous = join(root, 'bin/releases/ci-40');
  writeDownloadFixture(previous, '0.0.9');
  flattenRelease(root, previous, '0.0.9');
  writeFileSync(join(directories['linux-amd64'], 'postika-0.1.0-linux-amd64.deb'), 'Corrupted');
  assert.throws(() => flattenRelease(root, download, '0.1.0'), /Checksum mismatch/);
  assert.equal(existsSync(join(root, 'bin/releases/0.0.9')), true);
  assert.equal(existsSync(download), true);
});

test('an older build cannot supersede a newer local release', t => {
  const { root, download } = fixture(t);
  const newer = join(root, 'bin/releases/ci-50');
  writeDownloadFixture(newer, '0.2.0');
  flattenRelease(root, newer, '0.2.0');
  writeDownloadFixture(download, '0.1.0');
  assert.throws(() => flattenRelease(root, download, '0.1.0'), /Newer local release 0.2.0/);
  assert.equal(existsSync(join(root, 'bin/releases/0.2.0')), true);
  assert.equal(existsSync(download), true);
});

test('cleanup cannot discard older releases when the retained copy is corrupt', t => {
  const { root, download, output } = fixture(t);
  flattenRelease(root, download, '0.1.0');
  const older = join(root, 'bin/releases/0.0.9');
  mkdirSync(older);
  writeFileSync(join(older, 'old.txt'), 'Preserve');
  writeFileSync(join(output, 'postika-0.1.0-linux-amd64.deb'), 'Corrupt retained copy');
  assert.throws(() => prunePreviousReleases(root, '0.1.0'), /checksum verification failed/);
  assert.equal(existsSync(older), true);
});

test('cleanup leaves newer downloads and unknown cache contents intact', t => {
  const { root, download } = fixture(t);
  const future = join(root, 'bin/releases/ci-50');
  writeDownloadFixture(future, '0.2.0');
  const unknown = join(root, 'bin/releases/ci-49');
  mkdirSync(unknown);
  writeFileSync(join(unknown, 'notes.txt'), 'Unrecognized content');
  flattenRelease(root, download, '0.1.0');
  assert.equal(existsSync(future), true);
  assert.equal(existsSync(unknown), true);
});
