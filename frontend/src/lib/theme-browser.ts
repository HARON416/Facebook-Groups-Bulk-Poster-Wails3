export const colorFamilies = ['red', 'orange', 'peach', 'yellow', 'green', 'mint', 'teal', 'blue', 'navy', 'purple', 'pink', 'brown', 'beige', 'white', 'grey', 'black'] as const;
export type ColorFamily = typeof colorFamilies[number];
export type Appearance = 'all' | 'light' | 'dark';
export const themeFavoritesKey = 'postika:theme-favorites';

export interface BrowsableTheme {
  id: string;
  name: string;
  mode: 'light' | 'dark';
  colors: ColorFamily[];
}

function colorFamily(hex: string): ColorFamily {
  let value = hex.slice(1, 7);
  if (value.length === 3) value = [...value].map(c => c + c).join('');
  const [r, g, b] = [0, 2, 4].map(i => parseInt(value.slice(i, i + 2), 16) / 255);
  const max = Math.max(r, g, b), min = Math.min(r, g, b), delta = max - min;
  const lightness = (max + min) / 2;
  const saturation = delta === 0 ? 0 : delta / (1 - Math.abs(2 * lightness - 1));
  if (delta < .1 || saturation < .12) return lightness < .15 ? 'black' : lightness > .92 ? 'white' : 'grey';
  let hue = delta === 0 ? 0 : max === r ? ((g - b) / delta) % 6 : max === g ? (b - r) / delta + 2 : (r - g) / delta + 4;
  hue = (hue * 60 + 360) % 360;
  if (hue >= 10 && hue < 35 && lightness > .75 && delta < .45) return 'peach';
  if (hue >= 25 && hue < 65 && lightness > .72 && delta < .35) return 'beige';
  if (hue >= 10 && hue < 55 && lightness < .45) return 'brown';
  if (hue >= 205 && hue < 255 && lightness < .25) return 'navy';
  if (hue < 15 || hue >= 345) return 'red';
  if (hue < 45) return 'orange';
  if (hue < 70) return 'yellow';
  if (hue < 165) return lightness > .68 && hue >= 130 ? 'mint' : 'green';
  if (hue < 195) return 'teal';
  if (hue < 255) return 'blue';
  if (hue < 295) return 'purple';
  return 'pink';
}

// Background and accent are the visible identity of a theme. Neutral text alone
// must not make every dark theme match White (or every light theme match Black).
export function paletteColorFamilies(palette: { name: string; bg: string; main: string }): ColorFamily[] {
  const colors = new Set<ColorFamily>([colorFamily(palette.bg)]);
  const accent = colorFamily(palette.main);
  if (!['black', 'white', 'grey'].includes(accent)) colors.add(accent);
  const named = palette.name.split(' ')[0];
  if (colorFamilies.some(color => color === named)) colors.add(named as ColorFamily);
  return [...colors];
}

export function filterThemes<T extends BrowsableTheme>(options: readonly T[], filters: {
  query: string; appearance: Appearance; color: ColorFamily | 'all'; favoritesOnly: boolean; favorites: ReadonlySet<string>;
}): T[] {
  const query = filters.query.trim().toLocaleLowerCase();
  return options.filter(option => option.name.toLocaleLowerCase().includes(query)
    && (filters.appearance === 'all' || option.mode === filters.appearance)
    && (filters.color === 'all' || option.colors.includes(filters.color))
    && (!filters.favoritesOnly || filters.favorites.has(option.id)));
}

export function parseThemeFavorites(raw: string | null, validIds: ReadonlySet<string>): Set<string> {
  try {
    const values: unknown = JSON.parse(raw ?? '[]');
    return new Set(Array.isArray(values) ? values.filter((id): id is string => typeof id === 'string' && validIds.has(id)) : []);
  } catch { return new Set(); }
}
