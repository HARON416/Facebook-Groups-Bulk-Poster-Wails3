import { test, after } from 'node:test';
import assert from 'node:assert/strict';
import { readFile, writeFile, mkdtemp, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { pathToFileURL } from 'node:url';
import ts from 'typescript';
import { compileModule } from 'svelte/compiler';
import { flushSync } from 'svelte';

const folder = await mkdtemp(join(tmpdir(), 'postika-poster-test-'));
async function compile(source, name) {
  const js = ts.transpileModule(source, { compilerOptions: { target: ts.ScriptTarget.ESNext, module: ts.ModuleKind.ESNext } }).outputText;
  const compiled = compileModule(js, { filename: `${name}.svelte.js`, generate: 'client', dev: true }).js.code;
  const resolved = compiled.replace(/(['"])svelte(\/[^'"]*)?\1/g, (_, quote, suffix = '') => JSON.stringify(import.meta.resolve(`svelte${suffix}`)));
  const path = join(folder, `${name}.mjs`);
  await writeFile(path, resolved);
  return pathToFileURL(path).href;
}
const production = await compile(await readFile(new URL('../src/lib/video-poster.svelte.ts', import.meta.url), 'utf8'), 'poster');
const harnessURL = await compile(`
  import { createVideoPoster } from ${JSON.stringify(production)};
  export function mount(generate) {
    let api;
    const dispose = $effect.root(() => {
      let file = $state.raw(null), profile = $state({ id: 'one', name: 'Account' }), attempt = $state(0);
      const poster = createVideoPoster(() => file, () => profile.id, () => attempt, generate);
      api = { poster, setFile: next => { file = next; }, poll: () => { profile = { id: profile.id, name: 'Refreshed' }; }, account: id => { profile = { id, name: 'Other' }; }, retry: () => { attempt++; } };
    });
    return { ...api, dispose };
  }
`, 'harness');
const { mount } = await import(harnessURL);
const settle = async () => { for (let i = 0; i < 5; i++) { await Promise.resolve(); flushSync(); } };

test('profile polling does not regenerate or clear pending/completed video thumbnails', async () => {
  const calls = [];
  const fixture = mount((file, id, signal) => new Promise(resolve => calls.push({ file, id, signal, resolve })));
  try {
    fixture.setFile(new File(['video'], 'one.mp4')); flushSync();
    assert.equal(calls.length, 1);
    for (let i = 0; i < 4; i++) { fixture.poll(); flushSync(); }
    assert.equal(calls.length, 1);
    assert.equal(calls[0].signal.aborted, false);
    calls[0].resolve('jpeg'); await settle();
    assert.equal(fixture.poster.image, 'jpeg');
    assert.equal(fixture.poster.busy, false);
    for (let i = 0; i < 4; i++) { fixture.poll(); flushSync(); }
    assert.equal(calls.length, 1);
    assert.equal(fixture.poster.image, 'jpeg');
    assert.equal(fixture.poster.busy, false);
    fixture.retry(); flushSync();
    assert.equal(calls.length, 2);
    fixture.setFile(null); flushSync();
    assert.equal(calls[1].signal.aborted, true);
    calls[1].resolve('stale'); await settle();
    assert.equal(fixture.poster.image, '');
    assert.equal(fixture.poster.busy, false);
  } finally { fixture.dispose(); }
});

test('changing the file or actual account cancels stale thumbnail work', async () => {
  const calls = [];
  const fixture = mount((file, id, signal) => new Promise(resolve => calls.push({ file, id, signal, resolve })));
  try {
    fixture.setFile(new File(['one'], 'one.mp4')); flushSync();
    fixture.setFile(new File(['two'], 'two.mp4')); flushSync();
    assert.equal(calls[0].signal.aborted, true);
    fixture.account('two'); flushSync();
    assert.equal(calls.length, 3);
    assert.equal(calls[1].signal.aborted, true);
    assert.equal(calls[2].id, 'two');
    calls[2].resolve('current'); calls[0].resolve('old'); calls[1].resolve('old'); await settle();
    assert.equal(fixture.poster.image, 'current');
    assert.equal(fixture.poster.busy, false);
  } finally { fixture.dispose(); }
});

after(() => rm(folder, { recursive: true, force: true }));
