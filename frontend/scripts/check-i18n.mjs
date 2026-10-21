import { readFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import { pofile } from 'wuchale';
import { languages } from '../src/locales/languages.js';

// Every language offered in the interface must have a complete catalog.
// Never count an English fallback as a translated message.
const root = fileURLToPath(new URL('../', import.meta.url));
const storage = await pofile()({
  root,
  localesDir: `${root}src/locales`,
  locales: languages.map(({ code }) => code),
  sourceLocale: 'en',
  fs: {
    // The storage factory prepares its directory; this audit only reads it.
    async mkdir() {},
    async read(path) {
      try { return await readFile(path, 'utf8'); }
      catch (error) { if (error.code === 'ENOENT') return null; throw error; }
    },
  },
});
const items = (await storage.load()).filter((item) => item.references.length > 0);
let failed = false;
for (const { code } of languages) {
  const missing = items.filter((item) => {
    const value = item.translations.get(code);
    const source = item.translations.get('en');
    const flags = item.attribs.additionals?.get(code)?.flags;
    // Short shared terms (Gemini, API, Minimum, etc.) can be identical.
    const unchangedCopy = code !== 'en' && typeof value === 'string' && value.length > 20 && value === source;
    return !value || (Array.isArray(value) ? value.some((form) => !form.trim()) : !value.trim()) || flags?.fuzzy || unchangedCopy;
  });
  console.log(`${code}: ${items.length - missing.length}/${items.length} translated`);
  if (missing.length) {
    failed = true;
    console.error(`${code}: missing translations:\n${missing.map((item) => `  ${JSON.stringify(item.translations.get('en'))}`).join('\n')}`);
  }
}
if (failed) process.exitCode = 1;
