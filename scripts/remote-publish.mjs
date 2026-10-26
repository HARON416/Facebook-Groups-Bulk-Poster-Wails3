import { randomUUID } from 'node:crypto';
import { mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { validateSourceBuild } from './ci-publish.mjs';
import { saveWebsiteRelease } from './website-release.mjs';
import { releaseVersion } from './package-release.mjs';
import { downloadPrefix } from './release-layout.mjs';

export function rememberBuild(root, record) {
  mkdirSync(join(root, 'bin'), { recursive: true });
  writeFileSync(join(root, 'bin/release-build.json'), JSON.stringify(record, null, 2) + '\n');
}

export function previousBuild(root) {
  try {
    const record = JSON.parse(readFileSync(join(root, 'bin/release-build.json'), 'utf8'));
    if (record.version === releaseVersion(root) && /^\d+$/.test(String(record.runID))) return record;
  } catch { /* An explicit run ID also works on a fresh checkout. */ }
  throw new Error('Specify the existing build: make publish-release RUN_ID=<distribution-run-id>.');
}

export function checkRemotePublishing(run) {
  const secrets = JSON.parse(run('gh', ['secret', 'list', '--json', 'name'])).map(item => item.name);
  const variables = JSON.parse(run('gh', ['variable', 'list', '--json', 'name'])).map(item => item.name);
  if (!['BUNNY_STORAGE_PASSWORD', 'POSTIKA_UPDATE_PRIVATE_KEY'].every(name => secrets.includes(name)) ||
      !['BUNNY_STORAGE_ZONE', 'BUNNY_STORAGE_HOST'].every(name => variables.includes(name))) {
    throw new Error('GitHub publishing configuration is missing. Run make configure-publishing once before publishing.');
  }
}

export async function syncPublishedMetadata(root, version, { request = fetch, sleep = ms => new Promise(resolve => setTimeout(resolve, ms)) } = {}) {
  for (let attempt = 0; attempt < 3; attempt++) {
    try {
      const response = await request(`${downloadPrefix(version)}release.json`, { redirect: 'error', signal: AbortSignal.timeout(30_000) });
      if (!response.ok) throw new Error('Metadata unavailable');
      const text = await response.text();
      if (text.length > 256_000) throw new Error('Metadata too large');
      const metadata = JSON.parse(text);
      if (metadata.version !== version) throw new Error('Metadata version mismatch');
      return saveWebsiteRelease(root, metadata);
    } catch (error) {
      if (attempt === 2) throw new Error(`Could not sync published ${version} website metadata: ${error.message}. Publishing may already be complete; retry make publish-release without a new version bump.`);
      await sleep(1000 * (attempt + 1));
    }
  }
}

export async function publishExisting({ root, runID, sourceCommit, run, sleep = ms => new Promise(resolve => setTimeout(resolve, ms)), log = console.log, request = randomUUID(), sync = syncPublishedMetadata }) {
  runID ??= previousBuild(root).runID;
  const source = validateSourceBuild(root, runID, run);
  const head = run('git', ['rev-parse', 'HEAD']);
  if (sourceCommit && sourceCommit !== source.sha) throw new Error('SOURCE_COMMIT does not match the source build.');
  if (source.sha !== head && !sourceCommit) throw new Error(`The source build excludes later commits. Acknowledge its source with SOURCE_COMMIT=${source.sha}, or build a new release to include later changes.`);
  checkRemotePublishing(run);
  rememberBuild(root, { version: source.version, runID: String(runID), sha: source.sha });
  if (source.published) {
    log(`GitHub already published ${source.version}; syncing website metadata only.`);
    return sync(root, source.version);
  }
  let prior;
  try { prior = JSON.parse(readFileSync(join(root, 'bin/release-publish.json'), 'utf8')); } catch { /* First publication. */ }
  if (prior?.runID === String(runID) && prior.version === source.version && /^\d+$/.test(String(prior.publishRunID))) {
    const publishing = JSON.parse(run('gh', ['api', `repos/{owner}/{repo}/actions/runs/${prior.publishRunID}`]));
    if (publishing.path === '.github/workflows/publish.yml' && (publishing.status !== 'completed' || publishing.conclusion === 'success')) {
      log(`Continuing publishing run ${prior.publishRunID}; no duplicate publication dispatched.`);
      try {
        run('gh', ['run', 'watch', String(prior.publishRunID), '--exit-status'], true);
        return await sync(root, source.version);
      } catch (error) {
        throw new Error(`${error.message}\nCheck publishing run ${prior.publishRunID}; retry make publish-release RUN_ID=${runID} without a new version bump.`);
      }
    }
  }
  const branch = run('git', ['branch', '--show-current']);
  if (!branch) throw new Error('Check out the branch containing publish.yml before publishing.');
  run('gh', ['workflow', 'run', 'publish.yml', '--ref', branch, '-f', `run_id=${runID}`, '-f', `request_id=${request}`]);
  log(`Publishing built ${source.version} directly from GitHub to Bunny; no package download to this computer.`);
  let selected;
  for (let attempt = 0; attempt < 24; attempt++) {
    const runs = JSON.parse(run('gh', ['run', 'list', '--workflow', 'publish.yml', '--event', 'workflow_dispatch', '--limit', '50', '--json', 'databaseId,displayTitle,headSha,url']));
    selected = runs.find(item => item.displayTitle === `Postika publish ${runID} ${request}` && item.headSha === head);
    if (selected) break;
    await sleep(5000);
  }
  if (!selected) throw new Error(`Publishing was dispatched for request ${request}; check GitHub Actions before dispatching again.`);
  rememberBuild(root, { version: source.version, runID: String(runID), sha: source.sha, publishRunID: selected.databaseId });
  writeFileSync(join(root, 'bin/release-publish.json'), JSON.stringify({ version: source.version, runID: String(runID), publishRunID: selected.databaseId }) + '\n');
  log(selected.url);
  try {
    run('gh', ['run', 'watch', String(selected.databaseId), '--exit-status'], true);
    const output = await sync(root, source.version);
    log(`Published ${source.version}; website metadata synced. Rebuild and deploy the website.`);
    return output;
  } catch (error) {
    throw new Error(`${error.message}\nCheck publishing run ${selected.databaseId}; it runs independently of this terminal. To retry the existing build, use make publish-release RUN_ID=${runID}${source.sha !== head ? ` SOURCE_COMMIT=${source.sha}` : ''}.`);
  }
}
