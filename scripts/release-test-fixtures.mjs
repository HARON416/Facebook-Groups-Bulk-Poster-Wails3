import { createHash } from 'node:crypto';
import { mkdirSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { targets } from './package-release.mjs';

export function writeDownloadFixture(download, version = '0.1.0') {
  const directories = {};
  for (const [target, plan] of Object.entries(targets)) {
    const directory = join(download, `postika-${target}`, version, target);
    mkdirSync(directory, { recursive: true });
    directories[target] = directory;
    const contents = new Map(plan.files.map(([, suffix]) => [`postika-${version}-${target}${suffix}`, `Fixture payload: ${target}${suffix}`]));
    contents.set('LICENSE.txt', target.startsWith('windows-') ? 'Fixture license\r\nSecond line\r\n' : 'Fixture license\nSecond line\n');
    const sums = [];
    for (const [name, data] of contents) {
      writeFileSync(join(directory, name), data);
      sums.push(`${createHash('sha256').update(data).digest('hex')}  ${name}`);
    }
    writeFileSync(join(directory, 'SHA256SUMS'), `${sums.join('\n')}\n`);
  }
  return directories;
}
