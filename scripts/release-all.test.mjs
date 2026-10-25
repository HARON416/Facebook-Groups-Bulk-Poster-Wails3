import test from 'node:test';
import assert from 'node:assert/strict';
import { mkdtempSync, rmSync } from 'node:fs';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { command, releaseAll } from './release-all.mjs';

test('missing GitHub CLI explains installation, login, and local packaging', () => {
  assert.throws(() => command('.', 'gh', ['auth', 'status'], false, () => ({ error: { code: 'ENOENT' } })), error => {
    for (const instruction of ['sudo apt-get install gh', 'gh auth login', 'make release-all', 'make package-all']) assert.ok(error.message.includes(instruction));
    return true;
  });
});

function fixture(t, override = () => undefined) {
  const root = mkdtempSync(join(tmpdir(), 'postika-dispatch-'));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  const calls = [];
  const options = { root, request: 'test-request', sleep: async () => {}, log: () => {}, run: (name, args) => {
    calls.push([name, ...args]);
    const changed = override(name, args);
    if (changed !== undefined) return changed;
    if (name === 'git' && args[0] === 'status') return '';
    if (name === 'git' && args[0] === 'branch') return 'feature/release';
    if (name === 'git' && args[0] === 'rev-parse') return 'commit-sha';
    if (name === 'gh' && args[0] === 'api') return 'commit-sha';
    if (name === 'gh' && args[0] === 'run' && args[1] === 'list') return JSON.stringify([
      { databaseId: 999, displayTitle: 'Postika packages another-request', headSha: 'commit-sha' },
      { databaseId: 42, displayTitle: 'Postika packages test-request', headSha: 'commit-sha', url: 'https://github.com/example/repo/actions/runs/42' },
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

test('downloads this request’s run, never another concurrent build', async t => {
  const { options, calls } = fixture(t);
  const output = await releaseAll(options);
  assert.ok(output.endsWith('ci-42'));
  assert.ok(calls.some(call => call.join(' ') === 'gh run watch 42 --exit-status'));
  assert.ok(calls.some(call => call[1] === 'run' && call[2] === 'download' && call[3] === '42'));
});

test('failed builds do not download incomplete packages', async t => {
  const { options, calls } = fixture(t, (name, args) => {
    if (name === 'gh' && args[0] === 'run' && args[1] === 'watch') throw new Error('Build failed');
  });
  await assert.rejects(releaseAll(options), /Build failed/);
  assert.ok(!calls.some(call => call[2] === 'download'));
});
