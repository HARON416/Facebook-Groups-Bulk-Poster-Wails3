import { createHash } from 'node:crypto';
import { copyFileSync, existsSync, mkdirSync, mkdtempSync, readFileSync, readdirSync, renameSync, rmSync, writeFileSync } from 'node:fs';
import { basename, dirname, join, resolve, sep } from 'node:path';
import { targets } from './package-release.mjs';

const digest = file => createHash('sha256').update(readFileSync(file)).digest('hex');
const normalizeLicense = bytes => bytes.toString('utf8').replace(/\r\n/g, '\n');
function filesWithin(directory) {
  return readdirSync(directory, { withFileTypes: true }).flatMap(entry => {
    const path = join(directory, entry.name);
    if (entry.isSymbolicLink()) throw new Error(`Unexpected symlink in release artifacts: ${path}`);
    return entry.isDirectory() ? filesWithin(path) : [path];
  });
}

// Validate every target before copying or removing anything. Only verified
// download trees are removed; existing versioned releases are never replaced.
export function flattenRelease(root, download, version) {
  if (!/^\d+\.\d+\.\d+(?:-[\w.-]+)?$/.test(version)) throw new Error('Invalid release version');
  const releases = resolve(root, 'bin/releases');
  const source = resolve(download);
  const output = join(releases, version);
  if (dirname(source) !== releases || !basename(source).startsWith('ci-')) throw new Error('Expected a downloaded ci-<run-id> folder in bin/releases');
  const sourceFiles = filesWithin(source);
  const manifests = sourceFiles.filter(file => basename(file) === 'SHA256SUMS');
  if (manifests.length !== Object.keys(targets).length) throw new Error('Expected checksum manifests for all four release targets');
  const covered = new Set(manifests);
  const seenTargets = new Set();
  const packages = new Map();
  let license;
  for (const manifest of manifests.sort()) {
    const entries = new Map();
    for (const row of readFileSync(manifest, 'utf8').trim().split(/\r?\n/)) {
      const match = /^([a-f0-9]{64})  ([^/\\]+)$/.exec(row);
      if (!match || match[2] === '.' || match[2] === '..' || entries.has(match[2])) throw new Error(`Invalid checksum entry in ${manifest}`);
      const [, hash, name] = match;
      const file = join(dirname(manifest), name);
      if (!file.startsWith(source + sep) || !existsSync(file) || digest(file) !== hash) throw new Error(`Checksum mismatch or missing file: ${file}`);
      covered.add(file);
      entries.set(name, { file, hash });
    }
    const names = [...entries.keys()].filter(name => name !== 'LICENSE.txt').sort();
    const target = Object.keys(targets).find(target => {
      const expected = targets[target].files.map(([, suffix]) => `postika-${version}-${target}${suffix}`).sort();
      return JSON.stringify(names) === JSON.stringify(expected);
    });
    if (!target || seenTargets.has(target)) throw new Error(`Missing, duplicated, or unexpected target packages in ${manifest}`);
    seenTargets.add(target);
    const licenseEntry = entries.get('LICENSE.txt');
    if (!licenseEntry) throw new Error(`Missing license in ${manifest}`);
    const text = normalizeLicense(readFileSync(licenseEntry.file));
    if (license !== undefined && license !== text) throw new Error('Conflicting release licenses');
    license = text;
    for (const name of names) {
      if (packages.has(name)) throw new Error(`Duplicate package filename: ${name}`);
      packages.set(name, entries.get(name));
    }
  }
  if (sourceFiles.some(file => !covered.has(file))) throw new Error('Unexpected files in download folder; originals preserved');
  const licenseHash = createHash('sha256').update(license).digest('hex');
  const allHashes = new Map([...packages].map(([name, entry]) => [name, entry.hash]));
  allHashes.set('LICENSE.txt', licenseHash);
  const checksums = [...allHashes].sort(([a], [b]) => a.localeCompare(b, 'en')).map(([name, hash]) => `${hash}  ${name}\n`).join('');
  if (existsSync(output)) {
    const existing = filesWithin(output);
    const expected = new Set([...allHashes.keys()].map(name => join(output, name)));
    expected.add(join(output, 'SHA256SUMS'));
    if (existing.length !== expected.size || existing.some(file => !expected.has(file)) || [...allHashes].some(([name, hash]) => !existsSync(join(output, name)) || digest(join(output, name)) !== hash)) {
      throw new Error(`A different release already exists at ${output}. Move it or bump the release version; downloads preserved at ${source}`);
    }
    // Identical repeated downloads are safe to reuse. Preserve the existing
    // checksum file's valid ordering rather than requiring our sort order.
    const previous = readFileSync(join(output, 'SHA256SUMS'), 'utf8').trim().split(/\r?\n/).sort();
    if (JSON.stringify(previous) !== JSON.stringify(checksums.trim().split('\n').sort())) throw new Error('Existing release checksum manifest differs; downloads preserved');
  } else {
    mkdirSync(releases, { recursive: true });
    const staging = mkdtempSync(join(releases, `.${version}-flatten-`));
    try {
      for (const [name, entry] of packages) {
        copyFileSync(entry.file, join(staging, name));
        if (digest(join(staging, name)) !== entry.hash) throw new Error(`Copy verification failed: ${name}`);
      }
      writeFileSync(join(staging, 'LICENSE.txt'), license);
      writeFileSync(join(staging, 'SHA256SUMS'), checksums);
      renameSync(staging, output);
    } catch (error) { rmSync(staging, { recursive: true, force: true }); throw error; }
  }
  rmSync(source, { recursive: true });
  return output;
}
