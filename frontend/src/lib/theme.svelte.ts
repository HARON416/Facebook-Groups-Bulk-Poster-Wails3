import paletteModes from './theme-modes.json';

export type Theme = 'light' | 'dark';
type ThemeChoice = string;
export const themeStorageKey = 'postika:theme';
export const theme = $state({ mode: 'light' as Theme, selected: 'light' });
const validThemes = new Set(['light', 'dark', ...Object.keys(paletteModes)]);
const modes = paletteModes as Record<string, Theme>;

let preference: ThemeChoice | null = null;
let unsavedPreference = false;
let initialized = false;
let system: MediaQueryList;

function storedPreference(): ThemeChoice | null | undefined {
  try {
    const value = localStorage.getItem(themeStorageKey);
    return value && validThemes.has(value) ? value : null;
  } catch { return undefined; }
}

function apply(choice: ThemeChoice) {
  const mode = modes[choice] ?? choice as Theme;
  theme.selected = choice;
  theme.mode = mode;
  document.documentElement.dataset.theme = mode;
  if (modes[choice]) document.documentElement.dataset.palette = choice;
  else delete document.documentElement.dataset.palette;
  document.documentElement.style.colorScheme = mode;
}

export function selectTheme(choice: ThemeChoice) {
  if (!validThemes.has(choice)) return;
  preference = choice;
  apply(preference);
  try { localStorage.setItem(themeStorageKey, preference); unsavedPreference = false; }
  catch { unsavedPreference = true; }
}

export function initializeTheme() {
  if (initialized) return;
  initialized = true;
  system = window.matchMedia('(prefers-color-scheme: dark)');
  preference = storedPreference() ?? null;
  const systemMode = () => system.matches ? 'dark' : 'light';
  apply(preference ?? systemMode());

  const refresh = (fromStorage = false) => {
    // A blocked write must not discard the user's in-session choice on focus.
    if (unsavedPreference && !fromStorage) return;
    const saved = storedPreference();
    if (saved === undefined) return;
    preference = saved;
    unsavedPreference = false;
    apply(preference ?? systemMode());
  };
  window.addEventListener('storage', (event) => {
    if (event.key === themeStorageKey || event.key === null) refresh(true);
  });
  window.addEventListener('focus', () => refresh());
  window.addEventListener('postika:window-shown', () => refresh());
  system.addEventListener('change', () => { if (preference === null) apply(systemMode()); });
}
