import { createHash } from 'node:crypto';
import { spawnSync } from 'node:child_process';
import { closeSync, existsSync, mkdirSync, openSync, readFileSync, readdirSync, realpathSync, rmSync, writeFileSync } from 'node:fs';
import { delimiter, dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { packageRelease, releaseVersion, targets } from './package-release.mjs';
import { flattenRelease, verifyFlatRelease } from './flatten-release.mjs';
import { bumpVersion } from './version.mjs';
import { generateWebsiteRelease } from './website-release.mjs';
import { preflightPublish, publishRelease } from './publish-release.mjs';

const projectRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const digest = bytes => createHash('sha256').update(bytes).digest('hex');

function command(root, name, args, capture = false) {
  const result = spawnSync(name, args, {
    cwd: root, encoding: 'utf8', stdio: capture ? 'pipe' : 'inherit',
    env: { ...process.env, PATH: `${join(root, '.tools/bin')}${delimiter}${process.env.PATH}` },
  });
  if (result.error || result.status !== 0) throw new Error(`${name} ${args.join(' ')} failed${result.error ? `: ${result.error.message}` : ''}`);
  return result.stdout ?? '';
}

// Include tracked and untracked source, including local edits. Git is only a
// local file inventory here: no authentication, commit, push or API is used.
export function sourceFingerprint(root) {
  const names = command(root, 'git', ['ls-files', '-z', '--cached', '--others', '--exclude-standard'], true).split('\0').filter(Boolean).sort();
  const hash = createHash('sha256');
  for (const name of names) {
    // Generated release metadata is an output, not a build input.
    if (name.startsWith('website/releases/')) continue;
    hash.update(name).update('\0');
    hash.update(existsSync(join(root, name)) ? readFileSync(join(root, name)) : '<deleted>').update('\0');
  }
  return hash.digest('hex');
}

export function preflightLocal(root, platform = process.platform) {
  if (platform !== 'linux') throw new Error('make release-local currently requires a Linux host. Native packaging remains available with make package-all.');
  const checks = [
    ['go', ['version']], ['node', ['--version']], ['npm', ['--version']],
    ['wails3', ['version']], ['makensis', ['-VERSION']], ['rcodesign', ['--version']],
    ['pkg-config', ['--exists', 'gtk4', 'webkitgtk-6.0']],
    ['docker', ['info']], ['docker', ['image', 'inspect', 'wails-cross']],
  ];
  const failures = [];
  for (const [name, args] of checks) {
    try { command(root, name, args, true); }
    catch { failures.push(`${name} ${args.join(' ')}`); }
  }
  if (failures.length) throw new Error(`Local build prerequisites missing or unavailable:\n${failures.map(value => `  ${value}`).join('\n')}\nSee README.md: Local releases from Linux. Install/start Docker, install rcodesign, and run make setup-cross before releasing. No version was changed.`);
}

export function validateLocal(root) {
  command(root, 'npm', ['--prefix', 'frontend', 'run', 'check']);
  command(root, 'npm', ['--prefix', 'frontend', 'run', 'tour:test']);
  command(root, 'node', ['--test', ...[
    'package-release', 'release-all', 'release-local', 'flatten-release', 'version',
    'update-feeds', 'website-release', 'publish-release', 'ci-publish', 'remote-publish',
  ].map(name => `scripts/${name}.test.mjs`)]);
  command(root, 'go', ['test', '-buildvcs=false', '-timeout', '120s', './...']);
  command(join(root, 'website'), 'go', ['test', './...']);
}

function verifyTarget(directory, target, version) {
  const names = new Set(targets[target].files.filter(([, suffix]) => suffix !== '.dmg').map(([, suffix]) => `postika-${version}-${target}${suffix}`));
  names.add('LICENSE.txt');
  const entries = readdirSync(directory, { withFileTypes: true });
  if (entries.length !== names.size + 1 || entries.some(entry => !entry.isFile() || !names.has(entry.name) && entry.name !== 'SHA256SUMS')) throw new Error(`Incomplete staged target: ${target}`);
  for (const row of readFileSync(join(directory, 'SHA256SUMS'), 'utf8').trim().split(/\r?\n/)) {
    const match = /^([a-f0-9]{64})  ([^/\\]+)$/.exec(row);
    if (!match || !names.delete(match[2]) || digest(readFileSync(join(directory, match[2]))) !== match[1]) throw new Error(`Staged checksum mismatch: ${target}`);
  }
  if (names.size) throw new Error(`Incomplete staged checksums: ${target}`);
}

export async function releaseLocal({ root = projectRoot, change = process.env.POSTIKA_RELEASE_VERSION || process.env.POSTIKA_RELEASE_BUMP || 'patch',
  publish = false, preflight = preflightLocal, validate = validateLocal, fingerprint = sourceFingerprint, build = packageRelease,
  preparePublish = preflightPublish, publishPackages = publishRelease, log = console.log } = {}) {
  mkdirSync(join(root, 'bin'), { recursive: true });
  const lock = join(root, 'bin/.release-local.lock');
  let fd;
  try { fd = openSync(lock, 'wx'); }
  catch { throw new Error('Another local release is running (bin/.release-local.lock). If it crashed, check that it stopped before removing the lock.'); }
  let version;
  try {
    // Fail before reserving a version or spending time compiling.
    preflight(root);
    const client = publish ? await preparePublish(root) : undefined;
    validate(root);
    version = change === 'none' ? releaseVersion(root) : bumpVersion(root, change).version;
    const source = fingerprint(root);
    const work = join(root, 'bin/local-release', version);
    const stateFile = join(work, 'state.json');
    const staging = join(root, 'bin/releases', `ci-local-${version}`);
    mkdirSync(work, { recursive: true });
    let state = { version, source, completed: [] };
    if (existsSync(stateFile)) {
      state = JSON.parse(readFileSync(stateFile, 'utf8'));
      if (state.version !== version || state.source !== source) throw new Error(`Source changed since ${version} was started. Use a new version, or move bin/local-release/${version} and bin/releases/ci-local-${version} aside before rebuilding. Do not combine packages from different source.`);
    }
    const output = join(root, 'bin/releases', version);
    if (!existsSync(stateFile) && existsSync(output)) throw new Error(`Release ${version} already exists without a local build record. Use a new version; existing packages were preserved.`);
    writeFileSync(stateFile, JSON.stringify(state, null, 2) + '\n');
    if (!existsSync(output)) {
      for (const target of Object.keys(targets)) {
        if (state.completed.includes(target)) {
          verifyTarget(join(staging, target), target, version);
          log(`Reusing verified ${target} for ${version}.`);
          continue;
        }
        log(`Building ${target} locally for ${version}…`);
        build(root, target, { cross: true, releaseRoot: staging, preserveBundle: false });
        verifyTarget(join(staging, target), target, version);
        if (fingerprint(root) !== source) throw new Error('Source changed during the build. Packages were preserved but will not be combined or published.');
        state.completed.push(target);
        writeFileSync(stateFile, JSON.stringify(state, null, 2) + '\n');
      }
      flattenRelease(root, staging, version, { macZipOnly: true, prune: false, preserveSource: true });
    }
    verifyFlatRelease(output, version);
    if (fingerprint(root) !== source) throw new Error('Source changed during the release. Publication stopped.');
    generateWebsiteRelease(root);
    if (publish) await publishPackages({ root, client, log });
    log(`${publish ? 'Published' : 'Built'} ${version} locally. Packages: ${output}`);
    return output;
  } catch (error) {
    throw new Error(`${error.message}${version ? '\nRetry unchanged source with make release-local BUMP=none (add PUBLISH=1 to publish). No GitHub workflow was started.' : ''}`);
  } finally { closeSync(fd); rmSync(lock); }
}

if (process.argv[1] && realpathSync(resolve(process.argv[1])) === realpathSync(fileURLToPath(import.meta.url))) {
  try {
    if (process.argv.includes('--plan')) {
      console.log('Build Linux x64, Windows x64, macOS arm64 and macOS amd64 locally on Linux.');
      console.log('Requires Docker + wails-cross, NSIS, GTK/WebKit development libraries, and rcodesign.');
      console.log('Mac downloads use ad-hoc signed .app.zip files instead of DMGs; no Developer ID/notarization.');
      console.log('Bump patch locally by default; BUMP=none resumes verified packages from unchanged source.');
      console.log('PUBLISH=1 signs update feeds with the existing key and uploads directly to Bunny.');
      console.log('No GitHub Actions, artifacts, login, commit or push. No builds or uploads were performed.');
    } else if (process.argv.includes('--check')) preflightLocal(projectRoot);
    else await releaseLocal({ publish: process.env.POSTIKA_PUBLISH === '1' });
  } catch (error) { console.error(error.message); process.exitCode = 1; }
}
