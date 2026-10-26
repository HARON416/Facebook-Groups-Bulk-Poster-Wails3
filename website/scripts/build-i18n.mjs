import { readFile, writeFile, mkdir } from 'node:fs/promises';
import { compileTranslation } from 'wuchale/compiler';
import { rolldown } from 'rolldown';
import { languages } from '../../frontend/src/locales/languages.js';

const root = new URL('../', import.meta.url);
const messages = JSON.parse(await readFile(new URL('locales/en.json', root), 'utf8'));
await mkdir(new URL('static/locales/', root), { recursive: true });
for (const { code } of languages) {
  const po = await readFile(new URL(`locales/${code}.po`, root), 'utf8');
  const entries = new Map([...po.matchAll(/^msgid (".*")\nmsgstr (".*")$/gm)].map(([, key, value]) => [JSON.parse(key), JSON.parse(value)]));
  const c = messages.map(source => {
    const value = entries.get(source);
    if (!value?.trim()) throw new Error(`${code}: missing ${source}`);
    const placeholders = text => [...text.matchAll(/\{\d+\}/g)].map(([value]) => value).sort().join(',');
    if (placeholders(source) !== placeholders(value)) throw new Error(`${code}: invalid placeholders in ${source}`);
    const tags = text => [...text.matchAll(/<\/?\d+>/g)].map(([value]) => value).sort().join(',');
    if (tags(source) !== tags(value)) throw new Error(`${code}: invalid markup in ${source}`);
    return compileTranslation(value);
  });
  await writeFile(new URL(`static/locales/${code}.json`, root), JSON.stringify({ c }) + '\n');
}
const bundle = await rolldown({ transform: { define: { 'import.meta.env.DEV': 'false' } }, input: new URL('src/i18n.ts', root).pathname });
await bundle.write({ file: new URL('static/js/i18n.js', root).pathname, format: 'iife', name: 'PostikaLocaleHelpers', minify: true });
await bundle.close();
console.log(`${languages.length} Wuchale website catalogs compiled`);
