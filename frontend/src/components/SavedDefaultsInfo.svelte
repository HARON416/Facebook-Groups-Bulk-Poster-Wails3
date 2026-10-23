<script lang="ts">
  import type { AppSettings } from '../lib/posting';
  let { settings } = $props<{ settings: AppSettings | null }>();
  let pinned = $state(false), hovered = $state(false), focused = $state(false);
  const expanded = $derived(pinned || hovered || focused);
</script>

<div class="defaults-info" role="group" aria-label="Saved defaults help" onmouseenter={() => hovered = true} onmouseleave={() => hovered = false}>
  <button type="button" aria-label="About saved defaults" aria-expanded={expanded} aria-controls="saved-defaults-help"
    onfocus={() => focused = true} onblur={() => focused = false}
    onclick={() => { pinned = !pinned; focused = false; hovered = false; }}
    onkeydown={(event) => { if (event.key === 'Escape') { pinned = false; hovered = false; focused = false; } }}>
    <svg aria-hidden="true" viewBox="0 0 256 256"><path d="M128,24A104,104,0,1,0,232,128,104.11,104.11,0,0,0,128,24Zm0,192a88,88,0,1,1,88-88A88.1,88.1,0,0,1,128,216Zm16-40a8,8,0,0,1-8,8,16,16,0,0,1-16-16V128a8,8,0,0,1,0-16,16,16,0,0,1,16,16v40A8,8,0,0,1,144,176ZM112,84a12,12,0,1,1,12,12A12,12,0,0,1,112,84Z" /></svg>
  </button>
  <div id="saved-defaults-help" class="help" hidden={!expanded}>
    <strong>What does “Use saved defaults” do?</strong>
    <p>Replaces this run’s posting interval, Gemini rewrite option, and rewrite instructions with the values saved in Settings.</p>
    {#if settings}
      <dl>
        <div><dt>Saved interval</dt><dd>{settings.minimumInterval === settings.maximumInterval ? settings.minimumInterval : `${settings.minimumInterval}–${settings.maximumInterval}`} seconds</dd></div>
        <div><dt>Gemini rewrite</dt><dd>{settings.rewrite ? 'On' : 'Off'}</dd></div>
      </dl>
      <strong class="instructions-heading">Saved rewrite instructions</strong>
      <p class="instructions">{settings.instructions.trim() || 'No instructions saved.'}</p>
    {:else}
      <p>Saved values could not be loaded. Try “Use saved defaults” again.</p>
    {/if}
    <p class="note">Initial defaults: 60–120 seconds, Gemini rewrite off, and no rewrite instructions. Save your own preferences in Settings to change them.</p>
  </div>
</div>

<style>
  .defaults-info { position: relative; display: flex; align-items: center; }
  button { display: grid; place-items: center; width: 3.4rem; height: 3.4rem; border: 0; border-radius: .5rem; background: transparent; color: var(--accent); cursor: pointer; }
  button:hover, button[aria-expanded=true] { background: var(--surface-active); }
  button:focus-visible { outline: 2px solid var(--focus); outline-offset: 2px; }
  svg { width: 2rem; height: 2rem; fill: currentColor; }
  .help { position: absolute; bottom: calc(100% + .7rem); inset-inline-start: -17rem; z-index: 5; width: min(40rem, calc(100vw - 6rem)); padding: 1.8rem; border: 1px solid var(--border-accent); border-radius: .8rem; background: var(--surface); box-shadow: 0 .6rem 2.4rem var(--shadow); color: var(--text-strong); font-size: 1.3rem; line-height: 1.6; }
  .help::after { content: ''; position: absolute; top: 100%; left: 0; right: 0; height: 1rem; }
  p { margin-top: .8rem; }
  dl { margin: 1.2rem 0; }
  dl div { display: flex; justify-content: space-between; gap: 1rem; }
  dt { color: var(--text-secondary); }
  dd { font-weight: 550; }
  .instructions { max-height: 12rem; overflow: auto; white-space: pre-wrap; overflow-wrap: anywhere; padding: .8rem 1rem; border-radius: .4rem; background: var(--surface-subtle); }
  .note { color: var(--text-secondary); font-size: 1.2rem; border-top: 1px solid var(--border); padding-top: 1rem; }
  @media (max-width: 540px) { .help { inset-inline-start: -18rem; } }
</style>
