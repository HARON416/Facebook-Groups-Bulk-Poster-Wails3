<script lang="ts">
  import { onMount } from 'svelte';
  import { ProfileService, type AccountProfile } from '../../bindings/postika';
  import LocalizedFeedback from './LocalizedFeedback.svelte';
  import { logMessage } from './LogMessage.svelte';

  let { profile, onrenamed, onclose }: { profile: AccountProfile; onrenamed: (profile: AccountProfile) => void; onclose: () => void } = $props();
  let name = $state(''), busy = $state(false), error = $state('');
  let dialog: HTMLDialogElement;
  let input: HTMLInputElement;
  onMount(() => { name = profile.name; dialog.showModal(); input.focus(); });

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
  async function save(event: SubmitEvent) {
    event.preventDefault(); event.stopPropagation();
    const form = event.currentTarget as HTMLFormElement;
    if (!dialog.open || busy || event.target !== form || event.submitter !== form.querySelector('button[type="submit"]') || !form.reportValidity() || !name.trim() || name.trim() === profile.name) return;
    busy = true; error = '';
    try {
      const renamed = await ProfileService.RenameProfile(profile.id, name);
      onrenamed(renamed);
      dialog.close();
    } catch (cause) { error = cause instanceof Error ? cause.message : String(cause); }
    finally { busy = false; }
  }
</script>

<dialog bind:this={dialog} id="rename-profile-dialog" class="ui-dialog" aria-labelledby="rename-profile-heading" onclick={backdrop} oncancel={(event) => { event.preventDefault(); dismiss(); }} {onclose}>
  <form onsubmit={save}>
    <header class="ui-dialog-heading">
      <h2 id="rename-profile-heading" class="ui-title">Rename profile</h2>
      <button type="button" class="ui-dialog-close" disabled={busy} onclick={dismiss}>Close</button>
    </header>
    <div class="fields">
      <label for="rename-profile-name">Account name</label>
      <input bind:this={input} id="rename-profile-name" name="postika-profile-name" class="ui-input" autocomplete="off" bind:value={name} required maxlength="80" disabled={busy} oninput={(event) => validate(event.currentTarget)} oninvalid={(event) => validate(event.currentTarget)} />
      {#if error}<div class="error" role="alert"><LocalizedFeedback message={error} /></div>{/if}
    </div>
    <footer class="ui-dialog-footer">
      <button type="button" class="action-button action-button--secondary" disabled={busy} onclick={dismiss}>Cancel</button>
      <button type="submit" class="action-button action-button--primary" disabled={busy || !name.trim() || name.trim() === profile.name} aria-busy={busy}>{busy ? 'Saving…' : 'Save changes'}</button>
    </footer>
  </form>
</dialog>

<style>
  dialog { width: min(48rem, calc(100vw - 3.2rem)); max-height: calc(100dvh - 3.2rem); overflow-y: auto; overscroll-behavior: contain; }
  .fields { display: grid; gap: var(--ui-field-gap); padding: var(--ui-space); }
  label { color: var(--text); font-weight: var(--ui-label-weight); }
  .error { color: var(--danger); line-height: 1.6; overflow-wrap: anywhere; }
</style>
