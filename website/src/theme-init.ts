// Loaded before stylesheets so the saved palette is present on first paint.
(() => {
  const storageKey = 'postika:website:theme';
  const options = new Map(siteThemeOptions.map(option => [option.id, option]));
  const system = window.matchMedia('(prefers-color-scheme: dark)');
  let preference: string | null = null;
  let unsaved = false;
  function stored(): string | null | undefined {
    try {
      const value = localStorage.getItem(storageKey);
      return value && options.has(value) ? value : null;
    } catch { return undefined; }
  }
  function apply() {
    const selected = preference ?? (system.matches ? 'dark' : 'light');
    const option = options.get(selected)!;
    document.documentElement.dataset.theme = option.mode;
    if (selected === 'light' || selected === 'dark') delete document.documentElement.dataset.palette;
    else document.documentElement.dataset.palette = selected;
    document.querySelector<HTMLMetaElement>('meta[name="theme-color"]')?.setAttribute('content', option.bg);
    window.PostikaTheme.selected = selected;
    window.PostikaTheme.preference = preference;
    document.dispatchEvent(new Event('postika:themechange'));
  }
  window.PostikaTheme = {
    options: siteThemeOptions, selected: 'light', preference: null,
    select(id) {
      if (id !== 'system' && !options.has(id)) return;
      preference = id === 'system' ? null : id;
      try {
        if (preference === null) localStorage.removeItem(storageKey);
        else localStorage.setItem(storageKey, preference);
        unsaved = false;
      } catch { unsaved = true; }
      apply();
    },
  };
  preference = stored() ?? null;
  apply();
  function refresh(fromStorage = false) {
    if (unsaved && !fromStorage) return;
    const value = stored();
    if (value === undefined) return;
    preference = value;
    unsaved = false;
    apply();
  }
  window.addEventListener('storage', event => {
    if (event.key === storageKey || event.key === null) refresh(true);
  });
  window.addEventListener('focus', () => refresh());
  system.addEventListener('change', () => { if (preference === null) apply(); });
})();
