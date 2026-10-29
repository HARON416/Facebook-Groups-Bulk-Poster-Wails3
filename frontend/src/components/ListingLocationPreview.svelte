<script lang="ts">
  import type { MarketplaceLocationHit } from '../../bindings/postika';
  import { getLocale } from '../lib/i18n.svelte';
  import { listingMapURLs } from '../lib/listing-map';
  import PhosphorIcon from './PhosphorIcon.svelte';

  let { location, onopen }: { location: MarketplaceLocationHit; onopen: (event: MouseEvent) => void } = $props();
  const urls = $derived(listingMapURLs(location.latitude, location.longitude, getLocale()));
  let failedURL = $state('');
</script>

<div class="location-preview">
  <p class="location-name" dir="auto">{location.label}</p>
  {#if urls}
    <div class="map-frame">
      <a class="map-link" href={urls.link} target="_blank" rel="noopener noreferrer" aria-label="View listing location" onclick={onopen}>
        {#if failedURL !== urls.image}
          <img src={urls.image} alt="" loading="lazy" referrerpolicy="no-referrer" onerror={(event) => { failedURL = event.currentTarget.getAttribute('src') ?? ''; }} />
        {:else}
          <span class="map-fallback"><PhosphorIcon name="map-pin" /><span>Map preview unavailable.</span></span>
        {/if}
      </a>
      <details class="map-info">
        <summary aria-label="View Map Info"><PhosphorIcon name="info" size={18} /></summary>
        <div class="map-info-copy"><p>Location is approximate</p><a href={urls.image} target="_blank" rel="noopener noreferrer" onclick={onopen}>View map image</a></div>
      </details>
    </div>
  {/if}
  <p class="location-note">Location is approximate</p>
</div>

<style>
  .location-preview { margin-block: 1.6rem; min-width: 0; }
  .location-name { margin-bottom: 1rem; overflow-wrap: anywhere; }
  .map-frame { position: relative; }
  .map-link { display: block; aspect-ratio: 500 / 80; min-height: 4.8rem; overflow: hidden; border-radius: .8rem; background: var(--surface); color: var(--text-muted); }
  .map-link img { display: block; width: 100%; height: 100%; aspect-ratio: 500 / 80; object-fit: cover; }
  .map-link:focus-visible, summary:focus-visible { outline: 2px solid var(--focus); outline-offset: 3px; }
  .map-fallback { display: flex; align-items: center; justify-content: center; gap: .8rem; min-height: 4.8rem; font-size: var(--ui-small-size); }
  .map-info { position: absolute; inset-inline-end: .6rem; top: .6rem; z-index: 1; }
  summary { display: grid; place-items: center; width: 2.8rem; height: 2.8rem; border-radius: 50%; background: var(--bg); color: var(--text); cursor: pointer; list-style: none; }
  summary::-webkit-details-marker { display: none; }
  .map-info-copy { position: absolute; inset-inline-end: 0; top: calc(100% + .6rem); width: 19rem; max-width: 65vw; padding: 1rem; border: 1px solid var(--border); border-radius: .8rem; background: var(--bg); color: var(--text); font-size: var(--ui-small-size); }
  .map-info-copy a { color: var(--accent); text-underline-offset: .2em; }
  .location-note { margin-top: 1rem; color: var(--text-muted); font-size: var(--ui-small-size); line-height: 1.5; }
</style>
