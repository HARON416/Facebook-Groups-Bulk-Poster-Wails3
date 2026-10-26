import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import vm from 'node:vm';
import { paletteTokens, contrast } from './generate-themes.mjs';

const palettes = JSON.parse(await readFile(new URL('../src/lib/theme-palettes.json', import.meta.url), 'utf8'));
const colorhunt = JSON.parse(await readFile(new URL('../theme-sources/colorhunt.json', import.meta.url), 'utf8'));
const newest = JSON.parse(await readFile(new URL('../theme-sources/colorhunt-new.json', import.meta.url), 'utf8'));
const allColorhunt = [...colorhunt.palettes, ...newest.palettes];
const bootstrap = await readFile(new URL('../public/theme-bootstrap.js', import.meta.url), 'utf8');

test('all imported themes keep readable text, actions, and status indicators', () => {
  assert.equal(palettes.length, 187 + allColorhunt.length);
  assert.equal(new Set(palettes.map(p => p.id)).size, palettes.length);
  for (const palette of palettes) {
    const { tokens } = paletteTokens(palette);
    assert.equal(tokens.bg, palette.bg);
    for (const foreground of ['text', 'text-secondary', 'text-muted', 'accent']) {
      for (const background of ['bg', 'surface', 'surface-subtle', 'surface-hover', 'surface-active']) {
        assert.ok(contrast(tokens[foreground], tokens[background]) >= 4.5, `${palette.name}: ${foreground} on ${background}`);
      }
    }
    for (const status of ['danger', 'warning', 'success']) {
      assert.ok(contrast(tokens[status], tokens[`${status}-bg`]) >= 4.5, `${palette.name}: ${status}`);
    }
    assert.ok(contrast(tokens['on-accent'], tokens.accent) >= 4.5, `${palette.name}: button text`);
  }
});

test('saved palettes apply before first paint; invalid and inaccessible storage follow the system', () => {
  function run(saved, dark = false, blocked = false) {
    const root = { dataset: {}, style: {} };
    vm.runInNewContext(bootstrap, { document: { documentElement: root },
      localStorage: { getItem() { if (blocked) throw Error('blocked'); return saved; } },
      matchMedia: () => ({ matches: dark }) });
    return root;
  }
  for (const palette of palettes) {
    const root = run(palette.id);
    assert.equal(root.dataset.palette, palette.id);
    assert.equal(root.dataset.theme, paletteTokens(palette).mode);
    assert.equal(root.style.colorScheme, root.dataset.theme);
  }
  assert.equal(run('dark').dataset.theme, 'dark');
  assert.equal(run('light', true).dataset.theme, 'light');
  for (const saved of [null, 'unsupported', '__proto__', 'toString']) {
    assert.equal(run(saved, true).dataset.theme, 'dark');
    assert.equal(run(saved).dataset.palette, undefined);
  }
  assert.equal(run(null, true, true).dataset.theme, 'dark');
});

test('curated Color Hunt themes preserve source colors and balance light/dark choices', () => {
  assert.equal(colorhunt.palettes.length, 24);
  assert.equal(new Set(colorhunt.palettes.map(source => source.code)).size, 24);
  assert.equal(colorhunt.palettes.filter(source => source.mode === 'light').length, 12);
  assert.equal(colorhunt.palettes.filter(source => source.mode === 'dark').length, 12);
  for (const source of allColorhunt) {
    const palette = palettes.find(palette => palette.id === source.id);
    assert.ok(palette, source.name);
    assert.equal(palette.name, source.name);
    assert.equal(paletteTokens(palette).mode, source.mode);
    assert.deepEqual([palette.bg, palette.main, palette.sub, palette.text].sort(), [...source.colors].sort());
    assert.equal(source.sourceUrl, `https://colorhunt.co/palette/${source.code}`);
    assert.equal(source.colors.map(color => color.slice(1)).join(''), source.code);
  }
});


test('new feed adds twelve distinct balanced desktop palettes with lowercase names', async () => {
  assert.equal(newest.palettes.length, 12);
  assert.equal(new Set(allColorhunt.map(p => p.code)).size, allColorhunt.length);
  assert.equal(newest.palettes.filter(p => p.mode === 'light').length, 6);
  assert.equal(newest.palettes.filter(p => p.mode === 'dark').length, 6);
  for (const source of newest.palettes) {
    assert.equal(source.feed.sort, 'new');
    const palette = palettes.find(p => p.id === source.id);
    assert.ok(contrast(palette.bg, palette.text) >= 7, palette.name);
  }
  for (const palette of palettes) assert.equal(palette.name, palette.name.toLowerCase());
  const options = await readFile(new URL('../src/lib/theme-options.ts', import.meta.url), 'utf8');
  for (const [, name] of options.matchAll(/name: '([^']+)'/g)) assert.equal(name, name.toLowerCase());
});
