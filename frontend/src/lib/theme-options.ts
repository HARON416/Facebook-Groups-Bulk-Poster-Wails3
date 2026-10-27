import palettes from './theme-palettes.json';
import modes from './theme-modes.json';
import { paletteColorFamilies } from './theme-browser';

// Palette names are identifiers supplied with the original color previews.
export const themeOptions = [
  { id: 'light', name: 'postika light', bg: '#f8f9fa', main: '#087f75', sub: '#687a71', text: '#26332f' },
  { id: 'dark', name: 'postika dark', bg: '#151c19', main: '#60d4bc', sub: '#a1b5a8', text: '#e1eae4' },
  ...palettes,
].map(option => ({ ...option,
  mode: (option.id === 'light' || option.id === 'dark' ? option.id : (modes as Record<string, string>)[option.id]) as 'light' | 'dark',
  colors: paletteColorFamilies(option),
}));
