<script lang="ts">
  import { onMount } from 'svelte';
  import { ProfileService, type AccountProfile } from '../../bindings/postika';
  import LocalizedFeedback from './LocalizedFeedback.svelte';
  import { logMessage } from './LogMessage.svelte';

  let { oncreated, onclose }: { oncreated: (profile: AccountProfile) => void; onclose: () => void } = $props();
  let name = $state(''), busy = $state(false), error = $state('');
  let dialog: HTMLDialogElement, input: HTMLInputElement;
  onMount(() => { dialog.showModal(); input.focus(); });
  function validate(field: HTMLInputElement) {
    const isURL = /^(?:[a-z][a-z0-9+.-]*:\/\/|www\.|(?:[a-z0-9-]+\.)*facebook\.com(?:[/?#]|$))/i.test(field.value.trim());
    field.setCustomValidity(isURL ? logMessage(/* @wc-ignore */ 'Account names cannot be URLs') : '');
  }
  function dismiss() { if (!busy) dialog.close(); }
  function backdrop(event: MouseEvent) {
    if (event.target !== dialog) return;
    const bounds = dialog.getBoundingClientRect();
    if (event.clientX < bounds.left || event.clientX > bounds.right || event.clientY < bounds.top || event.clientY > bounds.bottom) dismiss();
  }
  async function add(event: SubmitEvent) {
    event.preventDefault(); event.stopPropagation();
    const form = event.currentTarget as HTMLFormElement;
    if (!dialog.open || busy || event.target !== form || event.submitter !== form.querySelector('button[type="submit"]') || !form.reportValidity()) return;
    busy = true; error = '';
    try { const profile = await ProfileService.CreateProfile(name); oncreated(profile); dialog.close(); }
    catch (cause) { error = cause instanceof Error ? cause.message : String(cause); }
    finally { busy = false; }
  }
</script>

<dialog bind:this={dialog} id="add-account-dialog" class="ui-dialog" aria-labelledby="add-account-heading" aria-describedby="add-account-hint" onclick={backdrop} oncancel={(event) => { event.preventDefault(); dismiss(); }} {onclose}>
  <form id="add-account-form" class="add-account" onsubmit={add}>
    <header class="ui-dialog-heading">
      <h2 id="add-account-heading" class="ui-title">Add account</h2>
      <button type="button" class="ui-dialog-close" disabled={busy} onclick={dismiss}>Close</button>
    </header>
    <div class="fields">
      <label for="account-name">Account name</label>
      <input bind:this={input} id="account-name" name="postika-account-name" autocomplete="off" class="ui-input" bind:value={name} placeholder={/* @wc-include */ 'e.g. Business'} required maxlength="80" disabled={busy} oninput={(event) => validate(event.currentTarget)} oninvalid={(event) => validate(event.currentTarget)} />
      <p id="add-account-hint">Sign in to Facebook in the new account’s browser window.</p>
      {#if error}<div class="error profile-error" role="alert"><LocalizedFeedback message={error} /></div>{/if}
    </div>
    <footer class="ui-dialog-footer">
      <button type="button" class="action-button action-button--secondary" disabled={busy} onclick={dismiss}>Cancel</button>
      <button type="submit" class="action-button action-button--primary" disabled={busy || !name.trim()} aria-busy={busy}>{busy ? 'Creating…' : 'Create & sign in'}</button>
    </footer>
  </form>
</dialog>

<style>
  .ui-dialog-heading { padding-bottom: 1.2rem; border-bottom: 1px solid var(--border); }
  dialog { width: min(48rem, calc(100vw - 3.2rem)); max-height: calc(100dvh - 3.2rem); overflow-y: auto; overscroll-behavior: contain; }
  .fields { display: grid; gap: var(--ui-field-gap); padding: var(--ui-space); }
  label { color: var(--text); font-weight: var(--ui-label-weight); }
  p { margin-top: .4rem; color: var(--text-secondary); line-height: 1.6; }
  .error { color: var(--danger); line-height: 1.6; overflow-wrap: anywhere; }
</style>
