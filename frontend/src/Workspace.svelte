<script lang="ts">
  import { onMount } from 'svelte';
  import { ProfileService, type ProfileActivity, type AccountProfile } from '../bindings/postika';
  import App from './App.svelte';
  import AccountDropdown from './components/AccountDropdown.svelte';
  import WindowControls from './components/WindowControls.svelte';
  import UpdateButton from './components/UpdateButton.svelte';
  import LanguageMenu from './components/LanguageMenu.svelte';
  import ThemeMenu from './components/ThemeMenu.svelte';
  import WhatsAppButton from './components/WhatsAppButton.svelte';
  import { createAccountService } from './lib/accounts';
  import { accountLabel } from './components/AccountLabel.svelte';
  import ConnectivityToast from './components/ConnectivityToast.svelte';
  import FeaturesCard from './components/FeaturesCard.svelte';
  import { pollWhileWindowOpen } from './lib/window-polling';
  import LocalizedFeedback from './components/LocalizedFeedback.svelte';

  let profiles = $state<ProfileActivity[]>([]);
  let opened = $state<(AccountProfile & { connect: boolean })[]>([]);
  let selected = $state('');
  let adding = $state(false), name = $state(''), busy = $state(false), error = $state('');
  let windowControls = $state<WindowControls>();
  let groupsLoading = $state<Record<string, boolean>>({});
  const groupsBusy = $derived(groupsLoading[selected] ?? false);
  const current = $derived(profiles.find((profile) => profile.id === selected));
  const displayProfiles = $derived(profiles.map((profile) => ({ ...profile, name: accountLabel(profile) })));
  const background = $derived(profiles.filter((profile) => profile.active && profile.id !== selected));

  function open(profile: AccountProfile, connect = false) {
    if (!opened.some((entry) => entry.id === profile.id)) opened = [...opened, { ...profile, connect }];
    selected = profile.id;
  }
  async function refresh() {
    try {
      const result = await ProfileService.GetProfiles();
      profiles = result.profiles ?? [];
      if (!selected) {
        const profile = profiles.find((profile) => profile.id === result.selected);
        if (profile) open(profile);
      }
    } catch (cause) { error = cause instanceof Error ? cause.message : String(cause); }
  }
  onMount(() => pollWhileWindowOpen(refresh, 2000));

  async function openAccountWindow(view: 'settings' | 'logs') {
    if (!selected || busy || groupsBusy) return;
    error = '';
    const account = createAccountService(selected);
    try { await (view === 'settings' ? account.OpenSettings() : account.OpenPostingLogs()); }
    catch (cause) { error = cause instanceof Error ? cause.message : String(cause); }
  }

  async function select(id: string) {
    if (busy || groupsBusy || id === selected) return;
    busy = true; error = '';
    try {
      await ProfileService.SelectProfile(id);
      const profile = profiles.find((profile) => profile.id === id);
      if (profile) open(profile);
    } catch (cause) { error = cause instanceof Error ? cause.message : String(cause); }
    finally { busy = false; }
  }
  async function add(event: SubmitEvent) {
    event.preventDefault();
    if (busy || groupsBusy) return;
    busy = true; error = '';
    try {
      const profile = await ProfileService.CreateProfile(name);
      profiles = [...profiles, { ...profile, active: false, state: '' }];
      open(profile, true);
      adding = false; name = '';
    } catch (cause) { error = cause instanceof Error ? cause.message : String(cause); }
    finally { busy = false; }
  }
</script>

<div class="workspace ui-window">
  <header class="top-bar">
    <div class="profile-control"><AccountDropdown profiles={displayProfiles} {selected} compact disabled={busy || groupsBusy} onselect={select} /></div>
    <div class="drag-area" aria-hidden="true" ondblclick={() => windowControls?.toggleMaximise()}></div>
    <nav class="app-menu" aria-label="Application menu">
    <fieldset class="menu-controls" disabled={groupsBusy}>
      <UpdateButton />
      <FeaturesCard />
      <button type="button" disabled={busy || groupsBusy} aria-expanded={adding} aria-controls={adding ? 'add-account-form' : undefined} onclick={() => { adding = !adding; error = ''; }}>Add account</button>
      <button type="button" disabled={busy || groupsBusy || !selected} onclick={() => openAccountWindow('logs')}>Posting logs</button>
      <button type="button" disabled={busy || groupsBusy || !selected} onclick={() => openAccountWindow('settings')}>Settings</button>
      <LanguageMenu />
      <ThemeMenu />
      <WhatsAppButton onerror={(message) => { error = message; }} />
    </fieldset>
    </nav>
    <WindowControls bind:this={windowControls} onerror={(message) => { error = message; }} />
  </header>
  {#if adding}
    <form id="add-account-form" class="add-account" onsubmit={add}>
      <label for="account-name">Account name</label>
      <input id="account-name" bind:value={name} placeholder={/* @wc-include */ 'e.g. Business'} required maxlength="80" disabled={busy || groupsBusy} />
      <button disabled={busy || groupsBusy || !name.trim()}>{busy ? 'Creating…' : 'Create & sign in'}</button>
      <button type="button" disabled={busy || groupsBusy} onclick={() => { adding = false; }}>Cancel</button>
      <p>Sign in to Facebook in the new account’s browser window.</p>
    </form>
  {/if}
  {#if error}<div class="profile-error" role="alert"><LocalizedFeedback message={error} /></div>{/if}
  {#if background.length}
    <div class="background-jobs" role="status">In the background:
      {#each background as profile (profile.id)}<button type="button" disabled={busy || groupsBusy} onclick={() => select(profile.id)}>{accountLabel(profile)} · {profile.state === 'scheduled' ? 'Scheduled' : 'Running'}</button>{/each}
    </div>
  {/if}
  {#each opened as profile (profile.id)}
    <div class="account-workspace" hidden={profile.id !== selected}>
      <App {profile} active={profile.id === selected} connect={profile.connect} onGroupsLoadingChange={(loading) => { groupsLoading[profile.id] = loading; }} />
    </div>
  {/each}
  {#if !selected && !error}<p class="loading" role="status">Loading accounts…</p>{/if}
  {#if current}{#key current.id}<ConnectivityToast profileID={current.id} />{/key}{/if}
</div>

<style>
  .workspace { height: 100%; min-height: 0; display: flex; flex-direction: column; overflow: hidden; overflow: clip; border: .2rem solid var(--window-border); }
  .top-bar { direction: ltr; display: flex; align-items: center; flex-shrink: 0; height: 4.8rem; padding-left: 1.2rem; border-bottom: 1px solid var(--border); background: var(--surface); color: var(--text-secondary); font-size: 1.2rem; user-select: none; }
  .profile-control { --wails-draggable: no-drag; flex-shrink: 0; }
  .drag-area { --wails-draggable: drag; align-self: stretch; flex: 1; min-width: 4rem; }
  .app-menu { --wails-draggable: no-drag; display: flex; min-width: 0; align-self: stretch; align-items: center; gap: .2rem; padding-right: 1rem; margin-right: .4rem; border-right: 1px solid var(--border); }
  .menu-controls { display: contents; }
  .app-menu button { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; padding: .7rem 1rem; border: 0; border-radius: .4rem; background: transparent; color: var(--text-secondary); font-size: 1.2rem; }
  .app-menu button:hover:not(:disabled), .app-menu button[aria-expanded=true] { background: var(--surface-hover); color: var(--text); }
  input { background: var(--surface); color: var(--text-strong); border: 1px solid var(--border-strong); border-radius: .6rem; padding: .8rem 1rem; font: inherit; max-width: 28rem; }
  button { border: 1px solid var(--border); background: var(--surface); color: var(--accent); padding: .8rem 1.2rem; border-radius: .6rem; font: inherit; cursor: pointer; }
  button:disabled { opacity: .5; cursor: default; }
  button:focus-visible, input:focus-visible { outline: 2px solid var(--focus); outline-offset: 2px; }
  .add-account { display: flex; align-items: center; flex-wrap: wrap; gap: 1rem; padding: 1.6rem 2.4rem; font-size: 1.3rem; background: var(--surface-active); }
  .add-account p { width: 100%; color: var(--text-secondary); }
  .account-workspace { flex: 1; min-height: 0; overflow: hidden; overflow: clip; }
  .account-workspace[hidden] { display: none; }
  .profile-error, .loading { padding: 1.2rem 2.4rem; font-size: 1.4rem; color: var(--danger); }
  .background-jobs { display: flex; align-items: center; flex-wrap: wrap; gap: .8rem; padding: .8rem 2.4rem; font-size: 1.2rem; background: var(--surface-active); color: var(--text-strong); }
  @media (max-width: 1000px) {
    .profile-control :global(.account-dropdown) { width: 17rem; }
    .app-menu button { padding-inline: .7rem; }
    .app-menu { padding-right: .5rem; margin-right: 0; }
    .drag-area { min-width: 1.2rem; }
  }
</style>
