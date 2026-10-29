<script lang="ts">
  import { onMount } from 'svelte';
  import type { AccountProfile } from '../../bindings/postika';
  import { createAccountService } from '../lib/accounts';
  import LocalizedFeedback from './LocalizedFeedback.svelte';

  let { profile, query, onclose }: { profile: AccountProfile; query: string; onclose: (opened: boolean) => void } = $props();
  let dialog: HTMLDialogElement, cancel: HTMLButtonElement;
  let busy = $state(false), error = $state('');
  let opened = false;
  onMount(() => { dialog.showModal(); cancel.focus(); });
  function dismiss() { if (!busy) dialog.close(); }
  function backdrop(event: MouseEvent) {
    if (event.target !== dialog) return;
    const bounds = dialog.getBoundingClientRect();
    if (event.clientX < bounds.left || event.clientX > bounds.right || event.clientY < bounds.top || event.clientY > bounds.bottom) dismiss();
  }
  async function connect() {
    if (busy) return;
    busy = true; error = '';
    try {
      await createAccountService(profile.id).OpenMarketplaceLocation(query.trim());
      opened = true;
      dialog.close();
    } catch (cause) { error = cause instanceof Error ? cause.message : String(cause); }
    finally { busy = false; }
  }
</script>

<dialog id="location-search-dialog" class="ui-dialog" bind:this={dialog} aria-label="Location" aria-describedby="location-search-hint" onclick={backdrop} oncancel={(event) => { event.preventDefault(); dismiss(); }} onclose={() => onclose(opened)}>
  <div class="fields">
    <p id="location-search-hint">We need to open Facebook to set up location search. If Facebook asks you to log in, complete that step in the browser.</p>
    {#if error}<p class="error" role="alert"><LocalizedFeedback message={error} /></p>{/if}
  </div>
  <footer class="ui-dialog-footer">
    <button bind:this={cancel} type="button" class="action-button action-button--secondary" disabled={busy} onclick={dismiss}>Cancel</button>
    <button type="button" class="action-button action-button--primary location-continue" disabled={busy} aria-busy={busy} onclick={connect}>{busy ? 'Opening browser…' : 'Continue to Facebook'}</button>
  </footer>
</dialog>

<style>
  dialog { width: min(48rem, calc(100vw - 3.2rem)); max-height: calc(100dvh - 3.2rem); overflow-y: auto; overscroll-behavior: contain; }
  .fields { display: grid; gap: 1.2rem; padding: var(--ui-space); }
  p { color: var(--text); line-height: 1.6; }
  .error { color: var(--danger); overflow-wrap: anywhere; }
</style>
