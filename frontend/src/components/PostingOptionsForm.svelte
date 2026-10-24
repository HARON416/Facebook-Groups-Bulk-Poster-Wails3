<script lang="ts">
  import { logMessage } from './LogMessage.svelte';
  function intervalInvalid(event: Event) {
    (event.currentTarget as HTMLInputElement).setCustomValidity(logMessage(/* @wc-ignore */ 'Set an interval between 60 and 86400 seconds, with the maximum at least the minimum'));
  }
  function intervalInput(event: Event) {
    const input = event.currentTarget as HTMLInputElement;
    input.closest('.intervals')?.querySelectorAll('input').forEach(field => field.setCustomValidity(''));
  }
  let { minimumInterval = $bindable(60), maximumInterval = $bindable(120), disabled = false } = $props<{ minimumInterval?: number; maximumInterval?: number; disabled?: boolean }>();
</script>

<fieldset {disabled}>
  <legend class="visually-hidden">Posting preferences</legend>
  <section class="option-card">
    <div class="section-heading"><span class="eyebrow">POSTING PACE</span><h2>Interval between posts</h2></div>
    <p>Choose how long to wait before posting to the next group.</p>
    <div class="intervals">
      <label><span>Minimum</span><span class="input-wrap"><input aria-label="Minimum (seconds)" type="number" min="60" max="86400" step="1" bind:value={minimumInterval} oninvalid={intervalInvalid} oninput={intervalInput} required /><span>seconds</span></span></label>
      <label><span>Maximum</span><span class="input-wrap"><input aria-label="Maximum (seconds)" type="number" min={minimumInterval || 60} max="86400" step="1" bind:value={maximumInterval} oninvalid={intervalInvalid} oninput={intervalInput} required /><span>seconds</span></span></label>
    </div>
    <p class="hint">At least 60 seconds. Use equal values for a fixed interval.</p>
  </section>
</fieldset>

<style>
  fieldset { border: 0; min-width: 0; display: grid; gap: 1.6rem; }
  .visually-hidden { position: absolute; width: 1px; height: 1px; clip-path: inset(50%); overflow: hidden; }
  .option-card { background: var(--surface); border: 1px solid var(--border); border-radius: var(--ui-panel-radius); padding: var(--ui-space); }
  .eyebrow { display: block; color: var(--accent); font-size: 1rem; letter-spacing: .12em; font-weight: 650; margin-bottom: .8rem; }
  h2 { font-size: var(--ui-title-size); font-weight: var(--ui-title-weight); line-height: 1.5; color: var(--text-strong); }
  p { font-size: 1.3rem; line-height: 1.6; color: var(--text-secondary); margin: .8rem 0 1.8rem; }
  .intervals { display: grid; grid-template-columns: 1fr 1fr; gap: 1.6rem; }
  label { display: flex; flex-direction: column; gap: var(--ui-field-gap); font-size: var(--ui-body-size); font-weight: var(--ui-label-weight); color: var(--text); }
  .input-wrap { display: flex; align-items: center; border: 1px solid var(--border-strong); border-radius: .6rem; overflow: hidden; }
  .input-wrap > span { padding-inline-end: 1.2rem; font-size: 1.2rem; color: var(--text-muted); font-weight: 400; }
  input { font-size: var(--ui-body-size); width: 100%; min-width: 0; padding: 1.2rem; border: 0; background: transparent; color: var(--text); }
  .hint { margin: .9rem 0 0; font-size: 1.2rem; }
  :disabled { opacity: .6; }
  .input-wrap:focus-within { outline: 2px solid var(--focus); outline-offset: 2px; }
  input:focus { outline: none; }
  @media (max-width: 480px) { .option-card { padding: var(--ui-space); } .intervals { gap: 1rem; } }
</style>
