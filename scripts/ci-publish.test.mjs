import test from 'node:test';
import assert from 'node:assert/strict';
import { generateKeyPairSync } from 'node:crypto';
import { existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { validateSourceBuild, checkPublishingSecrets, publishFromCI } from './ci-publish.mjs';
import { configurePublishing } from './configure-publishing.mjs';
import { writeDownloadFixture, writeVersionFixture } from './release-test-fixtures.mjs';
import { targets } from './package-release.mjs';

function fixture(t) {
  const root = mkdtempSync(join(tmpdir(), 'postika-ci-publish-'));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  writeVersionFixture(root, '0.1.8');
  const { privateKey, publicKey } = generateKeyPairSync('ed25519', { privateKeyEncoding: { format: 'pem', type: 'pkcs8' }, publicKeyEncoding: { format: 'pem', type: 'spki' } });
  mkdirSync(join(root, 'internal/updateauth'), { recursive: true });
  writeFileSync(join(root, 'internal/updateauth/public-key.pem'), publicKey);
  const env = { BUNNY_STORAGE_ZONE: 'fixture', BUNNY_STORAGE_HOST: 'storage.bunnycdn.com', BUNNY_STORAGE_PASSWORD: 'fixture-password', POSTIKA_UPDATE_PRIVATE_KEY: privateKey };
  const build = { path: '.github/workflows/distribution.yml', event: 'workflow_dispatch', head_sha: 'a'.repeat(40), status: 'in_progress' };
  const jobs = Object.keys(targets).map(name => ({ name, conclusion: 'success' }));
  const run = (name, args) => {
    if (name === 'gh') return JSON.stringify(args[1].includes('/jobs?') ? { jobs } : build);
    return args[1].endsWith('public-key.pem') ? publicKey : '  version: "0.1.8"';
  };
  return { root, env, build, jobs, run, publicKey };
}

test('CI publishing accepts completed package jobs while the overall workflow is still running', t => {
  const f = fixture(t);
  assert.equal(validateSourceBuild(f.root, '42', f.run).version, '0.1.8');
  for (const field of ['path', 'event', 'head_sha']) {
    const original = f.build[field]; f.build[field] = 'invalid';
    assert.throws(() => validateSourceBuild(f.root, '42', f.run)); f.build[field] = original;
  }
  f.jobs[0].conclusion = 'failure';
  assert.throws(() => validateSourceBuild(f.root, '42', f.run), /has not passed/);
});

test('CI publishing rejects mismatched release versions or embedded public keys', t => {
  const f = fixture(t);
  for (const value of ['  version: "0.1.9"', 'different public key']) {
    const run = (name, args) => name === 'git' && (value.includes('version:') ? args[1].endsWith('config.yml') : args[1].endsWith('public-key.pem')) ? value : f.run(name, args);
    assert.throws(() => validateSourceBuild(f.root, '42', run), /differs/);
  }
});

test('CI verifies secret keys and removes the temporary key after success or failure', async t => {
  for (const fail of [false, true]) {
    const f = fixture(t);
    checkPublishingSecrets(f.root, f.env);
    assert.throws(() => checkPublishingSecrets(f.root, { ...f.env, POSTIKA_UPDATE_PRIVATE_KEY: '' }), /Missing/);
    writeDownloadFixture(join(f.root, 'bin/releases/ci-42'), '0.1.8');
    const publish = async ({ root }) => {
      assert.equal(readFileSync(join(root, '.update-keys/updater.key'), 'utf8'), f.env.POSTIKA_UPDATE_PRIVATE_KEY);
      if (fail) throw new Error('fixture upload failure');
      return '0.1.8';
    };
    if (fail) await assert.rejects(publishFromCI(f.root, '42', { env: f.env, publish }), /fixture upload failure/);
    else assert.equal(await publishFromCI(f.root, '42', { env: f.env, publish }), '0.1.8');
    assert.equal(existsSync(join(f.root, '.update-keys/updater.key')), false);
  }
});

test('CI refuses a different private signing key before staging or publishing', t => {
  const f = fixture(t);
  const other = generateKeyPairSync('ed25519', { privateKeyEncoding: { format: 'pem', type: 'pkcs8' }, publicKeyEncoding: { format: 'pem', type: 'spki' } });
  assert.throws(() => checkPublishingSecrets(f.root, { ...f.env, POSTIKA_UPDATE_PRIVATE_KEY: other.privateKey }), /does not match/);
  assert.equal(existsSync(join(f.root, '.update-keys/updater.key')), false);
});

test('secret configuration sends passwords and signing keys over stdin, never arguments or logs', t => {
  const f = fixture(t);
  mkdirSync(join(f.root, '.update-keys'));
  writeFileSync(join(f.root, '.update-keys/updater.key'), f.env.POSTIKA_UPDATE_PRIVATE_KEY);
  const calls = [], logs = [];
  configurePublishing(f.root, { env: f.env, log: value => logs.push(value), spawn: (name, args, options) => {
    calls.push({ name, args, options }); return { status: 0 };
  } });
  assert.equal(calls.length, 4);
  for (const secret of [f.env.BUNNY_STORAGE_PASSWORD, f.env.POSTIKA_UPDATE_PRIVATE_KEY]) {
    assert.ok(!JSON.stringify(calls.map(call => call.args)).includes(secret));
    assert.ok(!logs.join('\n').includes(secret));
    assert.ok(calls.some(call => call.options.input === secret));
  }
});
