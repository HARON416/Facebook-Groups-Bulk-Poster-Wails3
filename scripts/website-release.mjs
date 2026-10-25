import { existsSync, mkdirSync, readFileSync, realpathSync, renameSync, writeFileSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { releaseVersion } from './package-release.mjs';
import { verifyFlatRelease } from './flatten-release.mjs';
import { websiteRelease } from './release-layout.mjs';

// Only point the website at a complete, checksum-verified local release.
export function generateWebsiteRelease(root) {
  const version = releaseVersion(root);
  verifyFlatRelease(join(root, 'bin/releases', version), version);
  const directory = join(root, 'website/releases');
  const file = join(directory, `${version}.json`);
  const previous = existsSync(file) ? JSON.parse(readFileSync(file, 'utf8')) : {};
  const metadata = websiteRelease(version, { date: previous.date, notes: previous.notes });
  const text = JSON.stringify(metadata, null, 2) + '\n';
  mkdirSync(directory, { recursive: true });
  for (const target of [file, join(directory, 'current.json')]) {
    writeFileSync(target + '.tmp', text);
    renameSync(target + '.tmp', target);
  }
  return file;
}

if (process.argv[1] && realpathSync(resolve(process.argv[1])) === realpathSync(fileURLToPath(import.meta.url))) {
  try { console.log(`Website release metadata: ${generateWebsiteRelease(resolve(dirname(fileURLToPath(import.meta.url)), '..'))}`); }
  catch (error) { console.error(error.message); process.exitCode = 1; }
}
