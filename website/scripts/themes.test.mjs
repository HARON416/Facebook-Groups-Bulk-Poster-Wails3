import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';
import { runInNewContext } from 'node:vm';

const script = await readFile(new URL('../static/js/theme-init.js', import.meta.url), 'utf8');
function boot({ saved = null, dark = false, blockedWrite = false, blockedRead = false } = {}) {
  const values = new Map(saved === null ? [] : [['postika:website:theme', saved]]);
  const listeners = new Map();
  let systemListener;
  const system = { matches: dark, addEventListener(_, listener) { systemListener = listener; } };
  const root = { dataset: {} };
  const window = { matchMedia: () => system, addEventListener: (name, listener) => listeners.set(name, listener) };
  const localStorage = {
    getItem(key) { if (blockedRead) throw new Error('Storage blocked'); return values.get(key) ?? null; },
    setItem(key, value) { if (blockedWrite) throw new Error('Storage blocked'); values.set(key, value); },
    removeItem(key) { if (blockedWrite) throw new Error('Storage blocked'); values.delete(key); },
  };
  runInNewContext(script, {
    window, localStorage, Event: class Event {},
    document: { documentElement: root, querySelector: () => null, dispatchEvent() {} },
  });
  return {
    theme: window.PostikaTheme, root, values,
    focus: () => listeners.get('focus')(),
    storage: key => listeners.get('storage')({ key }),
    system(dark) { system.matches = dark; systemListener(); },
  };
}

test('all desktop themes are present and saved palettes apply before page content', async () => {
  const app = boot({ saved: 'palette:catppuccin' });
  const desktop = JSON.parse(await readFile(new URL('../../frontend/src/lib/theme-palettes.json', import.meta.url), 'utf8'));
  assert.equal(app.theme.options.length, desktop.length + 2);
  assert.equal(app.theme.options.length, 189);
  assert.equal(app.root.dataset.palette, 'palette:catppuccin');
  assert.equal(app.root.dataset.theme, 'dark');
  for (const option of desktop) assert.ok(app.theme.options.some(theme => theme.id === option.id));
});

test('system mode follows the OS until an explicit choice, and reset restores it', () => {
  const app = boot({ dark: true });
  assert.equal(app.theme.selected, 'dark');
  app.system(false);
  assert.equal(app.theme.selected, 'light');
  app.theme.select('dark');
  assert.equal(app.values.get('postika:website:theme'), 'dark');
  app.system(false);
  assert.equal(app.theme.selected, 'dark');
  app.theme.select('system');
  assert.equal(app.theme.selected, 'light');
  assert.equal(app.values.has('postika:website:theme'), false);
});

test('invalid storage falls back to the OS and invalid selections are ignored', () => {
  const app = boot({ saved: 'not-a-theme', dark: true });
  assert.equal(app.theme.selected, 'dark');
  assert.equal(app.theme.preference, null);
  app.theme.select('not-a-theme');
  assert.equal(app.theme.selected, 'dark');
  assert.equal(boot({ blockedRead: true }).theme.selected, 'light');
});

test('blocked storage writes retain the in-session choice on focus', () => {
  const app = boot({ blockedWrite: true });
  app.theme.select('dark');
  app.focus();
  assert.equal(app.theme.selected, 'dark');
  assert.equal(app.root.dataset.theme, 'dark');
});

test('palette changes and preference removal synchronize across tabs', () => {
  const app = boot({ saved: 'dark' });
  app.values.set('postika:website:theme', 'palette:9009');
  app.storage('postika:website:theme');
  assert.equal(app.root.dataset.palette, 'palette:9009');
  assert.equal(app.root.dataset.theme, 'light');
  app.values.clear();
  app.storage(null);
  assert.equal(app.theme.preference, null);
  assert.equal(app.root.dataset.palette, undefined);
});
