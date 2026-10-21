<script lang="ts">
  import Checkbox from './Checkbox.svelte';
  import DisclosureCaret from './DisclosureCaret.svelte';
  let { minimumInterval = $bindable(60), maximumInterval = $bindable(120), rewrite = $bindable(false), instructions = $bindable(''), disabled = false } = $props<{ minimumInterval?: number; maximumInterval?: number; rewrite?: boolean; instructions?: string; disabled?: boolean }>();
</script>

<fieldset {disabled}>
  <legend class="visually-hidden">Posting preferences</legend>
  <section class="option-card">
    <div class="section-heading"><span class="eyebrow">POSTING PACE</span><h2>Interval between posts</h2></div>
    <p>Choose how long to wait before posting to the next group.</p>
    <div class="intervals">
      <label><span>Minimum</span><span class="input-wrap"><input aria-label="Minimum (seconds)" type="number" min="60" max="86400" step="1" bind:value={minimumInterval} required /><span>seconds</span></span></label>
      <label><span>Maximum</span><span class="input-wrap"><input aria-label="Maximum (seconds)" type="number" min={minimumInterval || 60} max="86400" step="1" bind:value={maximumInterval} required /><span>seconds</span></span></label>
    </div>
    <p class="hint">At least 60 seconds. Use equal values for a fixed interval.</p>
  </section>
  <section class="option-card rewrite-card">
    <div class="section-heading"><span class="eyebrow">OPTIONAL · AI REWRITE</span><h2>A fresh version for each group</h2></div>
    <p>Let Gemini rewrite your post using your instructions.</p>
    <Checkbox bind:checked={rewrite} text="Rewrite each post body with Gemini" {disabled} />
    {#if rewrite}
      <details class="instructions" open>
        <summary><span>Rewrite instructions<span class="summary-hint">Choose the tone and details Gemini should preserve.</span></span><DisclosureCaret /></summary>
        <div class="instructions-body">
          <label class="visually-hidden" for="rewrite-instructions">Rewrite instructions</label>
          <textarea id="rewrite-instructions" rows="6" maxlength="20000" bind:value={instructions} placeholder="Keep it concise and easy to scan. Preserve the price and contact details."></textarea>
          <p class="hint">Your post text and instructions are sent to Google Gemini. Images are not sent. Gemini rewrites use plain text.</p>
          <p class="fallback-note">If rewriting fails, that post uses your original text. The next post tries Gemini again.</p>
        </div>
      </details>
    {/if}
  </section>
</fieldset>

<style>
  fieldset { border: 0; min-width: 0; display: grid; gap: 1.6rem; }
  .visually-hidden { position: absolute; width: 1px; height: 1px; clip-path: inset(50%); overflow: hidden; }
  .option-card { background: var(--surface); border: 1px solid var(--border); border-radius: 1rem; padding: 2.4rem; }
  .eyebrow { display: block; color: var(--accent); font-size: 1rem; letter-spacing: .12em; font-weight: 650; margin-bottom: .8rem; }
  h2 { font-size: 1.7rem; font-weight: 600; color: var(--text-strong); }
  p { font-size: 1.3rem; line-height: 1.6; color: var(--text-secondary); margin: .8rem 0 1.8rem; }
  .intervals { display: grid; grid-template-columns: 1fr 1fr; gap: 1.6rem; }
  label { display: flex; flex-direction: column; gap: .8rem; font-size: 1.3rem; font-weight: 550; color: var(--text); }
  .input-wrap { display: flex; align-items: center; border: 1px solid var(--border-strong); border-radius: .6rem; overflow: hidden; }
  .input-wrap > span { padding-right: 1.2rem; font-size: 1.2rem; color: var(--text-muted); font-weight: 400; }
  input { width: 100%; min-width: 0; padding: 1.2rem; border: 0; background: transparent; color: var(--text); }
  textarea { width: 100%; padding: 1.2rem; background: var(--surface-subtle); border: 1px solid var(--border-strong); border-radius: .6rem; color: var(--text); resize: vertical; line-height: 1.7; font-size: 1.3rem; }
  .hint { margin: .9rem 0 0; font-size: 1.2rem; }
  .instructions { border-top: 1px solid var(--border); margin-top: 2rem; }
  summary { display: flex; align-items: center; justify-content: space-between; gap: 1rem; padding: 1.6rem 0; cursor: pointer; font-size: 1.3rem; font-weight: 550; color: var(--text-strong); list-style: none; }
  summary::-webkit-details-marker { display: none; }
  summary > span { flex: 1; min-width: 0; }
  .summary-hint { display: block; font-size: 1.2rem; color: var(--text-muted); font-weight: 400; margin-top: .4rem; }
  .fallback-note { background: var(--surface-active); padding: 1rem 1.2rem; border-radius: .5rem; font-size: 1.2rem; margin: 1.2rem 0 0; }
  :disabled { opacity: .6; }
  .input-wrap:focus-within, textarea:focus-visible, summary:focus-visible { outline: 2px solid var(--focus); outline-offset: 2px; }
  input:focus { outline: none; }
  @media (max-width: 480px) { .option-card { padding: 1.8rem; } .intervals { gap: 1rem; } }
</style>
