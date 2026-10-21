<script lang="ts">
  import { tick } from 'svelte';

  interface Props {
    profiles: { id: string; name: string; flag?: string }[];
    selected: string;
    disabled?: boolean;
    compact?: boolean;
    language?: boolean;
    id?: string;
    onselect: (id: string) => void;
  }
  let { profiles, selected, disabled = false, compact = false, language = false, id = 'account', onselect }: Props = $props();
  let expanded = $state(false), highlighted = $state(''), query = $state('');
  let root: HTMLDivElement, trigger: HTMLButtonElement;
  let searchInput = $state<HTMLInputElement>(), list = $state<HTMLDivElement>();
  const current = $derived(profiles.find((profile) => profile.id === selected));
  const unavailable = $derived(disabled || profiles.length === 0);
  const matches = $derived(profiles.filter((profile) => profile.name.toLocaleLowerCase().includes(query.trim().toLocaleLowerCase())));

  async function highlight(id: string) {
    highlighted = id;
    await tick();
    list?.querySelector<HTMLElement>(`[data-profile-id="${id}"]`)?.scrollIntoView({ block: 'nearest' });
  }
  async function show(id = selected) {
    if (unavailable) return;
    expanded = true; query = '';
    highlighted = id || profiles[0].id;
    await tick();
    searchInput?.focus();
    void highlight(highlighted);
  }
  function close(restoreFocus = false) {
    expanded = false;
    if (restoreFocus) trigger?.focus();
  }
  function choose(id: string) {
    if (unavailable || !id) return;
    close(true);
    onselect(id);
  }
  function triggerKey(event: KeyboardEvent) {
    if (!['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(event.key)) return;
    event.preventDefault();
    void show(event.key === 'Home' ? profiles[0]?.id : event.key === 'End' ? profiles.at(-1)?.id : selected);
  }
  function searchKey(event: KeyboardEvent) {
    if (event.key === 'Tab') { close(true); return; }
    if (event.key === 'Escape') { event.preventDefault(); event.stopPropagation(); close(true); return; }
    if (event.key === 'Enter') { event.preventDefault(); choose(highlighted); return; }
    if (!matches.length) return;
    const index = Math.max(0, matches.findIndex((profile) => profile.id === highlighted));
    const next = !query && event.key === 'Home' ? 0 : !query && event.key === 'End' ? matches.length - 1
      : event.key === 'ArrowDown' ? (index + 1) % matches.length
      : event.key === 'ArrowUp' ? (index - 1 + matches.length) % matches.length : -1;
    if (next >= 0) { event.preventDefault(); void highlight(matches[next].id); }
  }
  $effect(() => { if (unavailable) expanded = false; });
  $effect(() => {
    if (!matches.some((profile) => profile.id === highlighted)) {
      highlighted = matches.find((profile) => profile.id === selected)?.id ?? matches[0]?.id ?? '';
    }
  });
</script>

<svelte:window onpointerdown={(event) => { if (expanded && event.target instanceof Node && !root.contains(event.target)) close(); }} />

<div class="account-dropdown" class:compact bind:this={root} onfocusout={(event) => { if (!root.contains(event.relatedTarget as Node | null)) close(); }}>
  <button id={`${id}-profile`} class="account-trigger" type="button" bind:this={trigger}
    data-profile-id={selected} aria-label={language ? `Language: ${current?.name ?? ''}` : `Account: ${current?.name ?? 'Loading accounts'}`}
    aria-haspopup="listbox" aria-expanded={expanded} aria-controls={expanded ? `${id}-options` : undefined}
    disabled={unavailable} onclick={() => expanded ? close() : show()} onkeydown={triggerKey}>
    {#if current?.flag}<span class="flag" aria-hidden="true">{current.flag}</span>{/if}
    <span class="account-name" dir="auto">{current?.name ?? 'Select account'}</span>
    <svg class="chevron" viewBox="0 0 256 256" aria-hidden="true">
      {#if expanded}
        <path d="M213.66,165.66a8,8,0,0,1-11.32,0L128,91.31,53.66,165.66a8,8,0,0,1-11.32-11.32l80-80a8,8,0,0,1,11.32,0l80,80A8,8,0,0,1,213.66,165.66Z" />
      {:else}
        <path d="M213.66,101.66l-80,80a8,8,0,0,1-11.32,0l-80-80A8,8,0,0,1,53.66,90.34L128,164.69l74.34-74.35a8,8,0,0,1,11.32,11.32Z" />
      {/if}
    </svg>
  </button>
  {#if expanded}
    <div class="account-menu">
      <div class="search-row">
        <svg viewBox="0 0 20 20" fill="none" aria-hidden="true"><circle cx="8.5" cy="8.5" r="5.5" /><path d="m13 13 4 4" /></svg>
        <input id={`${id}-search`} type="text" role="combobox" autocomplete="off" placeholder={language ? 'Search languages…' : 'Search accounts…'}
          aria-label={language ? 'Search languages' : 'Search accounts'} aria-autocomplete="list" aria-expanded="true" aria-controls={`${id}-options`}
          aria-activedescendant={highlighted ? `${id}-option-${highlighted}` : undefined}
          bind:this={searchInput} bind:value={query} onkeydown={searchKey} />
        {#if query}<button class="clear-search" type="button" tabindex="-1" aria-label={language ? 'Clear language search' : 'Clear account search'} onclick={() => { query = ''; searchInput?.focus(); }}>
          <svg viewBox="0 0 20 20" fill="none" aria-hidden="true"><path d="m6 6 8 8M14 6l-8 8" /></svg>
        </button>{/if}
      </div>
      <p class="menu-heading" role="status">{query.trim() ? matches.length === 1 ? `${matches.length} result` : `${matches.length} results` : language ? 'All languages' : 'All accounts'}</p>
      <div id={`${id}-options`} role="listbox" aria-label={language ? 'Languages' : 'Accounts'} bind:this={list}>
        {#each matches as profile (profile.id)}
          <button id={`${id}-option-${profile.id}`} type="button" role="option" tabindex="-1"
            aria-selected={profile.id === selected} data-profile-id={profile.id}
            class:highlighted={profile.id === highlighted} class:selected={profile.id === selected}
            onpointermove={() => { highlighted = profile.id; }} onclick={() => choose(profile.id)}>
            {#if profile.flag}<span class="flag" aria-hidden="true">{profile.flag}</span>{/if}
            <span class="account-name" dir="auto">{profile.name}</span>
            {#if profile.id === selected}<svg class="check" viewBox="0 0 20 20" fill="none" aria-hidden="true"><path d="m4 10 4 4 8-8" /></svg>{/if}
          </button>
        {/each}
      </div>
      {#if !matches.length}
        <div class="no-results">
          <svg viewBox="0 0 20 20" fill="none" aria-hidden="true"><circle cx="8.5" cy="8.5" r="5.5" /><path d="m13 13 4 4" /></svg>
          <strong>{language ? 'No languages found' : 'No accounts found'}</strong><p>Try a different name.</p>
        </div>
      {/if}
    </div>
  {/if}
</div>

<style>
  .account-dropdown { position: relative; width: 24rem; max-width: 50vw; font-size: 1.3rem; }
  button, input { font: inherit; }
  button { cursor: pointer; }
  .account-trigger { display: flex; align-items: center; gap: 1rem; width: 100%; min-height: 4rem; padding: .9rem 1.4rem; border: 1px solid var(--border); border-radius: .7rem; color: var(--text-secondary); background: var(--surface); text-align: left; }
  .account-trigger:hover:not(:disabled), .account-trigger[aria-expanded=true] { border-color: var(--border-strong); }
  .account-trigger:disabled { opacity: .5; cursor: default; }
  .compact { width: 22rem; font-size: 1.2rem; }
  .compact .account-trigger { min-height: 3.2rem; padding: .6rem 1rem; }
  .account-name { display: block; flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .flag { flex-shrink: 0; font-size: 1.4rem; }
  svg { flex-shrink: 0; width: 1.5rem; height: 1.5rem; stroke: currentColor; stroke-width: 1.3; stroke-linecap: round; stroke-linejoin: round; }
  .account-trigger .chevron { width: 1.4rem; height: 1.4rem; fill: currentColor; stroke: none; }
  .account-menu { position: absolute; right: 0; top: calc(100% + .5rem); z-index: 50; width: 100%; border: 1px solid var(--border); border-radius: .7rem; background: var(--surface); box-shadow: 0 3px 12px var(--shadow-soft); overflow: hidden; }
  .search-row { display: flex; align-items: center; gap: .8rem; min-height: 4.1rem; padding: .8rem 1.2rem; border-bottom: 1px solid var(--border); color: var(--text-muted); }
  .search-row input { min-width: 0; flex: 1; width: 100%; padding: 0; border: 0; outline: none; background: transparent; color: var(--text); }
  .search-row input::placeholder { color: var(--text-muted); }
  .clear-search { display: grid; place-items: center; padding: .2rem; border: 0; border-radius: .3rem; background: transparent; color: var(--text-muted); }
  .menu-heading { padding: 1.1rem 1.2rem .5rem; color: var(--text-muted); font-size: 1.05rem; }
  [role=listbox] { max-height: min(28rem, 50dvh); padding: 0 .5rem .6rem; overflow-y: auto; }
  [role=option] { display: flex; align-items: center; gap: .8rem; width: 100%; min-height: 3rem; padding: .6rem .7rem; border: 0; border-radius: .3rem; background: var(--surface); color: var(--text); text-align: left; }
  [role=option]:hover, [role=option].highlighted, [role=option].selected { background: var(--surface-hover); }
  .check { color: var(--text-secondary); }
  .no-results { display: flex; align-items: center; flex-direction: column; gap: .8rem; padding: 2.5rem 1.5rem 3rem; text-align: center; }
  .no-results svg { width: 2rem; height: 2rem; margin-bottom: .5rem; color: var(--text-muted); }
  .no-results strong { color: var(--text); font-size: 1.2rem; font-weight: 550; }
  .no-results p { color: var(--text-muted); font-size: 1.1rem; }
  .account-trigger:focus-visible, .clear-search:focus-visible { outline: 2px solid var(--focus); outline-offset: 2px; }
  .search-row:focus-within { box-shadow: inset 0 -1px var(--border-strong); }
</style>
