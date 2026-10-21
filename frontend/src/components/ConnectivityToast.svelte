<script lang="ts">
  import { onMount, untrack } from 'svelte';
  import { createAccountService, windowProfileID } from '../lib/accounts';
  let { profileID = windowProfileID } = $props<{ profileID?: string }>();
  const GroupsService = untrack(() => createAccountService(profileID));
  import { pollWhileWindowOpen } from '../lib/window-polling';

  let visible = $state(false);
  let online = $state(false);
  let manualPause = $state(false);
  let previous: boolean | undefined;
  let dismissTimer: ReturnType<typeof setTimeout> | undefined;

  onMount(() => {
    let disposed = false;
    const stop = pollWhileWindowOpen(async () => {
      try {
        const status = await GroupsService.GetConnectivityStatus();
        if (disposed || !status.known) return;
        manualPause = status.manualPause;
        if (previous === status.online) return;
        const initial = previous === undefined;
        previous = status.online;
        online = status.online;
        clearTimeout(dismissTimer);
        visible = !initial || !online;
        if (visible && online) dismissTimer = setTimeout(() => { visible = false; }, 5000);
      } catch { /* A binding failure is not an internet connectivity signal. */ }
    }, 1000);
    return () => { disposed = true; stop(); clearTimeout(dismissTimer); };
  });
</script>

{#if visible}
  <aside class="connection-toast" class:online role="status" aria-live="polite" aria-atomic="true">
    <svg class="indicator" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      {#if online}<circle cx="12" cy="12" r="9" /><path d="m8 12 3 3 5-6" />
      {:else}<path d="m10.3 4-8 14a2 2 0 0 0 1.7 3h16a2 2 0 0 0 1.7-3l-8-14a2 2 0 0 0-3.4 0Z" /><path d="M12 9v4m0 3v.1" />{/if}
    </svg>
    <div><strong>{online ? 'You’re back online' : 'Internet connection lost'}</strong>
      <p>{online ? manualPause ? 'Your queue is still manually paused.' : 'Your queue will continue automatically.' : 'Your queue is paused. We’ll continue when the connection returns.'}</p>
    </div>
    <button type="button" aria-label="Dismiss connection notification" onclick={() => { visible = false; }}>
      <svg viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="m6 6 12 12M18 6 6 18" /></svg>
    </button>
  </aside>
{/if}

<style>
  .connection-toast { position: fixed; z-index: 1000; right: 2.4rem; bottom: 2.4rem; display: flex; align-items: flex-start; gap: 1.2rem; width: min(38.4rem, calc(100vw - 4.8rem)); padding: 1.6rem; border: 1px solid var(--toast-border); border-bottom: 2px solid var(--warning-border); border-radius: .9rem; background: var(--toast-bg); color: var(--toast-text); box-shadow: 0 8px 30px var(--shadow); }
  .connection-toast.online { border-color: var(--toast-success-border); border-bottom-color: var(--toast-success); }
  svg { width: 2rem; height: 2rem; stroke: currentColor; stroke-width: 1.5; stroke-linecap: round; stroke-linejoin: round; }
  .indicator { flex-shrink: 0; color: var(--warning-border); }
  .online .indicator { color: var(--toast-success); }
  strong { font-size: 1.4rem; font-weight: 550; line-height: 1.5; }
  p { margin-top: .4rem; font-size: 1.3rem; line-height: 1.5; color: var(--toast-muted); }
  button { display: grid; place-items: center; flex-shrink: 0; width: 2.4rem; height: 2.4rem; margin-left: auto; border: 0; border-radius: .4rem; background: transparent; color: var(--toast-muted); cursor: pointer; }
  button:hover { background: var(--toast-hover); color: var(--toast-text); }
  button:focus-visible { outline: 2px solid var(--warning-border); outline-offset: 2px; }
</style>
