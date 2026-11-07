import { existsSync, mkdirSync, readFileSync, realpathSync, renameSync, writeFileSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { releaseVersion } from './package-release.mjs';
import { verifyFlatRelease } from './flatten-release.mjs';
import { websiteRelease } from './release-layout.mjs';

export function saveWebsiteRelease(root, metadata) {
  const macZipOnly = metadata.downloads?.filter(item => item.os === 'macos').every(item => item.url?.endsWith('.app.zip'));
  const expected = websiteRelease(metadata.version, { date: metadata.date, notes: metadata.notes, macZipOnly });
  if (!/^\d{4}-\d{2}-\d{2}$/.test(metadata.date ?? '') || !Array.isArray(metadata.notes) ||
      metadata.notes.some(note => typeof note !== 'string' || note.length > 2000) ||
      JSON.stringify(metadata.downloads) !== JSON.stringify(expected.downloads)) {
    throw new Error('Published website metadata does not match the expected release download URLs.');
  }
  const directory = join(root, 'website/releases');
  const file = join(directory, `${metadata.version}.json`);
  const text = JSON.stringify(expected, null, 2) + '\n';
  mkdirSync(directory, { recursive: true });
  for (const target of [file, join(directory, 'current.json')]) {
    writeFileSync(target + '.tmp', text);
    renameSync(target + '.tmp', target);
  }
  return file;
}

// Only point the website at a complete, checksum-verified local release.
export function generateWebsiteRelease(root) {
  const version = releaseVersion(root);
  verifyFlatRelease(join(root, 'bin/releases', version), version);
  const directory = join(root, 'website/releases');
  const file = join(directory, `${version}.json`);
  const previous = existsSync(file) ? JSON.parse(readFileSync(file, 'utf8')) : {};
  const macZipOnly = !existsSync(join(root, 'bin/releases', version, `postika-${version}-darwin-arm64.dmg`));
  const metadata = websiteRelease(version, { date: previous.date, notes: previous.notes, macZipOnly });
  return saveWebsiteRelease(root, metadata);
}

if (process.argv[1] && realpathSync(resolve(process.argv[1])) === realpathSync(fileURLToPath(import.meta.url))) {
  try { console.log(`Website release metadata: ${generateWebsiteRelease(resolve(dirname(fileURLToPath(import.meta.url)), '..'))}`); }
  catch (error) { console.error(error.message); process.exitCode = 1; }
}
