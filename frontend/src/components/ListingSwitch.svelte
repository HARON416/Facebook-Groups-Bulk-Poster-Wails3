<script lang="ts">
  let { id, label, description = '', checked = $bindable(false) }: { id: string; label: string; description?: string; checked?: boolean } = $props();
</script>

<label class="listing-switch" class:compact={!description} for={id}>
  <span class="copy"><strong>{label}</strong>{#if description}<span class="description" id={`${id}-description`}>{description}</span>{/if}</span>
  <input {id} type="checkbox" role="switch" bind:checked aria-describedby={description ? `${id}-description` : undefined} />
  <span class="switch-track" aria-hidden="true"><span class="switch-thumb"></span></span>
</label>

<style>
  .listing-switch { display: flex; align-items: flex-start; justify-content: space-between; gap: 1.6rem; cursor: pointer; position: relative; }
  .copy { flex: 1; min-width: 0; }
  .compact { align-items: center; }
  .compact strong { font-weight: 400; }
  strong { color: var(--text-strong); font-size: var(--ui-body-size); font-weight: var(--ui-title-weight); line-height: 1.5; }
  .description { display: block; margin-top: .5rem; color: var(--text-muted); font-size: var(--ui-small-size); line-height: 1.6; }
  input { position: absolute; width: 1px; height: 1px; margin: -1px; padding: 0; border: 0; clip-path: inset(50%); overflow: hidden; }
  .switch-track { --switch-off: 0px; --switch-on: calc(4rem - 2px - .6rem - 1.6rem); position: relative; display: block; flex: 0 0 4rem; width: 4rem; height: 2.4rem; border: 1px solid var(--border-strong); border-radius: 2rem; background: var(--surface-hover); transition: background-color var(--motion-toggle) var(--motion-toggle-ease), border-color var(--motion-toggle) var(--motion-toggle-ease); }
  .switch-thumb { position: absolute; inset-block-start: .3rem; left: .3rem; width: 1.6rem; height: 1.6rem; border-radius: 50%; background: var(--text-secondary); transform: translateX(var(--switch-off)); transition: transform var(--motion-toggle) var(--motion-toggle-ease), background-color var(--motion-toggle) var(--motion-toggle-ease); }
  input:checked + .switch-track { background: var(--accent); border-color: var(--accent); }
  :global([dir=rtl]) .switch-track { --switch-off: calc(4rem - 2px - .6rem - 1.6rem); --switch-on: 0px; }
  input:checked + .switch-track .switch-thumb { transform: translateX(var(--switch-on)); background: var(--on-accent); }
  input:focus-visible + .switch-track { outline: 2px solid var(--focus); outline-offset: 3px; }
</style>
