<script lang="ts">
  import { onMount } from 'svelte';
  import { ProfileService, type AccountProfile } from '../../bindings/postika';
  import { deleteDraft } from '../lib/drafts';
  import { deleteItemListingDraft } from '../lib/marketplace-drafts';
  import { accountLabel } from './AccountLabel.svelte';
  import LocalizedFeedback from './LocalizedFeedback.svelte';

  let { profile, ondeleted, onclose }: { profile: AccountProfile; ondeleted?: (id: string) => void; onclose: () => void } = $props();
  let busy = $state(false), error = $state('');
  let removed = false;
  let dialog: HTMLDialogElement, cancel: HTMLButtonElement;
  onMount(() => { dialog.showModal(); cancel.focus(); });
  function dismiss() { if (!busy) dialog.close(); }
  function backdrop(event: MouseEvent) {
    if (event.target !== dialog) return;
    const bounds = dialog.getBoundingClientRect();
    if (event.clientX < bounds.left || event.clientX > bounds.right || event.clientY < bounds.top || event.clientY > bounds.bottom) dismiss();
  }
  async function remove() {
    if (!dialog.open || busy) return;
    busy = true; error = '';
    try {
      if (!removed) { await ProfileService.DeleteProfile(profile.id); removed = true; }
      await deleteDraft(profile.id);
      await deleteItemListingDraft(profile.id);
      ondeleted?.(profile.id);
      dialog.close();
    } catch (cause) { error = cause instanceof Error ? cause.message : String(cause); }
    finally { busy = false; }
  }
</script>

<dialog bind:this={dialog} id="delete-profile-dialog" class="ui-dialog" aria-labelledby="delete-profile-heading" aria-describedby="delete-profile-hint" onclick={backdrop} oncancel={(event) => { event.preventDefault(); dismiss(); }} {onclose}>
  <header class="ui-dialog-heading">
    <h2 id="delete-profile-heading" class="ui-title">Delete profile</h2>
    <button type="button" class="ui-dialog-close" disabled={busy} onclick={dismiss}>Close</button>
  </header>
  <div class="fields">
    <p class="profile-name" dir="auto">{accountLabel(profile)}</p>
    <p id="delete-profile-hint">This permanently deletes this profile’s Facebook login, groups, drafts, images, settings, and logs. Any running or scheduled queue will stop. Posts already on Facebook are kept.</p>
    {#if error}<div class="error" role="alert"><LocalizedFeedback message={error} /></div>{/if}
  </div>
  <footer class="ui-dialog-footer">
    <button bind:this={cancel} type="button" class="action-button action-button--secondary" disabled={busy} onclick={dismiss}>Cancel</button>
    <button type="button" class="action-button delete-confirm" disabled={busy} aria-busy={busy} onclick={remove}>{busy ? 'Deleting…' : 'Delete profile'}</button>
  </footer>
</dialog>

<style>
  dialog { width: min(48rem, calc(100vw - 3.2rem)); max-height: calc(100dvh - 3.2rem); overflow-y: auto; overscroll-behavior: contain; }
  .fields { display: grid; gap: var(--ui-field-gap); padding: var(--ui-space); }
  p { color: var(--text-secondary); line-height: 1.6; }
  .profile-name { color: var(--text); font-weight: var(--ui-label-weight); overflow-wrap: anywhere; }
  .delete-confirm { background: var(--danger-solid); color: var(--on-danger); border-color: var(--danger-solid); }
  .delete-confirm:hover:not(:disabled) { filter: brightness(.9); }
  .error { color: var(--danger); line-height: 1.6; overflow-wrap: anywhere; }
</style>
