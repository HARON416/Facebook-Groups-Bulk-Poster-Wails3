import { createPrivateKey, createPublicKey } from 'node:crypto';
import { spawnSync } from 'node:child_process';
import { appendFileSync, mkdirSync, readFileSync, realpathSync, rmSync, writeFileSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { releaseVersion } from './package-release.mjs';
import { targets } from './package-release.mjs';
import { flattenRelease } from './flatten-release.mjs';
import { bunnyConfig, publishRelease } from './publish-release.mjs';

function command(root, name, args) {
  const result = spawnSync(name, args, { cwd: root, encoding: 'utf8' });
  if (result.error || result.status !== 0) throw new Error(`${name} ${args[0]} failed while validating the source build.`);
  return result.stdout.trim();
}

export function validateSourceBuild(root, runID, run = (name, args) => command(root, name, args)) {
  if (!/^\d+$/.test(String(runID))) throw new Error('A numeric source run ID is required.');
  const version = releaseVersion(root);
  const build = JSON.parse(run('gh', ['api', `repos/{owner}/{repo}/actions/runs/${runID}`]));
  if (build.path !== '.github/workflows/distribution.yml' || build.event !== 'workflow_dispatch' || !/^[a-f0-9]{40}$/.test(build.head_sha)) {
    throw new Error('Source run is not a distribution build with a valid source commit.');
  }
  // A publish job inside the source workflow means the overall run is still
  // in progress. Check the four package jobs themselves, including on retries.
  const { jobs } = JSON.parse(run('gh', ['api', `repos/{owner}/{repo}/actions/runs/${runID}/jobs?per_page=100&filter=latest`]));
  if (!Array.isArray(jobs)) throw new Error('Source job inventory is unavailable.');
  for (const target of Object.keys(targets)) {
    const matches = jobs.filter(job => job.name === target);
    if (matches.length !== 1 || matches[0].conclusion !== 'success') throw new Error(`Source package job ${target} has not passed.`);
  }
  const config = run('git', ['show', `${build.head_sha}:build/config.yml`]);
  if (config.match(/^  version: "([^"]+)"/m)?.[1] !== version) throw new Error('Source build version differs from publishing checkout.');
  const sourceKey = run('git', ['show', `${build.head_sha}:internal/updateauth/public-key.pem`]);
  if (sourceKey.trim() !== readFileSync(join(root, 'internal/updateauth/public-key.pem'), 'utf8').trim()) {
    throw new Error('Publishing key differs from the key embedded in the source build.');
  }
  const published = jobs.some(job => ['publish', 'publish / publish'].includes(job.name) && job.conclusion === 'success');
  return { version, sha: build.head_sha, published };
}

export function checkPublishingSecrets(root, env = process.env) {
  bunnyConfig(env);
  if (!env.POSTIKA_UPDATE_PRIVATE_KEY?.trim()) throw new Error('Missing POSTIKA_UPDATE_PRIVATE_KEY repository secret.');
  let publicKey;
  try { publicKey = createPublicKey(createPrivateKey(env.POSTIKA_UPDATE_PRIVATE_KEY)).export({ type: 'spki', format: 'pem' }); }
  catch { throw new Error('Update signing key secret is not a valid private key.'); }
  if (publicKey.trim() !== readFileSync(join(root, 'internal/updateauth/public-key.pem'), 'utf8').trim()) {
    throw new Error('Update signing key secret does not match the app public key.');
  }
}

export async function publishFromCI(root, runID, { env = process.env, publish = publishRelease } = {}) {
  if (!/^\d+$/.test(String(runID))) throw new Error('A numeric source run ID is required.');
  checkPublishingSecrets(root, env);
  const version = releaseVersion(root);
  flattenRelease(root, join(root, 'bin/releases', `ci-${runID}`), version);
  const directory = join(root, '.update-keys');
  const key = join(directory, 'updater.key');
  mkdirSync(directory, { recursive: true, mode: 0o700 });
  writeFileSync(key, env.POSTIKA_UPDATE_PRIVATE_KEY, { flag: 'wx', mode: 0o600 });
  try { return await publish({ root }); }
  finally { rmSync(key, { force: true }); }
}

if (process.argv[1] && realpathSync(resolve(process.argv[1])) === realpathSync(fileURLToPath(import.meta.url))) {
  try {
    const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
    const runID = process.env.POSTIKA_SOURCE_RUN_ID;
    if (process.argv[2] === 'validate') {
      const result = validateSourceBuild(root, runID);
      if (process.env.GITHUB_OUTPUT) appendFileSync(process.env.GITHUB_OUTPUT, `version=${result.version}\n`);
      console.log(`Source run ${runID}: version ${result.version}, commit ${result.sha}; all four package jobs passed.`);
    } else if (process.argv[2] === 'check-secrets') {
      checkPublishingSecrets(root);
      console.log('Publishing configuration and signing key match the app.');
    } else if (process.argv[2] === 'publish') await publishFromCI(root, runID);
    else throw new Error('Use validate, check-secrets or publish.');
  } catch (error) { console.error(error.message); process.exitCode = 1; }
}
