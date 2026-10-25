import { spawnSync } from 'node:child_process';
import { existsSync, mkdirSync, mkdtempSync, readFileSync, readdirSync, realpathSync, renameSync, rmSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { verifyFlatRelease } from './flatten-release.mjs';
import { releaseVersion } from './package-release.mjs';
import { parseVersion } from './version.mjs';
import { downloadPrefix } from './release-layout.mjs';

export { downloadPrefix } from './release-layout.mjs';

export function feedPackages(version) {
  parseVersion(version);
  const prefix = `postika-${version}-`;
  return Object.fromEntries(Object.entries({
    'stable.json': ['windows-amd64.exe', 'darwin-amd64.app.zip', 'darwin-arm64.app.zip', 'linux-amd64.AppImage'],
    'stable-deb.json': ['linux-amd64.deb'],
    'stable-rpm.json': ['linux-amd64.rpm'],
    'stable-arch.json': ['linux-amd64.pkg.tar.zst'],
    'stable-binary.json': ['linux-amd64-binary'],
  }).map(([feed, names]) => [feed, names.map(name => prefix + name)]));
}

export function validateFeed(manifest, version, names) {
  const prefix = downloadPrefix(version);
  if (manifest.version !== version || manifest.channel !== 'stable' || manifest.artifacts?.length !== names.length) {
    throw new Error('Unexpected update manifest version, channel or artifact count');
  }
  const expected = new Set(names.map(name => prefix + name));
  for (const artifact of manifest.artifacts) {
    const name = artifact.url?.slice(prefix.length);
    const target = /-(windows|darwin|linux)-(amd64|arm64)(?:[.-]|$)/.exec(name ?? '');
    if (!expected.delete(artifact.url) || !target || artifact.platform !== target[1] || artifact.arch !== target[2] ||
        artifact.digestAlgo !== 'sha512' || artifact.signatureAlgo !== 'ed25519ph' || !artifact.signature) {
      throw new Error('Unexpected update artifact URL, platform, architecture or signature');
    }
  }
}

function wails(root, args) {
  const cli = join(root, '.tools/bin', process.platform === 'win32' ? 'wails3.exe' : 'wails3');
  const result = spawnSync(cli, args, { cwd: root, stdio: 'inherit' });
  if (result.error) throw new Error('Project-local Wails CLI unavailable; run make install-cli.');
  if (result.status !== 0) throw new Error('Wails update feed generation/verification failed');
}

export function verifyFeeds(root, directory, run = args => wails(root, args)) {
  const version = releaseVersion(root);
  const feeds = feedPackages(version);
  const entries = readdirSync(directory, { withFileTypes: true });
  if (entries.length !== Object.keys(feeds).length || entries.some(entry => !entry.isFile() || !Object.hasOwn(feeds, entry.name))) {
    throw new Error('Expected exactly five regular update feed files');
  }
  for (const [feed, names] of Object.entries(feeds)) {
    const manifest = join(directory, feed);
    validateFeed(JSON.parse(readFileSync(manifest, 'utf8')), version, names);
    run(['updater', 'verify', '-manifest', manifest, '-dir', join(root, 'bin/releases', version),
      '-publickey', join(root, 'internal/updateauth/public-key.pem')]);
  }
}

export function generateFeeds(root, run = args => wails(root, args)) {
  const version = releaseVersion(root);
  const packages = join(root, 'bin/releases', version);
  verifyFlatRelease(packages, version);
  const parent = join(root, 'bin/updates');
  const output = join(parent, version);
  if (existsSync(output)) {
    verifyFeeds(root, output, run);
    return output;
  }
  const key = join(root, '.update-keys/updater.key');
  if (!existsSync(key)) throw new Error('Missing .update-keys/updater.key; restore the existing signing key before generating feeds.');
  mkdirSync(parent, { recursive: true });
  const staging = mkdtempSync(join(parent, `.${version}-`));
  try {
    for (const [feed, names] of Object.entries(feedPackages(version))) {
      const manifest = join(staging, feed);
      run(['updater', 'manifest', '-version', version, '-channel', 'stable', '-key', key,
        '-url-prefix', downloadPrefix(version), '-output', manifest, ...names.map(name => join(packages, name))]);
    }
    verifyFeeds(root, staging, run);
    renameSync(staging, output);
  } finally {
    rmSync(staging, { recursive: true, force: true });
  }
  return output;
}

if (process.argv[1] && realpathSync(resolve(process.argv[1])) === realpathSync(fileURLToPath(import.meta.url))) {
  try {
    console.log(`Verified signed update feeds: ${generateFeeds(resolve(dirname(fileURLToPath(import.meta.url)), '..'))}`);
  } catch (error) {
    console.error(error.message);
    process.exitCode = 1;
  }
}
