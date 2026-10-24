import { loadLocale } from 'wuchale/load-utils';
import '../locales/main.loader.svelte.js';
import { languages } from '../locales/languages.js';
import { localeDirection, matchSupportedLocale, preferredLocale } from './locale-preferences';

export const localeStorageKey = 'postika:language';
export const i18n = $state({ locale: 'en', changing: false });
let pending: Promise<void> = Promise.resolve();

export function getLocale() { return i18n.locale; }
export function supportedLocale(value: string | null) {
  return languages.some(({ code }) => code === value) ? value! : 'en';
}
function storedLocale() {
  try { return matchSupportedLocale(localStorage.getItem(localeStorageKey)); }
  catch { return null; }
}
function initialLocale() {
  return preferredLocale(storedLocale(), [...(navigator.languages ?? []), navigator.language]);
}

// Serialize loads: an older, slower catalog must never overwrite a newer choice.
export function setLocale(value: string, persist = true): Promise<void> {
  const locale = supportedLocale(value);
  i18n.changing = true;
  const next = pending.catch(() => {}).then(async () => {
    await loadLocale(locale);
    i18n.locale = locale;
    document.documentElement.lang = locale;
    document.documentElement.dir = localeDirection(locale);
    if (persist) {
      try { localStorage.setItem(localeStorageKey, locale); }
      catch { /* The selected language still works when browser storage is unavailable. */ }
    }
  });
  pending = next;
  return next.finally(() => { if (pending === next) i18n.changing = false; });
}

export async function initializeLocale() {
  // Automatic choices remain unsaved, so only an explicit selection overrides
  // the system's preferred languages on future launches.
  try { await setLocale(initialLocale(), false); }
  catch { await setLocale('en', false); }
  window.addEventListener('storage', (event) => {
    if (event.key === localeStorageKey || event.key === null) {
      void setLocale(initialLocale(), false).catch(() => {});
    }
  });
  // Retained log windows also catch up when shown or focused again.
  const refresh = () => {
    const locale = storedLocale();
    if (locale && locale !== i18n.locale) void setLocale(locale, false).catch(() => {});
  };
  window.addEventListener('focus', refresh);
  window.addEventListener('postika:window-shown', refresh);
}
