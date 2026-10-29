// @ts-check
import { defineConfig } from 'wuchale';
import { adapter as svelte } from '@wuchale/svelte';
import { languages } from './src/locales/languages.js';

const keyboardKeys = new Set(['Enter', 'Escape', 'Tab', 'Home', 'End', 'ArrowUp', 'ArrowDown', 'ArrowLeft', 'ArrowRight', 'PageUp', 'PageDown', 'Backspace']);

export default defineConfig({
  locales: ['en', ...languages.filter(({ code }) => code !== 'en').map(({ code }) => code)],
  adapters: {
    main: svelte({
      sourceLocale: 'en',
      loader: 'svelte',
      files: { include: ['src/**/*.svelte'], ignore: ['src/components/EditorIcon.svelte', 'src/components/PhosphorIcon.svelte'] },
      heuristic(text) {
        if (typeof text.body === 'string' && keyboardKeys.has(text.body)) return false;
        const property = text.path.findLast((scope) => scope.type === 'property');
        if (property && ['title', 'description', 'detail', 'label'].includes(property.name)) return 'message';
        // These compare or rewrite English diagnostic messages from Go.
        if (text.path.some((scope) => scope.type === 'call' && /\.(startsWith|replace|replaceAll)$/.test(scope.name))) return false;
      },
    }),
  },
});
