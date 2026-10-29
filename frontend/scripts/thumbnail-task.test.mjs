import { test } from 'node:test';
import assert from 'node:assert/strict';
import { thumbnailTask } from '../src/lib/thumbnail-task.ts';

test('successful native thumbnail finishes without aborting its work', async () => {
  let taskSignal;
  const result = await thumbnailTask(async signal => { taskSignal = signal; return 'jpeg'; }, new AbortController().signal);
  assert.equal(result, 'jpeg');
  assert.equal(taskSignal.aborted, false);
});
test('stalled native IPC times out and cancels work even if its promise never settles', async () => {
  let taskSignal;
  const task = thumbnailTask(signal => { taskSignal = signal; return new Promise(() => {}); }, new AbortController().signal, 10);
  await assert.rejects(task, /thumbnail-timeout/);
  assert.equal(taskSignal.aborted, true);
});
test('removing a video rejects promptly and cancels an unresponsive native operation', async () => {
  const controller = new AbortController();
  let taskSignal;
  const task = thumbnailTask(signal => { taskSignal = signal; return new Promise(() => {}); }, controller.signal);
  await Promise.resolve();
  controller.abort();
  await assert.rejects(task, { name: 'AbortError' });
  assert.equal(taskSignal.aborted, true);
});
test('native failures propagate without retaining a busy task', async () => {
  await assert.rejects(thumbnailTask(async () => { throw new Error('ffmpeg-unavailable'); }, new AbortController().signal), /ffmpeg-unavailable/);
});
