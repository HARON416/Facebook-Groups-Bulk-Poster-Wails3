import test from 'node:test';
import assert from 'node:assert/strict';
import { existsSync, mkdirSync, mkdtempSync, readFileSync, readdirSync, rmSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { spawnSync } from 'node:child_process';
import { command, releaseAll, resumeRelease } from './release-all.mjs';
import { writeDownloadFixture, writeVersionFixture } from './release-test-fixtures.mjs';
import { checkVersions, versionFiles } from './version.mjs';

test('missing GitHub CLI explains installation, login, and local packaging', () => {
  assert.throws(() => command('.', 'gh', ['auth', 'status'], false, () => ({ error: { code: 'ENOENT' } })), error => {
    for (const instruction of ['sudo apt-get install gh', 'gh auth login', 'make release-all', 'make package-all']) assert.ok(error.message.includes(instruction));
    return true;
  });
});

function fixture(t, override = () => undefined) {
  const root = mkdtempSync(join(tmpdir(), 'postika-dispatch-'));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  writeVersionFixture(root);
  const calls = [];
  let committed = false;
  const options = { root, change: 'patch', request: 'test-request', sleep: async () => {}, log: () => {}, run: (name, args) => {
    calls.push([name, ...args]);
    const changed = override(name, args);
    if (changed !== undefined) return changed;
    if (name === 'git' && args[0] === 'status') return '';
    if (name === 'git' && args[0] === 'branch') return 'feature/release';
    if (name === 'git' && args[0] === 'commit') { committed = true; return ''; }
    if (name === 'git' && args[0] === 'rev-parse') return committed ? 'version-commit-sha' : 'commit-sha';
    if (name === 'gh' && args[0] === 'api') return 'commit-sha';
    if (name === 'gh' && args[0] === 'run' && args[1] === 'download') writeDownloadFixture(args[args.indexOf('--dir') + 1], checkVersions(root));
    if (name === 'gh' && args[0] === 'run' && args[1] === 'list') return JSON.stringify([
      { databaseId: 999, displayTitle: 'Postika packages another-request', headSha: 'commit-sha' },
      { databaseId: 42, displayTitle: 'Postika packages test-request', headSha: committed ? 'version-commit-sha' : 'commit-sha', url: 'https://github.com/example/repo/actions/runs/42' },
    ]);
    return '';
  } };
  return { options, calls };
}

test('dirty checkout cannot dispatch a remote build', async t => {
  const { options, calls } = fixture(t, (name, args) => name === 'git' && args[0] === 'status' ? ' M main.go' : undefined);
  await assert.rejects(releaseAll(options), /Commit and push/);
  assert.equal(calls.length, 1);
});

test('unpushed commit cannot dispatch', async t => {
  const { options, calls } = fixture(t, (name, args) => name === 'gh' && args[0] === 'api' ? 'older-commit' : undefined);
  await assert.rejects(releaseAll(options), /remote branch differs/);
  assert.ok(!calls.some(call => call[1] === 'workflow'));
});

test('mismatched native version metadata cannot dispatch a build', async t => {
  const { options, calls } = fixture(t);
  const path = join(options.root, 'build/windows/info.json');
  writeFileSync(path, readFileSync(path, 'utf8').replace('"ProductVersion": "0.1.0"', '"ProductVersion": "0.0.9"'));
  await assert.rejects(releaseAll(options), /version metadata is inconsistent/);
  assert.ok(!calls.some(call => call[0] === 'gh'));
});

test('downloads this request’s run, never another concurrent build', async t => {
  const { options, calls } = fixture(t);
  const output = await releaseAll(options);
  assert.equal(output, join(options.root, 'bin/releases/0.1.1'));
  assert.equal(readdirSync(output).length, 14);
  assert.equal(existsSync(join(options.root, 'bin/releases/ci-42')), false);
  assert.ok(readFileSync(join(output, 'SHA256SUMS'), 'utf8').includes('postika-0.1.1-windows-amd64-installer.exe'));
  const commit = calls.find(call => call[0] === 'git' && call[1] === 'commit');
  assert.deepEqual(commit.slice(commit.indexOf('--') + 1).sort(), [...versionFiles].sort());
  assert.ok(calls.some(call => call.join(' ') === 'git push --quiet origin HEAD:refs/heads/feature/release'));
  assert.ok(calls.some(call => call.join(' ') === 'gh run watch 42 --exit-status'));
  assert.ok(calls.some(call => call[1] === 'run' && call[2] === 'download' && call[3] === '42'));
});

test('failed builds do not download incomplete packages', async t => {
  const { options, calls } = fixture(t, (name, args) => {
    if (name === 'gh' && args[0] === 'run' && args[1] === 'watch') throw new Error('Build failed');
  });
  await assert.rejects(releaseAll(options), error => error.message.includes('Build failed') && error.message.includes('BUMP=none'));
  assert.ok(!calls.some(call => call[2] === 'download'));
  assert.equal(checkVersions(options.root), '0.1.1');
});

for (const [change, version] of [['minor', '0.2.0'], ['major', '1.0.0'], ['0.3.2', '0.3.2'], ['none', '0.1.0']]) {
  test(`release option ${change} builds ${version}`, async t => {
    const { options, calls } = fixture(t);
    options.change = change;
    const output = await releaseAll(options);
    assert.equal(output, join(options.root, 'bin/releases', version));
    assert.equal(checkVersions(options.root), version);
    assert.equal(calls.some(call => call[1] === 'commit'), change !== 'none');
    assert.equal(calls.some(call => call[1] === 'push'), change !== 'none');
  });
}

test('authentication fails before any version mutation', async t => {
  const { options, calls } = fixture(t, (name, args) => {
    if (name === 'gh' && args[0] === 'auth') throw new Error('Login required');
  });
  await assert.rejects(releaseAll(options), /Login required/);
  assert.equal(checkVersions(options.root), '0.1.0');
  assert.ok(!calls.some(call => call[1] === 'commit' || call[1] === 'push'));
});

test('failed version commit restores metadata and never pushes or dispatches', async t => {
  const { options, calls } = fixture(t, (name, args) => {
    if (name === 'git' && args[0] === 'commit') throw new Error('Commit rejected');
  });
  await assert.rejects(releaseAll(options), /metadata restored to 0.1.0/);
  assert.equal(checkVersions(options.root), '0.1.0');
  assert.ok(!calls.some(call => call[1] === 'push' || call[1] === 'workflow'));
});

test('failed version push preserves its commit and gives a retry without bumping', async t => {
  const { options, calls } = fixture(t, (name, args) => {
    if (name === 'git' && args[0] === 'push') throw new Error('Branch protected');
  });
  await assert.rejects(releaseAll(options), error => error.message.includes('committed locally') && error.message.includes('BUMP=none'));
  assert.equal(checkVersions(options.root), '0.1.1');
  assert.ok(!calls.some(call => call[1] === 'workflow'));
});

test('real local Git remote receives only the version commit used for the build', async t => {
  const base = mkdtempSync(join(tmpdir(), 'postika-release-git-'));
  t.after(() => rmSync(base, { recursive: true, force: true }));
  const root = join(base, 'checkout');
  const remote = join(base, 'remote.git');
  mkdirSync(root);
  writeVersionFixture(root);
  writeFileSync(join(root, '.gitignore'), 'bin/\n');
  writeFileSync(join(root, 'application.txt'), 'Original app source');
  const git = (...args) => {
    const result = spawnSync('git', args, { cwd: root, encoding: 'utf8' });
    assert.equal(result.status, 0, result.stderr);
    return result.stdout.trim();
  };
  git('init', '-b', 'main');
  git('config', 'user.name', 'Release fixture');
  git('config', 'user.email', 'fixture@example.invalid');
  git('config', 'commit.gpgsign', 'false');
  git('init', '--bare', remote);
  git('add', '.');
  git('commit', '-m', 'Fixture source');
  git('remote', 'add', 'origin', remote);
  git('push', '-u', 'origin', 'main');
  const sourceSHA = git('rev-parse', 'HEAD');
  const output = await releaseAll({ root, change: 'patch', request: 'git-test', sleep: async () => {}, log: () => {}, run(name, args) {
    if (name === 'git') return git(...args);
    if (args[0] === 'api') return sourceSHA;
    if (args[0] === 'run' && args[1] === 'list') return JSON.stringify([{ databaseId: 9, displayTitle: 'Postika packages git-test', headSha: git('rev-parse', 'HEAD'), url: 'https://example.invalid/run' }]);
    if (args[0] === 'run' && args[1] === 'download') writeDownloadFixture(args[args.indexOf('--dir') + 1], checkVersions(root));
    return '';
  } });
  assert.equal(output, join(root, 'bin/releases/0.1.1'));
  assert.equal(git('log', '-1', '--format=%s'), 'Bump Postika to 0.1.1');
  assert.equal(git('--git-dir', remote, 'rev-parse', 'refs/heads/main'), git('rev-parse', 'HEAD'));
  assert.deepEqual(git('diff', '--name-only', 'HEAD~', 'HEAD').split('\n').sort(), [...versionFiles].sort());
  assert.equal(readFileSync(join(root, 'application.txt'), 'utf8'), 'Original app source');
  assert.equal(git('diff', '--name-only'), '');
  const metadata = JSON.parse(readFileSync(join(root, 'website/releases/current.json'), 'utf8'));
  assert.equal(metadata.version, '0.1.1');
  assert.ok(metadata.downloads.every(download => download.url.includes('/downloads/0.1.1/')));
  assert.equal(git('status', '--porcelain'), '?? website/');
});

test('interrupted artifact downloads retry the same run in fresh directories', async t => {
  const directories = [];
  const { options, calls } = fixture(t, (name, args) => {
    if (name === 'gh' && args[0] === 'run' && args[1] === 'download') {
      const directory = args[args.indexOf('--dir') + 1];
      directories.push(directory);
      if (directories.length < 3) {
        writeFileSync(join(directory, 'partially-extracted.txt'), 'partial');
        throw new Error('connection reset by peer');
      }
    }
  });
  const output = await releaseAll(options);
  assert.equal(output, join(options.root, 'bin/releases/0.1.1'));
  assert.equal(new Set(directories).size, 3);
  assert.ok(directories.every(directory => !existsSync(directory)));
  assert.equal(calls.filter(call => call[0] === 'gh' && call[1] === 'workflow').length, 1);
});

test('exhausted downloads preserve the old release and give the successful run resume command', async t => {
  const { options, calls } = fixture(t, (name, args) => {
    if (name === 'gh' && args[0] === 'run' && args[1] === 'download') throw new Error('connection reset by peer');
  });
  const prior = join(options.root, 'bin/releases/0.1.0');
  mkdirSync(prior, { recursive: true });
  writeFileSync(join(prior, 'preserve.txt'), 'old release');
  await assert.rejects(releaseAll(options), /make release-resume RUN_ID=42/);
  assert.equal(calls.filter(call => call[0] === 'gh' && call[1] === 'run' && call[2] === 'download').length, 3);
  assert.equal(readFileSync(join(prior, 'preserve.txt'), 'utf8'), 'old release');
  assert.equal(checkVersions(options.root), '0.1.1');
});

function resumeFixture(t, build = {}, committed = '0.1.0') {
  const f = fixture(t, (name, args) => {
    if (name === 'gh' && args[0] === 'api') return JSON.stringify({
      path: '.github/workflows/distribution.yml', event: 'workflow_dispatch',
      status: 'completed', conclusion: 'success', head_sha: 'commit-sha', ...build,
    });
    if (name === 'git' && args[0] === 'show') return `  version: "${committed}"`;
  });
  f.options.runID = '42';
  return f;
}

test('resume downloads a successful matching commit without mutation, dispatch or rebuilding', async t => {
  const { options, calls } = resumeFixture(t);
  const output = await resumeRelease(options);
  assert.equal(output, join(options.root, 'bin/releases/0.1.0'));
  assert.equal(readdirSync(output).length, 14);
  assert.ok(!calls.some(call => call[0] === 'git' && ['status', 'commit', 'push'].includes(call[1])));
  assert.ok(!calls.some(call => call[0] === 'gh' && (call[1] === 'workflow' || call[2] === 'watch')));
  assert.equal(checkVersions(options.root), '0.1.0');
});

for (const build of [
  { path: '.github/workflows/unrelated.yml' }, { event: 'push' },
  { status: 'in_progress' }, { conclusion: 'failure' }, { head_sha: 'other-commit' },
]) {
  test(`resume rejects an unrelated or unsuccessful run: ${JSON.stringify(build)}`, async t => {
    const { options, calls } = resumeFixture(t, build);
    await assert.rejects(resumeRelease(options));
    assert.ok(!calls.some(call => call[0] === 'gh' && call[1] === 'run'));
  });
}

test('resume rejects changed version metadata and invalid run IDs before downloading', async t => {
  const { options, calls } = resumeFixture(t, {}, '0.0.9');
  await assert.rejects(resumeRelease(options), /version metadata differs/);
  assert.ok(!calls.some(call => call[0] === 'gh' && call[1] === 'run'));
  calls.length = 0;
  await assert.rejects(resumeRelease({ ...options, runID: '../other' }), /RUN_ID/);
  assert.equal(calls.length, 0);
});

test('a different checkout requires explicit acknowledgment of the exact built source commit', async t => {
  const { options, calls } = resumeFixture(t, { head_sha: 'already-built-commit' });
  await assert.rejects(resumeRelease({ ...options, sourceCommit: 'wrong-commit' }), /SOURCE_COMMIT/);
  const output = await resumeRelease({ ...options, sourceCommit: 'already-built-commit' });
  assert.equal(output, join(options.root, 'bin/releases/0.1.0'));
  assert.ok(calls.some(call => call[0] === 'git' && call[1] === 'show' && call[2] === 'already-built-commit:build/config.yml'));
});
