<script lang="ts">
  import { onMount } from 'svelte';
  import { Window } from '@wailsio/runtime';
  import { ProfileService, type ProfileActivity, type AccountProfile } from '../bindings/postika';
  import AccountDropdown from './components/AccountDropdown.svelte';
  import WindowControls from './components/WindowControls.svelte';
  import UpdateButton from './components/UpdateButton.svelte';
  import LanguageMenu from './components/LanguageMenu.svelte';
  import ThemeMenu from './components/ThemeMenu.svelte';
  import ContactButton from './components/ContactButton.svelte';
  import { createAccountService } from './lib/accounts';
  import { accountLabel } from './components/AccountLabel.svelte';
  import ConnectivityToast from './components/ConnectivityToast.svelte';
  import FeaturesCard from './components/FeaturesCard.svelte';
  import { pollWhileWindowOpen } from './lib/window-polling';
  import LocalizedFeedback from './components/LocalizedFeedback.svelte';
  import AddAccountDialog from './components/AddAccountDialog.svelte';
  import GuidedTour from './components/GuidedTour.svelte';

  // Load each screen at its render branch below. Marketplace windows and an
  // empty workspace should not download or initialize the posting editor.
  const marketplace = new URLSearchParams(window.location.search).get('view') === 'marketplace';
  const createListing = new URLSearchParams(window.location.search).get('view') === 'create-listing';
  const blankWorkspace = marketplace || createListing;
  const marketplaceTitle = $derived(createListing ? /* @wc-include */ 'Create new listing' : /* @wc-include */ 'Marketplace');
  $effect(() => {
    if (blankWorkspace) {
      document.title = `${marketplaceTitle} · Postika`;
      void Window.SetTitle(document.title).catch(() => {});
    }
  });

  let viewLoadAttempt = $state(0);
  let tour: GuidedTour;
  let tourPage = $state<number | null>(null), tourReady = $state(false), tourActive = $state(false);
  const setTourReady = (ready: boolean) => { tourReady = ready; };
  let profiles = $state<ProfileActivity[]>([]);
  let opened = $state<(AccountProfile & { connect: boolean })[]>([]);
  let selected = $state('');
  let adding = $state(false), busy = $state(false), error = $state('');
  let addTrigger: HTMLButtonElement;
  let windowControls = $state<WindowControls>();
  const current = $derived(profiles.find((profile) => profile.id === selected));
  const background = $derived(profiles.filter((profile) => profile.active && profile.id !== selected));
  const tourAvailable = $derived(Boolean(current) && tourReady && !adding && !busy && !profiles.some(profile => profile.active));

  function startGuide() {
    if (!tourAvailable) return;
    if (blankWorkspace) window.dispatchEvent(new Event('postika:marketplace-tour'));
    else void tour.start();
  }

  function open(profile: AccountProfile, connect = false) {
    if (!blankWorkspace && !opened.some((entry) => entry.id === profile.id)) opened = [...opened, { ...profile, connect }];
    tourReady = false;
    selected = profile.id;
  }
  async function refresh() {
    try {
      const result = await ProfileService.GetProfiles();
      profiles = result.profiles ?? [];
      opened = opened.filter(entry => profiles.some(profile => profile.id === entry.id)).map(entry => {
        const name = profiles.find(profile => profile.id === entry.id)?.name;
        return name !== undefined && name !== entry.name ? { ...entry, name } : entry;
      });
      if (!profiles.some(profile => profile.id === selected)) {
        selected = '';
        const profile = profiles.find((profile) => profile.id === result.selected);
        if (profile) open(profile);
      }
    } catch (cause) { error = cause instanceof Error ? cause.message : String(cause); }
  }
  onMount(() => pollWhileWindowOpen(refresh, 2000));

  function profileRenamed(profile: AccountProfile) {
    profiles = profiles.map(entry => entry.id === profile.id ? { ...entry, name: profile.name } : entry);
    opened = opened.map(entry => entry.id === profile.id ? { ...entry, name: profile.name } : entry);
  }

  async function openPostingLogs() {
    if (!selected || busy) return;
    error = '';
    const account = createAccountService(selected);
    try { await account.OpenPostingLogs(); }
    catch (cause) { error = cause instanceof Error ? cause.message : String(cause); }
  }

  async function openMarketplace() {
    error = '';
    try { await ProfileService.OpenMarketplace(); }
    catch (cause) { error = cause instanceof Error ? cause.message : String(cause); }
  }

  async function select(id: string) {
    if (busy || id === selected) return;
    busy = true; error = '';
    try {
      await ProfileService.SelectProfile(id);
      const profile = profiles.find((profile) => profile.id === id);
      if (profile) open(profile);
    } catch (cause) { error = cause instanceof Error ? cause.message : String(cause); }
    finally { busy = false; }
  }
  function showAddAccount() {
    if (!adding && !busy) adding = true;
  }
  function profileCreated(profile: AccountProfile) {
    profiles = [...profiles, { ...profile, active: false, state: '' }];
    open(profile, true);
  }

</script>

{#snippet viewLoadError(cause: unknown)}
  <div class="profile-error" role="alert"><LocalizedFeedback message={cause instanceof Error ? cause.message : String(cause)} /><button type="button" onclick={() => { viewLoadAttempt++; }}>Try again</button></div>
{/snippet}

<div class="workspace ui-window">
  <header class="top-bar">
    <div class="profile-control" data-tour="accounts"><AccountDropdown {profiles} {selected} compact disabled={busy} onselect={select} onrenamed={profileRenamed} oncreated={profileCreated} ondeleted={() => void refresh()} /></div>
    <div class="drag-area" aria-hidden="true" ondblclick={() => windowControls?.toggleMaximise()}></div>
    <nav class="app-menu" aria-label="Application menu">
    <fieldset class="menu-controls">
      <UpdateButton />
      <FeaturesCard {tourAvailable} onstarttour={startGuide} />
      <button type="button" data-guide-trigger disabled={!tourAvailable} aria-haspopup="dialog" title={tourAvailable ? 'Start guided tour' : 'Wait until the current operation finishes to start the tour.'} onclick={startGuide}>Guide</button>
      <button type="button" disabled={busy} aria-expanded={adding} bind:this={addTrigger} aria-haspopup="dialog" aria-controls="add-account-dialog" onclick={showAddAccount}>Add account</button>
      <button type="button" data-tour="logs" disabled={busy || !selected} onclick={() => openPostingLogs()}>Posting logs</button>
      <button type="button" data-tour="marketplace" disabled={busy} onclick={openMarketplace}>Marketplace</button>
      <LanguageMenu />
      <ThemeMenu />
      <ContactButton />
    </fieldset>
    </nav>
    <WindowControls bind:this={windowControls} onerror={(message) => { error = message; }} />
  </header>
  {#if adding}<AddAccountDialog oncreated={profileCreated} onclose={() => { adding = false; addTrigger?.focus(); }} />{/if}
  {#if error}<div class="profile-error" role="alert"><LocalizedFeedback message={error} /></div>{/if}
  {#if !blankWorkspace && background.length}
    <div class="background-jobs" role="status">In the background:
      {#each background as profile (profile.id)}<button type="button" disabled={busy} onclick={() => select(profile.id)}>{accountLabel(profile)} · {profile.state === 'scheduled' ? 'Scheduled' : 'Running'}</button>{/each}
    </div>
  {/if}
  {#key viewLoadAttempt}
  {#if blankWorkspace}
    <main class="account-workspace" aria-label={marketplaceTitle}>
      {#if marketplace}
        {#await import('./components/MarketplaceListings.svelte')}
          <p class="loading" role="status">Loading…</p>
        {:then { default: MarketplaceListings }}
          {#key current?.id}<MarketplaceListings profile={current} ontourready={setTourReady} />{/key}
        {:catch cause}{@render viewLoadError(cause)}{/await}
      {:else if createListing}
        {#await import('./components/CreateListing.svelte')}
          <p class="loading" role="status">Loading…</p>
        {:then { default: CreateListing }}
          {#key current?.id}<CreateListing profile={current} ontourready={setTourReady} />{/key}
        {:catch cause}{@render viewLoadError(cause)}{/await}
      {/if}
    </main>
  {:else}
    {#if opened.length}
      {#await import('./App.svelte')}
        <p class="loading" role="status">Loading…</p>
      {:then { default: App }}
        {#each opened as profile (profile.id)}
          <div class="account-workspace" hidden={profile.id !== selected}>
            <App {profile} active={profile.id === selected} connect={profile.connect} tourStep={profile.id === selected ? tourPage : null} tourActive={profile.id === selected && tourActive} onTourReadyChange={setTourReady} />
          </div>
        {/each}
      {:catch cause}{@render viewLoadError(cause)}{/await}
    {/if}
    {#if !selected && !error}<p class="loading" role="status">{profiles.length ? 'Loading accounts…' : 'Add an account to get started.'}</p>{/if}
  {/if}
  {/key}
  <GuidedTour bind:this={tour} ready={!blankWorkspace && tourAvailable} onpage={(page) => { tourPage = page; }} onactive={(active) => { tourActive = active; }} />
  {#if current}{#key current.id}<ConnectivityToast profileID={current.id} />{/key}{/if}
</div>

<style>
  .workspace { height: 100%; min-height: 0; display: flex; flex-direction: column; overflow: hidden; overflow: clip; border: .2rem solid var(--window-border); }
  .top-bar { direction: ltr; display: flex; align-items: center; flex-shrink: 0; height: 4.8rem; padding-left: 1.2rem; border-bottom: 1px solid var(--border); background: var(--surface); color: var(--text-secondary); font-size: 1.2rem; user-select: none; }
  .profile-control { --wails-draggable: no-drag; flex-shrink: 0; }
  .drag-area { --wails-draggable: drag; align-self: stretch; flex: 1; min-width: 4rem; }
  .app-menu { --wails-draggable: no-drag; display: flex; min-width: 0; align-self: stretch; align-items: center; gap: .2rem; padding-right: 1rem; margin-right: .4rem; border-right: 1px solid var(--border); }
  .menu-controls { display: contents; }
  .app-menu button { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; padding: .7rem 1rem; border: 0; border-radius: .4rem; background: transparent; color: var(--text-secondary); font-size: 1.2rem; cursor: pointer; }
  .app-menu button[data-guide-trigger] { flex-shrink: 0; }
  .app-menu button:hover:not(:disabled), .app-menu button[aria-expanded=true] { background: var(--surface-hover); color: var(--text); }
  .background-jobs button { border: 1px solid var(--border); background: var(--surface); color: var(--accent); padding: .8rem 1.2rem; border-radius: .6rem; font: inherit; cursor: pointer; }
  .background-jobs button:disabled, .app-menu button:disabled { opacity: .5; cursor: default; }
  .background-jobs button:focus-visible, .app-menu button:focus-visible { outline: 2px solid var(--focus); outline-offset: 2px; }
  .account-workspace { flex: 1; min-height: 0; overflow: hidden; overflow: clip; }
  .account-workspace[hidden] { display: none; }
  .profile-error, .loading { padding: 1.2rem 2.4rem; font-size: var(--ui-body-size); color: var(--danger); }
  .background-jobs { display: flex; align-items: center; flex-wrap: wrap; gap: .8rem; padding: .8rem 2.4rem; font-size: 1.2rem; background: var(--surface-active); color: var(--text-strong); }
  @media (max-width: 1200px) {
    .top-bar { height: auto; flex-wrap: wrap; padding-left: 0; }
    .profile-control { margin-left: 1.2rem; }
    .profile-control :global(.account-dropdown) { width: 17rem; }
    .menu-controls { display: flex; flex-shrink: 0; align-items: center; gap: .2rem; min-width: 0; border: 0; padding: 0; margin: 0; }
    .app-menu button { padding-inline: .7rem; }
    .app-menu { order: 1; flex-basis: 100%; min-height: 4.2rem; overflow-x: auto; padding: .4rem .8rem; margin-right: 0; border-right: 0; border-top: 1px solid var(--border); }
    .drag-area { min-width: 1.2rem; min-height: 4.8rem; }
  }
</style>
