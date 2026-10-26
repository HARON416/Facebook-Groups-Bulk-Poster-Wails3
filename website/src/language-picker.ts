import { languages } from '../../frontend/src/locales/languages.js';

// Match the desktop LanguageMenu/AccountDropdown without a framework dependency.
export function initializeLanguagePicker(api: Window['PostikaI18n']) {
  const root = document.querySelector<HTMLElement>('[data-language-picker]');
  const trigger = root?.querySelector<HTMLButtonElement>('[data-language-trigger]');
  const menu = root?.querySelector<HTMLElement>('[data-language-menu]');
  const search = root?.querySelector<HTMLInputElement>('#language-search');
  const list = root?.querySelector<HTMLElement>('#language-options');
  const heading = root?.querySelector<HTMLElement>('[data-language-heading]');
  const empty = root?.querySelector<HTMLElement>('[data-language-empty]');
  const error = root?.querySelector<HTMLElement>('[data-language-error]');
  if (!root || !trigger || !menu || !search || !list || !heading || !empty || !error) return;
  let matches = languages;
  let highlighted = api.locale;
  const rows = new Map<string, HTMLButtonElement>();
  for (const language of languages) {
    const row = document.createElement('button');
    row.type = 'button';
    row.id = `language-option-${language.code}`;
    row.dataset.language = language.code;
    row.setAttribute('role', 'option');
    row.tabIndex = -1;
    const flag = document.createElement('span');
    flag.className = 'language-flag';
    flag.setAttribute('aria-hidden', 'true');
    flag.textContent = language.flag;
    const name = document.createElement('span');
    name.className = 'language-name';
    name.lang = language.code;
    name.dir = 'auto';
    name.textContent = language.name;
    row.append(flag, name);
    const check = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
    check.setAttribute('viewBox', '0 0 20 20');
    check.setAttribute('aria-hidden', 'true');
    check.classList.add('language-check');
    const path = document.createElementNS('http://www.w3.org/2000/svg', 'path');
    path.setAttribute('d', 'm4 10 4 4 8-8');
    check.append(path);
    row.append(check);
    row.addEventListener('pointermove', () => { highlighted = language.code; updateHighlight(); });
    row.addEventListener('click', () => { void choose(language.code); });
    rows.set(language.code, row);
    list.append(row);
  }
  function updateHighlight(scroll = false) {
    rows.forEach((row, code) => row.classList.toggle('highlighted', code === highlighted));
    if (highlighted) search!.setAttribute('aria-activedescendant', `language-option-${highlighted}`);
    else search!.removeAttribute('aria-activedescendant');
    if (scroll) rows.get(highlighted)?.scrollIntoView({ block: 'nearest' });
  }
  function filter() {
    const query = search!.value.trim().toLocaleLowerCase();
    matches = languages.filter(language => language.name.toLocaleLowerCase().includes(query) || language.code.toLowerCase().includes(query));
    const visible = new Set(matches.map(language => language.code));
    rows.forEach((row, code) => { row.hidden = !visible.has(code); });
    if (!visible.has(highlighted)) highlighted = matches[0]?.code ?? '';
    heading!.textContent = api.t('All languages');
    empty!.hidden = matches.length > 0;
    updateHighlight();
  }
  function refresh() {
    const current = languages.find(language => language.code === api.locale)!;
    const flag = trigger!.querySelector<HTMLElement>('[data-language-flag]')!;
    const name = trigger!.querySelector<HTMLElement>('[data-language-name]')!;
    flag.textContent = current.flag;
    name.textContent = current.name;
    name.lang = current.code;
    trigger!.setAttribute('aria-label', api.t('Language: {0}', current.name));
    rows.forEach((row, code) => row.setAttribute('aria-selected', String(code === current.code)));
    filter();
  }
  function close(restoreFocus = false) {
    menu!.hidden = true;
    trigger!.setAttribute('aria-expanded', 'false');
    if (restoreFocus) trigger!.focus({ preventScroll: true });
  }
  function show(code = api.locale) {
    search!.value = '';
    highlighted = code;
    refresh();
    menu!.hidden = false;
    trigger!.setAttribute('aria-expanded', 'true');
    search!.focus({ preventScroll: true });
    updateHighlight(true);
  }
  async function choose(code: string) {
    close(true);
    error!.hidden = true;
    root!.setAttribute('aria-busy', 'true');
    trigger!.disabled = true;
    try {
      await api.select(code);
      if (api.locale !== code) {
        error!.textContent = api.t('Check your connection and try again.');
        error!.hidden = false;
      }
    } finally {
      trigger!.disabled = false;
      root!.removeAttribute('aria-busy');
      refresh();
      // Disabling a focused button can clear focus in some browsers.
      if (document.activeElement === document.body) trigger!.focus({ preventScroll: true });
    }
  }
  trigger.addEventListener('click', () => menu.hidden ? show() : close());
  trigger.addEventListener('keydown', event => {
    if (!['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(event.key)) return;
    event.preventDefault();
    show(event.key === 'Home' ? languages[0].code : event.key === 'End' ? languages.at(-1)!.code : api.locale);
  });
  search.addEventListener('input', filter);
  search.addEventListener('keydown', event => {
    if (event.key === 'Escape') { event.preventDefault(); event.stopPropagation(); close(true); return; }
    if (event.key === 'Tab') { close(true); return; }
    if (event.key === 'Enter') { event.preventDefault(); if (highlighted) void choose(highlighted); return; }
    if (!matches.length) return;
    const index = Math.max(0, matches.findIndex(language => language.code === highlighted));
    const next = !search.value && event.key === 'Home' ? 0 : !search.value && event.key === 'End' ? matches.length - 1
      : event.key === 'ArrowDown' ? (index + 1) % matches.length : event.key === 'ArrowUp' ? (index - 1 + matches.length) % matches.length : -1;
    if (next < 0) return;
    event.preventDefault();
    highlighted = matches[next].code;
    updateHighlight(true);
  });
  document.addEventListener('pointerdown', event => {
    if (event.target instanceof Node && !root.contains(event.target)) close();
  });
  root.addEventListener('focusout', event => { if (!root.contains(event.relatedTarget as Node | null)) close(); });
  document.addEventListener('postika:localechange', refresh);
  root.hidden = false;
  refresh();
}
