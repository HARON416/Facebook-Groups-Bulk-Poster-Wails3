import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import vm from 'node:vm';
import { paletteTokens, contrast } from './generate-themes.mjs';

const palettes = JSON.parse(await readFile(new URL('../src/lib/theme-palettes.json', import.meta.url), 'utf8'));
const bootstrap = await readFile(new URL('../public/theme-bootstrap.js', import.meta.url), 'utf8');

test('all imported themes keep readable text, actions, and status indicators', () => {
  assert.equal(palettes.length, 187);
  assert.equal(new Set(palettes.map(p => p.id)).size, 187);
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
