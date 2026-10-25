type DesktopOS = "windows" | "macos" | "linux";

// Mirror the desktop picker while keeping the public site framework-free.
(() => {
  const theme = window.PostikaTheme;
  const trigger = document.querySelector<HTMLButtonElement>('[data-theme-picker]');
  const dialog = document.querySelector<HTMLDialogElement>('#theme-picker');
  const search = document.querySelector<HTMLInputElement>('#theme-search');
  const list = document.querySelector<HTMLElement>('#theme-options');
  const count = document.querySelector<HTMLElement>('#theme-count');
  if (!theme || !trigger || !dialog || !search || !list || !count) return;
  const choices = [{ id: 'system', name: 'System default' }, ...theme.options];
  let matches = choices;
  let highlighted = theme.preference ?? 'system';
  const rows = new Map<string, HTMLButtonElement>();
  for (const choice of choices) {
    const row = document.createElement('button');
    row.type = 'button';
    row.id = `theme-${choice.id}`;
    row.dataset.themeChoice = choice.id;
    row.setAttribute('role', 'option');
    row.tabIndex = -1;
    const name = document.createElement('span');
    name.className = 'theme-name';
    name.textContent = choice.name;
    row.append(name);
    const selected = document.createElement('span');
    selected.className = 'theme-selected';
    selected.textContent = 'Selected';
    selected.setAttribute('aria-hidden', 'true');
    row.append(selected);
    if (choice.id !== 'system') {
      const preview = document.createElement('span');
      preview.className = 'theme-preview';
      preview.setAttribute('aria-hidden', 'true');
      preview.append(document.createElement('i'), document.createElement('i'), document.createElement('i'));
      row.append(preview);
    }
    row.addEventListener('pointermove', () => { highlighted = choice.id; updateHighlight(); });
    row.addEventListener('click', () => { theme.select(choice.id); dialog.close(); });
    rows.set(choice.id, row);
    list.append(row);
  }
  function updateHighlight(scroll = false) {
    rows.forEach((row, id) => row.classList.toggle('highlighted', id === highlighted));
    if (highlighted) search!.setAttribute('aria-activedescendant', `theme-${highlighted}`);
    else search!.removeAttribute('aria-activedescendant');
    if (scroll) rows.get(highlighted)?.scrollIntoView({ block: 'nearest', behavior: 'instant' });
  }
  function refreshSelection() {
    const selected = theme.preference ?? 'system';
    rows.forEach((row, id) => row.setAttribute('aria-selected', String(id === selected)));
    const name = theme.options.find(option => option.id === theme.selected)?.name ?? 'Postika Light';
    const label = trigger!.querySelector<HTMLElement>('[data-current-theme]');
    if (label) label.textContent = name;
    trigger!.title = theme.preference === null ? `System default (${name})` : name;
    trigger!.setAttribute('aria-label', `Color themes: ${theme.preference === null ? `System default (${name})` : name}`);
  }
  function filter() {
    const query = search!.value.trim().toLocaleLowerCase();
    matches = choices.filter(option => option.name.toLocaleLowerCase().includes(query));
    const visible = new Set(matches.map(option => option.id));
    rows.forEach((row, id) => { row.hidden = !visible.has(id); });
    if (!visible.has(highlighted)) highlighted = matches[0]?.id ?? '';
    count!.textContent = !query ? `${theme.options.length} themes and system default`
      : matches.length ? `${matches.length} shown` : 'No themes found. Try another name.';
    updateHighlight();
  }
  trigger.hidden = false;
  trigger.addEventListener('click', () => {
    search.value = '';
    highlighted = theme.preference ?? 'system';
    filter();
    refreshSelection();
    dialog.showModal();
    trigger.setAttribute('aria-expanded', 'true');
    search.focus();
    updateHighlight(true);
  });
  search.addEventListener('input', filter);
  search.addEventListener('keydown', event => {
    if (event.key === 'Escape') {
      event.preventDefault();
      event.stopPropagation();
      dialog.close();
      return;
    }
    if (event.key === 'Enter') {
      event.preventDefault();
      if (highlighted) { theme.select(highlighted); dialog.close(); }
      return;
    }
    if (!matches.length) return;
    const index = matches.findIndex(option => option.id === highlighted);
    const next = event.key === 'ArrowDown' ? (index + 1) % matches.length
      : event.key === 'ArrowUp' ? (index - 1 + matches.length) % matches.length
      : !search.value && event.key === 'Home' ? 0
      : !search.value && event.key === 'End' ? matches.length - 1 : -1;
    if (next < 0) return;
    event.preventDefault();
    highlighted = matches[next].id;
    updateHighlight(true);
  });
  dialog.querySelector<HTMLButtonElement>('[data-close-themes]')?.addEventListener('click', () => dialog.close());
  dialog.addEventListener('close', () => { trigger.setAttribute('aria-expanded', 'false'); trigger.focus(); });
  dialog.addEventListener('click', event => {
    if (event.target !== dialog) return;
    const bounds = dialog.getBoundingClientRect();
    if (event.clientX < bounds.left || event.clientX > bounds.right || event.clientY < bounds.top || event.clientY > bounds.bottom) dialog.close();
  });
  document.addEventListener('postika:themechange', refreshSelection);
  refreshSelection();
})();

// Screenshot links still open the original asset when scripting is unavailable.
(() => {
  const dialog = document.querySelector<HTMLDialogElement>('#screenshot-modal');
  const stage = dialog?.querySelector<HTMLElement>('[data-screenshot-stage]');
  const status = dialog?.querySelector<HTMLElement>('[data-screenshot-status]');
  const close = dialog?.querySelector<HTMLButtonElement>('[data-close-screenshot]');
  if (!dialog || !stage || !status || !close) return;
  const image = new Image();
  image.dataset.fullScreenshot = '';
  let opener: HTMLAnchorElement | null = null;
  let generation = 0;
  document.querySelectorAll<HTMLAnchorElement>('[data-screenshot]').forEach(link => {
    link.setAttribute('aria-haspopup', 'dialog');
    link.setAttribute('aria-controls', 'screenshot-modal');
    link.addEventListener('click', event => {
      if (event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
      const thumbnail = link.querySelector<HTMLImageElement>('img');
      if (!thumbnail) return;
      event.preventDefault();
      opener = link;
      const current = ++generation;
      dialog.setAttribute('aria-label', thumbnail.alt || 'Screenshot preview');
      image.hidden = true;
      image.alt = thumbnail.alt;
      image.src = link.href;
      stage.prepend(image);
      status.textContent = 'Loading screenshot…';
      status.hidden = false;
      document.documentElement.classList.add('screenshot-open');
      dialog.showModal();
      close.focus();
      void image.decode().then(() => {
        if (current !== generation || !dialog.open) return;
        image.hidden = false;
        status.hidden = true;
      }).catch(() => {
        if (current !== generation || !dialog.open) return;
        status.textContent = 'Couldn’t load this screenshot. Close it and try again.';
      });
    });
  });
  close.addEventListener('click', () => dialog.close());
  dialog.addEventListener('close', () => {
    generation++;
    image.remove();
    document.documentElement.classList.remove('screenshot-open');
    opener?.focus({ preventScroll: true });
  });
  dialog.addEventListener('click', event => {
    if (event.target !== dialog) return;
    const bounds = dialog.getBoundingClientRect();
    if (event.clientX < bounds.left || event.clientX > bounds.right || event.clientY < bounds.top || event.clientY > bounds.bottom) dialog.close();
  });
})();

function detectOS(): DesktopOS | null {
  const platform = navigator.userAgent.toLowerCase();
  if (/android|iphone|ipad|ipod/.test(platform) || (platform.includes("macintosh") && navigator.maxTouchPoints > 1)) return null;
  if (platform.includes("windows")) return "windows";
  if (platform.includes("mac")) return "macos";
  if (platform.includes("linux")) return "linux";
  return null;
}
const os = detectOS();
if (os) {
  const names = { windows: "Windows", macos: "macOS", linux: "Linux" };
  document.querySelectorAll<HTMLElement>("[data-download-label]").forEach(label => {
    label.textContent = `Download for ${names[os]}`;
  });
  document.querySelectorAll<HTMLElement>(`[data-platform="${os}"]`).forEach(card => {
    card.dataset.recommended = "true";
    const badge = card.querySelector<HTMLElement>(".recommended");
    if (badge) badge.hidden = false;
  });
}
// Native full-page content remains usable if JavaScript or HTMX is unavailable.
document.body.addEventListener("htmx:afterSwap", () => {
  const feedback = document.querySelector<HTMLElement>(".release-feedback");
  if (feedback) feedback.textContent = "Release notes are up to date.";
});
document.body.addEventListener("htmx:responseError", () => {
  const feedback = document.querySelector<HTMLElement>(".release-feedback");
  if (feedback) feedback.textContent = "Couldn’t refresh release notes. Please try again.";
});
document.body.addEventListener("htmx:sendError", () => {
  const feedback = document.querySelector<HTMLElement>(".release-feedback");
  if (feedback) feedback.textContent = "Check your connection and try again.";
});
