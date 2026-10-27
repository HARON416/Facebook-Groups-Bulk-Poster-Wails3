import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { filterThemes, paletteColorFamilies, parseThemeFavorites } from '../src/lib/theme-browser.ts';

const palettes = JSON.parse(await readFile(new URL('../src/lib/theme-palettes.json', import.meta.url), 'utf8'));
const modes = JSON.parse(await readFile(new URL('../src/lib/theme-modes.json', import.meta.url), 'utf8'));
const options = palettes.map(p => ({ ...p, mode: modes[p.id], colors: paletteColorFamilies(p) }));
const defaults = { query: '', color: 'all', appearance: 'all', favoritesOnly: false, favorites: new Set() };

test('color filters find palettes by visible colors as well as descriptive names', () => {
  const harvest = options.find(p => p.name === 'harvest linen');
  assert.ok(harvest.colors.includes('green'));
  assert.ok(harvest.colors.includes('beige'));
  assert.ok(filterThemes(options, { ...defaults, color: 'green' }).includes(harvest));
  const charcoal = options.find(p => p.name === 'black rose');
  assert.ok(charcoal.colors.includes('black'));
  assert.ok(!charcoal.colors.includes('white'), 'white text does not classify a dark theme as white');
  assert.deepEqual(paletteColorFamilies({ name: 'test', bg: '#fff', main: '#000' }), ['white']);
  for (const name of ['mint harbor', 'navy coral', 'grey berry', 'blue glacier', 'peach indigo']) {
    assert.ok(options.find(p => p.name === name).colors.includes(name.split(' ')[0]));
  }
});

test('search, appearance, color, and favorites compose without changing the source order', () => {
  const favorites = new Set(options.filter(p => p.name.startsWith('green ')).map(p => p.id));
  const result = filterThemes(options, { query: '  GREEN  ', appearance: 'dark', color: 'green', favoritesOnly: true, favorites });
  assert.ok(result.length > 0);
  assert.ok(result.every(p => p.mode === 'dark' && p.colors.includes('green') && favorites.has(p.id)));
  assert.deepEqual(result, options.filter(p => p.name.startsWith('green ') && p.mode === 'dark'));
  assert.equal(filterThemes(options, { ...defaults, favoritesOnly: true }).length, 0);
  assert.equal(filterThemes(options, { ...defaults, query: 'no such palette' }).length, 0);
  assert.equal(filterThemes(options, defaults).length, options.length);
});

test('stored favorites tolerate corrupt data, stale IDs, duplicates, and unusual strings', () => {
  const ids = new Set(['light', 'dark', 'palette:catppuccin']);
  for (const raw of [null, '', '{', '{}', 'null', '42', '"dark"']) assert.equal(parseThemeFavorites(raw, ids).size, 0);
  assert.deepEqual([...parseThemeFavorites('["light","light",null,3,"missing","__proto__","palette:catppuccin"]', ids)], ['light', 'palette:catppuccin']);
});
