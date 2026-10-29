<script lang="ts">
  import PhosphorIcon from './PhosphorIcon.svelte';
  let { value = $bindable('') }: { value?: string } = $props();
  let pending = $state(''), overflow = $state(false);
  let input: HTMLInputElement;
  const tags = $derived(value.split(',').map(tag => tag.trim()).filter(Boolean));
  function commit() {
    const next = [...tags];
    overflow = false;
    for (const tag of pending.split(',').map(tag => tag.trim()).filter(Boolean)) {
      if (next.some(existing => existing.toLowerCase() === tag.toLowerCase())) continue;
      if (next.length >= 20) { overflow = true; break; }
      next.push(tag);
    }
    value = next.join(', ');
    pending = '';
  }
  function remove(index: number) {
    value = tags.filter((_, i) => i !== index).join(', ');
    overflow = false;
    input.focus();
  }
</script>

<div class="tags-field" role="group" aria-labelledby="listing-tags-label" class:filled={tags.length > 0 || pending !== ''} class:has-tags={tags.length > 0} onpointerdown={(event) => {
  if (event.button !== 0 || !(event.target instanceof Element) || event.target.closest('input, button') || input.matches(':disabled')) return;
  event.preventDefault();
  input.focus();
}}>
  <label id="listing-tags-label" for="listing-tags">Product tags</label>
  <div class="tag-entry">
    {#if tags.length}<ul>{#each tags as tag, index}<li><span dir="auto">{tag}</span><button type="button" aria-label={`Remove ${tag}`} onclick={() => remove(index)}><PhosphorIcon name="x" size={14} /></button></li>{/each}</ul>{/if}
    <div class="tag-input-row">
    <input id="listing-tags" type="text" dir="auto" bind:this={input} bind:value={pending} aria-describedby="listing-tags-hint" aria-invalid={overflow || tags.length > 20} onblur={commit} onkeydown={(event) => {
      if (event.isComposing) return;
      if (event.key === 'Enter' || event.key === ',') { event.preventDefault(); commit(); }
      else if (event.key === 'Backspace' && !pending && tags.length) { event.preventDefault(); remove(tags.length - 1); }
    }} />
    <button class="add-tag" type="button" aria-label="Add tag" disabled={!pending.trim() || tags.length >= 20} onmousedown={(event) => event.preventDefault()} onclick={() => { commit(); input.focus(); }}><PhosphorIcon name="plus" size={16} /></button>
    </div>
  </div>
</div>
<p class="hint" class:invalid={overflow || tags.length > 20} id="listing-tags-hint" role="status">Optional. Limit: 20.</p>

<style>
  .tags-field { display: flex; flex-direction: column; position: relative; cursor: text; min-height: 8rem; border: 1px solid var(--border-strong); border-radius: 1rem; background: var(--surface); }
  .tags-field.has-tags { min-height: 12rem; }
  label { position: absolute; inset-inline-start: 1.4rem; top: 1.8rem; color: var(--text-muted); font-size: 1.4rem; line-height: 1.5; }
  .filled label, .tags-field:focus-within label { top: .6rem; font-size: var(--ui-small-size); line-height: 1.3; }
  .tags-field:focus-within { outline: 2px solid var(--focus); outline-offset: 2px; }
  .tags-field:focus-within label { color: var(--accent); }
  .tag-entry { display: flex; flex: 1; flex-direction: column; padding: 3rem 1.2rem 1rem; }
  .tag-input-row { display: flex; align-items: center; gap: .8rem; }
  ul { display: flex; flex-wrap: wrap; gap: .8rem; margin: 0 0 .6rem; padding: 0; list-style: none; }
  li { display: inline-flex; align-items: center; gap: .5rem; max-width: 100%; min-height: 3.6rem; padding-block: .6rem; padding-inline: 1.2rem .8rem; border-radius: 999px; font-weight: var(--ui-label-weight); border: 1px solid color-mix(in srgb, var(--accent) 45%, var(--border-strong)); background: color-mix(in srgb, var(--accent) 22%, var(--surface)); color: var(--text-strong); }
  li > span { min-width: 0; overflow-wrap: anywhere; }
  button { display: grid; place-items: center; flex-shrink: 0; width: 2rem; height: 2rem; padding: 0; border: 0; border-radius: .3rem; background: transparent; color: var(--text-secondary); cursor: pointer; }
  button:hover { background: var(--surface-hover); }
  button:focus-visible { outline: 2px solid var(--focus); outline-offset: 1px; }
  .add-tag { width: 2.4rem; height: 2.4rem; border-radius: 50%; background: var(--surface-active); }
  .add-tag:disabled { opacity: .5; cursor: default; }
  input { flex: 1; min-width: 0; width: 100%; min-height: 3.2rem; padding: 0; border: 0; outline: 0; background: transparent; color: var(--text); font: inherit; font-size: 1.4rem; }
  .hint { margin-top: .4rem; color: var(--text-muted); font-size: var(--ui-small-size); line-height: 1.6; }
  .invalid { color: var(--danger); }
</style>
