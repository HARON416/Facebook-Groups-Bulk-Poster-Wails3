<script lang="ts">
  import { onMount } from 'svelte';
  import { hasLogDetails } from '../lib/log-message';
  import { feedbackMessage, logMessage } from './LogMessage.svelte';
  import { createAccountService } from '../lib/accounts';
  import { pollWhileWindowOpen } from '../lib/window-polling';
  import type { FacebookPageFetchStatus, FacebookPagePost } from '../../bindings/postika/models';

  let { profileID, onfetched }: { profileID: string; onfetched: (posts: FacebookPagePost[]) => void } = $props();
  const service = $derived(createAccountService(profileID));
  let dialog: HTMLDialogElement;
  let trigger: HTMLButtonElement;
  let expanded = $state(false);
  let pageURL = $state('');
  let postCount = $state<number | undefined>(10);
  let starting = $state(false);
  let error = $state('');
  let generation = 0;
  let fetchingRun = 0;
  let status = $state<FacebookPageFetchStatus | null>(null);
  const statusMessage = $derived(error ? feedbackMessage(error) : status?.state === 'error' ? feedbackMessage(status.message) : logMessage(status?.message || ''));
  const busy = $derived(starting || !!status && !['idle', 'complete', 'error', 'cancelled'].includes(status.state));
  const dialogID = $derived(`facebook-page-dialog-${profileID}`);

  function receiveStatus(next: FacebookPageFetchStatus) {
    const terminal = ['idle', 'complete', 'error', 'cancelled'].includes(next.state);
    // Opening the form again must not resubmit a previous completed fetch.
    if (!fetchingRun && terminal) return;
    if (!fetchingRun) fetchingRun = next.runID;
    status = next;
    if (next.state === 'complete' && next.runID === fetchingRun) {
      fetchingRun = 0;
      onfetched(next.posts ?? []);
      generation++;
      dialog.close();
    }
  }

  async function refresh() {
    if (!expanded || starting || error) return;
    const current = generation;
    try {
      const next = await service.GetFacebookPageFetchStatus();
      if (current === generation && (!status || next.runID >= status.runID)) receiveStatus(next);
    } catch { if (current === generation) error = /* @wc-ignore */ 'Could not check fetch progress. Close this dialog and try again.'; }
  }

  onMount(() => pollWhileWindowOpen(refresh, 750));

  async function fetchPosts(event: SubmitEvent) {
    event.preventDefault();
    if (busy || !postCount || !Number.isSafeInteger(postCount)) return;
    generation++; starting = true; error = ''; status = null;
    try {
      const next = await service.StartFacebookPageFetch(pageURL.trim(), postCount);
      fetchingRun = next.runID;
      receiveStatus(next);
    }
    catch (cause) { error = cause instanceof Error ? cause.message : String(cause); }
    finally { starting = false; }
  }

  async function dismiss() {
    if (starting) return;
    if (busy && status) {
      try { await service.CancelFacebookPageFetch(status.runID); }
      catch { error = /* @wc-ignore */ 'Could not stop fetching. Try again.'; return; }
      status = { ...status, state: 'cancelled', message: /* @wc-ignore */ 'Fetching stopped.' };
    }
    fetchingRun = 0;
    generation++; dialog.close();
  }
  function validateURL(input: HTMLInputElement) {
    let valid = false;
    try {
      const url = new URL(input.value.trim());
      const parts = url.pathname.split('/').filter(Boolean);
      const reserved = new Set(['groups', 'events', 'watch', 'reel', 'reels', 'stories', 'marketplace', 'login', 'share', 'photo', 'photos', 'photo.php', 'permalink.php', 'story.php', 'posts', 'videos', 'gaming', 'search', 'help', 'settings', 'pages', 'checkpoint']);
      const pagePath = parts.length === 1 && /^[a-zA-Z0-9.]+$/.test(parts[0]) && !reserved.has(parts[0].toLowerCase());
      const profilePath = parts.length === 1 && parts[0] === 'profile.php' && /^\d+$/.test(url.searchParams.get('id') ?? '');
      const namedPagePath = parts.length === 3 && parts[0] === 'pages' && /^\d+$/.test(parts[2]);
      valid = ['https:', 'http:'].includes(url.protocol) &&
        ['facebook.com', 'www.facebook.com', 'web.facebook.com', 'm.facebook.com', 'mbasic.facebook.com'].includes(url.hostname) &&
        !url.username && !url.password && !url.port &&
        (profilePath || namedPagePath || (pagePath && parts[0] !== 'profile.php'));
    } catch { /* The field's validation message covers malformed URLs. */ }
    input.setCustomValidity(valid ? '' : logMessage(/* @wc-ignore */ 'Enter a Facebook page URL, such as https://web.facebook.com/swiftlineMotors'));
  }
  function adjustCount(change: number) {
    postCount = Math.max(1, Math.min(Number.MAX_SAFE_INTEGER, Math.trunc(postCount || 1) + change));
    dialog.querySelector<HTMLInputElement>('input[type="number"]')?.setCustomValidity('');
  }
  function backdrop(event: MouseEvent) {
    if (event.target !== dialog) return;
    const bounds = dialog.getBoundingClientRect();
    if (event.clientX < bounds.left || event.clientX > bounds.right || event.clientY < bounds.top || event.clientY > bounds.bottom) void dismiss();
  }
</script>
<button bind:this={trigger} type="button" class="facebook-page-option" aria-haspopup="dialog" aria-expanded={expanded} aria-controls={dialogID} onclick={() => { error = ''; dialog.showModal(); expanded = true; void refresh(); }}>
  Or click here to fetch posts from any Facebook page
</button>
<dialog class="ui-dialog" bind:this={dialog} id={dialogID} aria-labelledby={`${dialogID}-heading`} onclick={backdrop} oncancel={(event) => { event.preventDefault(); void dismiss(); }} onclose={() => { expanded = false; trigger.focus(); }}>
  <form onsubmit={fetchPosts}>
    <header class="ui-dialog-heading">
      <h2 class="ui-title" id={`${dialogID}-heading`}>Fetch posts from a Facebook page</h2>
      <button type="button" class="close ui-dialog-close" aria-label="Close" disabled={starting} onclick={dismiss}>
        <svg viewBox="0 0 24 24" aria-hidden="true"><path d="m6 6 12 12M18 6 6 18" /></svg>
      </button>
    </header>
    <fieldset class="fields" disabled={busy}>
      <legend class="sr-only">Facebook page fetch options</legend>
      <div class="field">
        <label for={`${dialogID}-url`}>Facebook page URL</label>
        <input id={`${dialogID}-url`} class="ui-input" dir="ltr" type="url" bind:value={pageURL} placeholder="https://web.facebook.com/swiftlineMotors" required oninput={(event) => validateURL(event.currentTarget)} oninvalid={(event) => validateURL(event.currentTarget)} />
      </div>
      <div class="field">
        <label for={`${dialogID}-count`}>Number of posts to fetch</label>
        <div class="number-control">
          <button type="button" aria-label="Decrease number of posts" disabled={postCount !== undefined && postCount <= 1} onclick={() => adjustCount(-1)}><svg viewBox="0 0 24 24" aria-hidden="true"><path d="M6 12h12" /></svg></button>
          <input id={`${dialogID}-count`} type="number" bind:value={postCount} min="1" max={Number.MAX_SAFE_INTEGER} step="1" required oninvalid={(event) => event.currentTarget.setCustomValidity(logMessage(/* @wc-ignore */ 'Enter a positive whole number of posts to fetch'))} oninput={(event) => event.currentTarget.setCustomValidity('')} />
          <button type="button" aria-label="Increase number of posts" disabled={postCount !== undefined && postCount >= Number.MAX_SAFE_INTEGER} onclick={() => adjustCount(1)}><svg viewBox="0 0 24 24" aria-hidden="true"><path d="M6 12h12M12 6v12" /></svg></button>
        </div>
      </div>
    </fieldset>
    {#if error || status && status.state !== 'idle'}
      <div class="fetch-status" class:error={!!error || status?.state === 'error'} role="status" aria-live="polite">
        <p>{statusMessage}</p>
        {#if (error || status?.state === 'error') && hasLogDetails(error || status?.message || '')}
          <details><summary>Debug details</summary><p dir="auto">{error || status?.message}</p></details>
        {/if}
        {#if busy && status}
          <progress max={status.target} value={status.posts?.length ?? 0} aria-label="Posts fetched"></progress>
        {/if}
      </div>
    {/if}
    <footer class="ui-dialog-footer">
      <button type="button" class="action-button action-button--secondary" disabled={starting} onclick={dismiss}>Cancel</button>
      <button type="submit" class="action-button action-button--primary" disabled={busy}>{busy ? 'Fetching…' : 'Fetch'}</button>
    </footer>
  </form>
</dialog>
<style>
  .facebook-page-option { justify-self: start; padding: 0; border: 0; background: transparent; color: var(--accent); font-size: 1.2rem; font-weight: 700; line-height: 1.5; text-align: start; text-decoration: underline; text-underline-offset: .2rem; cursor: pointer; }
  .facebook-page-option:hover { color: var(--accent-hover); }
  dialog { width: min(48rem, calc(100vw - 3.2rem)); max-height: calc(100dvh - 3.2rem); overflow-y: auto; overscroll-behavior: contain; }
  svg { width: 1.6rem; height: 1.6rem; fill: none; stroke: currentColor; stroke-width: 1.5; stroke-linecap: round; stroke-linejoin: round; }
  .fields { display: grid; min-width: 0; gap: var(--ui-space); padding: var(--ui-space); border: 0; }
  .fields:disabled { opacity: .6; }
  .sr-only { position: absolute; width: 1px; height: 1px; overflow: hidden; clip-path: inset(50%); }
  .fetch-status { margin: 0 var(--ui-space) var(--ui-space); color: var(--text-secondary); line-height: 1.6; overflow-wrap: anywhere; }
  .fetch-status.error { color: var(--danger); }
  progress { display: block; width: 100%; height: .6rem; margin-top: 1rem; accent-color: var(--accent); }
  .field { display: grid; min-width: 0; gap: var(--ui-field-gap); }
  label { color: var(--text); font-size: var(--ui-body-size); font-weight: var(--ui-label-weight); }
  .number-control { display: flex; width: 16rem; max-width: 100%; overflow: hidden; border: 1px solid var(--border-strong); border-radius: .6rem; }
  .number-control button { display: grid; place-items: center; width: 4rem; min-height: 4rem; flex-shrink: 0; border: 0; background: transparent; color: var(--text-secondary); cursor: pointer; }
  .number-control button:hover:not(:disabled) { background: var(--surface-hover); }
  .number-control button:disabled { opacity: .45; cursor: not-allowed; }
  .number-control input { width: 100%; min-width: 0; border: 0; background: var(--surface); text-align: center; font-size: 1.3rem; font-variant-numeric: tabular-nums; appearance: textfield; -moz-appearance: textfield; }
  .number-control input::-webkit-inner-spin-button, .number-control input::-webkit-outer-spin-button { appearance: none; margin: 0; }
  .facebook-page-option:focus-visible { outline: 2px solid var(--focus); outline-offset: 2px; }
  .number-control:focus-within { outline: 2px solid var(--focus); outline-offset: 2px; }
  .number-control input:focus { outline: none; }
  .number-control button:focus-visible { outline: 2px solid var(--focus); outline-offset: -2px; }
</style>
