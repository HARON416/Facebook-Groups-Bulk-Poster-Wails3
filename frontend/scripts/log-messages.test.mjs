import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import ts from 'typescript';

const source = await readFile(new URL('../src/lib/log-message.ts', import.meta.url), 'utf8');
const messages = JSON.parse(await readFile(new URL('../src/lib/log-messages.json', import.meta.url), 'utf8'));
const compiled = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022, esModuleInterop: true } });
const exported = {};
new Function('require', 'exports', compiled.outputText)(() => messages, exported);
const { parseLogMessage } = exported;

test('result metadata and group names survive translation parsing', () => {
  const result = parseLogMessage('Posted successfully to Group: A & B 日本語. (Code 368) https://www.facebook.com/groups/123/posts/456/ · Item time 12s');
  assert.equal(result.key, 'published');
  assert.deepEqual(result.args, ['Group: A & B 日本語']);
  assert.equal(result.code, '368');
  assert.equal(result.url, 'https://www.facebook.com/groups/123/posts/456/');
  const failed = parseLogMessage('Group: A & B: Facebook rejected this post. (Code 190) · Item time 9s');
  assert.equal(failed.prefix, 'Group: A & B');
  assert.equal(failed.key, messages['Facebook rejected this post.']);
  assert.equal(failed.code, '190');
});

test('progress templates retain numeric fields and arbitrary group text', () => {
  assert.deepEqual(parseLogMessage('Group 2/12: INFO: Posting paused.').args, ['2', '12', 'INFO: Posting paused.']);
  assert.deepEqual(parseLogMessage('Group: X: uploading image 2 of 4…').args, ['Group: X', '2', '4']);
  assert.deepEqual(parseLogMessage('Waiting 60s before posting to Group: X.').args, ['60', 'Group: X']);
  assert.deepEqual(parseLogMessage('Submission unconfirmed. Retrying request 2/3: connection reset').args, ['2', '3']);
  assert.deepEqual(parseLogMessage('Finished: 2 published, 3 awaiting approval, 1 failed or skipped.').args, ['2', '3', '1']);
});

test('scheduled timestamps, pending results, and rewrite failures are recognized', () => {
  const pending = parseLogMessage("Group: X: Your post is pending. Your post is awaiting admin approval. If the admin team approves it, it'll become visible in the group.");
  assert.equal(pending.key, 'pending');
  assert.deepEqual(pending.args, ['Group: X']);
  assert.deepEqual(parseLogMessage('Queue scheduled for 2026-10-21T20:00:00+03:00. Keep Postika open until posting finishes.').args, ['2026-10-21T20:00:00+03:00']);
  assert.equal(parseLogMessage('request error Using the original text for this post only. Gemini remains enabled for the next post.').key,
    messages['Gemini rewrite failed. Using the original text for this post; Gemini will be tried again for the next post.']);
});

test('unknown diagnostics remain unclassified and prototype property names are safe', () => {
  assert.equal(parseLogMessage('arbitrary external error').key, '');
  assert.equal(parseLogMessage('constructor').key, '');
  assert.equal(parseLogMessage('__proto__').key, '');
  assert.equal(parseLogMessage('API confirmation: HTTP 200 · requested group 123').key, messages['Facebook response received']);
});
