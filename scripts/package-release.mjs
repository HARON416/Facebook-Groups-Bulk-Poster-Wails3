import { createHash } from 'node:crypto';
import { spawnSync } from 'node:child_process';
import { copyFileSync, cpSync, existsSync, mkdirSync, readFileSync, renameSync, writeFileSync } from 'node:fs';
import { dirname, join, resolve, delimiter } from 'node:path';
import { fileURLToPath } from 'node:url';

export const targets = {
  'linux-amd64': {
    host: 'linux', task: 'linux:package', arch: 'amd64',
    files: [['postika-x86_64.AppImage', '.AppImage'], ['postika.deb', '.deb'], ['postika.rpm', '.rpm'], ['postika.pkg.tar.zst', '.pkg.tar.zst'], ['postika', '-binary']],
  },
  'windows-amd64': {
    host: 'win32', task: 'windows:package:all', arch: 'amd64',
    files: [['postika-amd64-installer.exe', '-installer.exe'], ['postika-windows-amd64-portable.zip', '-portable.zip'], ['postika.exe', '.exe']],
  },
  'darwin-arm64': {
    host: 'darwin', task: 'darwin:package:all', arch: 'arm64',
    files: [['postika.dmg', '.dmg'], ['postika-darwin-arm64.app.zip', '.app.zip']],
  },
  'darwin-amd64': {
    host: 'darwin', task: 'darwin:package:all', arch: 'amd64',
    files: [['postika.dmg', '.dmg'], ['postika-darwin-amd64.app.zip', '.app.zip']],
  },
};

export function releaseVersion(root) {
  const config = readFileSync(join(root, 'build/config.yml'), 'utf8');
  const version = config.match(/^  version: "([^"]+)"/m)?.[1];
  if (!version || !/^\d+\.\d+\.\d+(?:-[\w.-]+)?$/.test(version)) throw new Error('Invalid release version in build/config.yml');
  return version;
}

export function stageRelease(root, target, version) {
  const plan = targets[target];
  if (!plan) throw new Error(`Unknown target: ${target}`);
  const output = join(root, 'bin/releases', version, target);
  if (existsSync(output)) throw new Error(`Release already staged: ${output}. Preserve it or move it before rebuilding.`);
  // Validate the full set before creating a staged release.
  for (const [name] of plan.files) {
    if (!existsSync(join(root, 'bin', name))) throw new Error(`Missing package: bin/${name}`);
  }
  const temporary = `${output}.staging`;
  if (existsSync(temporary)) throw new Error(`Incomplete staging exists: ${temporary}. Inspect or move it before retrying.`);
  mkdirSync(temporary, { recursive: true });
  const sums = [];
  for (const [source, suffix] of plan.files) {
    const name = `postika-${version}-${target}${suffix}`;
    const destination = join(temporary, name);
    copyFileSync(join(root, 'bin', source), destination);
    sums.push(`${createHash('sha256').update(readFileSync(destination)).digest('hex')}  ${name}`);
  }
  copyFileSync(join(root, 'LICENSE'), join(temporary, 'LICENSE.txt'));
  sums.push(`${createHash('sha256').update(readFileSync(join(temporary, 'LICENSE.txt'))).digest('hex')}  LICENSE.txt`);
  // Keep an intact native app bundle locally. CI transports it in the app ZIP
  // so artifact upload/download cannot strip executable modes or symlinks.
  if (target.startsWith('darwin-')) cpSync(join(root, 'bin/postika.app'), join(temporary, `postika-${version}-${target}.app`), { recursive: true });
  writeFileSync(join(temporary, 'SHA256SUMS'), `${sums.join('\n')}\n`);
  renameSync(temporary, output);
  return output;
}

export function packageRelease(root, target, { platform = process.platform, run = spawnSync } = {}) {
  const plan = targets[target];
  if (!plan) throw new Error(`Target must be one of: ${Object.keys(targets).join(', ')}`);
  if (platform !== plan.host) throw new Error(`${target} needs a native ${plan.host} runner. Use make release-all to build every target in CI.`);
  const version = releaseVersion(root);
  const output = join(root, 'bin/releases', version, target);
  if (existsSync(output) || existsSync(`${output}.staging`)) throw new Error(`Release staging already exists: ${output}`);
  const cli = platform === 'win32' ? 'wails3.exe' : 'wails3';
  const localCLI = join(root, '.tools/bin', cli);
  const result = run(existsSync(localCLI) ? localCLI : cli, ['task', plan.task, `ARCH=${plan.arch}`], {
    cwd: root, stdio: 'inherit', env: { ...process.env, PATH: `${join(root, '.tools/bin')}${delimiter}${process.env.PATH}` },
  });
  if (result.error) throw result.error;
  if (result.status !== 0) throw new Error(`Packaging ${target} failed (${result.status ?? result.signal})`);
  return stageRelease(root, target, version);
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
  try {
    const target = process.argv[2];
    if (target === '--plan') console.log(JSON.stringify(targets, null, 2));
    else console.log(`Staged release: ${packageRelease(root, target)}`);
  } catch (error) { console.error(error.message); process.exitCode = 1; }
}
