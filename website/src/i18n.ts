import toRuntime, { type CatalogModule } from 'wuchale/runtime';
import englishCatalog from '../static/locales/en.json';
import { languages } from '../../frontend/src/locales/languages.js';
import messages from '../locales/en.json';
import { initializeLanguagePicker } from './language-picker';

const storageKey = 'postika:website:language';
const indices = new Map(messages.map((message, index) => [message, index]));
let runtime = toRuntime(englishCatalog as CatalogModule, 'en');
let generation = 0;
const textSources = new WeakMap<Text, { source: string; rendered: string }>();
const attributeSources = new WeakMap<Element, Map<string, { source: string; rendered: string }>>();
const catalogs = new Map<string, ReturnType<typeof toRuntime>>([['en', runtime]]);

export function matchLocale(value: string | null): string | null {
  if (!value) return null;
  try {
    const tag = Intl.getCanonicalLocales(value.trim().replaceAll('_', '-'))[0].toLowerCase();
    return languages.find(({ code }) => code.toLowerCase() === tag)?.code
      ?? languages.find(({ code }) => code.toLowerCase().split('-')[0] === tag.split('-')[0])?.code ?? null;
  } catch { return null; }
}
export function preferredLocale(saved: string | null): string {
  return matchLocale(saved) ?? 'en';
}
function readPreference() {
  try { return matchLocale(localStorage.getItem(storageKey)); } catch { return null; }
}
function t(source: string, ...args: (string | number)[]): string {
  const index = indices.get(source);
  return index === undefined ? source : runtime(index, args);
}
function translated(source: string): string {
  if (indices.has(source)) return t(source);
  // Download names come from release metadata; unknown future formats remain readable.
  if (source.startsWith('Download ')) return t('Download {0}', t(source.slice(9)));
  return source;
}
function apply() {
  document.querySelectorAll<HTMLElement>('[data-i18n-message]').forEach(element => {
    element.textContent = t(element.dataset.i18nMessage!);
  });
  document.querySelectorAll<HTMLElement>('[data-i18n-rich]').forEach(element => {
    const index = indices.get(element.dataset.i18nRich!);
    if (index === undefined) return;
    const parts = runtime.c(index);
    const nodes: Node[] = [];
    for (const part of parts) {
      if (typeof part === 'string') nodes.push(document.createTextNode(part));
      else if (Array.isArray(part)) {
        const accent = document.createElement('span');
        accent.className = 'accent-word';
        accent.textContent = part.slice(1).join('');
        nodes.push(accent);
      }
    }
    element.replaceChildren(...nodes);
  });
  document.querySelectorAll<HTMLElement>('[data-release-date]').forEach(element => {
    const value = element.dataset.releaseDate!;
    const date = new Date(`${value}T00:00:00Z`);
    if (!Number.isNaN(date.getTime())) element.textContent = new Intl.DateTimeFormat(runtime.l, { dateStyle: 'long', timeZone: 'UTC' }).format(date);
  });
  const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT);
  while (walker.nextNode()) {
    const node = walker.currentNode as Text;
    if (node.parentElement?.closest('script, style, option, [data-language-name], [data-no-translate], [data-i18n-rich], [data-i18n-message]')) continue;
    const existing = textSources.get(node);
    const source = existing?.rendered === node.data ? existing.source : node.data;
    const rendered = source.replace(/\S[\s\S]*\S|\S/, value => translated(value));
    textSources.set(node, { source, rendered });
    if (node.data !== rendered) node.data = rendered;
  }
  document.querySelectorAll('[alt], [aria-label], [placeholder], meta[name="description"], meta[property="og:title"], meta[property="og:description"], meta[name="twitter:title"], meta[name="twitter:description"]').forEach(element => {
    const attributes = attributeSources.get(element) ?? new Map();
    for (const name of ['alt', 'aria-label', 'placeholder', 'content']) {
      const current = element.getAttribute(name);
      if (!current) continue;
      const existing = attributes.get(name);
      const source = existing?.rendered === current ? existing.source : current;
      const rendered = translated(source);
      attributes.set(name, { source, rendered });
      if (current !== rendered) element.setAttribute(name, rendered);
    }
    attributeSources.set(element, attributes);
  });
  const title = document.querySelector('title');
  if (title) {
    const source = title.dataset.source ?? title.textContent ?? '';
    title.dataset.source = source;
    title.textContent = t(source);
  }
}
async function select(saved: string | null, persist = true) {
  const request = ++generation;
  const next = matchLocale(saved);
  const locale = preferredLocale(next);
  try {
    let catalog = catalogs.get(locale);
    if (!catalog) {
      const response = await fetch(`/static/locales/${locale}.json`);
      if (!response.ok) throw new Error('Catalog unavailable');
      const data = await response.json();
      if (!Array.isArray(data.c) || data.c.length !== messages.length) throw new Error('Invalid catalog');
      catalog = toRuntime(data, locale);
      catalogs.set(locale, catalog);
    }
    if (request !== generation) return;
    runtime = catalog;
    if (persist) {
      try {
        if (next) localStorage.setItem(storageKey, next);
        else localStorage.removeItem(storageKey);
      } catch { /* Language selection also works without storage. */ }
    }
    document.documentElement.lang = locale;
    document.documentElement.dir = ['ar', 'he'].includes(locale) ? 'rtl' : 'ltr';
    apply();
    document.dispatchEvent(new Event('postika:localechange'));
    apply();
  } catch {
    // Keep the current, usable page if a catalog cannot be fetched.
  }
}
window.PostikaI18n = { t, apply, select, get locale() { return runtime.l; } };
initializeLanguagePicker(window.PostikaI18n);
void select(readPreference(), false);
window.addEventListener('storage', event => {
  if (event.key === storageKey || event.key === null) void select(readPreference(), false);
});
