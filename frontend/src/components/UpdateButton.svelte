<script lang="ts">
  import { onMount, tick } from 'svelte';
  import { UpdateService, type UpdateStatus } from '../../bindings/postika';
  import { pollWhileWindowOpen } from '../lib/window-polling';
  import { flushDrafts } from '../lib/drafts';
  import LocalizedFeedback from './LocalizedFeedback.svelte';

  let status = $state<UpdateStatus | null>(null);
  let open = $state(false), busy = $state(false), error = $state('');
  const available = $derived(Boolean(status?.version));
  const working = $derived(busy || ['checking', 'downloading', 'installing', 'restarting'].includes(status?.state ?? ''));

  async function refresh() {
    try { status = await UpdateService.GetUpdateStatus(); }
    catch { /* Native services are unavailable in a browser preview. */ }
  }
  onMount(() => pollWhileWindowOpen(refresh, 2000));

  async function check() {
    if (working) return;
    busy = true; error = '';
    try { status = await UpdateService.CheckForUpdates(); }
    catch (cause) { error = cause instanceof Error ? cause.message : String(cause); }
    finally { busy = false; }
  }

  async function update(action: 'retry' | 'restart') {
    if (working) return;
    busy = true; error = '';
    try {
      if (action === 'retry') await UpdateService.DownloadUpdate();
      else {
        await tick();
        await flushDrafts();
        await UpdateService.RestartToUpdate();
      }
      await refresh();
    } catch (cause) { error = cause instanceof Error ? cause.message : String(cause); }
    finally { busy = false; }
  }
</script>

<svelte:window onkeydown={(event) => { if (event.key === 'Escape' && open) open = false; }} />

<div class="update-control">
  <button class:available type="button" aria-expanded={open} aria-controls={open ? 'update-panel' : undefined} onclick={() => { open = !open; }}>{status?.state === 'ready' ? 'Restart to update' : available ? 'Update available' : 'Updates'}</button>
  {#if open}
    <section id="update-panel" aria-label="Updates">
      <div class="heading"><strong>Updates</strong><button type="button" onclick={() => { open = false; }}>Close</button></div>
      <p dir="auto">{/* @wc-ignore */ 'Postika'} {status?.currentVersion ?? ''}{#if status?.version} → {status.version}{/if}</p>
      <div role="status" aria-live="polite">
        {#if status?.state === 'checking'}<p>Checking for updates…</p>
        {:else if status?.state === 'downloading'}<p>Downloading update…</p>
        {:else if status?.state === 'ready'}<p>Update ready. Stop all posting queues before restarting.</p>
        {:else if status?.state === 'installing' || status?.state === 'restarting'}<p>Installing update…</p>
        {:else if status?.state === 'up-to-date'}<p>Postika is up to date.</p>{/if}
      </div>
      {#if error || status?.error}<p class="error" role="alert"><LocalizedFeedback message={error || status?.error || ''} /></p>{/if}
      <div class="actions">
        {#if status?.state === 'ready'}<button class="primary" type="button" disabled={working} onclick={() => update('restart')}>Restart to update</button>
        {:else if status?.state === 'available'}<button class="primary" type="button" disabled={working} onclick={() => update('retry')}>Download update</button>{/if}
        <button type="button" disabled={working || status?.state === 'ready'} onclick={check}>Check for updates</button>
      </div>
    </section>
  {/if}
</div>

<style>
  .update-control { position: relative; --wails-draggable: no-drag; }
  button { border: 1px solid var(--border); border-radius: .5rem; padding: .7rem 1rem; color: var(--text); background: var(--surface); font: inherit; cursor: pointer; white-space: nowrap; }
  .update-control > button { min-height: 3.2rem; border: 0; border-radius: .4rem; background: transparent; color: var(--text-secondary); font-size: var(--ui-small-size); }
  .update-control > button:hover, .update-control > button[aria-expanded=true] { background: var(--surface-hover); color: var(--text); filter: none; }
  .update-control > button.available { background: var(--warning-bg); color: var(--warning); font-weight: var(--ui-title-weight); }
  button:hover { filter: brightness(.94); }
  button:disabled { opacity: .5; cursor: default; }
  button:focus-visible { outline: 2px solid var(--focus); outline-offset: 2px; }
  section { position: absolute; z-index: 100; inset-inline-end: 0; top: calc(100% + 1.1rem); width: min(36rem, calc(100vw - 3rem)); padding: 1.6rem; border: 1px solid var(--border); border-radius: .8rem; background: var(--surface); color: var(--text); box-shadow: 0 .8rem 2.4rem var(--shadow-strong); font-size: var(--ui-body-size); user-select: text; }
  .heading, .actions { display: flex; gap: .8rem; align-items: center; justify-content: space-between; }
  .actions { flex-wrap: wrap; justify-content: flex-start; margin-top: 1.2rem; }
  p { margin-top: 1rem; overflow-wrap: anywhere; line-height: 1.5; }
  .primary { border-color: var(--accent); color: var(--accent); }
  .error { color: var(--danger); }
</style>
