<script lang="ts">
  import { tick } from 'svelte';
  import { theme, selectTheme } from '../lib/theme.svelte';
  import { themeOptions } from '../lib/theme-options';

  let dialog: HTMLDialogElement;
  let search = $state<HTMLInputElement>();
  let trigger: HTMLButtonElement;
  let query = $state('');
  let highlighted = $state('');
  let expanded = $state(false);
  const matches = $derived(themeOptions.filter(option => option.name.toLocaleLowerCase().includes(query.trim().toLocaleLowerCase())));
  $effect(() => {
    if (!matches.some(option => option.id === highlighted)) highlighted = matches[0]?.id ?? '';
  });

  async function open() {
    query = '';
    highlighted = theme.selected;
    dialog.showModal();
    expanded = true;
    await tick();
    search?.focus();
    scrollToHighlight();
  }
  function close() { dialog.close(); expanded = false; trigger.focus(); }
  function choose(id: string) { selectTheme(id); close(); }
  function scrollToHighlight() {
    document.getElementById(`theme-${highlighted}`)?.scrollIntoView({ block: 'nearest' });
  }
  async function searchKey(event: KeyboardEvent) {
    if (event.key === 'Escape') { event.preventDefault(); event.stopPropagation(); close(); return; }
    if (event.key === 'Enter') { event.preventDefault(); if (highlighted) choose(highlighted); return; }
    if (!matches.length) return;
    const index = matches.findIndex(option => option.id === highlighted);
    const next = event.key === 'ArrowDown' ? (index + 1) % matches.length
      : event.key === 'ArrowUp' ? (index - 1 + matches.length) % matches.length
      : !query && event.key === 'Home' ? 0 : !query && event.key === 'End' ? matches.length - 1 : -1;
    if (next < 0) return;
    event.preventDefault();
    highlighted = matches[next].id;
    await tick();
    scrollToHighlight();
  }
</script>

<button class="picker-trigger" type="button" data-theme-picker bind:this={trigger} aria-label="Color themes" aria-haspopup="dialog" aria-expanded={expanded} onclick={open}>
  Themes
</button>

<dialog class="ui-dialog" bind:this={dialog} aria-labelledby="theme-picker-heading" onclose={() => { expanded = false; }} onclick={(event) => { if (event.target === dialog) { const r = dialog.getBoundingClientRect(); if (event.clientX < r.left || event.clientX > r.right || event.clientY < r.top || event.clientY > r.bottom) close(); } }}>
  {#if expanded}
  <div class="picker-heading ui-dialog-heading"><h2 class="ui-title" id="theme-picker-heading">Color themes</h2><button type="button" class="close-picker ui-dialog-close" aria-label="Close" onclick={close}><svg viewBox="0 0 24 24" aria-hidden="true"><path d="m6 6 12 12M18 6 6 18"/></svg></button></div>
  <div class="search-row"><svg viewBox="0 0 20 20" aria-hidden="true"><circle cx="8.5" cy="8.5" r="5.5"/><path d="m13 13 4 4"/></svg>
    <input type="search" placeholder="Search themes…" aria-label="Search themes…" role="combobox" aria-autocomplete="list" aria-expanded="true" aria-controls="theme-options" aria-activedescendant={highlighted ? `theme-${highlighted}` : undefined} bind:this={search} bind:value={query} onkeydown={searchKey} />
  </div>
  <div class="theme-list" id="theme-options" role="listbox" aria-label="Color themes">
    {#each matches as option (option.id)}
      <button id={`theme-${option.id}`} type="button" role="option" aria-selected={theme.selected === option.id} data-theme-id={option.id} tabindex="-1" class:highlighted={highlighted === option.id} onpointermove={() => { highlighted = option.id; }} onclick={() => choose(option.id)}>
        <span class="check" aria-hidden="true">{theme.selected === option.id ? '✓' : ''}</span><span class="theme-name">{option.name}</span>
        <span class="swatches" aria-hidden="true" style:background={option.bg}><i style:background={option.main}></i><i style:background={option.sub}></i><i style:background={option.text}></i></span>
      </button>
    {/each}
    {#if !matches.length}<div class="no-results" role="status"><strong>No themes found</strong><p>Try a different name.</p></div>{/if}
  </div>
  {/if}
</dialog>

<style>
  .picker-trigger { --wails-draggable: no-drag; display: grid; place-items: center; flex-shrink: 0; height: 3.2rem; padding: .7rem 1rem; font-size: 1.2rem; white-space: nowrap; border: 0; border-radius: .5rem; background: transparent; color: var(--text-secondary); cursor: pointer; }
  .picker-trigger:hover { background: var(--surface-hover); color: var(--accent); }
  button:focus-visible { outline: 2px solid var(--focus); outline-offset: -2px; }
  dialog { width: min(52rem, calc(100vw - 3.2rem)); max-height: min(64rem, calc(100dvh - 3.2rem)); overflow: hidden; }
  dialog[open] { display: flex; flex-direction: column; }

  .search-row svg { width: 1.6rem; height: 1.6rem; fill: none; stroke: currentColor; stroke-width: 1.5; }
  .search-row { display: flex; align-items: center; gap: 1rem; padding: .8rem 1.6rem 1.2rem; border-bottom: 1px solid var(--border); color: var(--text-muted); }
  input { flex: 1; min-width: 0; padding: .5rem 0; background: transparent; border: 0; outline: 0; color: var(--text); font-size: 1.3rem; }
  .search-row:focus-within { box-shadow: inset 0 -2px var(--focus); }
  .theme-list { min-height: 0; overflow-y: auto; overscroll-behavior: contain; padding: .6rem; }
  [role=option] { display: flex; align-items: center; gap: .8rem; width: 100%; min-height: 3.7rem; padding: .7rem 1rem; border: 0; border-radius: .5rem; background: transparent; color: var(--text-secondary); font-size: 1.3rem; text-align: start; cursor: pointer; }
  [role=option].highlighted, [role=option][aria-selected=true] { background: var(--surface-hover); color: var(--text); }
  .check { flex-shrink: 0; width: 1.4rem; color: var(--accent); }
  .theme-name { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .swatches { display: flex; flex-shrink: 0; align-items: center; gap: .4rem; padding: .5rem .8rem; border: 1px solid var(--border-strong); border-radius: 2rem; }
  .swatches i { display: block; width: 1.2rem; height: 1.2rem; border-radius: 50%; }
  .no-results { padding: 3rem 1.6rem; text-align: center; font-size: 1.3rem; }
  .no-results p { margin-top: .8rem; color: var(--text-muted); }
</style>
