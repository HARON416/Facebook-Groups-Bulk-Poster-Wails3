import { test } from 'node:test';
import assert from 'node:assert/strict';
import { localeDirection, matchSupportedLocale, preferredLocale } from '../src/lib/locale-preferences.ts';

test('regional system languages select an available catalog', () => {
  for (const [preference, expected] of [
    ['sw-KE', 'sw'], ['en-KE', 'en'], ['fr-CA', 'fr'], ['ar-EG', 'ar'],
    ['pt-BR', 'pt-BR'], ['pt-PT', 'pt-BR'], ['zh-Hans-CN', 'zh-CN'],
    ['ZH-cn', 'zh-CN'], ['de_DE', 'de'], [' es-MX ', 'es'],
    ['he-IL', 'he'], ['iw-IL', 'he'],
  ]) assert.equal(matchSupportedLocale(preference), expected, preference);
});

test('Hebrew choices and direction apply to regional and legacy language tags', () => {
  assert.equal(preferredLocale('he', ['en-US']), 'he');
  assert.equal(preferredLocale(null, ['he-IL', 'en-US']), 'he');
  for (const locale of ['he', 'he-IL', 'iw-IL', 'ar', 'ar-EG']) {
    assert.equal(localeDirection(locale), 'rtl', locale);
  }
  for (const locale of ['en', 'sw', 'fr-CA']) assert.equal(localeDirection(locale), 'ltr', locale);
});

test('user language order is preserved and unsupported preferences are skipped', () => {
  assert.equal(preferredLocale(null, ['fi-FI', 'sw-KE', 'en-US']), 'sw');
  assert.equal(preferredLocale(null, ['en-KE', 'sw-KE']), 'en');
  assert.equal(preferredLocale(null, ['ja-JP', 'fr-FR']), 'ja');
});

test('a saved choice takes precedence, including explicitly selected English', () => {
  assert.equal(preferredLocale('ja', ['sw-KE']), 'ja');
  assert.equal(preferredLocale('en', ['ar-EG']), 'en');
  assert.equal(preferredLocale('pt-BR', ['fr-FR']), 'pt-BR');
});

test('missing or invalid saved preferences use system languages, then English', () => {
  for (const saved of [null, '', 'unsupported-locale']) {
    assert.equal(preferredLocale(saved, ['sw-KE']), 'sw');
    assert.equal(preferredLocale(saved, ['fi-FI', 'zu-ZA']), 'en');
    assert.equal(preferredLocale(saved, []), 'en');
  }
  for (const invalid of ['', 'not a locale', '___', 'unknown']) {
    assert.equal(matchSupportedLocale(invalid), null);
  }
});
