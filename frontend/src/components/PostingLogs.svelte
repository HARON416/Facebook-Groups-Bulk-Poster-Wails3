<script lang="ts">
  import { onMount } from 'svelte';
  import { ProfileService, type AccountProfile } from '../../bindings/postika';
  import { createAccountService, windowProfileID, windowAccountName } from '../lib/accounts';
  import { pollWhileWindowOpen } from '../lib/window-polling';
  import AuxiliaryWindow from './AuxiliaryWindow.svelte';
  import AccountDropdown from './AccountDropdown.svelte';
  import LocalizedFeedback from './LocalizedFeedback.svelte';
  import PostingLogView from './PostingLogView.svelte';

  let profiles = $state<AccountProfile[]>([{ id: windowProfileID, name: windowAccountName }]);
  let selected = $state(windowProfileID);
  let selectionVersion = 0;
  let error = $state('');
  const profile = $derived(profiles.find((item) => item.id === selected) ?? profiles[0]);

  // This selection belongs to the log viewer, independently of the main workspace.
  function selectProfile(id: string) {
    const next = profiles.find((item) => item.id === id);
    if (!next) return;
    selectionVersion++;
    selected = id;
    syncProfileURL(next);
  }
  function syncProfileURL(next: AccountProfile) {
    const id = next.id;
    const url = new URL(window.location.href);
    if (url.searchParams.get('profile') !== id || url.searchParams.get('account') !== next.name) {
      url.searchParams.set('profile', id);
      url.searchParams.set('account', next.name);
      window.history.replaceState(null, '', url);
    }
  }
  function profileRenamed(profile: AccountProfile) {
    profiles = profiles.map(entry => entry.id === profile.id ? profile : entry);
    if (selected === profile.id) syncProfileURL(profile);
  }
  async function refreshProfiles() {
    try {
      const result = await ProfileService.GetProfiles();
      profiles = result.profiles ?? [];
      if (!profiles.some(profile => profile.id === selected)) {
        selected = profiles.find(profile => profile.id === result.selected)?.id ?? profiles[0]?.id ?? '';
      }
      const current = profiles.find(profile => profile.id === selected);
      if (current) syncProfileURL(current);
    } catch { /* Keep the last available account list while the next poll retries. */ }
  }
  async function profileCreated(next: AccountProfile) {
    profiles = [...profiles, next];
    selectProfile(next.id);
    try { await createAccountService(next.id).OpenJoinedGroups(); }
    catch (cause) { error = cause instanceof Error ? cause.message : String(cause); }
  }
  onMount(() => {
    const openProfile = async () => {
      // Native reopen updates the URL before dispatching this event, including
      // when the retained webview was still starting up on the first request.
      const version = ++selectionVersion;
      const requested = new URLSearchParams(window.location.search).get('profile');
      await refreshProfiles();
      if (version === selectionVersion && requested) selectProfile(requested);
    };
    window.addEventListener('postika:logs-profile', openProfile);
    void openProfile();
    const stop = pollWhileWindowOpen(refreshProfiles, 2000);
    return () => { stop(); window.removeEventListener('postika:logs-profile', openProfile); };
  });
</script>

{#if error}<div role="alert"><LocalizedFeedback message={error} /></div>{/if}
{#if profile}
  {#key profile.id}
    <PostingLogView {profile} {profiles} onselect={selectProfile} onrenamed={profileRenamed} oncreated={profileCreated} ondeleted={() => void refreshProfiles()} />
  {/key}
{:else}
  <AuxiliaryWindow title="Posting logs">
    {#snippet profileSwitcher()}<AccountDropdown {profiles} {selected} onselect={selectProfile} oncreated={profileCreated} compact />{/snippet}
    <p class="empty">Add an account to get started.</p>
  </AuxiliaryWindow>
{/if}

<style>.empty { padding: 2.4rem; color: var(--text-secondary); }</style>
