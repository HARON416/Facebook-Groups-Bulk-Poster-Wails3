<script lang="ts">
  import { ProfileService } from '../../bindings/postika';
  import LocalizedFeedback from './LocalizedFeedback.svelte';
  import PhosphorIcon from './PhosphorIcon.svelte';

  let { selected = $bindable('all'), selectedCount = 0, onpublish }: { selected?: string; selectedCount?: number; onpublish: () => void } = $props();
  let opening = $state(false), error = $state('');
  async function createListing() {
    if (opening) return;
    opening = true;
    error = '';
    try { await ProfileService.OpenCreateListing(); }
    catch (cause) { error = cause instanceof Error ? cause.message : String(cause); }
    finally { opening = false; }
  }
  let expanded = $state(false);
  let highlighted = $state(0);
  let picker: HTMLDivElement;
  let trigger: HTMLButtonElement;
  const options = $derived([
    { value: 'all', label: 'All listing types' },
    { value: 'items', label: 'Items for sale' },
    { value: 'vehicles', label: 'Vehicles for sale' },
    { value: 'homes', label: 'Homes for rent or sale' },
  ]);
  const current = $derived(options.find(option => option.value === selected) ?? options[0]);

  function open() {
    highlighted = options.findIndex(option => option.value === selected);
    expanded = true;
  }
  function choose(index: number) {
    selected = options[index].value;
    expanded = false;
    trigger.focus();
  }
  function keydown(event: KeyboardEvent) {
    if (event.isComposing) return;
    if (event.key === 'Tab') { expanded = false; return; }
    if (event.key === 'Escape' && expanded) {
      event.preventDefault(); event.stopPropagation(); expanded = false; return;
    }
    if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
      event.preventDefault();
      if (!expanded) open();
      else highlighted = (highlighted + (event.key === 'ArrowDown' ? 1 : -1) + options.length) % options.length;
    } else if (event.key === 'Home' || event.key === 'End') {
      event.preventDefault();
      if (!expanded) open();
      highlighted = event.key === 'Home' ? 0 : options.length - 1;
    } else if ((event.key === 'Enter' || event.key === ' ') && expanded) {
      event.preventDefault(); choose(highlighted);
    }
  }
</script>

<svelte:window onpointerdown={(event) => { if (!picker?.contains(event.target as Node)) expanded = false; }} onfocusin={(event) => { if (!picker?.contains(event.target as Node)) expanded = false; }} />

<nav class="marketplace-toolbar" aria-label="Marketplace">
  <div class="listing-filter" bind:this={picker}>
    <button class="filter-trigger" type="button" role="combobox" aria-label="Listing type" aria-haspopup="listbox" aria-expanded={expanded} aria-controls="marketplace-listing-types" aria-activedescendant={expanded ? `listing-type-${options[highlighted].value}` : undefined} bind:this={trigger} onclick={() => { if (expanded) expanded = false; else open(); }} onkeydown={keydown}>
      <span>{current.label}</span>
      <span class="chevron"><PhosphorIcon name="caret-down" size={16} /></span>
    </button>
    {#if expanded}
      <div class="filter-options" id="marketplace-listing-types" role="listbox" aria-label="Listing type">
        {#each options as option, index (option.value)}
          <button id={`listing-type-${option.value}`} type="button" role="option" aria-selected={selected === option.value} class:highlighted={highlighted === index} tabindex="-1" onpointermove={() => { highlighted = index; }} onmousedown={(event) => event.preventDefault()} onclick={() => choose(index)}>
            <span>{option.label}</span>
            {#if selected === option.value}<PhosphorIcon name="check" size={16} />{/if}
          </button>
        {/each}
      </div>
    {/if}
  </div>
  <div class="toolbar-actions">
  <button class="action-button action-button--primary" type="button" disabled={!selectedCount} onclick={onpublish}>Publish selected{#if selectedCount} ({selectedCount}){/if}</button>
  <button class="create-listing action-button action-button--secondary" type="button" disabled={opening} aria-busy={opening} onclick={createListing}>
    <PhosphorIcon name="plus" size={16} />
    <span>Create new listing</span>
  </button>
  </div>
</nav>
{#if error}<p class="create-error" role="alert"><LocalizedFeedback message={error} /></p>{/if}

<style>
  .marketplace-toolbar { display: flex; flex-shrink: 0; align-items: center; justify-content: space-between; gap: var(--ui-space); padding: 1.2rem 2.4rem; border-bottom: 1px solid var(--border); background: var(--surface); }
  .listing-filter { position: relative; width: 28rem; max-width: 100%; min-width: 0; }
  .filter-trigger { display: flex; align-items: center; justify-content: space-between; gap: 1.6rem; width: 100%; min-height: 4rem; padding: .9rem 1.2rem; border: 1px solid var(--border); border-radius: .7rem; color: var(--text); background: var(--surface); font: inherit; text-align: start; cursor: pointer; }
  .filter-trigger:hover, .filter-trigger[aria-expanded=true] { border-color: var(--border-strong); background: var(--surface-hover); }
  .filter-trigger:focus-visible { outline: 2px solid var(--focus); outline-offset: 2px; }
  .chevron { color: var(--text-secondary); }
  .filter-options { position: absolute; z-index: 30; inset-inline-start: 0; top: calc(100% + .6rem); width: max-content; min-width: 100%; max-width: calc(100vw - 4.8rem); max-height: calc(100dvh - 18rem); overflow-y: auto; padding: .4rem; border: 1px solid var(--border); border-radius: .7rem; background: var(--surface); }
  .filter-options button { display: flex; align-items: center; justify-content: space-between; gap: 1.6rem; width: 100%; min-height: 4rem; padding: .9rem .8rem; border: 0; border-radius: .4rem; background: transparent; color: var(--text); font: inherit; text-align: start; cursor: pointer; }
  .filter-options button.highlighted { background: var(--surface-hover); }
  .filter-options button[aria-selected=true] { color: var(--accent); font-weight: var(--ui-label-weight); }
  .toolbar-actions { display: flex; flex-wrap: wrap; align-items: center; gap: .8rem; }
  .create-listing { flex-shrink: 0; }
  .create-error { padding: 1rem 2.4rem; color: var(--danger); font-size: var(--ui-body-size); }
  @media (max-width: 600px) {
    .marketplace-toolbar { flex-wrap: wrap; padding-inline: var(--ui-space); }
    .listing-filter { flex: 1 1 24rem; }
    .toolbar-actions { display: flex; flex-wrap: wrap; align-items: center; gap: .8rem; }
  .create-listing { margin-inline-start: auto; }
    .filter-options { max-width: calc(100vw - 3.2rem); }
  }
</style>
