import { spawnSync } from 'node:child_process';
import { existsSync, mkdirSync, mkdtempSync, readFileSync, realpathSync, renameSync, rmSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { verifyFlatRelease } from './flatten-release.mjs';
import { releaseVersion } from './package-release.mjs';
import { parseVersion } from './version.mjs';

export const downloadPrefix = 'https://postika-desktop-application.b-cdn.net/downloads/';

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
  if (manifest.version !== version || manifest.channel !== 'stable' || manifest.artifacts?.length !== names.length) {
    throw new Error('Unexpected update manifest version, channel or artifact count');
  }
  const expected = new Set(names.map(name => downloadPrefix + name));
  for (const artifact of manifest.artifacts) {
    const name = artifact.url?.slice(downloadPrefix.length);
    const target = /-(windows|darwin|linux)-(amd64|arm64)(?:[.-]|$)/.exec(name ?? '');
    if (!expected.delete(artifact.url) || !target || artifact.platform !== target[1] || artifact.arch !== target[2] ||
        artifact.digestAlgo !== 'sha512' || artifact.signatureAlgo !== 'ed25519ph' || !artifact.signature) {
      throw new Error('Unexpected update artifact URL, platform, architecture or signature');
    }
  }
}

export function generateFeeds(root) {
  const version = releaseVersion(root);
  const packages = join(root, 'bin/releases', version);
  verifyFlatRelease(packages, version);
  const cli = join(root, '.tools/bin', process.platform === 'win32' ? 'wails3.exe' : 'wails3');
  const key = join(root, '.update-keys/updater.key');
  const publicKey = join(root, 'internal/updateauth/public-key.pem');
  for (const path of [cli, key, publicKey]) if (!existsSync(path)) throw new Error(`Required file missing: ${path}`);
  const parent = join(root, 'bin/updates');
  const output = join(parent, version);
  if (existsSync(output)) throw new Error(`Feeds already exist at ${output}; preserve the published release or move this folder before regenerating.`);
  mkdirSync(parent, { recursive: true });
  const staging = mkdtempSync(join(parent, `.${version}-`));
  const run = args => {
    const result = spawnSync(cli, args, { cwd: root, stdio: 'inherit' });
    if (result.error) throw result.error;
    if (result.status !== 0) throw new Error('Wails update feed generation/verification failed');
  };
  try {
    for (const [feed, names] of Object.entries(feedPackages(version))) {
      const manifest = join(staging, feed);
      run(['updater', 'manifest', '-version', version, '-channel', 'stable', '-key', key,
        '-url-prefix', downloadPrefix, '-output', manifest, ...names.map(name => join(packages, name))]);
      validateFeed(JSON.parse(readFileSync(manifest, 'utf8')), version, names);
      run(['updater', 'verify', '-manifest', manifest, '-dir', packages, '-publickey', publicKey]);
    }
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
