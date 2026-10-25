import { randomUUID } from 'node:crypto';
import { spawnSync } from 'node:child_process';
import { mkdirSync, realpathSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { releaseVersion } from './package-release.mjs';
import { flattenRelease } from './flatten-release.mjs';
import { bumpVersion, nextVersion, syncVersions } from './version.mjs';

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
      run('git', ['push', 'origin', `HEAD:refs/heads/${branch}`], true);
    } catch (error) {
      throw new Error(`Version ${version} was committed locally, but push failed. ${error.message}\nPush this commit, then retry make release-all BUMP=none. No workflow was started.`);
    }
  }
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
    const download = resolve(root, 'bin/releases', `ci-${selectedRun.databaseId}`);
    mkdirSync(download, { recursive: true });
    run('gh', ['run', 'download', String(selectedRun.databaseId), '--pattern', 'postika-*', '--dir', download], true);
    const output = flattenRelease(root, download, version);
    log(`Release ready to upload: ${output} (flat packages, one license, combined SHA256SUMS)`);
    return output;
  } catch (error) {
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
      console.log('Requires a clean, pushed branch, the workflow on the default branch, and authenticated GitHub CLI. Does not publish a release.');
    } else await releaseAll();
  } catch (error) { console.error(error.message); process.exitCode = 1; }
}
