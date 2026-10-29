<script lang="ts">
  import { onMount } from 'svelte';
  import NotificationBar from './NotificationBar.svelte';
  import { notifications, notify, dismissNotification, type NotificationOptions } from '../lib/notifications.svelte';

  // Also accepts plain notifications from native ExecJS and isolated modules.
  onMount(() => {
    const receive = (event: Event) => {
      const options = (event as CustomEvent<NotificationOptions>).detail;
      if (options && typeof options.message === 'string' && options.message.trim()) notify(options);
    };
    window.addEventListener('postika:notification', receive);
    return () => window.removeEventListener('postika:notification', receive);
  });
</script>

<div class="notification-stack">
  {#each notifications.items as item (item.id)}
    <NotificationBar {...item} onclose={() => dismissNotification(item.id)} />
  {/each}
</div>

<style>
  .notification-stack { position: fixed; z-index: 2000; right: max(1.6rem, env(safe-area-inset-right)); bottom: max(1.6rem, env(safe-area-inset-bottom)); display: grid; gap: 1rem; width: min(56rem, calc(100% - 3.2rem)); max-height: calc(100dvh - 3.2rem); overflow-y: auto; padding: .8rem; pointer-events: none; }
  .notification-stack :global(.notification-bar) { pointer-events: auto; }
  .notification-stack:empty { display: none; }
</style>
