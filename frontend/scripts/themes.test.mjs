import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import vm from 'node:vm';
import { paletteTokens, contrast } from './generate-themes.mjs';

const palettes = JSON.parse(await readFile(new URL('../src/lib/theme-palettes.json', import.meta.url), 'utf8'));
const colorhunt = JSON.parse(await readFile(new URL('../theme-sources/colorhunt.json', import.meta.url), 'utf8'));
const newest = JSON.parse(await readFile(new URL('../theme-sources/colorhunt-new.json', import.meta.url), 'utf8'));
const mint = JSON.parse(await readFile(new URL('../theme-sources/colorhunt-mint.json', import.meta.url), 'utf8'));
const yellow = JSON.parse(await readFile(new URL('../theme-sources/colorhunt-yellow.json', import.meta.url), 'utf8'));
const orange = JSON.parse(await readFile(new URL('../theme-sources/colorhunt-orange.json', import.meta.url), 'utf8'));
const red = JSON.parse(await readFile(new URL('../theme-sources/colorhunt-red.json', import.meta.url), 'utf8'));
const black = JSON.parse(await readFile(new URL('../theme-sources/colorhunt-black.json', import.meta.url), 'utf8'));
const grey = JSON.parse(await readFile(new URL('../theme-sources/colorhunt-grey.json', import.meta.url), 'utf8'));
const white = JSON.parse(await readFile(new URL('../theme-sources/colorhunt-white.json', import.meta.url), 'utf8'));
const navy = JSON.parse(await readFile(new URL('../theme-sources/colorhunt-navy.json', import.meta.url), 'utf8'));
const purple = JSON.parse(await readFile(new URL('../theme-sources/colorhunt-purple.json', import.meta.url), 'utf8'));
const pink = JSON.parse(await readFile(new URL('../theme-sources/colorhunt-pink.json', import.meta.url), 'utf8'));
const beige = JSON.parse(await readFile(new URL('../theme-sources/colorhunt-beige.json', import.meta.url), 'utf8'));
const green = JSON.parse(await readFile(new URL('../theme-sources/colorhunt-green.json', import.meta.url), 'utf8'));
const teal = JSON.parse(await readFile(new URL('../theme-sources/colorhunt-teal.json', import.meta.url), 'utf8'));
const blue = JSON.parse(await readFile(new URL('../theme-sources/colorhunt-blue.json', import.meta.url), 'utf8'));
const peach = JSON.parse(await readFile(new URL('../theme-sources/colorhunt-peach.json', import.meta.url), 'utf8'));
const coolors = JSON.parse(await readFile(new URL('../theme-sources/coolors-trending.json', import.meta.url), 'utf8'));
const allColorhunt = [...colorhunt.palettes, ...newest.palettes, ...mint.palettes, ...yellow.palettes, ...orange.palettes, ...red.palettes, ...black.palettes, ...grey.palettes, ...white.palettes, ...navy.palettes, ...purple.palettes, ...pink.palettes, ...beige.palettes, ...green.palettes, ...teal.palettes, ...blue.palettes, ...peach.palettes];
const bootstrap = await readFile(new URL('../public/theme-bootstrap.js', import.meta.url), 'utf8');

test('all imported themes keep readable text, actions, and status indicators', () => {
  assert.equal(palettes.length, 187 + allColorhunt.length + coolors.palettes.length);
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

test('paginated mint feed adds ten distinct, readable desktop themes', () => {
  assert.equal(mint.palettes.length, 10);
  for (const mode of ['light', 'dark']) assert.equal(mint.palettes.filter(p => p.mode === mode).length, 5);
  for (const source of mint.palettes) {
    assert.equal(source.feed.tags, 'mint');
    assert.equal(source.feed.timeframe, 30);
    assert.ok(Number.isInteger(source.feed.step) && source.feed.step >= 0 && source.feed.step <= 4);
    assert.ok(source.name.startsWith('mint '));
    const palette = palettes.find(p => p.id === source.id);
    assert.ok(contrast(palette.bg, palette.text) >= 7, source.name);
  }
});

test('paginated yellow feed adds ten distinct, readable desktop themes', () => {
  assert.equal(yellow.palettes.length, 10);
  for (const mode of ['light', 'dark']) assert.equal(yellow.palettes.filter(p => p.mode === mode).length, 5);
  for (const source of yellow.palettes) {
    assert.equal(source.feed.tags, 'yellow');
    assert.equal(source.feed.timeframe, 30);
    assert.ok(Number.isInteger(source.feed.step) && source.feed.step >= 0 && source.feed.step <= 4);
    assert.ok(source.name.startsWith('yellow '));
    const palette = palettes.find(p => p.id === source.id);
    assert.ok(contrast(palette.bg, palette.text) >= 7, source.name);
  }
});

test('paginated orange feed adds ten distinct, readable desktop themes', () => {
  assert.equal(orange.palettes.length, 10);
  for (const mode of ['light', 'dark']) assert.equal(orange.palettes.filter(p => p.mode === mode).length, 5);
  for (const source of orange.palettes) {
    assert.equal(source.feed.tags, 'orange');
    assert.equal(source.feed.timeframe, 30);
    assert.ok(Number.isInteger(source.feed.step) && source.feed.step >= 0 && source.feed.step <= 4);
    assert.ok(source.name.startsWith('orange '));
    const palette = palettes.find(p => p.id === source.id);
    assert.ok(contrast(palette.bg, palette.text) >= 7, source.name);
  }
});

test('paginated red feed adds ten distinct, readable desktop themes', () => {
  assert.equal(red.palettes.length, 10);
  for (const mode of ['light', 'dark']) assert.equal(red.palettes.filter(p => p.mode === mode).length, 5);
  for (const source of red.palettes) {
    assert.equal(source.feed.tags, 'red');
    assert.equal(source.feed.timeframe, 30);
    assert.ok(Number.isInteger(source.feed.step) && source.feed.step >= 0 && source.feed.step <= 4);
    assert.ok(source.name.startsWith('red '));
    const palette = palettes.find(p => p.id === source.id);
    assert.ok(contrast(palette.bg, palette.text) >= 7, source.name);
  }
});

test('paginated black feed adds ten distinct, readable dark desktop themes', () => {
  assert.equal(black.palettes.length, 10);
  for (const source of black.palettes) {
    assert.equal(source.mode, 'dark');
    assert.equal(source.feed.tags, 'black');
    assert.equal(source.feed.timeframe, 30);
    assert.ok(Number.isInteger(source.feed.step) && source.feed.step >= 0 && source.feed.step <= 4);
    assert.ok(source.name.startsWith('black '));
    const palette = palettes.find(p => p.id === source.id);
    assert.ok(contrast(palette.bg, palette.text) >= 7, source.name);
    assert.ok(contrast(palette.bg, '#ffffff') >= 14, source.name);
  }
});

test('paginated grey feed adds ten distinct, readable desktop themes', () => {
  assert.equal(grey.palettes.length, 10);
  for (const mode of ['light', 'dark']) assert.equal(grey.palettes.filter(p => p.mode === mode).length, 5);
  for (const source of grey.palettes) {
    assert.equal(source.feed.tags, 'grey');
    assert.equal(source.feed.timeframe, 30);
    assert.ok(Number.isInteger(source.feed.step) && source.feed.step >= 0 && source.feed.step <= 4);
    assert.ok(source.name.startsWith('grey '));
    const palette = palettes.find(p => p.id === source.id);
    assert.ok(contrast(palette.bg, palette.text) >= 7, source.name);
    assert.equal(allColorhunt.filter(p => [...p.colors].sort().join('') === [...source.colors].sort().join('')).length, 1, source.name);
  }
});

test('paginated white feed adds ten distinct, readable light desktop themes', () => {
  assert.equal(white.palettes.length, 10);
  for (const source of white.palettes) {
    assert.equal(source.mode, 'light');
    assert.equal(source.feed.tags, 'white');
    assert.equal(source.feed.timeframe, 30);
    assert.ok(Number.isInteger(source.feed.step) && source.feed.step >= 0 && source.feed.step <= 4);
    assert.ok(source.name.startsWith('white '));
    const palette = palettes.find(p => p.id === source.id);
    assert.ok(contrast(palette.bg, palette.text) >= 7, source.name);
    assert.ok(contrast(palette.bg, '#000000') >= 17, source.name);
    assert.equal(allColorhunt.filter(p => [...p.colors].sort().join('') === [...source.colors].sort().join('')).length, 1, source.name);
  }
});

test('paginated navy feed adds ten distinct, readable dark desktop themes', () => {
  assert.equal(navy.palettes.length, 10);
  for (const source of navy.palettes) {
    assert.equal(source.mode, 'dark');
    assert.equal(source.feed.tags, 'navy');
    assert.equal(source.feed.timeframe, 30);
    assert.ok(Number.isInteger(source.feed.step) && source.feed.step >= 0 && source.feed.step <= 4);
    assert.ok(source.name.startsWith('navy '));
    const palette = palettes.find(p => p.id === source.id);
    assert.ok(contrast(palette.bg, palette.text) >= 7, source.name);
    assert.equal(allColorhunt.filter(p => [...p.colors].sort().join('') === [...source.colors].sort().join('')).length, 1, source.name);
  }
});

test('paginated purple feed adds ten distinct, readable desktop themes', () => {
  assert.equal(purple.palettes.length, 10);
  for (const mode of ['light', 'dark']) assert.equal(purple.palettes.filter(p => p.mode === mode).length, 5);
  for (const source of purple.palettes) {
    assert.equal(source.feed.tags, 'purple');
    assert.equal(source.feed.timeframe, 30);
    assert.ok(Number.isInteger(source.feed.step) && source.feed.step >= 0 && source.feed.step <= 4);
    assert.ok(source.name.startsWith('purple '));
    const palette = palettes.find(p => p.id === source.id);
    assert.ok(contrast(palette.bg, palette.text) >= 7, source.name);
    assert.equal(allColorhunt.filter(p => [...p.colors].sort().join('') === [...source.colors].sort().join('')).length, 1, source.name);
  }
});

test('paginated pink feed adds ten distinct, readable desktop themes', () => {
  assert.equal(pink.palettes.length, 10);
  for (const mode of ['light', 'dark']) assert.equal(pink.palettes.filter(p => p.mode === mode).length, 5);
  for (const source of pink.palettes) {
    assert.equal(source.feed.tags, 'pink');
    assert.equal(source.feed.timeframe, 30);
    assert.ok(Number.isInteger(source.feed.step) && source.feed.step >= 0 && source.feed.step <= 4);
    assert.ok(source.name.startsWith('pink '));
    const palette = palettes.find(p => p.id === source.id);
    assert.ok(contrast(palette.bg, palette.text) >= 7, source.name);
    assert.equal(allColorhunt.filter(p => [...p.colors].sort().join('') === [...source.colors].sort().join('')).length, 1, source.name);
  }
});

test('paginated beige feed adds ten distinct, readable desktop themes', () => {
  assert.equal(beige.palettes.length, 10);
  for (const mode of ['light', 'dark']) assert.equal(beige.palettes.filter(p => p.mode === mode).length, 5);
  for (const source of beige.palettes) {
    assert.equal(source.feed.tags, 'beige');
    assert.equal(source.feed.timeframe, 30);
    assert.ok(Number.isInteger(source.feed.step) && source.feed.step >= 0 && source.feed.step <= 4);
    assert.ok(source.name.startsWith('beige '));
    const palette = palettes.find(p => p.id === source.id);
    assert.ok(contrast(palette.bg, palette.text) >= 7, source.name);
    assert.equal(allColorhunt.filter(p => [...p.colors].sort().join('') === [...source.colors].sort().join('')).length, 1, source.name);
  }
});

test('paginated green feed adds ten distinct, readable desktop themes', () => {
  assert.equal(green.palettes.length, 10);
  for (const mode of ['light', 'dark']) assert.equal(green.palettes.filter(p => p.mode === mode).length, 5);
  for (const source of green.palettes) {
    assert.equal(source.feed.tags, 'green');
    assert.equal(source.feed.timeframe, 30);
    assert.ok(Number.isInteger(source.feed.step) && source.feed.step >= 0 && source.feed.step <= 4);
    assert.ok(source.name.startsWith('green '));
    const palette = palettes.find(p => p.id === source.id);
    assert.ok(contrast(palette.bg, palette.text) >= 7, source.name);
    assert.equal(allColorhunt.filter(p => [...p.colors].sort().join('') === [...source.colors].sort().join('')).length, 1, source.name);
  }
});

test('paginated teal feed adds ten distinct, readable desktop themes', () => {
  assert.equal(teal.palettes.length, 10);
  for (const mode of ['light', 'dark']) assert.equal(teal.palettes.filter(p => p.mode === mode).length, 5);
  for (const source of teal.palettes) {
    assert.equal(source.feed.tags, 'teal');
    assert.equal(source.feed.timeframe, 30);
    assert.ok(Number.isInteger(source.feed.step) && source.feed.step >= 0 && source.feed.step <= 4);
    assert.ok(source.name.startsWith('teal '));
    const palette = palettes.find(p => p.id === source.id);
    assert.ok(contrast(palette.bg, palette.text) >= 7, source.name);
    assert.equal(allColorhunt.filter(p => [...p.colors].sort().join('') === [...source.colors].sort().join('')).length, 1, source.name);
  }
});

test('paginated blue feed adds ten distinct, readable desktop themes', () => {
  assert.equal(blue.palettes.length, 10);
  for (const mode of ['light', 'dark']) assert.equal(blue.palettes.filter(p => p.mode === mode).length, 5);
  for (const source of blue.palettes) {
    assert.equal(source.feed.tags, 'blue');
    assert.equal(source.feed.timeframe, 30);
    assert.ok(Number.isInteger(source.feed.step) && source.feed.step >= 0 && source.feed.step <= 4);
    assert.ok(source.name.startsWith('blue '));
    const palette = palettes.find(p => p.id === source.id);
    assert.ok(contrast(palette.bg, palette.text) >= 7, source.name);
    assert.equal(allColorhunt.filter(p => [...p.colors].sort().join('') === [...source.colors].sort().join('')).length, 1, source.name);
  }
});

test('paginated blue-peach feed adds ten distinct, readable desktop themes', () => {
  assert.equal(peach.palettes.length, 10);
  for (const mode of ['light', 'dark']) assert.equal(peach.palettes.filter(p => p.mode === mode).length, 5);
  for (const source of peach.palettes) {
    assert.equal(source.feed.tags, 'blue-peach');
    assert.equal(source.feed.timeframe, 30);
    assert.ok(Number.isInteger(source.feed.step) && source.feed.step >= 0 && source.feed.step <= 2);
    assert.ok(source.name.startsWith('peach '));
    const palette = palettes.find(p => p.id === source.id);
    assert.ok(contrast(palette.bg, palette.text) >= 7, source.name);
    assert.equal(allColorhunt.filter(p => [...p.colors].sort().join('') === [...source.colors].sort().join('')).length, 1, source.name);
  }
});

test('Coolors trending adds sixteen distinct desktop themes using original source colors', () => {
  assert.equal(coolors.palettes.length, 16);
  assert.equal(new Set(coolors.palettes.map(p => p.sourcePaletteId)).size, 16);
  assert.equal(new Set(palettes.map(p => p.name)).size, palettes.length);
  for (const mode of ['light', 'dark']) assert.equal(coolors.palettes.filter(p => p.mode === mode).length, 8);
  const normalize = color => color.length === 4 ? '#' + [...color.slice(1)].map(c => c + c).join('') : color.toLowerCase();
  const colorKey = palette => ['bg', 'main', 'sub', 'text'].map(key => normalize(palette[key])).sort().join('');
  for (const source of coolors.palettes) {
    const palette = palettes.find(p => p.id === source.id);
    assert.ok(palette, source.name);
    assert.equal(palette.name, source.name);
    assert.equal(paletteTokens(palette).mode, source.mode);
    assert.equal(source.feed.sort, 'TRENDING');
    assert.equal(source.feed.search, '');
    assert.ok(Number.isInteger(source.feed.page) && source.feed.page >= 1 && source.feed.page <= 4);
    assert.equal(source.sourceUrl, `https://coolors.co/${source.colors.map(c => c.slice(1)).join('-')}`);
    for (const key of ['bg', 'main', 'sub', 'text']) {
      assert.equal(palette[key], source.uiColors[key]);
      assert.ok(source.colors.includes(palette[key]), `${source.name}: ${key} preserves source color`);
    }
    assert.ok(contrast(palette.bg, palette.text) >= 7, source.name);
    assert.equal(palettes.filter(p => colorKey(p) === colorKey(palette)).length, 1, source.name);
  }
});
