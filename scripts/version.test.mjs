import test from 'node:test';
import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { bumpVersion, checkVersions, currentVersion, parseVersion, syncVersions, versionFields, versionFiles } from './version.mjs';
import { writeVersionFixture } from './release-test-fixtures.mjs';
import { packageRelease } from './package-release.mjs';

function fixture(t) {
  const root = mkdtempSync(join(tmpdir(), 'postika-version-'));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  writeVersionFixture(root);
  return root;
}
const snapshot = root => new Map(versionFiles.map(file => [file, readFileSync(join(root, file), 'utf8')]));

for (const [change, expected] of [['patch', '0.1.1'], ['minor', '0.2.0'], ['major', '1.0.0'], ['2.3.4', '2.3.4']]) {
  test(`${change} synchronizes every native package without changing OS requirements or examples`, t => {
    const root = fixture(t);
    const originals = snapshot(root);
    const result = bumpVersion(root, change);
    assert.equal(result.previous, '0.1.0');
    assert.equal(result.version, expected);
    assert.equal(result.files.length, versionFiles.length);
    assert.equal(checkVersions(root), expected);
    const info = JSON.parse(readFileSync(join(root, 'build/windows/info.json'), 'utf8'));
    assert.equal(info.fixed.file_version, `${expected}.0`);
    assert.equal(info.info['0000'].ProductVersion, expected);
    const config = readFileSync(join(root, 'build/config.yml'), 'utf8');
    assert.ok(config.includes(`#   version: "0.1.0"`));
    for (const file of ['build/linux/app.postika.desktop', 'build/linux/desktop']) assert.match(readFileSync(join(root, file), 'utf8'), /^Version=1\.0$/m);
    for (const file of ['build/darwin/Info.plist', 'build/darwin/Info.dev.plist']) assert.match(readFileSync(join(root, file), 'utf8'), /<key>LSMinimumSystemVersion<\/key>\s*<string>12\.0\.0<\/string>/);
    // Reverting only the version fields must recover every original byte.
    syncVersions(root, '0.1.0');
    assert.deepEqual(snapshot(root), originals);
  });
}

for (const entry of versionFields.slice(1)) {
  test(`release check detects drift: ${entry.file} ${entry.field}`, t => {
    const root = fixture(t);
    const path = join(root, entry.file);
    writeFileSync(path, readFileSync(path, 'utf8').replace(entry.pattern, (_match, prefix, _value, suffix) => `${prefix}9.8.7${suffix}`));
    assert.throws(() => checkVersions(root), error => error.message.includes(entry.file) && error.message.includes('version-sync'));
    assert.equal(currentVersion(root), '0.1.0');
    syncVersions(root);
    assert.equal(checkVersions(root), '0.1.0');
  });
}

test('invalid, equal and lower versions leave every file untouched', t => {
  const root = fixture(t);
  const original = snapshot(root);
  for (const value of ['0.1.0', '0.0.9', '01.2.3', '1.2', '1.2.3.4', '1.2.3-beta', '65536.0.0', '-1.0.0', 'patch; echo bad']) {
    assert.throws(() => bumpVersion(root, value));
    assert.deepEqual(snapshot(root), original);
  }
});

test('late malformed metadata fails before any version is written', t => {
  const root = fixture(t);
  const path = join(root, 'build/linux/desktop');
  writeFileSync(path, readFileSync(path, 'utf8').replace('X-AppImage-Version=', 'Missing-Version='));
  const original = snapshot(root);
  assert.throws(() => bumpVersion(root, 'patch'), /Expected one/);
  assert.deepEqual(snapshot(root), original);
});

test('duplicate fields and Windows numeric component overflow are rejected', t => {
  const root = fixture(t);
  const path = join(root, 'build/linux/desktop');
  writeFileSync(path, `${readFileSync(path, 'utf8')}\nX-AppImage-Version=0.1.0\n`);
  assert.throws(() => bumpVersion(root, 'patch'), /found 2/);
  assert.throws(() => parseVersion('1.65536.0'), /0–65535/);
});

test('version bumps preserve Windows CRLF file formatting', t => {
  const root = fixture(t);
  for (const file of versionFiles) {
    const path = join(root, file);
    writeFileSync(path, readFileSync(path, 'utf8').replace(/\r?\n/g, '\r\n'));
  }
  bumpVersion(root, 'patch');
  assert.equal(checkVersions(root), '0.1.1');
  for (const file of versionFiles) assert.equal(/(?<!\r)\n/.test(readFileSync(join(root, file), 'utf8')), false);
});

test('packaging rejects inconsistent runtime version before invoking Wails', t => {
  const root = fixture(t);
  const path = join(root, 'build/windows/info.json');
  writeFileSync(path, readFileSync(path, 'utf8').replace('"ProductVersion": "0.1.0"', '"ProductVersion": "0.0.9"'));
  assert.throws(() => packageRelease(root, 'linux-amd64', { platform: 'linux', run: () => assert.fail('Wails invoked') }), /version metadata is inconsistent/);
});

test('CLI uses environment values for bump and explicit version without touching the checkout', t => {
  const root = fixture(t);
  const script = join(root, 'scripts/version.mjs');
  // The CLI resolves its project from its own location, as it does in a checkout.
  mkdirSync(join(root, 'scripts'));
  writeFileSync(script, readFileSync(new URL('./version.mjs', import.meta.url)));
  for (const [action, environment, expected] of [
    ['bump', { POSTIKA_RELEASE_BUMP: 'minor' }, '0.2.0'],
    ['set', { POSTIKA_RELEASE_VERSION: '1.2.3' }, '1.2.3'],
  ]) {
    const result = spawnSync(process.execPath, [script, action], { encoding: 'utf8', env: { ...process.env, ...environment } });
    assert.equal(result.status, 0, result.stderr);
    assert.equal(checkVersions(root), expected);
  }
});

test('make exposes version commands without running a bump', () => {
  if (process.platform === 'win32') return; // Windows CI exercises the Node command directly.
  for (const args of [['-n', 'version-bump', 'BUMP=minor'], ['-n', 'version-set', 'VERSION=0.2.0']]) {
    const result = spawnSync('make', args, { cwd: resolve('.'), encoding: 'utf8' });
    assert.equal(result.status, 0, result.stderr);
    assert.match(result.stdout, /node scripts\/version\.mjs (bump|set)/);
  }
});
