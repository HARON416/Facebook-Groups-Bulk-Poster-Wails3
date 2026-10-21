<script lang="ts">
  import { onMount } from 'svelte';
  import { windowAccountName, windowProfileID } from '../lib/accounts';
  import { accountLabel } from './AccountLabel.svelte';
  import AuxiliaryWindow from './AuxiliaryWindow.svelte';
  import PostingOptionsForm from './PostingOptionsForm.svelte';
  import LanguageMenu from './LanguageMenu.svelte';
  import Checkbox from './Checkbox.svelte';
  import { confetti, elementOrigin } from '../lib/confetti';
  import { getSettings, saveSettings } from '../lib/posting';
  let minimumInterval = $state(60), maximumInterval = $state(120), rewrite = $state(false), instructions = $state('');
  let model = $state('gemini-flash-latest'), key = $state(''), keyConfigured = $state(false), clearKey = $state(false);
  let busy = $state(true), message = $state(''), error = $state('');
  const accountName = $derived(accountLabel({ id: windowProfileID, name: windowAccountName }));
  onMount(() => { void load(); });
  async function load() {
    try { const settings = await getSettings(); minimumInterval = settings.minimumInterval; maximumInterval = settings.maximumInterval; rewrite = settings.rewrite; instructions = settings.instructions; model = settings.geminiModel; keyConfigured = settings.keyConfigured; }
    catch (cause) { error = cause instanceof Error ? cause.message : String(cause); }
    finally { busy = false; }
  }
  async function save(event: SubmitEvent) {
    event.preventDefault(); busy = true; message = ''; error = '';
    const origin = event.submitter instanceof HTMLElement ? elementOrigin(event.submitter) : undefined;
    try { await saveSettings({ minimumInterval, maximumInterval, rewrite, instructions, geminiModel: model, keyConfigured }, key, clearKey); key = ''; clearKey = false; await load(); message = 'Settings saved. New runs can use these defaults.'; if (origin) void confetti(origin); }
    catch (cause) { error = cause instanceof Error ? cause.message : String(cause); }
    finally { busy = false; }
  }
</script>

<AuxiliaryWindow title="Settings">
<main>
  <span class="eyebrow">YOUR PREFERENCES</span><h1>Settings</h1><p class="intro">Set up Gemini and save posting defaults for {accountName}.</p>
  <section class="language-preference"><h2>Language</h2><LanguageMenu /></section>
  <form onsubmit={save}>
    <section class="gemini-card" aria-labelledby="gemini-heading">
      <h2 id="gemini-heading">Gemini</h2>
      <label>API key<input dir="ltr" type="password" autocomplete="new-password" bind:value={key} disabled={busy || clearKey} placeholder={keyConfigured ? 'Key saved · leave blank to keep it' : 'Enter your Gemini API key'} /></label>
      <p class="hint">{keyConfigured ? 'A key is saved on this device.' : 'No API key saved.'} The saved key is never displayed.</p>
      {#if keyConfigured}<div class="remove"><Checkbox bind:checked={clearKey} disabled={busy} text="Remove saved key" /></div>{/if}
      <label>Model<input dir="ltr" bind:value={model} disabled={busy} required pattern="[a-zA-Z0-9._-]+" /></label>
    </section>
    <section class="defaults"><h2>Posting defaults</h2><p class="defaults-hint">Applied when you start a new posting run.</p><PostingOptionsForm bind:minimumInterval bind:maximumInterval bind:rewrite bind:instructions disabled={busy} /></section>
    {#if error}<p class="error" role="alert">{error}</p>{/if}
    {#if message}<p class="success" role="status">{message}</p>{/if}
    <footer><button type="submit" class="action-button action-button--primary" disabled={busy}>{busy ? 'Saving…' : 'Save settings'}</button></footer>
  </form>
</main>
</AuxiliaryWindow>

<style>
  .language-preference { margin-bottom: 2.4rem; }
  .language-preference h2 { margin-bottom: 1rem; }
  main { width: 100%; max-width: 80rem; margin: auto; padding: 3.2rem 3rem; color: var(--text); }
  .eyebrow { display: block; color: var(--accent); font-size: 1rem; letter-spacing: .12em; font-weight: 650; margin-bottom: 1rem; }
  h1 { font-size: 2.4rem; font-weight: 600; }
  .intro { font-size: 1.4rem; color: var(--text-secondary); margin: 1rem 0 3rem; line-height: 1.6; }
  .gemini-card { background: var(--surface); border: 1px solid var(--border); border-radius: 1rem; padding: 2.4rem; }
  .defaults { margin-top: 3rem; }
  .defaults h2 { margin-bottom: .6rem; }
  .defaults-hint { font-size: 1.3rem; color: var(--text-secondary); margin-bottom: 1.8rem; }
  h2 { font-size: 1.7rem; font-weight: 600; margin-bottom: 2rem; }
  label { display: flex; flex-direction: column; gap: 0.8rem; font-size: 1.3rem; margin-bottom: 1.6rem; }
  input:not([type=checkbox]) { width: 100%; padding: 1.2rem; border: 0.1rem solid var(--border-strong); border-radius: 0.6rem; background: var(--surface); }
  .hint { font-size: 1.3rem; margin-bottom: 1.6rem; color: var(--text-secondary); line-height: 1.6; }
  .remove { margin: 0 0 2rem; }
  .error, .success { font-size: 1.4rem; line-height: 1.6; color: var(--danger); }
  .success { color: var(--accent); }
  footer { display: flex; justify-content: flex-end; padding: 2rem 0; }
  input:focus-visible { outline: 0.2rem solid var(--focus); outline-offset: 0.2rem; }
</style>
