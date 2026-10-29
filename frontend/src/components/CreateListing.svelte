<script lang="ts">
  import { ProfileService } from '../../bindings/postika';
  import { notify } from '../lib/notifications.svelte';
  import { tick } from 'svelte';
  import PhosphorIcon from './PhosphorIcon.svelte';
  import ItemListingEditor from './ItemListingEditor.svelte';
  import type { ListingEditorSnapshot, MarketplaceListingKind } from '../lib/marketplace-drafts';
  import type { AccountProfile } from '../../bindings/postika';

  let { profile }: { profile?: AccountProfile } = $props();

  let listingType = $state<string | null>(null);
  let editorKind = $state<MarketplaceListingKind | null>(null);
  let editor = $state<{ snapshot(): ListingEditorSnapshot | undefined }>();
  const editorStates = new Map<MarketplaceListingKind, ListingEditorSnapshot>();
  const types = $derived([
    { id: 'items', title: 'Item for sale', description: 'Create a single listing for one or more items to sell.' },
    { id: 'vehicles', title: 'Vehicle for sale', description: 'Sell a car, truck or other type of vehicle.' },
    { id: 'homes', title: 'Home for sale or rent', description: 'List a house or apartment for sale or rent.' },
  ]);
  const currentType = $derived(types.find(type => type.id === listingType));

  function saved() {
    editorStates.clear();
    listingType = null;
    editorKind = null;
    notify({ kind: 'success', message: 'Listing saved.' });
    void ProfileService.OpenMarketplace().catch(() => {});
  }

  async function advance(id: string) {
    if ((id === 'items' || id === 'vehicles' || id === 'homes') && id !== editorKind) {
      if (editorKind && editor) { const snapshot = editor.snapshot(); if (snapshot) editorStates.set(editorKind, snapshot); }
      editorKind = id;
    }
    listingType = id;
    await tick();
    document.getElementById('listing-step-heading')?.focus();
  }
  async function previous() {
    const id = listingType;
    listingType = null;
    await tick();
    document.getElementById(`create-listing-${id}`)?.focus();
  }
</script>

{#if editorKind}
  {#key editorKind}
    <div class="item-editor" hidden={listingType !== editorKind}><ItemListingEditor bind:this={editor} {profile} kind={editorKind} initialDraft={editorStates.get(editorKind)} visible={listingType === editorKind} onsaved={saved} onprevious={previous} onvehicle={() => advance('vehicles')} /></div>
  {/key}
{/if}
{#if listingType !== 'items' && listingType !== 'vehicles' && listingType !== 'homes'}
<section class="create-listing-step" aria-labelledby={currentType ? 'listing-step-heading' : 'listing-type-heading'}>
  <div class="step-content">
    {#if currentType}
      <div class="listing-details" data-listing-type={currentType.id}>
        <button class="previous action-button action-button--secondary" type="button" onclick={previous}>
          <span class="previous-icon"><PhosphorIcon name="arrow-left" /></span>
          <span>Previous</span>
        </button>
        <h1 id="listing-step-heading" tabindex="-1">{currentType.title}</h1>
      </div>
    {:else}
    <h1 id="listing-type-heading">Choose listing type</h1>
    <div class="listing-types">
      {#each types as type (type.id)}
        <button class="listing-type" id={`create-listing-${type.id}`} type="button" aria-labelledby={`listing-title-${type.id}`} aria-describedby={`listing-description-${type.id}`} onclick={() => advance(type.id)}>
          <span class="card-top" aria-hidden="true">
            <span class="type-icon">
              <PhosphorIcon name={type.id === 'items' ? 'package' : type.id === 'vehicles' ? 'car' : 'house'} />
            </span>
          </span>
          <span class="type-title" id={`listing-title-${type.id}`}>{type.title}</span>
          <span class="type-description" id={`listing-description-${type.id}`}>{type.description}</span>
        </button>
      {/each}
    </div>
    {/if}
  </div>
</section>
{/if}

<style>
  .item-editor { height: 100%; min-height: 0; }
  .item-editor[hidden] { display: none; }
  .create-listing-step { height: 100%; overflow-y: auto; overscroll-behavior: contain; padding: 3.2rem 2.4rem; }
  .step-content { width: 100%; }
  h1 { margin-bottom: 2rem; color: var(--text-strong); font-size: var(--ui-title-size); font-weight: var(--ui-title-weight); line-height: 1.5; }
  .listing-types { display: flex; flex-wrap: nowrap; gap: 1.2rem; }
  .listing-type { display: flex; flex: 1 1 0; flex-direction: column; align-items: stretch; min-width: 0; padding: 2rem; border: 1px solid var(--border); border-radius: 1.6rem; background: var(--surface); color: var(--text); font: inherit; text-align: start; cursor: pointer; }
  .listing-type:hover { background: var(--surface-hover); border-color: var(--border-strong); }
  .listing-type:focus-visible { outline: 2px solid var(--focus); outline-offset: 3px; }
  .card-top { display: flex; align-items: center; justify-content: space-between; min-height: 3.8rem; margin-bottom: 1.6rem; color: var(--accent); }
  .type-icon { display: grid; place-items: center; width: 3.8rem; height: 3.8rem; border-radius: 50%; background: var(--accent-faint); }
  .previous-icon { display: inline-flex; }
  .previous { width: fit-content; margin-bottom: 2rem; }
  :global([dir=rtl]) .previous-icon { transform: rotate(180deg); }
  #listing-step-heading:focus { outline: none; }
  .type-title { color: var(--text-strong); font-size: var(--ui-title-size); font-weight: var(--ui-title-weight); line-height: 1.5; overflow-wrap: anywhere; }
  .type-description { margin-top: .8rem; color: var(--text-secondary); font-size: var(--ui-body-size); line-height: 1.6; overflow-wrap: anywhere; }
  @media (max-width: 620px) {
    .create-listing-step { padding: 2.4rem var(--ui-space); }
  }
</style>
