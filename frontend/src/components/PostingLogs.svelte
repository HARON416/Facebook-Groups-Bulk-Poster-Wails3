<script lang="ts">
  import { onMount } from 'svelte';
  import { ProfileService, type AccountProfile } from '../../bindings/postika';
  import { windowProfileID, windowAccountName } from '../lib/accounts';
  import { pollWhileWindowOpen } from '../lib/window-polling';
  import PostingLogView from './PostingLogView.svelte';

  let profiles = $state<AccountProfile[]>([{ id: windowProfileID, name: windowAccountName }]);
  let selected = $state(windowProfileID);
  let selectionVersion = 0;
  const profile = $derived(profiles.find((item) => item.id === selected) ?? profiles[0]);

  // This selection belongs to the log viewer, independently of the main workspace.
  function selectProfile(id: string) {
    const next = profiles.find((item) => item.id === id);
    if (!next) return;
    selectionVersion++;
    selected = id;
    const url = new URL(window.location.href);
    if (url.searchParams.get('profile') !== id || url.searchParams.get('account') !== next.name) {
      url.searchParams.set('profile', id);
      url.searchParams.set('account', next.name);
      window.history.replaceState(null, '', url);
    }
  }
  async function refreshProfiles() {
    try {
      const result = await ProfileService.GetProfiles();
      if (result.profiles?.length) profiles = result.profiles;
    } catch { /* Keep the last available account list while the next poll retries. */ }
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

{#key profile.id}
  <PostingLogView {profile} {profiles} onselect={selectProfile} />
{/key}
