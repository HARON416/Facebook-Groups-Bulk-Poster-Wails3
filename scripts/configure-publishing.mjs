import { spawnSync } from 'node:child_process';
import { readFileSync, realpathSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { checkPublishingSecrets } from './ci-publish.mjs';

export function configurePublishing(root, { env = process.env, spawn = spawnSync, log = console.log } = {}) {
  const key = readFileSync(join(root, '.update-keys/updater.key'), 'utf8');
  checkPublishingSecrets(root, { ...env, POSTIKA_UPDATE_PRIVATE_KEY: key });
  const entries = [
    ['secret', 'BUNNY_STORAGE_PASSWORD', env.BUNNY_STORAGE_PASSWORD],
    ['secret', 'POSTIKA_UPDATE_PRIVATE_KEY', key],
    ['variable', 'BUNNY_STORAGE_ZONE', env.BUNNY_STORAGE_ZONE],
    ['variable', 'BUNNY_STORAGE_HOST', env.BUNNY_STORAGE_HOST],
  ];
  for (const [type, name, value] of entries) {
    // Secret values travel over stdin, never command arguments or log output.
    const result = spawn('gh', [type, 'set', name, ...(type === 'variable' ? ['--body', value] : [])], {
      cwd: root, encoding: 'utf8', input: type === 'secret' ? value : undefined, stdio: ['pipe', 'pipe', 'pipe'],
    });
    if (result.error || result.status !== 0) throw new Error(`Could not configure repository ${type} ${name}. Check GitHub authentication and repository permissions.`);
    log(`Configured repository ${type}: ${name}`);
  }
}

if (process.argv[1] && realpathSync(resolve(process.argv[1])) === realpathSync(fileURLToPath(import.meta.url))) {
  try { configurePublishing(resolve(dirname(fileURLToPath(import.meta.url)), '..')); }
  catch (error) { console.error(error.message); process.exitCode = 1; }
}
