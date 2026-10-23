import { languages } from '../locales/languages.js';

export function localeDirection(locale: string): 'rtl' | 'ltr' {
  const language = locale.toLowerCase().split(/[-_]/)[0];
  return language === 'ar' || language === 'he' || language === 'iw' ? 'rtl' : 'ltr';
}

// Webviews usually expose regional tags (fr-CA, sw-KE), while most of our
// catalogs use a language tag. Match the available regional catalog first.
export function matchSupportedLocale(value: string | null): string | null {
  if (!value) return null;
  let tag: string;
  try { tag = Intl.getCanonicalLocales(value.trim().replaceAll('_', '-'))[0].toLowerCase(); }
  catch { return null; }
  const exact = languages.find(({ code }) => code.toLowerCase() === tag);
  if (exact) return exact.code;
  const language = tag.split('-')[0];
  return languages.find(({ code }) => code.toLowerCase().split('-')[0] === language)?.code ?? null;
}

export function preferredLocale(saved: string | null, preferences: readonly string[]): string {
  const selected = matchSupportedLocale(saved);
  if (selected) return selected;
  for (const preference of preferences) {
    const locale = matchSupportedLocale(preference);
    if (locale) return locale;
  }
  return 'en';
}
