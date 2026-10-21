<script lang="ts">
  let { checked = $bindable(false), disabled = false, text } = $props<{ checked?: boolean; disabled?: boolean; text: string }>();
</script>

<label class="checkbox">
  <input type="checkbox" bind:checked {disabled} />
  <span class="symbol">
    <svg aria-hidden="true" viewBox="0 0 28 28"><path d="M4 14l8 7L24 7" /></svg>
  </span>
  <span class="text">{text}</span>
</label>

<style>
  .checkbox { display: flex; width: 100%; min-width: 0; align-items: center; gap: 1rem; position: relative; cursor: pointer; font-size: 1.4rem; font-weight: 550; line-height: 1.5; color: var(--text-strong); }
  .text { flex: 1; min-width: 0; }
  input { position: absolute; width: 1px; height: 1px; margin: -1px; padding: 0; border: 0; clip-path: inset(50%); overflow: hidden; }
  .symbol { display: flex; flex: 0 0 2.2rem; height: 2.2rem; border: 1px solid var(--accent); border-radius: 0.3rem; position: relative; background: var(--surface); transition: box-shadow .4s, background-color .4s; }
  .symbol::after { content: ''; position: absolute; inset: 0; background: var(--accent-soft); border-radius: 50%; opacity: 0; pointer-events: none; }
  svg { width: 1.6rem; height: 1.6rem; margin: auto; fill: none; stroke: var(--accent); stroke-width: 3; stroke-linecap: round; stroke-linejoin: round; }
  path { stroke-dasharray: 30 31; stroke-dashoffset: 31; transition: stroke-dashoffset .2s ease-in; }
  input:checked + .symbol { background: var(--surface-active); }
  input:checked + .symbol path { stroke-dashoffset: 0; transition-duration: .4s; }
  input:checked + .symbol::after { animation: ripple .6s ease-out; }
  input:focus-visible + .symbol { box-shadow: 0 0 0 .3rem var(--accent-soft); }
  input:disabled ~ span { opacity: .55; cursor: default; }
  @keyframes ripple { from { transform: scale(.2); opacity: 1; } to { transform: scale(2.4); opacity: 0; } }
  @media (prefers-reduced-motion: reduce) { .symbol, path { transition: none; } input:checked + .symbol::after { animation: none; } }
</style>
