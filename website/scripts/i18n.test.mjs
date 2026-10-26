import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { runInNewContext } from 'node:vm';
import test from 'node:test';
import { languages } from '../../frontend/src/locales/languages.js';

const script = await readFile(new URL('../static/js/i18n.js', import.meta.url), 'utf8');
const messages = JSON.parse(await readFile(new URL('../locales/en.json', import.meta.url)));
const catalogs = new Map(await Promise.all(languages.map(async ({ code }) => [code, JSON.parse(await readFile(new URL(`../static/locales/${code}.json`, import.meta.url)))])));
function boot({ saved = null, preferences = ['en'], blockedStorage = false, fetchCatalog } = {}) {
  const values = new Map(saved ? [['postika:website:language', saved]] : []);
  const listeners = new Map();
  const root = {};
  const window = { addEventListener: (name, callback) => listeners.set(name, callback) };
  const context = {
    window, navigator: { languages: preferences, language: preferences[0] }, Intl, Event,
    NodeFilter: { SHOW_TEXT: 4 },
    localStorage: {
      getItem: key => { if (blockedStorage) throw Error('Blocked'); return values.get(key) ?? null; },
      setItem: (key, value) => { if (blockedStorage) throw Error('Blocked'); values.set(key, value); },
      removeItem: key => { if (blockedStorage) throw Error('Blocked'); values.delete(key); },
    },
    document: {
      body: {}, documentElement: root,
      querySelector: () => null, querySelectorAll: () => [],
      createTreeWalker: () => ({ nextNode: () => false }), dispatchEvent() {},
    },
    fetch: fetchCatalog ?? (async url => ({ ok: true, json: async () => catalogs.get(url.split('/').at(-1).replace('.json', '')) })),
  };
  runInNewContext(script, context);
  return { api: window.PostikaI18n, helpers: context.PostikaLocaleHelpers, values, root, listeners };
}
test('website supports every desktop language with complete compiled catalogs', () => {
  assert.equal(languages.length, 33);
  for (const { code } of languages) {
    const catalog = catalogs.get(code);
    assert.equal(catalog.c.length, messages.length, code);
    assert.ok(catalog.c.every(value => value !== '' && value != null), code);
  }
});
test('locale matching handles regional preferences, aliases and invalid values', () => {
  const { helpers } = boot();
  for (const [input, expected] of [['fr-CA','fr'], ['sw_KE','sw'], ['pt-PT','pt-BR'], ['zh-TW','zh-CN'], ['iw-IL','he'], ['nonsense!',null]]) {
    assert.equal(helpers.matchLocale(input), expected);
  }
  assert.equal(helpers.preferredLocale(null), 'en');
  assert.equal(helpers.preferredLocale('sw'), 'sw');
});
test('English is default regardless of browser language; explicit choices persist and RTL sets direction', async () => {
  const app = boot({ preferences: ['fr-CA'] });
  await app.api.select(null, false);
  assert.equal(app.root.lang,'en');
  assert.equal(app.values.size,0);
  await app.api.select('ar');
  assert.equal(app.root.dir,'rtl');
  assert.equal(app.values.get('postika:website:language'),'ar');
  await app.api.select('he');
  assert.equal(app.root.dir,'rtl');
  await app.api.select(null);
  assert.equal(app.root.lang,'en');
  assert.equal(app.root.dir,'ltr');
  assert.equal(app.values.size,0);
});
test('blocked storage still allows language selection and Wuchale interpolation', async () => {
  const app = boot({ blockedStorage:true });
  await app.api.select('sw');
  assert.equal(app.root.lang,'sw');
  assert.equal(app.api.t('Download for {0}','Windows'),'Pakua kwa Windows');
});
test('a failed catalog keeps the existing language usable', async () => {
  const app = boot({ fetchCatalog: async () => { throw Error('Offline'); } });
  await app.api.select('en');
  await app.api.select('de');
  assert.equal(app.api.locale,'en');
  assert.equal(app.api.t('Download'),'Download');
});
test('a slow catalog cannot overwrite the latest selection', async () => {
  let finish;
  const app = boot({ fetchCatalog: url => url.includes('/ar.') ? new Promise(resolve => { finish = () => resolve({ ok:true,json:async()=>catalogs.get('ar') }); }) : Promise.resolve({ ok:true,json:async()=>catalogs.get('de') }) });
  const first = app.api.select('ar');
  await app.api.select('de');
  finish();
  await first;
  assert.equal(app.api.locale,'de');
  assert.equal(app.root.dir,'ltr');
});

test('English fallback also compiles placeholders', () => {
  const app = boot();
  assert.equal(app.api.t('Download for {0}', 'Windows'), 'Download for Windows');
});

test('language changes in another tab follow the stored choice', async () => {
  const app = boot();
  app.values.set('postika:website:language', 'ar');
  app.listeners.get('storage')({ key: 'postika:website:language' });
  await new Promise(setImmediate);
  assert.equal(app.api.locale, 'ar');
  app.values.delete('postika:website:language');
  app.listeners.get('storage')({ key: 'postika:website:language' });
  await new Promise(setImmediate);
  assert.equal(app.api.locale, 'en');
});
