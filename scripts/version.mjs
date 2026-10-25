import { readFileSync, realpathSync, writeFileSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const definition = /^([ \t]*!define[ \t]+INFO_PRODUCTVERSION[ \t]+")([^"]+)(")/gm;
const plist = key => new RegExp(`(<key>${key}</key>\\s*<string>)([^<]+)(</string>)`, 'g');
const appImage = /^(X-AppImage-Version=)([^\r\n]+)()/gm;
// Each pattern has prefix/value/suffix captures. Match specific fields rather
// than globally replacing version-looking strings, comments, or OS minimums.
export const versionFields = [
  { file: 'build/config.yml', field: 'info.version', pattern: /^(  version: ")([^"]+)(")/gm },
  { file: 'build/windows/info.json', field: 'ProductVersion', pattern: /("ProductVersion"\s*:\s*")([^"]+)(")/g },
  { file: 'build/windows/info.json', field: 'file_version', numeric: true, pattern: /("file_version"\s*:\s*")([^"]+)(")/g },
  { file: 'build/windows/nsis/project.nsi', field: 'INFO_PRODUCTVERSION', pattern: definition },
  { file: 'build/windows/nsis/wails_tools.nsh', field: 'INFO_PRODUCTVERSION', pattern: definition },
  { file: 'build/windows/msix/app_manifest.xml', field: 'Identity.Version', numeric: true, pattern: /(<Identity\b[^>]*\bVersion=")([^"]+)(")/g },
  { file: 'build/windows/msix/template.xml', field: 'PackageInformation.Version', numeric: true, pattern: /(<PackageInformation\b[^>]*\bVersion=")([^"]+)(")/g },
  ...['build/darwin/Info.plist', 'build/darwin/Info.dev.plist'].flatMap(file => [
    { file, field: 'CFBundleShortVersionString', pattern: plist('CFBundleShortVersionString') },
    { file, field: 'CFBundleVersion', pattern: plist('CFBundleVersion') },
  ]),
  { file: 'build/linux/nfpm/nfpm.yaml', field: 'version', pattern: /^(version: ")([^"]+)(")/gm },
  { file: 'build/linux/app.postika.desktop', field: 'X-AppImage-Version', pattern: appImage },
  { file: 'build/linux/desktop', field: 'X-AppImage-Version', pattern: appImage },
];
export const versionFiles = [...new Set(versionFields.map(entry => entry.file))];

export function parseVersion(value) {
  if (typeof value !== 'string' || !/^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$/.test(value)) {
    throw new Error('Use a stable version in MAJOR.MINOR.PATCH format, for example 0.1.1. Prerelease labels and leading zeros are not supported by the native package metadata.');
  }
  const parts = value.split('.').map(Number);
  if (parts.some(part => part > 65535)) throw new Error('Version components must be 0–65535 for Windows package compatibility.');
  return parts;
}

function readField(text, entry) {
  const matches = [...text.matchAll(entry.pattern)];
  if (matches.length !== 1) throw new Error(`Expected one ${entry.field} field in ${entry.file}, found ${matches.length}. No versions were changed.`);
  return matches[0][2];
}

export function currentVersion(root) {
  const entry = versionFields[0];
  const version = readField(readFileSync(join(root, entry.file), 'utf8'), entry);
  parseVersion(version);
  return version;
}

function readMetadata(root) {
  return new Map(versionFiles.map(file => [file, readFileSync(join(root, file), 'utf8')]));
}

export function checkVersions(root) {
  const version = currentVersion(root);
  const contents = readMetadata(root);
  const mismatches = [];
  for (const entry of versionFields) {
    const actual = readField(contents.get(entry.file), entry);
    const expected = entry.numeric ? `${version}.0` : version;
    if (actual !== expected) mismatches.push(`${entry.file} (${entry.field}): ${actual}, expected ${expected}`);
  }
  if (mismatches.length) throw new Error(`Release version metadata is inconsistent:\n${mismatches.join('\n')}\nRun make version-sync, then commit and push the synchronized metadata.`);
  return version;
}

export function syncVersions(root, version = currentVersion(root)) {
  parseVersion(version);
  const originals = readMetadata(root);
  const next = new Map(originals);
  // Validate every field before writing any file. Malformed or missing
  // metadata must never leave a half-applied version bump.
  for (const entry of versionFields) {
    const text = next.get(entry.file);
    readField(text, entry);
    next.set(entry.file, text.replace(entry.pattern, (_match, prefix, _value, suffix) => `${prefix}${entry.numeric ? `${version}.0` : version}${suffix}`));
  }
  const changed = [...next].filter(([file, text]) => text !== originals.get(file));
  const written = [];
  try {
    for (const [file, text] of changed) {
      written.push(file);
      writeFileSync(join(root, file), text);
    }
  } catch (error) {
    const failures = [];
    for (const file of written) {
      try { writeFileSync(join(root, file), originals.get(file)); }
      catch { failures.push(file); }
    }
    if (failures.length) throw new Error(`Version update failed: ${error.message}. Restore these files before releasing: ${failures.join(', ')}`);
    throw error;
  }
  return { version, files: changed.map(([file]) => file) };
}

export function nextVersion(previous, change = 'patch') {
  const parts = parseVersion(previous);
  let version;
  if (change === 'patch') version = `${parts[0]}.${parts[1]}.${parts[2] + 1}`;
  else if (change === 'minor') version = `${parts[0]}.${parts[1] + 1}.0`;
  else if (change === 'major') version = `${parts[0] + 1}.0.0`;
  else {
    const proposed = parseVersion(change);
    const firstDifference = proposed.findIndex((part, index) => part !== parts[index]);
    if (firstDifference < 0 || proposed[firstDifference] < parts[firstDifference]) throw new Error(`New version must be higher than ${previous}. Use make version-sync to repair metadata without incrementing it.`);
    version = change;
  }
  parseVersion(version);
  return version;
}

export function bumpVersion(root, change = 'patch') {
  const previous = currentVersion(root);
  const version = nextVersion(previous, change);
  return { previous, ...syncVersions(root, version) };
}

// Node resolves the module's real path, but argv can retain a symlink alias
// (notably /var versus /private/var for temporary directories on macOS).
if (process.argv[1] && realpathSync(resolve(process.argv[1])) === realpathSync(fileURLToPath(import.meta.url))) {
  const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
  try {
    const [action = 'check', argument] = process.argv.slice(2);
    if (action === 'check') console.log(`Release version ${checkVersions(root)}: all platform metadata agrees.`);
    else if (action === 'sync') {
      const result = syncVersions(root);
      console.log(`Version ${result.version}: synchronized ${result.files.length} file(s).`);
    } else if (action === 'bump' || action === 'set') {
      const change = argument ?? (action === 'bump' ? process.env.POSTIKA_RELEASE_BUMP || 'patch' : process.env.POSTIKA_RELEASE_VERSION);
      if (!change) throw new Error('Specify a version: make version-set VERSION=0.2.0');
      if (action === 'set') parseVersion(change);
      const result = bumpVersion(root, change);
      console.log(`Version ${result.previous} → ${result.version}; updated ${result.files.length} file(s). Commit and push before make release-all.`);
    } else throw new Error('Usage: node scripts/version.mjs check|sync|bump [patch|minor|major]|set MAJOR.MINOR.PATCH');
  } catch (error) { console.error(error.message); process.exitCode = 1; }
}
