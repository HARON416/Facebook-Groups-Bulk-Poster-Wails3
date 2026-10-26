import { randomUUID } from 'node:crypto';
import { spawnSync } from 'node:child_process';
import { mkdirSync, mkdtempSync, realpathSync, rmSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { releaseVersion } from './package-release.mjs';
import { flattenRelease } from './flatten-release.mjs';
import { bumpVersion, nextVersion, syncVersions } from './version.mjs';
import { generateWebsiteRelease } from './website-release.mjs';
import { preflightPublish, publishRelease } from './publish-release.mjs';

const projectRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const workflow = 'distribution.yml';
export function command(root, name, args, inherit = false, spawn = spawnSync) {
  const result = spawn(name, args, { cwd: root, encoding: 'utf8', stdio: inherit ? 'inherit' : 'pipe' });
  if (result.error?.code === 'ENOENT' && name === 'gh') {
    throw new Error('GitHub CLI (gh) is required to start and download the multi-OS builds. On Ubuntu/Debian, install it with: sudo apt-get install gh\nThen authenticate with: gh auth login\nRetry: make release-all\nFor local packages without GitHub, use: make package-all');
  }
  if (result.error) throw new Error(`${name} is unavailable: ${result.error.message}`);
  if (result.status !== 0) throw new Error(`${name} ${args[0]} failed. ${result.stderr?.trim() ?? ''}`);
  return result.stdout?.trim() ?? '';
}

export async function downloadRun({ root, version, runID, run, sleep, log }) {
  if (!/^\d+$/.test(String(runID))) throw new Error('A numeric GitHub Actions run ID is required.');
  const releases = resolve(root, 'bin/releases');
  mkdirSync(releases, { recursive: true });
  let download;
  for (let attempt = 0; attempt < 3; attempt++) {
    // gh cannot overwrite partially extracted files. Retry in a fresh directory,
    // preserving earlier complete releases and any pre-existing downloads.
    download = mkdtempSync(join(releases, `ci-${runID}-`));
    try {
      run('gh', ['run', 'download', String(runID), '--pattern', 'postika-*', '--dir', download], true);
      break;
    } catch (error) {
      rmSync(download, { recursive: true, force: true });
      if (attempt === 2) throw error;
      log(`Artifact download interrupted. Retrying the same successful run (${attempt + 2}/3)…`);
      await sleep(2000 * (attempt + 1));
    }
  }
  const output = flattenRelease(root, download, version);
  log(`Website download metadata generated: ${generateWebsiteRelease(root)}`);
  log(`Release ready to upload: ${output} (flat packages, one license, combined SHA256SUMS)`);
  return output;
}

export async function resumeRelease({ root = projectRoot, runID, sourceCommit = process.env.POSTIKA_RELEASE_SOURCE_COMMIT, run = (...args) => command(root, ...args), sleep = ms => new Promise(resolve => setTimeout(resolve, ms)), log = console.log } = {}) {
  if (!/^\d+$/.test(String(runID))) throw new Error('Use make release-resume RUN_ID=<successful-run-id>.');
  const version = releaseVersion(root);
  const sha = run('git', ['rev-parse', 'HEAD']);
  run('gh', ['auth', 'status']);
  const build = JSON.parse(run('gh', ['api', `repos/{owner}/{repo}/actions/runs/${runID}`]));
  if (build.path !== `.github/workflows/${workflow}` || build.event !== 'workflow_dispatch') throw new Error('Selected run is not the distribution packaging workflow.');
  if (build.status !== 'completed' || build.conclusion !== 'success') throw new Error('Selected build has not completed successfully.');
  if (sourceCommit && sourceCommit !== build.head_sha) throw new Error('SOURCE_COMMIT does not match the selected successful build.');
  if (build.head_sha !== sha) {
    if (!sourceCommit) throw new Error(`Selected build does not match this checkout commit. To publish its already-built code, explicitly set SOURCE_COMMIT=${build.head_sha}; newer local changes will not be included.`);
    log(`Using built commit ${build.head_sha}; changes in checkout ${sha} are not included in these packages.`);
  }
  const committed = run('git', ['show', `${build.head_sha}:build/config.yml`]).match(/^  version: "([^"]+)"/m)?.[1];
  if (committed !== version) throw new Error('Local version metadata differs from the built release commit.');
  log(`Resuming successful build ${runID} for ${version}; no version bump or rebuild.`);
  try {
    return await downloadRun({ root, version, runID, run, sleep, log });
  } catch (error) {
    throw new Error(`${error.message}\nRetry the same build with make release-resume RUN_ID=${runID}. Add PUBLISH=1 to upload after downloading.`);
  }
}

export async function releaseAll({ root = projectRoot, run = (...args) => command(root, ...args), sleep = ms => new Promise(resolve => setTimeout(resolve, ms)), request = randomUUID(), log = console.log, change = process.env.POSTIKA_RELEASE_VERSION || process.env.POSTIKA_RELEASE_BUMP || 'patch' } = {}) {
  // The remote workflow builds pushed source, not local working-tree edits.
  if (run('git', ['status', '--porcelain']).length) throw new Error('Commit and push your changes before building a distributed release. Local edits cannot be built by GitHub Actions.');
  const branch = run('git', ['branch', '--show-current']);
  if (!branch) throw new Error('Check out a branch before running make release-all.');
  let sha = run('git', ['rev-parse', 'HEAD']);
  const previous = releaseVersion(root);
  const version = change === 'none' ? previous : nextVersion(previous, change);
  run('gh', ['auth', 'status']);
  const remoteSHA = run('gh', ['api', `repos/{owner}/{repo}/commits/${encodeURIComponent(branch)}`, '--jq', '.sha']);
  if (remoteSHA !== sha) throw new Error('The remote branch differs from this checkout. Push this commit before building.');
  if (change !== 'none') {
    log(`Preparing release ${previous} → ${version}. Committing and pushing version metadata only.`);
    const update = bumpVersion(root, change);
    try {
      // --only commits these known metadata paths, never unrelated files.
      run('git', ['commit', '--only', '-m', `Bump Postika to ${version}`, '--', ...update.files], true);
    } catch (error) {
      syncVersions(root, previous);
      throw new Error(`Could not commit release version; metadata restored to ${previous}. ${error.message}`);
    }
    sha = run('git', ['rev-parse', 'HEAD']);
    try {
      run('git', ['push', '--quiet', 'origin', `HEAD:refs/heads/${branch}`]);
    } catch (error) {
      throw new Error(`Version ${version} was committed locally, but push failed. ${error.message}\nPush this commit, then retry make release-all BUMP=none. No workflow was started.`);
    }
  }
  let successfulRun;
  try {
    run('gh', ['workflow', 'run', workflow, '--ref', branch, '-f', `request_id=${request}`]);
    log('Requested all four release targets. Waiting for GitHub to create the run…');
    let selectedRun;
    for (let attempt = 0; attempt < 24; attempt++) {
      const runs = JSON.parse(run('gh', ['run', 'list', '--workflow', workflow, '--event', 'workflow_dispatch', '--limit', '50', '--json', 'databaseId,displayTitle,headSha,url']));
      selectedRun = runs.find(item => item.displayTitle === `Postika packages ${request}` && item.headSha === sha);
      if (selectedRun) break;
      await sleep(5000);
    }
    if (!selectedRun) throw new Error(`Run not visible yet. Find request ${request} in GitHub Actions; it was already dispatched. Do not dispatch it again to find it.`);
    log(selectedRun.url);
    run('gh', ['run', 'watch', String(selectedRun.databaseId), '--exit-status'], true);
    successfulRun = selectedRun.databaseId;
    return await downloadRun({ root, version, runID: successfulRun, run, sleep, log });
  } catch (error) {
    if (successfulRun) throw new Error(`${error.message}\nVersion ${version} remains committed and the build succeeded. Resume without rebuilding: make release-resume RUN_ID=${successfulRun}. Add PUBLISH=1 to upload after downloading.`);
    throw new Error(`${error.message}\nVersion ${version} remains committed. To retry without another increment, use make release-all BUMP=none.`);
  }
}


if (process.argv[1] && realpathSync(resolve(process.argv[1])) === realpathSync(fileURLToPath(import.meta.url))) {
  try {
    if (process.argv.includes('--plan')) {
      console.log('Increment patch by default, synchronize native metadata, then commit and push only those version files.');
      console.log('Choose BUMP=minor, BUMP=major, VERSION=X.Y.Z, or BUMP=none to rebuild the existing version.');
      console.log('Build Linux x64, Windows x64, macOS arm64 and macOS amd64 on native GitHub runners.');
      console.log('Wait for all jobs, verify and flatten packages into bin/releases/<version>/ with one license and combined SHA256SUMS.');
      console.log('Generate website/releases/<version>.json and current.json automatically; rebuild/deploy the website to use them.');
      console.log('PUBLISH=1 also signs update feeds, uploads packages to downloads/<version>/ on Bunny, verifies them publicly, then replaces updates/*.json.');
      console.log('Resume an interrupted download with make release-resume RUN_ID=<successful-run-id> PUBLISH=1; no version bump or rebuild.');
      console.log('Requires a clean, pushed branch, the workflow on the default branch, and authenticated GitHub CLI. Publishing also requires .env and the existing update signing key.');
    } else {
      const client = process.env.POSTIKA_PUBLISH === '1' ? await preflightPublish(projectRoot) : null;
      if (process.argv.includes('--resume')) await resumeRelease({ runID: process.env.POSTIKA_RELEASE_RUN_ID });
      else await releaseAll();
      if (client) await publishRelease({ root: projectRoot, client });
    }
  } catch (error) { console.error(error.message); process.exitCode = 1; }
}
