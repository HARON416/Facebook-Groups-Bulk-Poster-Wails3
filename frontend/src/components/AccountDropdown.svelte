<script lang="ts">
  import { tick } from 'svelte';
  import { getLocale } from '../lib/i18n.svelte';
  import { localeDirection } from '../lib/locale-preferences';
  import { accountLabel } from './AccountLabel.svelte';
  import AddAccountDialog from './AddAccountDialog.svelte';
  import DeleteProfileDialog from './DeleteProfileDialog.svelte';
  import RenameProfileDialog from './RenameProfileDialog.svelte';

  interface Props {
    profiles: { id: string; name: string; flag?: string }[];
    selected: string;
    disabled?: boolean;
    compact?: boolean;
    language?: boolean;
    id?: string;
    onselect: (id: string) => void;
    oncreated?: (profile: { id: string; name: string }) => void;
    ondeleted?: (id: string) => void;
    onrenamed?: (profile: { id: string; name: string }) => void;
  }
  let { profiles, selected, disabled = false, compact = false, language = false, id = 'account', onselect, onrenamed, oncreated, ondeleted }: Props = $props();
  let expanded = $state(false), highlighted = $state(''), query = $state('');
  let adding = $state(false);
  let deleting = $state<{ id: string; name: string } | null>(null);
  let renaming = $state<{ id: string; name: string } | null>(null);
  let root: HTMLDivElement, trigger: HTMLButtonElement;
  let searchInput = $state<HTMLInputElement>(), list = $state<HTMLDivElement>();
  const displayProfiles = $derived(profiles.map(profile => ({ ...profile, name: language ? profile.name : accountLabel(profile) })));
  const current = $derived(displayProfiles.find((profile) => profile.id === selected));
  const unavailable = $derived(disabled || (language && profiles.length === 0));
  const matches = $derived(displayProfiles.filter((profile) => profile.name.toLocaleLowerCase().includes(query.trim().toLocaleLowerCase())));

  async function highlight(id: string) {
    highlighted = id;
    await tick();
    list?.querySelector<HTMLElement>(`[data-profile-id="${id}"]`)?.scrollIntoView({ block: 'nearest' });
  }
  async function show(id = selected) {
    if (unavailable) return;
    expanded = true; query = '';
    highlighted = id || profiles[0]?.id || '';
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
  function rename(id: string) {
    if (unavailable || language) return;
    const profile = profiles.find(profile => profile.id === id);
    if (!profile) return;
    close();
    renaming = { id: profile.id, name: profile.name };
  }
  function remove(id: string) {
    if (unavailable || language) return;
    const profile = profiles.find(profile => profile.id === id);
    if (!profile) return;
    close(); deleting = { id: profile.id, name: profile.name };
  }
  function addAccount() { close(); adding = true; }
  function restoreFocus() { void tick().then(() => trigger?.focus()); }
  function closeRename() {
    renaming = null;
    void tick().then(() => trigger?.focus());
  }
  function triggerKey(event: KeyboardEvent) {
    if (!['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(event.key)) return;
    event.preventDefault();
    void show(event.key === 'Home' ? profiles[0]?.id : event.key === 'End' ? profiles.at(-1)?.id : selected);
  }
  function searchKey(event: KeyboardEvent) {
    if (event.key === 'Delete' && !language && highlighted && !query) { event.preventDefault(); remove(highlighted); return; }
    if (event.key === 'F2' && !language && highlighted) { event.preventDefault(); rename(highlighted); return; }
    if (event.key === 'Tab') { if (language) close(true); else if (!event.shiftKey) { event.preventDefault(); root.querySelector<HTMLButtonElement>('[data-add-account]')?.focus(); } return; }
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

<div class="account-dropdown" class:compact dir={localeDirection(getLocale())} bind:this={root} onfocusout={(event) => { if (!root.contains(event.relatedTarget as Node | null)) close(); }}>
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
          <div class="profile-row" role="presentation" class:highlighted={profile.id === highlighted} onpointermove={() => { highlighted = profile.id; }}>
          <button id={`${id}-option-${profile.id}`} type="button" role="option" tabindex="-1"
            aria-selected={profile.id === selected} data-profile-id={profile.id}
            class:highlighted={profile.id === highlighted} class:selected={profile.id === selected} class:renamable={!language}
            onpointermove={() => { highlighted = profile.id; }} onclick={() => choose(profile.id)}>
            {#if profile.flag}<span class="flag" aria-hidden="true">{profile.flag}</span>{/if}
            <span class="account-name" dir="auto">{profile.name}</span>
            {#if profile.id === selected}<svg class="check" viewBox="0 0 20 20" fill="none" aria-hidden="true"><path d="m4 10 4 4 8-8" /></svg>{/if}
          </button>
          {#if !language}
            <button type="button" class="profile-action rename-profile" tabindex="-1" aria-label="Rename profile" aria-describedby={`${id}-option-${profile.id}`} aria-haspopup="dialog" aria-keyshortcuts="F2" data-rename-profile={profile.id} onclick={(event) => { event.stopPropagation(); rename(profile.id); }}>
              <svg viewBox="0 0 256 256" aria-hidden="true"><path d="M227.32,73.37,182.63,28.69a16,16,0,0,0-22.63,0L36.69,152A15.86,15.86,0,0,0,32,163.31V208a16,16,0,0,0,16,16H92.69A15.86,15.86,0,0,0,104,219.31l83.67-83.66,3.48,13.9-36.8,36.79a8,8,0,0,0,11.31,11.32l40-40a8,8,0,0,0,2.11-7.6l-6.9-27.61L227.32,96A16,16,0,0,0,227.32,73.37ZM48,179.31,76.69,208H48Zm48,25.38L51.31,160,136,75.31,180.69,120Zm96-96L147.32,64l24-24L216,84.69Z" /></svg>
            </button>
            <button type="button" class="profile-action delete-profile" tabindex="-1" aria-label="Delete profile" aria-describedby={`${id}-option-${profile.id}`} aria-haspopup="dialog" aria-keyshortcuts="Delete" data-delete-profile={profile.id} onclick={(event) => { event.stopPropagation(); remove(profile.id); }}>
              <svg viewBox="0 0 256 256" aria-hidden="true"><path d="M216,48H176V40a24,24,0,0,0-24-24H104A24,24,0,0,0,80,40v8H40a8,8,0,0,0,0,16h8V208a16,16,0,0,0,16,16H192a16,16,0,0,0,16-16V64h8a8,8,0,0,0,0-16ZM96,40a8,8,0,0,1,8-8h48a8,8,0,0,1,8,8v8H96Zm96,168H64V64H192ZM112,104v64a8,8,0,0,1-16,0V104a8,8,0,0,1,16,0Zm48,0v64a8,8,0,0,1-16,0V104a8,8,0,0,1,16,0Z" /></svg>
            </button>
          {/if}
          </div>
        {/each}
      </div>
      {#if !matches.length}
        <div class="no-results">
          <svg viewBox="0 0 20 20" fill="none" aria-hidden="true"><circle cx="8.5" cy="8.5" r="5.5" /><path d="m13 13 4 4" /></svg>
          <strong>{language ? 'No languages found' : 'No accounts found'}</strong><p>Try a different name.</p>
        </div>
      {/if}
      {#if !language}
        <button type="button" class="add-account" data-add-account aria-haspopup="dialog" onclick={addAccount} onkeydown={(event) => { if (event.key === 'Escape') { event.preventDefault(); close(true); } else if (event.key === 'Tab' && event.shiftKey) { event.preventDefault(); searchInput?.focus(); } }}><span aria-hidden="true">+</span><span>Add account</span></button>
      {/if}
    </div>
  {/if}
</div>

{#if renaming}
  <RenameProfileDialog profile={renaming} onrenamed={(profile) => onrenamed?.(profile)} onclose={closeRename} />
{/if}

{#if deleting}
  <DeleteProfileDialog profile={deleting} {ondeleted} onclose={() => { deleting = null; restoreFocus(); }} />
{/if}
{#if adding}
  <AddAccountDialog oncreated={(profile) => oncreated?.(profile)} onclose={() => { adding = false; restoreFocus(); }} />
{/if}

<style>
  .account-dropdown { position: relative; width: 24rem; max-width: 50vw; font-size: var(--ui-body-size); }
  button, input { font: inherit; }
  button { cursor: pointer; }
  .account-trigger { display: flex; align-items: center; gap: 1rem; width: 100%; min-height: 4rem; padding: .9rem 1.4rem; border: 1px solid var(--border); border-radius: .7rem; color: var(--text-secondary); background: var(--surface); text-align: start; }
  .account-trigger:hover:not(:disabled), .account-trigger[aria-expanded=true] { border-color: var(--border-strong); }
  .account-trigger:disabled { opacity: .5; cursor: default; }
  .compact { width: 22rem; font-size: var(--ui-small-size); }
  .compact .account-trigger { min-height: 3.2rem; padding: .6rem 1rem; }
  .account-name { display: block; flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .flag { flex-shrink: 0; font-size: 1.4rem; }
  svg { flex-shrink: 0; width: 1.5rem; height: 1.5rem; stroke: currentColor; stroke-width: 1.3; stroke-linecap: round; stroke-linejoin: round; }
  .account-trigger .chevron { width: 1.4rem; height: 1.4rem; fill: currentColor; stroke: none; }
  .account-menu { position: absolute; inset-inline-end: 0; top: calc(100% + .5rem); z-index: 50; width: 100%; border: 1px solid var(--border); border-radius: .7rem; background: var(--surface); box-shadow: 0 3px 12px var(--shadow-soft); overflow: hidden; }
  .search-row { display: flex; align-items: center; gap: .8rem; min-height: 4.1rem; padding: .8rem 1.2rem; border-bottom: 1px solid var(--border); color: var(--text-muted); }
  .search-row input { min-width: 0; flex: 1; width: 100%; padding: 0; border: 0; outline: none; background: transparent; color: var(--text); }
  .search-row input::placeholder { color: var(--text-muted); }
  .clear-search { display: grid; place-items: center; padding: .2rem; border: 0; border-radius: .3rem; background: transparent; color: var(--text-muted); }
  .menu-heading { padding: 1.1rem 1.2rem .5rem; color: var(--text-muted); font-size: var(--ui-small-size); }
  [role=listbox] { max-height: min(28rem, 50dvh); padding: 0 .5rem .6rem; overflow-y: auto; }
  [role=option] { display: flex; align-items: center; gap: .8rem; width: 100%; min-height: 3rem; padding: .6rem .7rem; border: 0; border-radius: .3rem; background: var(--surface); color: var(--text); text-align: start; }
  [role=option]:hover, [role=option].highlighted, [role=option].selected { background: var(--surface-hover); }
  .check { color: var(--text-secondary); }
  .profile-row { position: relative; }
  [role=option].renamable { padding-inline-end: 6.4rem; }
  .profile-action { position: absolute; inset-inline-end: .3rem; top: 50%; transform: translateY(-50%); display: grid; place-items: center; width: 2.6rem; height: 2.6rem; padding: .5rem; border: 0; border-radius: .4rem; background: transparent; color: var(--text-muted); opacity: 0; pointer-events: none; }
  .profile-action svg { width: 1.6rem; height: 1.6rem; fill: currentColor; stroke: none; }
  .profile-row:hover .profile-action, .profile-row:focus-within .profile-action, .profile-row.highlighted .profile-action { opacity: 1; pointer-events: auto; }
  .rename-profile { inset-inline-end: 3.2rem; }
  .delete-profile:hover { background: var(--surface-active); color: var(--danger); }
  .add-account { display: flex; align-items: center; gap: .8rem; width: 100%; padding: 1rem 1.2rem; border: 0; border-top: 1px solid var(--border); background: var(--surface); color: var(--accent); text-align: start; }
  .add-account:hover { background: var(--surface-hover); }
  .add-account:focus-visible { outline: 2px solid var(--focus); outline-offset: -3px; }
  .rename-profile:hover { background: var(--surface-active); color: var(--accent); }
  .profile-action:focus-visible { outline: 2px solid var(--focus); outline-offset: -2px; }
  @media (hover: none) { .profile-action { opacity: 1; pointer-events: auto; } }
  .no-results { display: flex; align-items: center; flex-direction: column; gap: .8rem; padding: 2.5rem 1.5rem 3rem; text-align: center; }
  .no-results svg { width: 2rem; height: 2rem; margin-bottom: .5rem; color: var(--text-muted); }
  .no-results strong { color: var(--text); font-size: var(--ui-small-size); font-weight: 550; }
  .no-results p { color: var(--text-muted); font-size: var(--ui-small-size); }
  .account-trigger:focus-visible, .clear-search:focus-visible { outline: 2px solid var(--focus); outline-offset: 2px; }
  .search-row:focus-within { box-shadow: inset 0 -1px var(--border-strong); }
</style>
