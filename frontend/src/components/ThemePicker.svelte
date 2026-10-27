<script lang="ts">
  import { onMount, tick } from 'svelte';
  import { theme, selectTheme } from '../lib/theme.svelte';
  import { themeOptions } from '../lib/theme-options';
  import { filterThemes, parseThemeFavorites, themeFavoritesKey, type Appearance, type ColorFamily } from '../lib/theme-browser';

  let dialog: HTMLDialogElement;
  let search = $state<HTMLInputElement>();
  let list = $state<HTMLUListElement>();
  let trigger: HTMLButtonElement;
  let query = $state('');
  let color = $state<ColorFamily | 'all'>('all');
  let appearance = $state<Appearance>('all');
  let favoritesOnly = $state(false);
  let favorites = $state(new Set<string>());
  let highlighted = $state('');
  let expanded = $state(false);
  let unsavedFavorites = false;
  const validIds = new Set(themeOptions.map(option => option.id));
  const matches = $derived(filterThemes(themeOptions, { query, color, appearance, favoritesOnly, favorites }));
  const filtered = $derived(query !== '' || color !== 'all' || appearance !== 'all' || favoritesOnly);
  const colorOptions = $derived([
    { value: 'all', label: 'All colors' }, { value: 'red', label: 'Red' },
    { value: 'orange', label: 'Orange' }, { value: 'peach', label: 'Peach' }, { value: 'yellow', label: 'Yellow' },
    { value: 'green', label: 'Green' }, { value: 'mint', label: 'Mint' },
    { value: 'teal', label: 'Teal' }, { value: 'blue', label: 'Blue' },
    { value: 'navy', label: 'Navy' }, { value: 'purple', label: 'Purple' },
    { value: 'pink', label: 'Pink' }, { value: 'brown', label: 'Brown' },
    { value: 'beige', label: 'Beige' }, { value: 'white', label: 'White' },
    { value: 'grey', label: 'Grey' }, { value: 'black', label: 'Black' },
  ]);
  const appearanceOptions = $derived([
    { value: 'all', label: 'Light & dark' }, { value: 'light', label: 'Light' }, { value: 'dark', label: 'Dark' },
  ]);
  $effect(() => {
    if (!matches.some(option => option.id === highlighted)) highlighted = matches[0]?.id ?? '';
  });
  $effect(() => {
    query; color; appearance; favoritesOnly;
    if (list) list.scrollTop = 0;
  });

  onMount(() => {
    const refresh = (force = false) => {
      if (unsavedFavorites && !force) return;
      try { favorites = parseThemeFavorites(localStorage.getItem(themeFavoritesKey), validIds); unsavedFavorites = false; }
      catch { /* Keep session favorites when storage is unavailable. */ }
    };
    const storage = (event: StorageEvent) => { if (event.key === themeFavoritesKey || event.key === null) refresh(true); };
    const focus = () => refresh();
    refresh();
    window.addEventListener('storage', storage);
    window.addEventListener('focus', focus);
    window.addEventListener('postika:window-shown', focus);
    return () => {
      window.removeEventListener('storage', storage);
      window.removeEventListener('focus', focus);
      window.removeEventListener('postika:window-shown', focus);
    };
  });

  async function open() {
    query = '';
    highlighted = theme.selected;
    dialog.showModal();
    expanded = true;
    await tick();
    search?.focus();
    document.getElementById(`theme-${highlighted}`)?.scrollIntoView({ block: 'nearest' });
  }
  function close() { dialog.close(); }
  function choose(id: string) { selectTheme(id); close(); }
  function clearFilters() { query = ''; color = 'all'; appearance = 'all'; favoritesOnly = false; search?.focus(); }
  async function toggleFavorite(id: string) {
    const index = matches.findIndex(option => option.id === id);
    const next = new Set(favorites);
    if (next.has(id)) next.delete(id); else next.add(id);
    favorites = next;
    try { localStorage.setItem(themeFavoritesKey, JSON.stringify([...next])); unsavedFavorites = false; }
    catch { unsavedFavorites = true; }
    // Removing a favorite must leave keyboard focus on a surviving control.
    if (favoritesOnly && !next.has(id)) {
      await tick();
      const neighbor = matches[Math.min(index, matches.length - 1)];
      if (neighbor) document.getElementById(`favorite-${neighbor.id}`)?.focus();
      else search?.focus();
    }
  }
  function focusTheme(index: number, favorite = false) {
    const option = matches[index];
    if (!option) return;
    highlighted = option.id;
    document.getElementById(`${favorite ? 'favorite' : 'theme'}-${option.id}`)?.focus();
  }
  function searchKey(event: KeyboardEvent) {
    if (event.isComposing) return;
    if (event.key === 'Escape') { event.preventDefault(); event.stopPropagation(); close(); }
    else if (event.key === 'Enter' && highlighted) { event.preventDefault(); choose(highlighted); }
    else if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
      event.preventDefault();
      focusTheme(event.key === 'ArrowDown' ? 0 : matches.length - 1);
    }
  }
  function rowKey(event: KeyboardEvent, id: string, favorite = false) {
    const index = matches.findIndex(option => option.id === id);
    const next = event.key === 'ArrowDown' ? (index + 1) % matches.length
      : event.key === 'ArrowUp' ? (index - 1 + matches.length) % matches.length
      : event.key === 'Home' ? 0 : event.key === 'End' ? matches.length - 1 : -1;
    if (next < 0) return;
    event.preventDefault();
    focusTheme(next, favorite);
  }
</script>

<button class="picker-trigger" type="button" data-theme-picker aria-label="Color themes" aria-haspopup="dialog" aria-expanded={expanded} bind:this={trigger} onclick={open}>Themes</button>

<dialog class="ui-dialog" bind:this={dialog} aria-labelledby="theme-picker-heading" onclose={() => { expanded = false; trigger.focus(); }} oncancel={(event) => { event.preventDefault(); close(); }} onclick={(event) => { if (event.target === dialog) { const r = dialog.getBoundingClientRect(); if (event.clientX < r.left || event.clientX > r.right || event.clientY < r.top || event.clientY > r.bottom) close(); } }}>
  {#if expanded}
    <div class="picker-heading ui-dialog-heading"><h2 class="ui-title" id="theme-picker-heading">Color themes</h2><button type="button" class="ui-dialog-close" onclick={close}>Close</button></div>
    <div class="search-row"><svg viewBox="0 0 20 20" aria-hidden="true"><circle cx="8.5" cy="8.5" r="5.5"/><path d="m13 13 4 4"/></svg>
      <input type="search" placeholder="Search themes…" aria-label="Search themes…" aria-controls="theme-options" bind:this={search} bind:value={query} onkeydown={searchKey} />
    </div>
    <div class="browse-controls">
      <div class="views" role="group" aria-label="Theme collection">
        <button type="button" aria-pressed={!favoritesOnly} onclick={() => { favoritesOnly = false; }}>All themes <span>{themeOptions.length}</span></button>
        <button type="button" aria-pressed={favoritesOnly} data-theme-favorites-view onclick={() => { favoritesOnly = true; }}>Favorites <span>{favorites.size}</span></button>
      </div>
      <div class="filters">
        <label>Color<select data-theme-color bind:value={color}>
          {#each colorOptions as item}<option value={item.value}>{item.label}</option>{/each}
        </select></label>
        <label>Appearance<select data-theme-appearance bind:value={appearance}>{#each appearanceOptions as item}<option value={item.value}>{item.label}</option>{/each}</select></label>
      </div>
    </div>
    <div class="results-bar"><span role="status" aria-live="polite">{matches.length} / {themeOptions.length}</span>{#if filtered}<button type="button" onclick={clearFilters}>Clear filters</button>{/if}</div>
    <ul class="theme-list" id="theme-options" aria-label="Color themes" bind:this={list}>
      {#each matches as option (option.id)}
        <li class:highlighted={highlighted === option.id}>
          <button id={`theme-${option.id}`} class="theme-choice" type="button" aria-pressed={theme.selected === option.id} data-theme-id={option.id} tabindex={highlighted === option.id ? 0 : -1} onfocus={() => { highlighted = option.id; }} onkeydown={(event) => rowKey(event, option.id)} onclick={() => choose(option.id)}>
            <span class="check" aria-hidden="true">{#if theme.selected === option.id}<svg viewBox="0 0 20 20"><path d="m4 10 4 4 8-8"/></svg>{/if}</span>
            <span class="theme-name" id={`name-${option.id}`}>{option.name}</span>
            <span class="swatches" aria-hidden="true" style:background={option.bg}><i style:background={option.main}></i><i style:background={option.sub}></i><i style:background={option.text}></i></span>
          </button>
          <button id={`favorite-${option.id}`} class="favorite" class:saved={favorites.has(option.id)} type="button" data-theme-favorite={option.id} aria-label={favorites.has(option.id) ? 'Remove favorite' : 'Add favorite'} title={favorites.has(option.id) ? 'Remove favorite' : 'Add favorite'} aria-describedby={`name-${option.id}`} aria-pressed={favorites.has(option.id)} tabindex={highlighted === option.id ? 0 : -1} onfocus={() => { highlighted = option.id; }} onkeydown={(event) => rowKey(event, option.id, true)} onclick={() => toggleFavorite(option.id)}><svg viewBox="0 0 20 20" aria-hidden="true"><path d="m10 2 2.47 5.01L18 7.82l-4 3.9.94 5.51L10 14.63l-4.94 2.6.94-5.51-4-3.9 5.53-.81Z"/></svg></button>
        </li>
      {/each}
    </ul>
    {#if !matches.length}<div class="no-results" role="status">{#if favoritesOnly && !favorites.size}<strong>No favorites yet</strong><p>Use the star beside a theme to save it here.</p>{:else}<strong>No themes found</strong><p>Try another search or clear the filters.</p>{/if}</div>{/if}
  {/if}
</dialog>

<style>
  .picker-trigger { --wails-draggable: no-drag; display: grid; place-items: center; flex-shrink: 0; height: 3.2rem; padding: .7rem 1rem; font-size: var(--ui-small-size); white-space: nowrap; border: 0; border-radius: .5rem; background: transparent; color: var(--text-secondary); cursor: pointer; }
  .picker-trigger:hover { background: var(--surface-hover); color: var(--accent); }
  button:focus-visible, select:focus-visible { outline: 2px solid var(--focus); outline-offset: -2px; }
  dialog { width: min(56rem, calc(100vw - 3.2rem)); max-height: min(70rem, calc(100dvh - 3.2rem)); overflow: hidden; }
  dialog[open] { display: flex; flex-direction: column; }
  .picker-heading, .search-row, .browse-controls, .results-bar { flex-shrink: 0; }
  svg { width: 1.6rem; height: 1.6rem; fill: none; stroke: currentColor; stroke-width: 1.5; stroke-linecap: round; stroke-linejoin: round; }
  .search-row { display: flex; align-items: center; gap: 1rem; margin: 0 1.6rem; padding: .4rem 0 .8rem; border-bottom: 1px solid var(--border); color: var(--text-muted); }
  input { flex: 1; min-width: 0; padding: .5rem 0; background: transparent; border: 0; outline: 0; color: var(--text); font-size: var(--ui-body-size); }
  input::placeholder { color: var(--text-muted); opacity: 1; }
  .search-row:focus-within { box-shadow: inset 0 -2px var(--focus); }
  .browse-controls { padding: 1.2rem 1.6rem 0; }
  .views { display: flex; gap: .4rem; margin-bottom: 1.2rem; }
  .views button { min-height: 3.4rem; border: 1px solid transparent; border-radius: .6rem; padding: .6rem 1rem; background: transparent; color: var(--text-secondary); font-size: var(--ui-body-size); cursor: pointer; }
  .views button[aria-pressed=true] { border-color: var(--border-accent); background: var(--accent-faint); color: var(--accent); }
  .views span { margin-inline-start: .6rem; font-size: var(--ui-small-size); font-variant-numeric: tabular-nums; }
  .filters { display: flex; gap: 1.2rem; }
  label { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: .5rem; font-size: var(--ui-small-size); color: var(--text-secondary); }
  select { width: 100%; min-height: 3.4rem; padding: .6rem .8rem; border: 1px solid var(--border-strong); border-radius: var(--ui-input-radius); font: inherit; font-size: var(--ui-body-size); color: var(--text); background: var(--surface); cursor: pointer; }
  .results-bar { display: flex; align-items: center; justify-content: space-between; min-height: 3.6rem; padding: .4rem 1.6rem; margin-top: .8rem; border-bottom: 1px solid var(--border); color: var(--text-muted); font-size: var(--ui-small-size); font-variant-numeric: tabular-nums; }
  .results-bar button { border: 0; border-radius: .4rem; padding: .4rem; color: var(--accent); background: transparent; font: inherit; cursor: pointer; }
  .theme-list { min-height: 0; overflow-y: auto; overscroll-behavior: contain; padding: .6rem; list-style: none; }
  li { display: flex; align-items: center; border-radius: .5rem; }
  li:has(.theme-choice[aria-pressed=true]), li:focus-within, li:hover { background: var(--surface-hover); }
  .theme-choice { display: flex; align-items: center; gap: .8rem; flex: 1; min-width: 0; min-height: 4rem; padding: .7rem .6rem .7rem 1rem; border: 0; border-radius: .5rem; background: transparent; color: var(--text-secondary); font-size: var(--ui-body-size); text-align: start; cursor: pointer; }
  .theme-choice[aria-pressed=true], .theme-choice:hover { color: var(--text); }
  .check { flex-shrink: 0; width: 1.6rem; height: 1.6rem; color: var(--accent); }
  .theme-name { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .swatches { display: flex; flex-shrink: 0; align-items: center; gap: .4rem; padding: .5rem .8rem; border: 1px solid var(--border-strong); border-radius: 2rem; }
  .swatches i { display: block; width: 1.2rem; height: 1.2rem; border-radius: 50%; }
  .favorite { flex-shrink: 0; display: grid; place-items: center; width: 3.6rem; height: 3.6rem; margin-inline-end: .2rem; border: 0; border-radius: .5rem; background: transparent; color: var(--text-muted); cursor: pointer; }
  .favorite.saved { color: var(--accent); }
  .favorite.saved svg { fill: currentColor; }
  .favorite:hover, .views button:hover, .results-bar button:hover { background: var(--surface-hover); color: var(--accent); }
  .no-results { overflow-y: auto; padding: 2.4rem 1.6rem 3.2rem; text-align: center; font-size: var(--ui-body-size); color: var(--text); }
  .no-results p { margin-top: .8rem; color: var(--text-muted); }
  @media (max-height: 540px) { .browse-controls { padding-top: .6rem; } .views { margin-bottom: .6rem; } .no-results { padding-block: 1.2rem; } }
</style>
