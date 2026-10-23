import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import ts from 'typescript';

const source = await readFile(new URL('../src/lib/log-message.ts', import.meta.url), 'utf8');
const messages = JSON.parse(await readFile(new URL('../src/lib/log-messages.json', import.meta.url), 'utf8'));
const compiled = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022, esModuleInterop: true } });
const exported = {};
new Function('require', 'exports', compiled.outputText)(() => messages, exported);
const { parseLogMessage, hasLogDetails } = exported;

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
  const namedLikeError = parseLogMessage('Facebook rejected the groups request: Posting paused.');
  assert.equal(namedLikeError.prefix, 'Facebook rejected the groups request');
  assert.equal(namedLikeError.key, messages['Posting paused.']);
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

test('Facebook fetch progress and rotating posts preserve counts and user text', () => {
  assert.deepEqual(parseLogMessage('Fetched 2 of 10 posts…').args, ['2', '10']);
  assert.equal(parseLogMessage('Fetched 2 posts. The page has no more posts available.').key, 'facebook_end');
  assert.equal(parseLogMessage('Preparing post 2/5 for My Group: 日本語...').key, 'rotate_prepare');
  assert.deepEqual(parseLogMessage('Preparing post 2/5 for My Group: 日本語...').args, ['2', '5', 'My Group: 日本語']);
  assert.deepEqual(parseLogMessage('Uploading images from post 3/5 for Group: A...').args, ['3', '5', 'Group: A']);
  assert.deepEqual(parseLogMessage('Posting post 4/5 to Group: A...').args, ['4', '5', 'Group: A']);
  assert.deepEqual(parseLogMessage('Downloading full-size image 2 of 4 for post 3 of 10…').args, ['2', '4', '3', '10']);
  assert.equal(parseLogMessage('No photo query appeared after opening the image. Reloading the photo page to capture fresh values…').key, 'facebook_reload');
  assert.equal(parseLogMessage('No separate photo query appeared after reload. Opening the next photo to capture its viewer query…').key, 'facebook_next');
  assert.equal(parseLogMessage('Facebook rejected the photo query (HTTP 403). Fetch again').key, 'facebook_photo_error');
  assert.equal(parseLogMessage("Facebook's timeline response was interrupted. Fetch again").key, 'facebook_fetch_error');
  assert.deepEqual(parseLogMessage('Add text to fetched post 2 before posting').args, ['2']);
  assert.equal(parseLogMessage('Could not resolve full-size image 3 for a fetched post. Facebook did not include its photo ID').key, 'facebook_photo_error');
});

test('Facebook URL validation keeps its example URL inside the translated message', () => {
  const source = 'Enter a Facebook page URL, such as https://web.facebook.com/swiftlineMotors';
  const result = parseLogMessage(source);
  assert.equal(result.key, 'facebook_url_error');
  assert.equal(result.url, '');
});

test('ordinary translated statuses never create duplicate English debug details', () => {
  for (const message of ['Posting stopped.', 'Posting paused.', 'Post published.', 'Ready to post.', 'Posted successfully to Group: A 日本語.', 'Loaded and saved 12 groups.', 'Set an interval between 60 and 86400 seconds, with the maximum at least the minimum']) {
    assert.ok(parseLogMessage(message).key, message);
    assert.equal(hasLogDetails(message), false, message);
  }
  for (const message of ['unknown transport error', 'API confirmation: HTTP 200 · requested group 123', 'Submission unconfirmed. Retrying request 2/3: connection reset', 'Could not read the groups response', 'Facebook rejected the photo query (HTTP 403). Fetch again']) {
    assert.equal(hasLogDetails(message), true, message);
  }
  assert.equal(hasLogDetails(''), false);
});

test('group progress and native image errors preserve counts and filenames', () => {
  assert.deepEqual(parseLogMessage('Loaded 12 groups…').args, ['12']);
  assert.deepEqual(parseLogMessage('Restored 7 saved groups. Load groups to refresh the list.').args, ['7']);
  assert.deepEqual(parseLogMessage('Loaded 7 groups, but could not save them. Load groups again to retry saving.').args, ['7']);
  assert.deepEqual(parseLogMessage('Could not read photo 日本語.png. Try browsing for it again.').args, ['photo 日本語.png']);
  assert.deepEqual(parseLogMessage('Unsupported file: a.jpg. Choose JPG, PNG, WebP, or GIF images.').args, ['a.jpg']);
  assert.deepEqual(parseLogMessage('My pictures is not an image file. Drop individual images, not folders.').args, ['My pictures']);
  assert.equal(parseLogMessage('Facebook rejected the groups request (HTTP 403). Check your login and try Load groups again').key, 'groups_error');
  const interrupted = "Could not read Facebook's confirmation; retained response is incomplete: unexpected EOF";
  assert.equal(parseLogMessage(interrupted).key, 'post_uncertain');
  assert.equal(hasLogDetails(interrupted), true);
});

test('backend settings, accounts and group errors have translated summaries', async () => {
  for (const file of ['posting_settings.go', 'profiles.go', 'groups_fetch.go', 'groups_cache.go', 'groups_request.go', 'groups_sort.go', 'image_drop.go']) {
    const source = await readFile(new URL('../../' + file, import.meta.url), 'utf8');
    for (const [, message] of source.matchAll(/(?:errors\.New|fmt\.Errorf|fail)\("([^"\n]+)"/g)) {
      const example = message.replaceAll('%d', '2').replaceAll('%s', ' (code fixture)').replaceAll('%w', 'fixture detail');
      assert.ok(parseLogMessage(example).key, `${file}: ${message}`);
    }
    for (const [, message] of source.matchAll(/load\.set\("[^"]+", "([^"\n]+)"/g)) assert.ok(parseLogMessage(message).key, `${file}: ${message}`);
    if (file === 'groups_fetch.go' || file === 'groups_cache.go') {
      for (const [, message] of source.matchAll(/fmt\.Sprintf\("([^"\n]+)"/g)) assert.ok(parseLogMessage(message.replaceAll('%d', '2')).key, `${file}: ${message}`);
    }
  }
  const renderer = await readFile(new URL('../src/components/LogMessage.svelte', import.meta.url), 'utf8');
  const cases = new Set([...renderer.matchAll(/case ['"]([^'"]+)['"]:/g)].map(match => match[1]));
  for (const [message, key] of Object.entries(messages)) assert.ok(cases.has(key), `Missing translation renderer for ${message}`);
});
