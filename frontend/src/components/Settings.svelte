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
  import { logMessage } from './LogMessage.svelte';
  import LocalizedFeedback from './LocalizedFeedback.svelte';
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
    try { await saveSettings({ minimumInterval, maximumInterval, rewrite, instructions, geminiModel: model, keyConfigured }, key, clearKey); key = ''; clearKey = false; await load(); message = /* @wc-ignore */ 'Settings saved. New runs can use these defaults.'; if (origin) void confetti(origin); }
    catch (cause) { error = cause instanceof Error ? cause.message : String(cause); }
    finally { busy = false; }
  }
</script>

<AuxiliaryWindow title="Settings">
<main class="ui-window">
  <span class="eyebrow">YOUR PREFERENCES</span><h1 class="ui-title">Settings</h1><p class="intro">Set up Gemini and save posting defaults for {accountName}.</p>
  <section class="language-preference"><h2>Language</h2><LanguageMenu /></section>
  <form onsubmit={save}>
    <section class="gemini-card" aria-labelledby="gemini-heading">
      <h2 id="gemini-heading">Gemini</h2>
      <label>API key<input dir="ltr" class="ui-input" type="password" autocomplete="new-password" bind:value={key} disabled={busy || clearKey} placeholder={keyConfigured ? 'Key saved · leave blank to keep it' : 'Enter your Gemini API key'} /></label>
      <p class="hint">{keyConfigured ? 'A key is saved on this device.' : 'No API key saved.'} The saved key is never displayed.</p>
      {#if keyConfigured}<div class="remove"><Checkbox bind:checked={clearKey} disabled={busy} text="Remove saved key" /></div>{/if}
      <label>Model<input class="ui-input" dir="ltr" bind:value={model} disabled={busy} required pattern="[a-zA-Z0-9._\-]+" oninvalid={(event) => event.currentTarget.setCustomValidity(logMessage(/* @wc-ignore */ 'Enter a valid Gemini model name'))} oninput={(event) => event.currentTarget.setCustomValidity('')} /></label>
    </section>
    <section class="defaults"><h2>Posting defaults</h2><p class="defaults-hint">Applied when you start a new posting run.</p><PostingOptionsForm bind:minimumInterval bind:maximumInterval bind:rewrite bind:instructions disabled={busy} /></section>
    {#if error}<div class="error" role="alert"><LocalizedFeedback message={error} /></div>{/if}
    {#if message}<p class="success" role="status">{logMessage(message)}</p>{/if}
    <footer><button type="submit" class="action-button action-button--primary" disabled={busy}>{busy ? 'Saving…' : 'Save settings'}</button></footer>
  </form>
</main>
</AuxiliaryWindow>

<style>
  .language-preference { margin-bottom: 2.4rem; }
  .language-preference h2 { margin-bottom: 1rem; }
  main { width: 100%; max-width: 80rem; margin: auto; padding: var(--ui-space); color: var(--text); }
  .eyebrow { display: block; color: var(--accent); font-size: 1rem; letter-spacing: .12em; font-weight: 650; margin-bottom: 1rem; }
  .intro { font-size: var(--ui-body-size); color: var(--text-secondary); margin: .8rem 0 2.4rem; line-height: 1.6; }
  .gemini-card { background: var(--surface); border: 1px solid var(--border); border-radius: var(--ui-panel-radius); padding: var(--ui-space); }
  .defaults { margin-top: 2.4rem; }
  .defaults h2 { margin-bottom: .6rem; }
  .defaults-hint { font-size: 1.3rem; color: var(--text-secondary); margin-bottom: 1.8rem; }
  h2 { color: var(--text-strong); font-size: var(--ui-title-size); font-weight: var(--ui-title-weight); line-height: 1.5; margin-bottom: var(--ui-space); }
  label { display: flex; flex-direction: column; gap: var(--ui-field-gap); font-size: var(--ui-body-size); font-weight: var(--ui-label-weight); margin-bottom: var(--ui-space); }
  .hint { font-size: 1.3rem; margin-bottom: 1.6rem; color: var(--text-secondary); line-height: 1.6; }
  .remove { margin: 0 0 2rem; }
  .error, .success { font-size: var(--ui-body-size); line-height: 1.6; color: var(--danger); }
  .success { color: var(--accent); }
  footer { display: flex; justify-content: flex-end; padding: var(--ui-space) 0; border-top: 1px solid var(--border); margin-top: var(--ui-space); }
</style>
