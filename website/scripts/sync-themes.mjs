import { readFile, writeFile, mkdir } from 'node:fs/promises';

const root = new URL('../', import.meta.url);
const desktop = new URL('../../frontend/src/', import.meta.url);
const palettes = JSON.parse(await readFile(new URL('lib/theme-palettes.json', desktop), 'utf8'));
const modes = JSON.parse(await readFile(new URL('lib/theme-modes.json', desktop), 'utf8'));
const options = [
  { id: 'light', name: 'Postika Light', bg: '#f8f9fa', main: '#087f75', sub: '#687a71', text: '#26332f', mode: 'light' },
  { id: 'dark', name: 'Postika Dark', bg: '#151c19', main: '#60d4bc', sub: '#a1b5a8', text: '#e1eae4', mode: 'dark' },
  ...palettes.map(option => ({ ...option, mode: modes[option.id] })),
];
for (const option of options) {
  if (!/^[a-z0-9:-]+$/.test(option.id) || !['light', 'dark'].includes(option.mode)) throw new Error(`Invalid theme: ${option.id}`);
}
const css = await Promise.all(['theme.css', 'palettes.css'].map(file => readFile(new URL(file, desktop), 'utf8')));
const swatches = options.map(option => `[data-theme-choice="${option.id}"] { --preview-bg: ${option.bg}; --preview-main: ${option.main}; --preview-sub: ${option.sub}; --preview-text: ${option.text}; }`).join('\n');
await mkdir(new URL('static/css/', root), { recursive: true });
await writeFile(new URL('static/css/themes.css', root), `/* Generated from the desktop themes by scripts/sync-themes.mjs. */\n${css.join('\n')}\n${swatches}\n`);
await writeFile(new URL('src/theme-data.ts', root), `// Generated from the desktop themes. Run npm run themes:sync to refresh.\nconst siteThemeOptions: SiteThemeOption[] = ${JSON.stringify(options, null, 2)};\n`);
console.log(`Synced ${options.length} desktop themes.`);
