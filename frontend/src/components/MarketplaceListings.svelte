<script lang="ts">
  import { onMount, tick } from 'svelte';
  import type { AccountProfile } from '../../bindings/postika';
  import MarketplaceToolbar from './MarketplaceToolbar.svelte';
  import ItemListingEditor from './ItemListingEditor.svelte';
  import LocalizedFeedback from './LocalizedFeedback.svelte';
  import { publishMarketplaceListingMedia, canTestMarketplaceListing } from '../lib/media-upload';
  import PhosphorIcon from './PhosphorIcon.svelte';
  import { listMarketplaceListings, deleteMarketplaceListing, type MarketplaceListing } from '../lib/marketplace-drafts';
  import { itemListingFieldOptions, vehicleListingFieldOptions, homeListingFieldOptions } from '../lib/marketplace-fields';
  import { getLocale } from '../lib/i18n.svelte';
  import { notify } from '../lib/notifications.svelte';
  import GuidedTour from './GuidedTour.svelte';

  let { profile, ontourready }: { profile?: AccountProfile; ontourready?: (ready: boolean) => void } = $props();
  let listings = $state<MarketplaceListing[]>([]);
  let selected = $state<string[]>([]);
  let filter = $state('all'), search = $state(''), loading = $state(true), error = $state(''), busy = $state(false);
  let editing = $state<MarketplaceListing | null>(null);
  let publishQueue = $state.raw<MarketplaceListing[]>([]);
  let publishIndex = $state(0), publishing = $state(false);
  let publishFeedback = $state<'idle' | 'publishing' | 'submitted' | 'unknown' | 'error' | 'cancelled'>('idle');
  let publishError = $state('');
  let publishController: AbortController | undefined;
  const working = $derived(busy || publishQueue.length > 0);
  const tourReady = $derived(Boolean(profile) && !loading && !working && !editing);
  $effect(() => { if (!editing) ontourready?.(tourReady); });

  let deleting = $state<string | null>(null);
  let thumbnails = $state<Record<string, string>>({});
  type SortColumn = 'title' | 'price' | 'updated';
  let sortBy = $state<SortColumn>('updated');
  let sortDirection = $state<'ascending' | 'descending'>('descending');
  let highlighted = $state('');
  let alive = true, version = 0;
  const title = (listing: MarketplaceListing) => listing.fields.title.trim() || (listing.kind === 'vehicles' ? [listing.fields.year, listing.fields.make, listing.fields.model].filter(Boolean).join(' ') : listing.fields.address.trim()) || 'Untitled listing';
  const typeLabel = (listing: MarketplaceListing) => listing.kind === 'items' ? 'Item' : listing.kind === 'vehicles' ? 'Vehicle' : /* @wc-include */ 'Home';
  function subtitle(listing: MarketplaceListing) {
    const fields = listing.fields;
    const detail = listing.kind === 'items'
      ? itemListingFieldOptions.category.find(option => option.value === fields.category)?.label
      : listing.kind === 'vehicles'
        ? vehicleListingFieldOptions.vehicleType.find(option => option.value === fields.vehicleType)?.label
        : homeListingFieldOptions.propertyType.find(option => option.value === fields.propertyType)?.label;
    return [detail, fields.sku].filter(Boolean).join(' · ');
  }
  function changeSort(column: SortColumn) {
    if (sortBy === column) sortDirection = sortDirection === 'ascending' ? 'descending' : 'ascending';
    else sortDirection = column === 'updated' ? 'descending' : 'ascending';
    sortBy = column;
  }
  const sortState = (column: SortColumn) => sortBy === column ? sortDirection : undefined;
  function compare(a: MarketplaceListing, b: MarketplaceListing) {
    let result = 0;
    if (sortBy === 'price') {
      if (!a.fields.price || !b.fields.price) return a.fields.price ? -1 : b.fields.price ? 1 : b.updatedAt - a.updatedAt;
      result = Number(a.fields.price) - Number(b.fields.price);
    } else if (sortBy === 'title') result = title(a).localeCompare(title(b), getLocale(), { numeric: true, sensitivity: 'base' });
    else result = a.updatedAt - b.updatedAt;
    return (sortDirection === 'ascending' ? result : -result) || b.updatedAt - a.updatedAt || a.id.localeCompare(b.id);
  }
  function updatedDate(timestamp: number) {
    const date = new Date(timestamp);
    const now = new Date();
    return date.toDateString() === now.toDateString() ? 'Today' : date.toLocaleDateString(getLocale(), { day: 'numeric', month: 'short', ...(date.getFullYear() !== now.getFullYear() ? { year: 'numeric' } : {}) });
  }
  const fullTimestamp = (timestamp: number) => new Date(timestamp).toLocaleString(getLocale(), { dateStyle: 'full', timeStyle: 'short' });
  const visible = $derived(listings.filter(listing => (filter === 'all' || listing.kind === filter) && [title(listing), listing.fields.locationData?.label, listing.fields.location, listing.fields.address].join(' ').toLocaleLowerCase(getLocale()).includes(search.trim().toLocaleLowerCase(getLocale()))).sort(compare));
  const chosen = $derived(listings.filter(listing => canTestMarketplaceListing(listing.kind, listing.fields) && selected.includes(listing.id)));
  const selectable = $derived(visible.filter(listing => canTestMarketplaceListing(listing.kind, listing.fields)));
  const allSelected = $derived(selectable.length > 0 && selectable.every(listing => selected.includes(listing.id)));
  const someSelected = $derived(selectable.some(listing => selected.includes(listing.id)) && !allSelected);

  async function publishSelected() {
    if (!profile || working || !chosen.length) return;
    publishQueue = [...chosen];
    publishIndex = 0;
    deleting = null;
    await publishItem();
  }
  async function publishItem() {
    if (!profile || publishing || !publishQueue[publishIndex]) return;
    const listing = publishQueue[publishIndex];
    const profileID = profile.id;
    const controller = new AbortController();
    publishController = controller;
    publishing = true;
    publishFeedback = 'publishing';
    publishError = '';
    try {
      const result = await publishMarketplaceListingMedia(listing.kind, profileID, listing.photos, listing.video ?? null, listing.fields, controller.signal);
      if (!alive) return;
      selected = selected.filter(id => id !== listing.id);
      if (controller.signal.aborted) { publishFeedback = result.state === 'submitted' ? 'submitted' : 'cancelled'; publishQueue = []; return; }
      publishFeedback = result.state === 'submitted' ? 'submitted' : 'unknown';
      if (publishIndex === publishQueue.length - 1) publishQueue = [];
    } catch (cause) {
      if (!alive) return;
      if (controller.signal.aborted) { publishFeedback = 'cancelled'; publishQueue = []; }
      else { publishFeedback = 'error'; publishError = cause instanceof Error ? cause.message : String(cause); }
    } finally { if (alive) publishing = false; }
  }
  function cancelPublish() {
    publishController?.abort();
    if (!publishing) { publishQueue = []; publishFeedback = 'cancelled'; }
  }
  async function nextItem() {
    if (publishing || !['submitted', 'unknown'].includes(publishFeedback) || publishIndex >= publishQueue.length - 1) return;
    publishIndex++;
    await publishItem();
  }
  async function closeEditor() {
    const id = editing?.id;
    editing = null;
    await tick();
    document.getElementById(`marketplace-listing-${id}`)?.focus();
  }
  function toggle(id: string) { if (!working) selected = selected.includes(id) ? selected.filter(value => value !== id) : [...selected, id]; }
  function toggleAll() { if (!working) selected = allSelected ? selected.filter(id => !selectable.some(listing => listing.id === id)) : [...new Set([...selected, ...selectable.map(listing => listing.id)])]; }
  function releaseThumbnails() { Object.values(thumbnails).forEach(url => URL.revokeObjectURL(url)); }
  async function refresh() {
    const request = ++version;
    if (!profile) { loading = false; return; }
    try {
      const records = await listMarketplaceListings(profile.id);
      if (!alive || request !== version) return;
      highlighted = records.find(record => !listings.some(previous => previous.id === record.id && previous.updatedAt === record.updatedAt))?.id ?? '';
      releaseThumbnails();
      thumbnails = Object.fromEntries(records.filter(record => record.photos[0]).map(record => [record.id, URL.createObjectURL(record.photos[0])]));
      listings = records;
      selected = selected.filter(id => records.some(record => record.id === id));
      error = '';
    } catch { if (alive && request === version) error = 'Could not load saved listings. Please try again.'; }
    finally { if (alive && request === version) loading = false; }
  }
  onMount(() => {
    const channel = new BroadcastChannel('postika-marketplace');
    const reload = () => { void refresh(); };
    channel.onmessage = reload;
    window.addEventListener('postika:listings-changed', reload);
    window.addEventListener('focus', reload);
    void refresh();
    return () => { alive = false; publishController?.abort(); version++; channel.close(); window.removeEventListener('postika:listings-changed', reload); window.removeEventListener('focus', reload); releaseThumbnails(); };
  });
  async function remove(id: string) {
    if (!profile || working) return;
    busy = true;
    try { await deleteMarketplaceListing(profile.id, id); deleting = null; notify({ kind: 'success', message: 'Listing deleted.' }); }
    catch { error = 'Could not delete listing. Please try again.'; }
    finally { busy = false; }
  }
</script>

{#snippet sortHeading(column: SortColumn, label: string)}
  <button class="sort-heading" type="button" onclick={() => changeSort(column)}>
    <span>{label}</span><span class="sort-indicator" class:active={sortBy === column} class:ascending={sortBy === column && sortDirection === 'ascending'}><PhosphorIcon name="caret-down" size={12} /></span>
  </button>
{/snippet}

{#snippet selectionCheckbox(id: string, label: string, checked: boolean, mixed: boolean, onchange: () => void, disabled = false)}
  <div class="checkbox-wrapper-46">
    <input class="inp-cbx" {id} type="checkbox" {checked} {disabled} indeterminate={mixed} {onchange} />
    <label class="cbx" for={id} title={disabled && !working ? 'This listing type is not supported for publishing yet.' : undefined}>
      <span class="checkbox-symbol" aria-hidden="true">
        <svg width="12" height="10" viewBox="0 0 12 10">
          {#if mixed}<path class="mixed-mark" d="M2 5h8" />{:else}<polyline points="1.5 6 4.5 9 10.5 1" />{/if}
        </svg>
      </span>
      <span class="sr-only">{label}</span>
    </label>
  </div>
{/snippet}

{#if editing}
  {#key editing.id}<ItemListingEditor {profile} kind={editing.kind} initialDraft={editing} {ontourready} onprevious={closeEditor} onsaved={() => { void closeEditor(); notify({ kind: 'success', message: 'Listing saved.' }); }} />{/key}
{:else}
<div class="marketplace-listings">
  <MarketplaceToolbar bind:selected={filter} selectedCount={chosen.length} publishDisabled={working} publishing={publishing} onpublish={publishSelected} />
  <section class="listing-content" data-step-scroll aria-label="Saved listings" aria-busy={loading}>
    <div class="list-heading">
      <div><h1>Saved listings</h1><p>Save as many listings as you need. Keep them ready to edit and reuse when publishing.</p></div>
      <input class="search" type="search" aria-label="Search saved listings" placeholder="Search listings" bind:value={search} />
    </div>
    {#if error}<div class="feedback" role="alert"><p>{error}</p><button type="button" class="action-button action-button--secondary" onclick={refresh}>Try again</button></div>{/if}
    {#if publishFeedback !== 'idle'}
      <div class="test-status" role="status" aria-live="polite">
        <div>
          {#if publishFeedback === 'publishing'}
            <p>Publishing listing {publishIndex + 1} of {publishQueue.length}: {title(publishQueue[publishIndex])}</p>
            <p>Finish signing in or any verification prompts in Chrome. Your listing will be filled and submitted automatically.</p>
          {:else if publishFeedback === 'submitted'}<p>Publish sent to Facebook. Review the result in Chrome before continuing.</p>
          {:else if publishFeedback === 'unknown'}<p>Could not confirm whether Publish was sent. Check Facebook before continuing or retrying this listing.</p>
          {:else if publishFeedback === 'cancelled'}<p>Publishing stopped. Check Facebook before retrying this listing.</p>
          {:else if publishFeedback === 'error'}<p class="test-error"><LocalizedFeedback message={publishError} /></p>{/if}
        </div>
        <div class="test-actions">
          {#if ['submitted', 'unknown'].includes(publishFeedback) && publishQueue.length}<button class="action-button action-button--secondary" type="button" onclick={nextItem}>Next listing ({publishIndex + 2} / {publishQueue.length})</button>{/if}
          {#if publishFeedback === 'error' && publishQueue.length}<button class="action-button action-button--secondary" type="button" onclick={publishItem}>Try again</button>{/if}
          {#if publishQueue.length}<button class="action-button action-button--secondary" type="button" onclick={cancelPublish}>Cancel</button>{/if}
        </div>
      </div>
    {/if}
    <div class="selection-summary" aria-live="polite"><span>{visible.length} listings</span>{#if selected.length}<span>{selected.length} selected</span><button type="button" disabled={working} onclick={() => { selected = []; }}>Clear selection</button>{/if}</div>
    {#if loading}<div class="empty" role="status"><p>Loading saved listings…</p></div>
    {:else if !listings.length}
      <div class="empty"><PhosphorIcon name="package" size={32} /><h2>{profile ? 'No saved listings yet' : 'Select an account to see its listings'}</h2><p>{profile ? 'Start saving listings now so they’re ready to reuse whenever you publish to Marketplace.' : 'Your listings stay separate for each account.'}</p></div>
    {:else if !visible.length}<div class="empty"><h2>No matching listings</h2><p>Try another search or listing type.</p></div>
    {:else}
    <!-- svelte-ignore a11y_no_noninteractive_tabindex (Keyboard users focus this region to scroll the wide table.) -->
    <div class="table-scroll" role="region" aria-label="Saved Marketplace listings" tabindex="0">
      <table>
        <caption class="sr-only">Saved Marketplace listings</caption>
        <thead><tr><th class="check-cell">{@render selectionCheckbox('marketplace-select-all', 'Select all visible listings', allSelected, someSelected, toggleAll, working || !selectable.length)}</th><th scope="col" aria-sort={sortState('title')}>{@render sortHeading('title', 'Listing')}</th><th class="numeric" scope="col" aria-sort={sortState('price')}>{@render sortHeading('price', 'Price')}</th><th class="type" scope="col">Type</th><th scope="col">Location</th><th scope="col">Status</th><th scope="col" aria-sort={sortState('updated')}>{@render sortHeading('updated', 'Updated')}</th><th class="actions-cell" scope="col">Actions</th></tr></thead>
        <tbody>{#each visible as listing (listing.id)}
          {@const listingDetails = subtitle(listing)}
          <tr class:selected={selected.includes(listing.id)} class:recent={highlighted === listing.id}>
            <td class="check-cell">{@render selectionCheckbox(`marketplace-select-${listing.id}`, `Select ${title(listing)}`, selected.includes(listing.id), false, () => toggle(listing.id), working || !canTestMarketplaceListing(listing.kind, listing.fields))}</td>
            <th scope="row"><button type="button" class="listing-name" disabled={working} id={`marketplace-listing-${listing.id}`} onclick={() => { editing = listing; }}><span class="thumbnail">{#if thumbnails[listing.id]}<img src={thumbnails[listing.id]} alt="" />{:else}<PhosphorIcon name={listing.kind === 'vehicles' ? 'car' : listing.kind === 'homes' ? 'house' : 'package'} />{/if}</span><span class="title" dir="auto">{title(listing)}{#if listingDetails}<small>{listingDetails}</small>{/if}</span></button></th>
            <td class="numeric"><bdi>{listing.fields.price ? Number(listing.fields.price).toLocaleString(getLocale()) : '—'}</bdi></td>
            <td class="type">{typeLabel(listing)}</td><td class="location">{listing.fields.locationData?.label || listing.fields.address || listing.fields.location || '—'}</td>
            <td><span class="draft">Draft</span></td><td class="updated"><time datetime={new Date(listing.updatedAt).toISOString()} title={fullTimestamp(listing.updatedAt)} aria-label={fullTimestamp(listing.updatedAt)}><span>{updatedDate(listing.updatedAt)}</span><small>{new Date(listing.updatedAt).toLocaleTimeString(getLocale(), { hour: '2-digit', minute: '2-digit' })}</small></time></td>
            <td class="actions-cell"><div class="row-actions"><button class="button-7" type="button" aria-label={`Edit ${title(listing)}`} disabled={working} onclick={() => { editing = listing; }}>Edit</button><button class="button-7 button-7--delete" type="button" aria-label={`Delete ${title(listing)}`} disabled={working} onclick={() => { deleting = listing.id; }}>Delete</button></div></td>
          </tr>
          {#if deleting === listing.id}<tr><td colspan="8"><div class="delete-confirm"><p>Delete “{title(listing)}” and its saved media?</p><button class="action-button action-button--secondary" type="button" disabled={working} onclick={() => { deleting = null; }}>Cancel</button><button class="action-button action-button--secondary delete" type="button" disabled={working} onclick={() => remove(listing.id)}>Delete listing</button></div></td></tr>{/if}
        {/each}</tbody>
      </table>
    </div>
    {/if}
  </section>
</div>
<GuidedTour scope="marketplace" ready={tourReady} />
{/if}

<style>
  .marketplace-listings { height: 100%; display: flex; flex-direction: column; min-height: 0; }
  .listing-content { flex: 1; overflow: auto; padding: 2.4rem; }
  .list-heading { display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 1.6rem; }
  h1 { color: var(--text-strong); font-size: var(--ui-title-size); font-weight: var(--ui-title-weight); margin: 0 0 .6rem; }
  h2 { color: var(--text-strong); font-size: var(--ui-body-size); margin: 0; }
  p { color: var(--text-secondary); font-size: var(--ui-body-size); line-height: 1.6; margin: 0; }
  .search { width: 26rem; max-width: 100%; min-height: 4rem; padding: .9rem 1.2rem; background: var(--surface); color: var(--text); border: 1px solid var(--border); border-radius: .7rem; font: inherit; }
  .search::placeholder { color: var(--text-secondary); opacity: 1; }
  .selection-summary { display: flex; align-items: center; gap: 1.6rem; flex-wrap: wrap; min-height: 5.6rem; color: var(--text-secondary); font-size: var(--ui-small-size); }
  .selection-summary button { padding: .6rem .8rem; border: 0; background: transparent; color: var(--accent); cursor: pointer; font: inherit; border-radius: .4rem; }
  button:hover:not(:disabled) { background: var(--surface-hover); }
  button:disabled { opacity: .5; cursor: default; }
  button:focus-visible, input:focus-visible { outline: 2px solid var(--focus); outline-offset: 2px; }
  .checkbox-wrapper-46 { position: relative; display: inline-flex; vertical-align: middle; }
  .inp-cbx { position: absolute; width: 1px; height: 1px; margin: -1px; padding: 0; border: 0; clip-path: inset(50%); overflow: hidden; }
  .cbx { display: inline-flex; align-items: center; justify-content: center; min-width: 3.2rem; min-height: 3.2rem; user-select: none; cursor: pointer; }
  .checkbox-symbol { display: inline-block; position: relative; width: 18px; height: 18px; border: 1px solid #9098a9; border-radius: 3px; transform: scale(1); transition: border-color .2s ease, background-color .2s ease; }
  .checkbox-symbol svg { position: absolute; top: 3px; left: 2px; fill: none; stroke: #fff; stroke-width: 2; stroke-linecap: round; stroke-linejoin: round; stroke-dasharray: 16px; stroke-dashoffset: 16px; transition: stroke-dashoffset .3s ease .1s; transform: translate3d(0, 0, 0); }
  .checkbox-symbol::before { content: ''; position: absolute; inset: 0; background: #506eec; transform: scale(0); opacity: 1; border-radius: 50%; pointer-events: none; }
  .cbx:hover .checkbox-symbol { border-color: #506eec; }
  .inp-cbx:checked + .cbx .checkbox-symbol, .inp-cbx:indeterminate + .cbx .checkbox-symbol { background: #506eec; border-color: #506eec; animation: wave-46 .4s ease; }
  .inp-cbx:checked + .cbx svg, .inp-cbx:indeterminate + .cbx svg { stroke-dashoffset: 0; }
  .inp-cbx:checked + .cbx .checkbox-symbol::before, .inp-cbx:indeterminate + .cbx .checkbox-symbol::before { transform: scale(3.5); opacity: 0; transition: transform .6s ease, opacity .6s ease; }
  .inp-cbx:focus-visible + .cbx .checkbox-symbol { outline: 2px solid var(--focus); outline-offset: 3px; }
  .checkbox-symbol .mixed-mark { stroke-dasharray: none; }
  @keyframes wave-46 { 50% { transform: scale(.9); } }
  .table-scroll { overflow-x: auto; padding-bottom: .4rem; }
  table { width: 100%; border-collapse: separate; border-spacing: 0; text-align: start; color: var(--text); font-size: var(--ui-body-size); }
  th, td { text-align: start; padding: 1.4rem 1.2rem; border-bottom: 1px solid var(--border); vertical-align: middle; }
  thead th { background: var(--surface-subtle); color: var(--text-secondary); font-size: var(--ui-small-size); font-weight: var(--ui-label-weight); white-space: nowrap; }
  tbody th { min-width: 26rem; max-width: 42rem; font-weight: var(--ui-label-weight); }
  tbody tr:hover { background: var(--surface-hover); }
  tbody tr.selected, tbody tr.selected:hover { background: var(--accent-faint); }
  tr.recent:not(.selected) { animation: saved-row 2s var(--motion-ease); }
  .check-cell { width: 4rem; padding-inline: 1rem; }
  .listing-name { display: flex; align-items: center; gap: 1.2rem; width: 100%; border: 0; padding: 0; color: var(--text-strong); background: transparent; text-align: start; font: inherit; cursor: pointer; }
  .thumbnail { display: grid; place-items: center; width: 5.2rem; height: 5.2rem; flex-shrink: 0; border-radius: .6rem; overflow: hidden; background: var(--surface-active); color: var(--text-secondary); }
  .thumbnail img { width: 100%; height: 100%; object-fit: cover; }
  .title { overflow-wrap: anywhere; font-weight: var(--ui-label-weight); line-height: 1.45; }
  small { display: block; margin-top: .4rem; color: var(--text-secondary); font-size: var(--ui-small-size); font-weight: 400; }
  .numeric, time { font-variant-numeric: tabular-nums; }
  .numeric { min-width: 10rem; text-align: end; white-space: nowrap; }
  .sort-heading { display: inline-flex; align-items: center; gap: .6rem; min-height: 3.2rem; padding: 0; border: 0; border-radius: .3rem; background: transparent; color: inherit; font: inherit; font-weight: inherit; cursor: pointer; }
  .numeric .sort-heading { justify-content: flex-end; }
  .sort-indicator { opacity: .4; }
  .sort-indicator.active { color: var(--accent); opacity: 1; }
  .sort-indicator.ascending { transform: rotate(180deg); }
  .type, .location { color: var(--text-secondary); font-size: var(--ui-small-size); }
  .table-scroll:focus-visible { outline: 2px solid var(--focus); outline-offset: 2px; }
  .location { min-width: 11rem; max-width: 21rem; overflow-wrap: anywhere; }
  .draft { color: var(--text-secondary); background: var(--surface-active); padding: .4rem .8rem; border-radius: .4rem; font-size: var(--ui-small-size); }
  .updated { min-width: 10rem; color: var(--text-secondary); font-size: var(--ui-small-size); }
  .updated time { display: block; white-space: nowrap; }
  .updated small { margin-top: .3rem; }
  .actions-cell { width: 1%; white-space: nowrap; }
  .row-actions { display: flex; align-items: center; gap: .8rem; }
  .row-actions .button-7 { --button-bg: var(--surface); --button-hover: var(--accent-faint); --button-text: var(--accent); --button-border: var(--border-accent); --button-ring: var(--accent-soft); background-color: var(--button-bg); border: 1px solid var(--button-border); border-radius: 3px; box-sizing: border-box; color: var(--button-text); cursor: pointer; display: inline-block; font-family: inherit; font-size: var(--ui-small-size); font-weight: 400; line-height: 1.2; min-height: 3.6rem; margin: 0; padding: .8rem 1rem; position: relative; text-align: center; text-decoration: none; user-select: none; touch-action: manipulation; vertical-align: baseline; white-space: nowrap; }
  .row-actions .button-7:hover:not(:disabled), .row-actions .button-7:focus-visible { background-color: var(--button-hover); }
  .row-actions .button-7:focus-visible { box-shadow: 0 0 0 4px var(--button-ring); }
  .row-actions .button-7:active:not(:disabled) { background-color: color-mix(in srgb, var(--button-hover) 88%, var(--button-text)); box-shadow: none; }
  .row-actions .button-7--delete { --button-bg: var(--surface); --button-hover: var(--danger-bg); --button-text: var(--danger); --button-border: var(--danger-border); --button-ring: color-mix(in srgb, var(--danger) 20%, transparent); }
  .row-actions .button-7--delete:focus-visible { outline-color: var(--danger); }
  .test-status { display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 1.2rem; margin-top: 2rem; }
  .test-actions { display: flex; align-items: center; flex-wrap: wrap; gap: .8rem; }
  .test-status .test-error { color: var(--danger); }
  .inp-cbx:disabled + .cbx { cursor: default; opacity: .5; }
  .empty { display: flex; align-items: center; flex-direction: column; gap: 1.2rem; padding: 6rem 2rem; text-align: center; color: var(--text-secondary); }
  .empty p { max-width: 60ch; }
  .feedback { display: flex; align-items: center; flex-wrap: wrap; gap: 1.2rem; margin-block: 1.6rem; }
  .feedback p, .delete { color: var(--danger); }
  .delete-confirm { display: flex; align-items: center; justify-content: flex-end; flex-wrap: wrap; gap: 1rem; }
  .delete-confirm p { margin-inline-end: auto; }
  .sr-only { position: absolute; width: 1px; height: 1px; overflow: hidden; clip-path: inset(50%); white-space: nowrap; }
  @keyframes saved-row { from { background-color: var(--accent-soft); } }
  @media (prefers-reduced-motion: reduce) { tr.recent:not(.selected), .inp-cbx:checked + .cbx .checkbox-symbol, .inp-cbx:indeterminate + .cbx .checkbox-symbol { animation: none; } .checkbox-symbol, .checkbox-symbol svg, .checkbox-symbol::before { transition: none !important; } }
  @media (max-width: 1000px) {
    .actions-cell { position: sticky; inset-inline-end: 0; background: var(--bg); border-inline-start: 1px solid var(--border); }
    thead .actions-cell { z-index: 2; background: var(--surface-subtle); }
    tbody tr:hover .actions-cell { background: var(--surface-hover); }
    tbody tr.selected .actions-cell { background: color-mix(in srgb, var(--accent) 10%, var(--bg)); }
  }
  @media (max-width: 800px) { .listing-content { padding: var(--ui-space); } .search { width: 100%; } }
</style>
