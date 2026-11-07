<script lang="ts">
  import { onMount, onDestroy, untrack, tick } from 'svelte';
  import { Browser, System, Window } from '@wailsio/runtime';
  import type { AccountProfile, MarketplaceLocationStatus, MarketplaceLocationHit } from '../../bindings/postika';
  import { notify, dismissNotification } from '../lib/notifications.svelte';
  import { createAccountService } from '../lib/accounts';
  import { pollWhileWindowOpen } from '../lib/window-polling';
  import { accountLabel } from './AccountLabel.svelte';
  import PhosphorIcon from './PhosphorIcon.svelte';
  import ListingFieldSelect from './ListingFieldSelect.svelte';
  import ListingLocationPreview from './ListingLocationPreview.svelte';
  import ListingVideo from './ListingVideo.svelte';
  import { videoThumbnail } from '../lib/video-thumbnail';
  import { createVideoPoster } from '../lib/video-poster.svelte';
  import { testMarketplaceListingMedia, canTestMarketplaceListing } from '../lib/media-upload';
  import ListingDatePicker from './ListingDatePicker.svelte';
  import ListingTags from './ListingTags.svelte';
  import GuidedTour from './GuidedTour.svelte';
  import LocationSearchDialog from './LocationSearchDialog.svelte';
  import ListingSwitch from './ListingSwitch.svelte';
  import Checkbox from './Checkbox.svelte';
  import LocalizedFeedback from './LocalizedFeedback.svelte';
  import type { ListingFieldOption } from '../lib/marketplace-fields';
  import { itemListingFieldOptions, vehicleListingFieldOptions, homeListingFieldOptions } from '../lib/marketplace-fields';
  import { emptyItemListing, readItemListingDraft, saveMarketplaceListing, type ItemListingFields, type MarketplaceListingKind, type ListingEditorSnapshot } from '../lib/marketplace-drafts';
  import { reorder } from '../lib/reorder';
  import { overlappingTarget } from '../lib/drag-target';
  import { logMessage } from './LogMessage.svelte';
  import { snapshotImage } from '../lib/drafts';

  let { profile, kind = 'items', initialDraft, visible = true, onprevious, onsaved, onnext, onvehicle, ontourready }: { profile?: AccountProfile; kind?: MarketplaceListingKind; initialDraft?: ListingEditorSnapshot; visible?: boolean; onprevious: () => void; onsaved?: () => void; onnext?: (fields: ItemListingFields, photos: File[], video?: File | null) => void; onvehicle?: () => void; ontourready?: (ready: boolean) => void } = $props();
  let root = $state<HTMLDivElement>();
  function pauseMedia(except?: EventTarget | null) { root?.querySelectorAll('video').forEach(player => { if (player !== except) player.pause(); }); }
  $effect(() => { if (!visible) pauseMedia(); });
  onMount(() => { const hide = () => pauseMedia(); window.addEventListener('postika:window-hidden', hide); return () => window.removeEventListener('postika:window-hidden', hide); });
  const vehicle = $derived(kind === 'vehicles');
  const home = $derived(kind === 'homes');
  const maxPhotos = $derived(home ? 50 : vehicle ? 20 : 10);
  let listingID = $state<string | undefined>(undefined);
  let fields = $state(emptyItemListing());
  const rental = $derived(home && fields.homeOffer === 'rent');
  const privateRental = $derived(rental && fields.privateRoom);
  let photos = $state<{ id: string; file: File; url: string }[]>([]);
  const photosOverLimit = $derived(photos.length > maxPhotos);
  const carTruck = $derived(vehicle && fields.vehicleType === 'car-truck');
  const motorcycle = $derived(vehicle && fields.vehicleType === 'motorcycle');
  const powersport = $derived(vehicle && fields.vehicleType === 'powersport');
  const rvCamper = $derived(vehicle && fields.vehicleType === 'rv-camper');
  const trailer = $derived(vehicle && fields.vehicleType === 'trailer');
  const boat = $derived(vehicle && fields.vehicleType === 'boat');
  const commercialIndustrial = $derived(vehicle && fields.vehicleType === 'commercial-industrial');
  const otherVehicle = $derived(vehicle && fields.vehicleType === 'other');
  const detailedVehicle = $derived(carTruck || motorcycle);
  const hasVehicleDetails = $derived(detailedVehicle || powersport || rvCamper || boat || commercialIndustrial);
  const hasVehicleMedia = $derived(hasVehicleDetails || trailer || otherVehicle);
  const hasVideo = $derived(kind === 'items' || home || hasVehicleMedia);
  const carOptions = $derived(vehicleListingFieldOptions.carTruck);
  const detailOptions = $derived(motorcycle ? vehicleListingFieldOptions.motorcycle : carOptions);
  // Retain values already entered in drafts.
  const availableMakes = $derived(detailOptions.make);
  const makeOptions = $derived(fields.make && !availableMakes.some(option => option.value === fields.make) ? [{ value: fields.make, label: fields.make }, ...availableMakes] : availableMakes);
  let videoControl = $state<ListingVideo>();
  let videoError = $state(false);
  let posterAttempt = $state(0);
  let testing = $state(false), testError = $state('');
  let testController: AbortController | undefined;
  let videoFile = $state.raw<File | null>(null), videoBusy = $state(false), showingVideo = $state(false);
  const poster = createVideoPoster(() => videoFile, () => profile?.id, () => posterAttempt, videoThumbnail);
  const videoPoster = $derived(poster.image), posterBusy = $derived(poster.busy), posterError = $derived(poster.error);
  const mediaBusy = $derived(videoBusy || posterBusy);
  const videoActive = $derived(Boolean(hasVideo && videoFile && (showingVideo || !photos.length)));
  $effect(() => { if (!videoFile) showingVideo = false; });
  const previewTitle = $derived(vehicle ? [fields.year, fields.make.trim(), fields.model.trim()].filter(Boolean).join(' ') : home ? homeListingFieldOptions.propertyType.find(option => option.value === fields.propertyType)?.label ?? '' : fields.title);
  export function snapshot(): ListingEditorSnapshot | undefined { if (loading) return; return { listingID, fields: $state.snapshot(fields), photos: photos.map(photo => photo.file), activePhoto, video: videoFile, showingVideo, locationQuery, locationTouched }; }
  let activePhoto = $state(0), loading = $state(true), saving = $state(false), adding = $state(false), dragging = $state(false);
  let draggedPhoto = $state(''), dropTarget = $state(''), dropSide = $state<'before' | 'after'>('before');
  let reorderMessage = $state('');
  const busy = $derived(loading || saving || adding || mediaBusy || testing);
  let pointer = $state.raw<{ id: number; photoID: string; button: HTMLButtonElement; startX: number; startY: number; x: number; y: number; offsetX: number; offsetY: number; width: number; height: number } | null>(null);
  let dragPreview = $state<HTMLDivElement>();
  let frame = 0, suppressClick = false;
  function clearDrag() {
    cancelAnimationFrame(frame); frame = 0;
    const previous = pointer;
    pointer = null;
    if (previous?.button.hasPointerCapture(previous.id)) previous.button.releasePointerCapture(previous.id);
    draggedPhoto = ''; dropTarget = ''; dragging = false;
  }
  async function rearrange(id: string, target: string, side: 'before' | 'after') {
    if (busy) return;
    const next = reorder(photos, id, target, side);
    if (next === photos) return;
    const activeID = active?.id;
    photos = next;
    activePhoto = Math.max(0, photos.findIndex(photo => photo.id === activeID));
    const index = photos.findIndex(photo => photo.id === id);
    reorderMessage = /* @wc-ignore */ `${photos[index].file.name} moved to position ${index + 1} of ${photos.length}.`;
    await tick();
    if (!destroyed) document.getElementById(`listing-photo-${id}`)?.focus({ preventScroll: true });
  }
  function startPointer(event: PointerEvent, photoID: string) {
    if (busy || !event.isPrimary || event.button !== 0 || photos.length < 2) return;
    clearDrag(); suppressClick = false;
    const button = event.currentTarget as HTMLButtonElement;
    const bounds = button.getBoundingClientRect();
    pointer = { id: event.pointerId, photoID, button, startX: event.clientX, startY: event.clientY, x: event.clientX, y: event.clientY, offsetX: event.clientX - bounds.left, offsetY: event.clientY - bounds.top, width: bounds.width, height: bounds.height };
    button.setPointerCapture(event.pointerId);
  }
  function updateTarget() {
    dropTarget = '';
    if (!pointer || !draggedPhoto) return;
    const area = pointer.button.closest('.photo-upload');
    if (!area) return;
    const scroll = area.closest('.form-scroll')?.getBoundingClientRect();
    const clip = { left: Math.max(0, scroll?.left ?? 0), top: Math.max(0, scroll?.top ?? 0), right: Math.min(window.innerWidth, scroll?.right ?? window.innerWidth), bottom: Math.min(window.innerHeight, scroll?.bottom ?? window.innerHeight) };
    const left = pointer.x - pointer.offsetX, top = pointer.y - pointer.offsetY;
    const targets = Array.from(area.querySelectorAll<HTMLElement>('.photo-tile, .add-photo-tile'))
      .filter(tile => tile.dataset.photoId !== draggedPhoto)
      .map(tile => ({ tile, bounds: tile.getBoundingClientRect() }));
    const target = overlappingTarget({ left, top, right: left + pointer.width, bottom: top + pointer.height }, targets, clip);
    if (!target) return;
    dropTarget = target.tile.dataset.photoId ?? photos.at(-1)?.id ?? '';
    // Insert in the direction of travel, including a slight overlap with a neighbour.
    dropSide = target.tile.classList.contains('add-photo-tile') || photos.findIndex(photo => photo.id === draggedPhoto) < photos.findIndex(photo => photo.id === dropTarget) ? 'after' : 'before';
  }
  function renderDrag() {
    if (!pointer || !draggedPhoto) return;
    if (dragPreview) dragPreview.style.transform = `translate3d(${pointer.x - pointer.offsetX}px, ${pointer.y - pointer.offsetY}px, 0)`;
    const scroll = pointer.button.closest<HTMLElement>('.form-scroll');
    if (scroll) {
      const bounds = scroll.getBoundingClientRect();
      if (pointer.x >= bounds.left && pointer.x <= bounds.right && pointer.y >= bounds.top && pointer.y <= bounds.bottom) {
        const delta = pointer.y < bounds.top + 48 ? -10 : pointer.y > bounds.bottom - 48 ? 10 : 0;
        if (delta) scroll.scrollBy({ top: delta, behavior: 'instant' });
      }
    }
    updateTarget();
    frame = requestAnimationFrame(renderDrag);
  }
  function movePointer(event: PointerEvent) {
    if (!pointer || pointer.id !== event.pointerId) return;
    pointer.x = event.clientX; pointer.y = event.clientY;
    if (!draggedPhoto && Math.hypot(pointer.x - pointer.startX, pointer.y - pointer.startY) >= 5) {
      draggedPhoto = pointer.photoID; suppressClick = true;
      frame = requestAnimationFrame(renderDrag);
    }
    if (draggedPhoto) { event.preventDefault(); updateTarget(); }
  }
  function endPointer(event: PointerEvent) {
    if (!pointer || pointer.id !== event.pointerId) return;
    pointer.x = event.clientX; pointer.y = event.clientY;
    updateTarget();
    const id = draggedPhoto, target = dropTarget, side = dropSide;
    clearDrag();
    if (id) { event.preventDefault(); void rearrange(id, target, side); }
  }
  function overPhotos(event: DragEvent) {
    if (!busy && event.dataTransfer?.types.includes(/* @wc-ignore */ 'Files')) {
      event.preventDefault(); event.dataTransfer.dropEffect = 'copy'; dragging = true;
    }
  }
  async function dropPhotos(event: DragEvent) {
    event.preventDefault();
    const files = Array.from(event.dataTransfer?.files ?? []);
    clearDrag();
    const videos = files.filter(file => file.type.startsWith('video/'));
    await addPhotos(files.filter(file => !file.type.startsWith('video/')));
    await tick();
    if (!destroyed && videos.length) await videoControl?.chooseFiles(videos);
  }
  function photoKeydown(event: KeyboardEvent, id: string) {
    if (!event.altKey || !['ArrowLeft', 'ArrowRight'].includes(event.key) || busy) return;
    event.preventDefault();
    const rtl = getComputedStyle(event.currentTarget as HTMLElement).direction === 'rtl';
    const direction = (event.key === 'ArrowRight') !== rtl ? 1 : -1;
    const target = photos[photos.findIndex(photo => photo.id === id) + direction];
    if (target) void rearrange(id, target.id, direction > 0 ? 'after' : 'before');
  }
  let feedback = $state('');
  let helpError = $state('');
  let locationDialogOpen = $state(false), locationBrowserOpened = $state(false);
  let restoringLocationFocus = false, awaitingLocationSuccess = false;
  let locationTouched = $state(false);
  let locationQuery = $state(''), locationHits = $state<MarketplaceLocationHit[]>([]);
  let locationSearching = $state(false), locationSearched = $state(false), locationSearchError = $state(false), locationPromptDismissed = $state(false);
  let locationSearchVersion = 0, lastLocationQuery = '', locationSearchAfter = 0;
  let locationTimer: ReturnType<typeof setTimeout> | undefined;
  const locationOptions = $derived(locationHits.map(hit => ({ value: hit.pageID || hit.addressKey || '', label: hit.label, description: hit.subtitle })));
  async function searchLocations() {
    if (destroyed || !visible || locationDialogOpen || Date.now() < locationSearchAfter || !profile || !locationStatus?.captured || locationStatus.state !== 'ready' || !locationQuery.trim() || fields.location) return;
    const query = home ? locationQuery : locationQuery.trim();
    if (lastLocationQuery === query) return;
    lastLocationQuery = query;
    const version = ++locationSearchVersion;
    locationSearching = true; locationSearchError = false; locationSearched = false; locationHits = [];
    try {
      const hits = await createAccountService(profile.id).SearchMarketplaceLocations(query, home);
      if (destroyed || version !== locationSearchVersion || query !== (home ? locationQuery : locationQuery.trim())) return;
      locationHits = hits ?? []; locationSearched = true;
    } catch {
      if (!destroyed && version === locationSearchVersion) { locationSearchError = true; lastLocationQuery = ''; void refreshLocationStatus(); }
    } finally { if (!destroyed && version === locationSearchVersion) locationSearching = false; }
  }
  function locationQueryChanged(query: string) {
    clearTimeout(locationTimer); locationSearchVersion++; lastLocationQuery = ''; locationSearchAfter = Date.now() + 250;
    locationTouched = false;
    fields.location = ''; fields.locationData = null;
    if (home) fields.address = '';
    locationHits = []; locationSearching = false; locationSearched = false; locationSearchError = false;
    if (!query.trim()) { locationPromptDismissed = false; return; }
    locationTimer = setTimeout(() => {
      if (destroyed || !visible) return;
      if (locationStatus?.state === 'ready' && locationStatus.captured) void searchLocations();
      else if (!locationBrowserOpened && !locationPromptDismissed && !locationDialogOpen) locationDialogOpen = true;
    }, 250);
  }
  function selectLocation(option: ListingFieldOption) {
    const hit = locationHits.find(hit => (hit.pageID || hit.addressKey) === option.value);
    if (!hit) return;
    clearTimeout(locationTimer); locationSearchVersion++;
    fields.location = option.value; fields.locationData = { ...hit }; locationQuery = hit.label;
    if (home) fields.address = hit.label;
    locationHits = []; locationSearched = false; locationSearching = false;
  }
  let locationStatus = $state<MarketplaceLocationStatus | null>(null), locationPollError = $state(''), cancellingLocation = $state(false);
  let locationGeneration = 0;
  const locationWaiting = $derived(locationStatus && ['checking', 'waiting_login', 'expanding_details', 'capturing_location'].includes(locationStatus.state));
  const tourReady = $derived(Boolean(profile) && visible && !busy && !locationWaiting && !locationDialogOpen && !dragging);
  $effect(() => { if (visible) ontourready?.(tourReady); });
  async function refreshLocationStatus() {
    if (!profile || !visible || destroyed) return;
    const generation = locationGeneration;
    try {
      const status = await createAccountService(profile.id).GetMarketplaceLocationStatus(home);
      if (destroyed || generation !== locationGeneration || locationStatus && status.runID < locationStatus.runID) return;
      if (locationStatus && status.runID > locationStatus.runID) {
        locationSearchVersion++; lastLocationQuery = ''; locationSearchError = false;
        locationHits = []; locationSearching = false; locationSearched = false;
      }
      locationStatus = status;
      locationPollError = '';
      locationBrowserOpened = !['idle', 'cancelled', 'error', 'browser_closed', 'details_error', 'connection_required'].includes(status.state);
      if (status.state === 'ready' && status.captured && awaitingLocationSuccess) {
        awaitingLocationSuccess = false;
        await tick();
        if (destroyed || !visible || generation !== locationGeneration) return;
        try { await Window.Focus(); } catch { /* The input can still be focused inside this window. */ }
        if (destroyed || !visible || generation !== locationGeneration) return;
        const input = document.getElementById('listing-location') as HTMLInputElement | null;
        input?.focus({ preventScroll: true });
        input?.setSelectionRange(input.value.length, input.value.length);
        notify({ id: `location-ready-${profile.id}`, kind: 'success', message: /* @wc-include */ 'Location search is ready.' });
      }
      if (status.state === 'ready' && status.captured && locationQuery.trim() && !fields.location && !locationSearching && !locationSearchError) void searchLocations();
    } catch (cause) { if (!destroyed && generation === locationGeneration) locationPollError = cause instanceof Error ? cause.message : String(cause); }
  }
  async function cancelLocationWait() {
    if (!profile || !locationStatus || cancellingLocation) return;
    cancellingLocation = true; awaitingLocationSuccess = false;
    locationGeneration++;
    locationPromptDismissed = true; clearTimeout(locationTimer);
    try {
      await createAccountService(profile.id).CancelMarketplaceLocation(locationStatus.runID);
      if (destroyed) return;
      locationStatus = { ...locationStatus, state: 'cancelled' };
      locationBrowserOpened = false;
    } catch (cause) { if (!destroyed) locationPollError = cause instanceof Error ? cause.message : String(cause); }
    finally { if (!destroyed) cancellingLocation = false; }
  }
  onMount(() => pollWhileWindowOpen(refreshLocationStatus, 1000));
  function requestLocationSearch() {
    if (profile && locationQuery.trim() && !restoringLocationFocus && !locationDialogOpen && !locationBrowserOpened) locationDialogOpen = true;
  }
  async function closeLocationDialog(opened: boolean) {
    restoringLocationFocus = true;
    locationGeneration++;
    locationBrowserOpened ||= opened;
    locationPromptDismissed = !opened;
    if (opened) { awaitingLocationSuccess = true; locationSearchVersion++; locationHits = []; locationSearching = false; locationSearched = false; locationSearchError = false; lastLocationQuery = ''; }
    locationDialogOpen = false;
    await tick();
    if (!destroyed) document.getElementById('listing-location')?.focus({ preventScroll: true });
    restoringLocationFocus = false;
    if (opened) void refreshLocationStatus();
  }
  let input: HTMLInputElement;
  let thumbnails = $state<HTMLDivElement>();
  let destroyed = false;
  const active = $derived(photos[activePhoto] ?? photos[0]);
  const tags = $derived(fields.tags.split(',').map(tag => tag.trim()).filter(Boolean));
  const tagCount = $derived(tags.length);
  const valid = $derived((photos.length > 0 || Boolean(videoFile)) && !photosOverLimit && !videoError && (home ? fields.homeOffer !== '' && fields.propertyType !== '' && /^[0-9]+$/.test(fields.bedrooms) && /^[0-9]+([.][05])?$/.test(fields.bathrooms) && fields.address.trim() !== '' && fields.description.trim() !== '' && (fields.squareFeet === '' || /^[0-9]+([.][0-9]+)?$/.test(fields.squareFeet)) : vehicle ? fields.vehicleType !== '' && fields.year !== '' && fields.make.trim() !== '' && fields.model.trim() !== '' : fields.title.trim().length > 0 && fields.category !== '' && fields.condition !== '') && /^[0-9]+$/.test(fields.price) && (!detailedVehicle || /^[0-9]+$/.test(fields.mileage) && fields.transmission.trim() !== '') && (!privateRental || /^[0-9]+$/.test(fields.occupants) && fields.bathroomType !== '') && tagCount <= 20 && (kind === 'items' ? !locationQuery.trim() || Boolean(fields.locationData) : Boolean(fields.locationData)));
  const saveReady = $derived(Boolean(profile) && valid);
  const testReady = $derived(saveReady && canTestMarketplaceListing(kind, fields));
  async function testMedia() {
    if (busy || !testReady) return;
    if (!profile) return;
    testing = true; testError = ''; feedback = 'testing';
    testController = new AbortController();
    try {
      await testMarketplaceListingMedia(kind, profile.id, photos.map(photo => photo.file), videoFile, $state.snapshot(fields), testController.signal);
      if (!destroyed) feedback = 'tested';
    } catch (error) {
      if (!destroyed) {
        if (testController.signal.aborted) feedback = 'test-cancelled';
        else { feedback = 'test-error'; testError = error instanceof Error ? error.message : String(error); }
      }
    } finally { if (!destroyed) testing = false; }
  }


  $effect(() => {
    JSON.stringify(fields);
    videoFile;
    photos.map(photo => photo.id).join(',');
    untrack(() => { if (feedback === 'saved') feedback = ''; });
  });

  function openHelp(event: MouseEvent) {
    if (!(System.IsWindows() || System.IsMac() || System.IsLinux())) return;
    event.preventDefault();
    const href = (event.currentTarget as HTMLAnchorElement).href;
    helpError = '';
    void Browser.OpenURL(href).catch(cause => { helpError = cause instanceof Error ? cause.message : String(cause); });
  }

  $effect(() => {
    activePhoto;
    videoActive;
    active?.id;
    const selected = thumbnails?.querySelector('[aria-pressed=true]');
    if (!thumbnails || !selected) return;
    const viewport = thumbnails.getBoundingClientRect(), item = selected.getBoundingClientRect();
    if (item.left < viewport.left + 10) thumbnails.scrollBy({ left: item.left - viewport.left - 10 });
    else if (item.right > viewport.right - 10) thumbnails.scrollBy({ left: item.right - viewport.right + 10 });
  });

  onMount(() => {
    void (async () => {
      try {
        const draft = initialDraft ?? (profile ? await readItemListingDraft(profile.id, kind) : null);
        if (!destroyed && draft) {
          listingID = initialDraft ? initialDraft.listingID : `legacy-${kind}`;
          fields = { ...emptyItemListing(), ...draft.fields };
          videoFile = draft.video ?? null; showingVideo = initialDraft?.showingVideo ?? false;
          locationQuery = initialDraft?.locationQuery ?? draft.fields.locationData?.label ?? (home ? draft.fields.address : '') ?? '';
          locationTouched = initialDraft?.locationTouched ?? false;
          if (home && fields.locationData?.searchType !== 'STREET_PLACE_TYPEAHEAD') {
            fields.location = ''; fields.locationData = null; fields.address = '';
            locationTouched = Boolean(locationQuery.trim());
          }
          photos = draft.photos.map(file => ({ id: crypto.randomUUID(), file, url: URL.createObjectURL(file) }));
          activePhoto = Math.min(initialDraft?.activePhoto ?? 0, Math.max(0, photos.length - 1));
        }
      } catch { if (!destroyed) feedback = 'load-error'; }
      finally { if (!destroyed) loading = false; }
    })();
  });
  onDestroy(() => { destroyed = true; testController?.abort(); if (profile) dismissNotification(`location-ready-${profile.id}`); clearTimeout(locationTimer); locationSearchVersion++; clearDrag(); photos.forEach(photo => URL.revokeObjectURL(photo.url)); });

  async function addPhotos(files: FileList | File[] | null) {
    if (!files || busy) return;
    adding = true;
    feedback = '';
    const accepted = new Set(['image/jpeg', 'image/png', 'image/webp', 'image/gif']);
    try {
      for (const file of Array.from(files)) {
        if (!accepted.has(file.type)) { feedback = 'photo-error'; continue; }
        if (photos.some(photo => photo.file.name === file.name && photo.file.size === file.size && photo.file.lastModified === file.lastModified)) continue;
        try {
          const copy = await snapshotImage(file);
          if (destroyed) return;
          photos = [...photos, { id: crypto.randomUUID(), file: copy, url: URL.createObjectURL(copy) }];
        } catch { feedback = 'photo-error'; }
      }
    } finally { adding = false; }
  }
  function removePhoto(id: string) {
    const activeID = active?.id;
    const removed = photos.find(photo => photo.id === id);
    if (removed) URL.revokeObjectURL(removed.url);
    photos = photos.filter(photo => photo.id !== id);
    activePhoto = activeID && activeID !== id ? Math.max(0, photos.findIndex(photo => photo.id === activeID)) : Math.min(activePhoto, Math.max(0, photos.length - 1));
    feedback = '';
  }
  async function save() {
    if (!profile || busy || !valid) return;
    saving = true;
    feedback = '';
    try {
      listingID = await saveMarketplaceListing(profile.id, { fields: $state.snapshot(fields), photos: photos.map(photo => photo.file), activePhoto, video: videoFile }, kind, listingID);
      feedback = 'saved';
      onsaved?.();
    }
    catch { feedback = 'save-error'; }
    finally { saving = false; }
  }
  function priceInput(event: Event) {
    const input = event.currentTarget as HTMLInputElement;
    if (/^[0-9]*$/.test(input.value)) fields.price = input.value;
    else input.value = fields.price;
  }
  function mileageInput(event: Event) {
    const input = event.currentTarget as HTMLInputElement;
    if (/^[0-9]*$/.test(input.value)) fields.mileage = input.value;
    else input.value = fields.mileage;
  }
  function movePhoto(direction: number) { showingVideo = false; if (photos.length) activePhoto = (activePhoto + direction + photos.length) % photos.length; }
</script>

{#snippet locationField()}
            <ListingFieldSelect id="listing-location" required={kind !== 'items'} invalid={locationTouched && !locationSearching && !locationWaiting && !locationDialogOpen && !locationSearchError && !fields.locationData && (kind !== 'items' || locationQuery.trim() !== '')} errorMessage="Please enter a valid location." onfocus={() => { locationTouched = false; }} onblur={() => { if (!locationDialogOpen && !restoringLocationFocus) locationTouched = true; }} label={home ? 'Address' : 'Location'} placeholder={home ? 'Enter an address' : 'Enter a town or city'} leadingIcon="map-pin" searchable options={locationOptions} bind:value={fields.location} bind:query={locationQuery} remote showResults={locationSearching || locationSearched} loading={locationSearching} emptyText="No locations found." onquery={locationQueryChanged} onselect={selectLocation} />
            {#if locationStatus && !['idle', 'cancelled', 'ready'].includes(locationStatus.state)}
              <div class="location-status" role="status" aria-live="polite">
                {#if locationStatus.state === 'waiting_login'}<p class="hint">Finish signing in or any verification prompts in Chrome. Your login will be detected automatically.</p>
                {:else if locationStatus.state === 'capturing_location'}<p class="hint">Searching…</p>
                {:else if locationStatus.state === 'expanding_details'}<p class="hint">Preparing Marketplace…</p>
                {:else if locationStatus.state === 'ready' && locationStatus.captured}<p class="hint">Location search is ready.</p>
                {:else if locationStatus.state === 'connection_required'}<p class="hint">Reconnect to Facebook to search locations.</p>
                {:else if locationStatus.state === 'browser_closed'}<p class="hint">Facebook was closed. Continue to Facebook to try again.</p>
                {:else if locationStatus.state === 'details_error'}<p class="hint invalid">Could not expand More details. Please try again.</p>
                {:else if locationStatus.state === 'error'}<p class="hint invalid"><LocalizedFeedback message={locationStatus.message} /></p>
                {:else}<p class="hint">Checking Facebook login</p>{/if}
                {#if locationWaiting}<button class="location-status-action" type="button" disabled={cancellingLocation} onclick={cancelLocationWait}>Cancel</button>
                {:else if ['browser_closed', 'error', 'details_error', 'connection_required'].includes(locationStatus.state)}<button class="location-status-action" type="button" onclick={() => { locationDialogOpen = true; }}>Continue to Facebook</button>{/if}
              </div>
            {/if}
            {#if locationQuery.trim() && !locationBrowserOpened && !locationDialogOpen && (!locationStatus || ['idle', 'cancelled'].includes(locationStatus.state))}<button class="location-status-action" type="button" onclick={requestLocationSearch}>Continue to Facebook</button>{/if}
            {#if locationSearchError && locationStatus?.state !== 'connection_required'}<div><p class="hint invalid" role="alert">Could not search locations. Please try again.</p><button class="location-status-action" type="button" onclick={() => { locationSearchError = false; locationSearchAfter = 0; lastLocationQuery = ''; void searchLocations(); }}>Try again</button></div>{/if}
            {#if locationPollError}<p class="hint invalid" role="alert"><LocalizedFeedback message={locationPollError} /></p>{/if}
{/snippet}

{#if locationDialogOpen && profile}<LocationSearchDialog {profile} query={locationQuery} address={home} onclose={closeLocationDialog} />{/if}

<svelte:window onpointermove={movePointer} onpointerup={endPointer} onpointercancel={clearDrag} onblur={() => { clearDrag(); pauseMedia(); }} onkeydown={(event) => { if (event.key === 'Escape') clearDrag(); }} />

{#if draggedPhoto && pointer}
  <div class="photo-drag-preview" bind:this={dragPreview} style:width={`${pointer.width}px`} style:height={`${pointer.height}px`} style:transform={`translate3d(${pointer.x - pointer.offsetX}px, ${pointer.y - pointer.offsetY}px, 0)`} aria-hidden="true"><img src={photos.find(photo => photo.id === draggedPhoto)?.url} alt="" draggable="false" /></div>
{/if}

<div bind:this={root} onplaycapture={(event) => pauseMedia(event.target)} class="item-listing-editor" class:listing-details={visible} data-listing-type={kind}>
  <form class="form-pane" oninput={() => { if (feedback === 'saved') feedback = ''; }} onsubmit={(event) => { event.preventDefault(); if (saveReady && !busy) onnext?.({ ...fields }, photos.map(photo => photo.file), hasVideo ? videoFile : null); }}>
    <header class="form-heading">
      <div><button class="previous" type="button" disabled={busy} onclick={onprevious}><span class="previous-icon"><PhosphorIcon name="arrow-left" size={14} /></span><span>Previous</span></button><h1 id={visible ? 'listing-step-heading' : undefined} tabindex="-1">{#if home}New home listing{:else if vehicle}Vehicle for sale{:else}Item for sale{/if}</h1></div>
      <button class="save-draft" type="button" disabled={!saveReady || busy} aria-busy={saving} onclick={save}>Save listing</button>
    </header>
    <div class="form-scroll" data-step-scroll>
      {#if profile}<div class="account-line"><PhosphorIcon name="user-circle" size={36} /><div><strong>{accountLabel(profile)}</strong><p class="listing-visibility"><span>Listing to Marketplace</span><span>Public</span></p></div></div>{/if}
      {#if loading}<p class="feedback" role="status">Loading…</p>{/if}
      <fieldset disabled={loading || saving || adding || testing}>
        {#if vehicle}
          <div class="vehicle-type"><ListingFieldSelect id="listing-vehicle-type" label="Vehicle type" placeholder="Vehicle type" required options={vehicleListingFieldOptions.vehicleType} bind:value={fields.vehicleType} /></div>
          {#if !hasVehicleMedia}<h2 class="photo-section-heading">Photo upload</h2>{/if}
        {/if}
        <div class="listing-media">
          <div class="photo-upload-block">
        {#if mediaBusy}<div class="video-loading-status" role="status"><span>Loading video…</span><progress aria-label="Loading video"></progress></div>{/if}
        <div class="photo-label" class:invalid={photosOverLimit || videoError}><strong>Photos</strong><span role="status" aria-live="polite">{photos.length} / {maxPhotos}</span>{#if hasVideo}<strong>Videos</strong><span role="status" aria-live="polite">{videoFile || mediaBusy ? 1 : 0} / 1</span>{/if}</div>
        <p class="hint">{#if home}You can add up to 50 photos.{:else if vehicle}You can add up to 20 photos.{:else}You can add up to 10 photos.{/if}</p>
        {#if !photos.length && !videoFile && !videoBusy}<p class="hint">Add at least one photo or video.</p>{/if}
        <input class="file-input" id="listing-photos" type="file" multiple accept="image/jpeg,image/png,image/webp,image/gif" bind:this={input} onchange={(event) => { void addPhotos(event.currentTarget.files); event.currentTarget.value = ''; }} aria-label="Choose images" aria-invalid={photosOverLimit} aria-describedby={photosOverLimit ? 'listing-photo-limit-error' : undefined} />
        <div class="photo-upload" class:media-invalid={photosOverLimit || mediaBusy || videoError} class:dragging ondragover={overPhotos} ondragleave={(event) => { if (!event.currentTarget.contains(event.relatedTarget as Node | null)) { dragging = false; } }} ondrop={dropPhotos} role="group" aria-label="Photos">
            <div class="photo-grid" class:empty-media={!photos.length && !videoFile && !mediaBusy}>
              {#if hasVideo}<ListingVideo bind:this={videoControl} bind:file={videoFile} bind:busy={videoBusy} bind:error={videoError} disabled={loading || saving || adding} selected={videoActive} thumbnail={videoPoster} thumbnailBusy={posterBusy} onselect={() => showingVideo = true} />{/if}
              {#each photos as photo, index (photo.id)}
                <div class="photo-tile" data-photo-id={photo.id} class:drag-source={draggedPhoto === photo.id} class:drop-before={dropTarget === photo.id && draggedPhoto !== photo.id && dropSide === 'before'} class:drop-after={dropTarget === photo.id && draggedPhoto !== photo.id && dropSide === 'after'}>
                  <button id={`listing-photo-${photo.id}`} class="photo-select" type="button" draggable="false" onpointerdown={(event) => startPointer(event, photo.id)} onlostpointercapture={clearDrag} ondragstart={(event) => event.preventDefault()} onkeydown={(event) => photoKeydown(event, photo.id)} aria-describedby={photos.length > 1 ? 'listing-reorder-help' : undefined} aria-keyshortcuts={/* @wc-ignore */ "Alt+ArrowLeft Alt+ArrowRight"} aria-pressed={!videoActive && activePhoto === index} onclick={(event) => { if (suppressClick) { suppressClick = false; event.preventDefault(); return; } showingVideo = false; activePhoto = index; }}><img src={photo.url} alt={photo.file.name} draggable="false" onerror={() => { removePhoto(photo.id); feedback = 'photo-error'; }} /></button>
                  <button class="remove-photo" type="button" aria-label={`Remove ${photo.file.name}`} onclick={() => removePhoto(photo.id)}><PhosphorIcon name="x" size={14} /></button>
                </div>
              {/each}
              {#if photos.length || videoFile || mediaBusy}
                <button class="add-photo-tile" type="button" aria-label="Add photos" onclick={() => input.click()}><PhosphorIcon name="plus" /></button>
              {:else}
                <button class="add-photos" type="button" onclick={() => input.click()}><PhosphorIcon name="image" size={28} /><strong>Add photos</strong><span>or drag and drop</span></button>
              {/if}
            </div>
        </div>
        {#if mediaBusy}<p class="photo-limit-error" role="alert"><PhosphorIcon name="warning" size={16} /><span>Your video must finish loading before proceeding.</span></p>{/if}
        {#if videoError}<p class="photo-limit-error" role="alert"><PhosphorIcon name="warning" size={16} /><span>Choose a supported video up to 1 minute long.</span></p>{/if}
        {#if posterError}<div role="status"><p class="hint">Could not load video thumbnail.</p><button class="location-status-action" type="button" onclick={() => { posterAttempt++; }}>Try again</button></div>{/if}
        {#if photosOverLimit}<p class="photo-limit-error" id="listing-photo-limit-error" role="alert"><PhosphorIcon name="warning" size={16} /><span>You may only select a maximum of {maxPhotos} photos.</span></p>{/if}
        {#if photos.length > 1}<p class="hint reorder-help" id="listing-reorder-help">Drag images to rearrange. Or focus an image and use Alt + Left / Right.</p>{/if}
        <p class="sr-only" role="status">{logMessage(reorderMessage)}</p>
          </div>
        </div>
        {#if home}
          <div class="fields home-fields">
            <ListingFieldSelect id="listing-home-offer" label="Home for sale or rent" required options={homeListingFieldOptions.homeOffer} bind:value={fields.homeOffer} />
            <ListingFieldSelect id="listing-property-type" label={rental ? 'Rental type' : 'Property type'} required options={homeListingFieldOptions.propertyType} bind:value={fields.propertyType} />
            {#if rental}<ListingSwitch id="listing-private-room" label="This is a private room in a shared property." bind:checked={fields.privateRoom} />{/if}
            {#if privateRental}
              <div class="text-field" class:filled={Boolean(fields.occupants)}><label for="listing-occupants">How many people live here?</label><input class="ui-input" id="listing-occupants" type="text" inputmode="numeric" pattern="[0-9]+" bind:value={fields.occupants} required /></div>
              <section class="home-room-details" aria-labelledby="home-room-heading">
                <h2 id="home-room-heading">Room Details</h2>
                <div class="text-field" class:filled={Boolean(fields.price)}><label for="listing-room-price">Price per month</label><input class="ui-input" id="listing-room-price" type="text" inputmode="numeric" pattern="[0-9]+" value={fields.price} oninput={priceInput} required /></div>
                <ListingFieldSelect id="listing-bathroom-type" label="Bathroom Type" required options={homeListingFieldOptions.bathroomType} bind:value={fields.bathroomType} />
                <ListingSwitch id="listing-room-furnished" label="Is this room furnished?" bind:checked={fields.roomFurnished} />
              </section>
              <h2 class="home-property-heading">Property Details</h2>
            {/if}
            <div class="text-field" class:filled={Boolean(fields.bedrooms)}><label for="listing-bedrooms">Number of bedrooms</label><input class="ui-input" id="listing-bedrooms" type="text" inputmode="numeric" pattern="[0-9]+" bind:value={fields.bedrooms} required /></div>
            <div class="text-field" class:filled={Boolean(fields.bathrooms)}><label for="listing-bathrooms">Number of bathrooms</label><input class="ui-input" id="listing-bathrooms" type="text" inputmode="decimal" pattern={'[0-9]+([.][05])?'} bind:value={fields.bathrooms} required /></div>
            {#if !privateRental}<div class="text-field" class:filled={Boolean(fields.price)}><label for="listing-price">{#if rental}Price per month{:else}Price{/if}</label><input class="ui-input" id="listing-price" type="text" inputmode="numeric" pattern="[0-9]+" value={fields.price} oninput={priceInput} required /></div>{/if}
            {@render locationField()}
            <div><div class="text-field" class:filled={Boolean(fields.description)}><label for="listing-description">{#if rental}Rental description{:else}Property description{/if}</label><textarea class="ui-input" id="listing-description" dir="auto" rows="5" bind:value={fields.description} aria-describedby="listing-description-hint" required></textarea></div><p class="hint" id="listing-description-hint">Include details like utilities, amenities, any deposits needed and when it's available.</p></div>
          </div>
          <section class="home-advanced" aria-labelledby="home-advanced-heading">
            <div class="home-advanced-heading"><h2 id="home-advanced-heading">Advanced Details</h2><span class="hint">Optional</span></div>
            <div class="fields">
              <div class="text-field" class:filled={Boolean(fields.squareFeet)}><label for="listing-square-feet">Property square feet</label><input class="ui-input" id="listing-square-feet" type="text" inputmode="decimal" pattern={'[0-9]+([.][0-9]+)?'} bind:value={fields.squareFeet} /></div>
              {#if rental}<ListingDatePicker id="listing-date-available" label="Date available" bind:value={fields.dateAvailable} />{/if}
              <ListingFieldSelect id="listing-laundry" label="Laundry type" options={homeListingFieldOptions.laundry} bind:value={fields.laundry} />
              <ListingFieldSelect id="listing-parking" label="Parking type" options={homeListingFieldOptions.parking} bind:value={fields.parking} />
              <ListingFieldSelect id="listing-air-conditioning" label="Air conditioning type" options={homeListingFieldOptions.airConditioning} bind:value={fields.airConditioning} />
              <ListingFieldSelect id="listing-heating" label="Heating type" options={homeListingFieldOptions.heating} bind:value={fields.heating} />
              {#if rental}
                <ListingSwitch id="listing-cat-friendly" label="Cat friendly" bind:checked={fields.catFriendly} />
                <ListingSwitch id="listing-dog-friendly" label="Dog friendly" bind:checked={fields.dogFriendly} />
              {/if}
            </div>
          </section>
        {:else if vehicle}
          <section class="vehicle-section" data-tour="vehicle-details" aria-labelledby="vehicle-about-heading">
            <div><h2 id="vehicle-about-heading">About this vehicle</h2><p class="hint">Help buyers know more about the vehicle you're listing.</p></div>
            {@render locationField()}
            <ListingFieldSelect id="listing-year" label="Year" placeholder="Year" required options={vehicleListingFieldOptions.year} bind:value={fields.year} />
            {#if detailedVehicle}
              <ListingFieldSelect id="listing-make" label="Make" placeholder="Make" required options={makeOptions} bind:value={fields.make} />
            {:else}
            <div class="text-field" class:filled={Boolean(fields.make)}><label for="listing-make">Make</label><input class="ui-input" id="listing-make" dir="auto" bind:value={fields.make} required /></div>
            {/if}
            <div class="text-field" class:filled={Boolean(fields.model)}><label for="listing-model">Model</label><input class="ui-input" id="listing-model" dir="auto" bind:value={fields.model} required /></div>
            {#if detailedVehicle}<div class="text-field" class:filled={Boolean(fields.mileage)}><label for="listing-mileage">Mileage</label><input class="ui-input" id="listing-mileage" type="text" inputmode="numeric" pattern="[0-9]+" value={fields.mileage} oninput={mileageInput} required /></div>{/if}
          </section>
          <section class="vehicle-section" aria-labelledby="vehicle-price-heading">
            <div><h2 id="vehicle-price-heading">Price</h2><p class="hint">Enter your price for this vehicle.</p></div>
            <div class="text-field" class:filled={Boolean(fields.price)}><label for="listing-price">Price</label><input class="ui-input" id="listing-price" type="text" inputmode="numeric" value={fields.price} oninput={priceInput} pattern="[0-9]+" required /></div>
          </section>
          {#if carTruck}
            <section class="vehicle-section" aria-labelledby="vehicle-appearance-heading">
              <div><h2 id="vehicle-appearance-heading">Vehicle appearance and features</h2><p class="hint">Add more about what your vehicle looks like and the features that it has.</p></div>
              <ListingFieldSelect id="listing-body-style" label="Body style" placeholder="Body style" options={carOptions.bodyStyle} bind:value={fields.bodyStyle} />
              <ListingFieldSelect id="listing-exterior-color" label="Exterior color" placeholder="Exterior color" options={detailOptions.exteriorColor} bind:value={fields.exteriorColor} />
              <ListingFieldSelect id="listing-interior-color" label="Interior color" placeholder="Interior color" options={carOptions.interiorColor} bind:value={fields.interiorColor} />
            </section>
          {/if}
          {#if motorcycle || powersport || rvCamper || trailer || boat || commercialIndustrial}
            <div class="vehicle-color"><ListingFieldSelect id="listing-exterior-color" label="Exterior color" placeholder="Exterior color" options={detailOptions.exteriorColor} bind:value={fields.exteriorColor} /></div>
          {/if}
          {#if powersport || rvCamper || trailer || boat || commercialIndustrial}
            <div class="vehicle-color"><ListingFieldSelect id="listing-interior-color" label="Interior color" placeholder="Interior color" options={carOptions.interiorColor} bind:value={fields.interiorColor} /></div>
          {/if}
          {#if hasVehicleDetails}
            <section class="vehicle-section" aria-labelledby="vehicle-details-heading">
              <div><h2 id="vehicle-details-heading">Vehicle details</h2><p class="hint">Include more details to help connect interested buyers to your vehicle.</p></div>
              {#if carTruck}
                <Checkbox id="listing-clean-title" text="This vehicle has a clean title." description="This vehicle has no significant damage or persistent problems." bind:checked={fields.cleanTitle} />
                <ListingFieldSelect id="listing-vehicle-condition" label="Vehicle condition" placeholder="Vehicle condition" options={carOptions.condition} bind:value={fields.vehicleCondition} />
              {/if}
              <ListingFieldSelect id="listing-fuel-type" label="Fuel type" placeholder="Fuel type" options={detailOptions.fuelType} bind:value={fields.fuelType} />
              {#if detailedVehicle}<ListingFieldSelect id="listing-transmission" label="Transmission" placeholder="Transmission" required options={detailOptions.transmission} bind:value={fields.transmission} />{/if}
            </section>
          {/if}
          <section class="vehicle-section" aria-labelledby="vehicle-description-heading">
            <div><h2 id="vehicle-description-heading">Description</h2><p class="hint">Tell buyers anything that you haven't had the chance to include yet about your vehicle.</p></div>
            <div class="text-field" class:filled={Boolean(fields.description)}><label for="listing-description">Description</label><textarea class="ui-input" id="listing-description" dir="auto" rows="5" bind:value={fields.description}></textarea></div>
          </section>
        {:else}
        <h2 class="required-heading">Required</h2>
        <p class="hint">Be as descriptive as possible.</p>
        <div class="fields" data-tour="item-details">
          <div class="text-field" class:filled={Boolean(fields.title)}><label for="listing-title">Title</label><input class="ui-input" id="listing-title" dir="auto" bind:value={fields.title} required /></div>
          <div class="text-field" class:filled={Boolean(fields.price)}><label for="listing-price">Price</label><input class="ui-input" id="listing-price" type="text" inputmode="numeric" value={fields.price} oninput={priceInput} pattern="[0-9]+" required /></div>
          <ListingFieldSelect id="listing-category" label="Category" placeholder="Category" required options={itemListingFieldOptions.category} value={fields.category} onselect={(option) => { if (option.value === 'vehicles' && onvehicle) onvehicle(); else fields.category = option.value; }} />
          <ListingFieldSelect id="listing-condition" label="Condition" placeholder="Condition" required options={itemListingFieldOptions.condition} bind:value={fields.condition} />
          <div class="text-field" class:filled={Boolean(fields.description)}><label for="listing-description">Description</label><textarea class="ui-input" id="listing-description" dir="auto" rows="5" bind:value={fields.description}></textarea></div>
        </div>
        <details class="more-details">
          <summary><span><strong>More details</strong><span class="hint">Attract more interest by including more details.</span></span><span class="details-caret"><PhosphorIcon name="caret-down" size={16} /></span></summary>
          <div class="fields">
            <ListingFieldSelect id="listing-availability" label="Availability" placeholder="List as Single Item" options={itemListingFieldOptions.availability} bind:value={fields.availability} />
            <div><ListingTags bind:value={fields.tags} /></div>
            <div><div class="text-field" class:filled={Boolean(fields.sku)}><label for="listing-sku">SKU</label><input class="ui-input" id="listing-sku" bind:value={fields.sku} aria-describedby="listing-sku-hint" /></div><span class="hint" id="listing-sku-hint">Optional. Only visible to you.</span></div>
            {@render locationField()}
            <section class="meetup-preferences" aria-labelledby="listing-meetup-heading">
              <h2 id="listing-meetup-heading">Meetup preferences</h2>
              <p class="hint">Buyers will be able to see your preferences on your listing.</p>
              <a class="help-link" href="https://www.facebook.com/help/329122794534612" target="_blank" rel="noopener noreferrer" aria-label="Learn more about meetup preferences" onclick={openHelp}>Learn more</a>
              <div class="meetup-options" role="group" aria-labelledby="listing-meetup-heading">
                <Checkbox id="listing-public-meetup" text="Public meetup" description="Meet at a public location." bind:checked={fields.publicMeetup} />
                <a class="help-link safety-link" href="https://www.facebook.com/help/1156544111079919" target="_blank" rel="noopener noreferrer" aria-label="See Marketplace meetup safety tips" onclick={openHelp}>See safety tips</a>
                <Checkbox id="listing-door-pickup" text="Door pickup" description="Buyer picks up at your door." bind:checked={fields.doorPickup} />
                <Checkbox id="listing-door-dropoff" text="Door dropoff" description="You drop off at buyer's door." bind:checked={fields.doorDropoff} />
              </div>
            </section>
          </div>
        </details>
        {/if}
        <div class="listing-preferences">
          <ListingSwitch id="listing-promote" label="Promote listing after publish" description="Add a step to promote your listing after it publishes, to help you reach more potential buyers by turning it into an ad." bind:checked={fields.promoteAfterPublish} />
          {#if kind === 'items'}<ListingSwitch id="listing-hide-friends" label="Hide from friends" description="This listing is still public. If you hide this listing from friends, they won't see it in most cases." bind:checked={fields.hideFromFriends} />
          <a class="help-link" href="https://www.facebook.com/help/1096431074294038" target="_blank" rel="noopener noreferrer" aria-label="Learn more about hiding listings from friends" onclick={openHelp}>Learn more</a>{/if}
        </div>
        <p class="public-notice">Marketplace items are public and can be seen by anyone on or off Facebook. Items like animals, drugs, weapons, counterfeits, and other items that infringe intellectual property aren't allowed on Marketplace.</p>
        <a class="help-link" href="https://www.facebook.com/policies_center/commerce/" target="_blank" rel="noopener noreferrer" onclick={openHelp}>Commerce Policies</a>
      </fieldset>
      {#if helpError}<p class="invalid" role="alert"><LocalizedFeedback message={helpError} /></p>{/if}
    </div>
    <footer class="form-footer">
      <div class="listing-progress" role="progressbar" aria-label="Create new listing" aria-valuemin="0" aria-valuemax="2" aria-valuenow="1"><span class="complete"></span><span></span></div>
      <div class="feedback" class:has-feedback={feedback !== ''} class:invalid={feedback.endsWith('error')} role="status" aria-live="polite">
        {#if feedback === 'testing'}Finish signing in or any verification prompts in Chrome. Your listing will be filled automatically.
        {:else if feedback === 'tested'}Listing filled in Facebook. Review it in Chrome.
        {:else if feedback === 'test-cancelled'}Test cancelled.
        {:else if feedback === 'test-error'}<LocalizedFeedback message={testError} />
        {:else if feedback === 'saved'}Listing saved.
        {:else if feedback === 'save-error'}Could not save listing. Please try again.
        {:else if feedback === 'load-error'}Could not load draft. Please try again.
        {:else if feedback === 'photo-error'}Choose JPG, PNG, WebP, or GIF images{/if}
      </div>
      {#if testing}<button class="location-status-action" type="button" onclick={() => testController?.abort()}>Cancel</button>{/if}
      <div class="footer-actions">
        <button class="action-button action-button--secondary" data-tour="listing-test" type="button" disabled={!testReady || busy} aria-busy={testing} onclick={testMedia}>Test</button>
        <button class="action-button action-button--primary" data-tour="listing-save" type="button" disabled={!saveReady || busy} aria-busy={saving} onclick={save}>Save</button>
      </div>
    </footer>
  </form>

  <section class="preview-pane" aria-labelledby="listing-preview-heading">
    <div class="preview-card">
      <h2 id="listing-preview-heading">Preview</h2>
      <div class="preview-body" data-step-scroll>
        <div class="preview-media">
          <div class="preview-image-area">
            {#if videoActive && videoFile}
              {#if videoPoster}<img class="preview-image" src={videoPoster} alt={videoFile.name} />
              {:else}<div class="preview-empty"><PhosphorIcon name="video-camera" size={32} /><p dir="auto">{videoFile.name}</p></div>{/if}
            {:else if active}
              <img draggable="false" class="preview-image" src={active.url} alt={active.file.name} />
              {#if photos.length > 1}<div class="gallery-controls"><button type="button" aria-label="Previous" onclick={() => movePhoto(-1)}><PhosphorIcon name="arrow-left" /></button><button class="next-photo" type="button" aria-label="Next" onclick={() => movePhoto(1)}><PhosphorIcon name="arrow-left" /></button></div>{/if}
            {:else}
              <div class="preview-empty"><PhosphorIcon name="image" size={32} /><h3>Your listing preview</h3><p>As you create your listing, you can preview how it will appear on Marketplace.</p></div>
            {/if}
          </div>
          {#if photos.length > 1 || hasVideo && videoFile && photos.length > 0}<div class="preview-thumbnails" aria-label="Photos" bind:this={thumbnails}>{#each photos as photo, index (photo.id)}<button type="button" aria-pressed={!videoActive && activePhoto === index} onclick={() => { showingVideo = false; activePhoto = index; }}><img draggable="false" src={photo.url} alt={photo.file.name} /></button>{/each}{#if hasVideo && videoFile}<button class="video-thumbnail" type="button" aria-label="Video preview" aria-pressed={videoActive} onclick={() => showingVideo = true}>{#if videoPoster}<img src={videoPoster} alt={videoFile.name} />{:else}<PhosphorIcon name="video-camera" size={20} />{/if}</button>{/if}</div>{/if}
        </div>
        <div class="preview-details">
          <h3 class:placeholder={!previewTitle} id="preview-item-title" dir="auto">{#if previewTitle}{previewTitle}{:else}Title{/if}</h3>
          <p class="preview-price" class:placeholder={!fields.price}>{#if fields.price}{fields.price}{:else}Price{/if}{#if home && fields.homeOffer !== 'sale'} / Month{/if}</p>
          {#if !home}{#if fields.locationData}<p class="hint preview-location" dir="auto">{fields.locationData.label}</p>{:else if fields.location}<p class="hint">{fields.location}</p>{/if}{/if}
          {#if home}
            <h4>{#if fields.homeOffer === 'sale'}Location{:else}Rental Location{/if}</h4>
            {#if fields.locationData}<ListingLocationPreview location={fields.locationData} onopen={openHelp} />{/if}
            <dl class="preview-attributes">
              {#if fields.bedrooms}<div><dt>Number of bedrooms</dt><dd>{fields.bedrooms}</dd></div>{/if}
              {#if fields.bathrooms}<div><dt>Number of bathrooms</dt><dd>{fields.bathrooms}</dd></div>{/if}
              {#if fields.squareFeet}<div><dt>Property square feet</dt><dd>{fields.squareFeet}</dd></div>{/if}
              {#each [{ key: 'laundry', label: 'Laundry type' }, { key: 'parking', label: 'Parking type' }, { key: 'airConditioning', label: 'Air conditioning type' }, { key: 'heating', label: 'Heating type' }] as detail}
                {@const key = detail.key as 'laundry' | 'parking' | 'airConditioning' | 'heating'}
                {#if fields[key]}<div><dt>{detail.label}</dt><dd>{homeListingFieldOptions[key].find(option => option.value === fields[key])?.label ?? fields[key]}</dd></div>{/if}
              {/each}
            </dl>
            {#if rental}
              {#if fields.dateAvailable}<p><strong>Date available:</strong> <time datetime={fields.dateAvailable}>{fields.dateAvailable}</time></p>{/if}
              {#if privateRental}
                <p>This is a private room in a shared property.</p>
                <dl class="preview-attributes">
                  {#if fields.occupants}<div><dt>How many people live here?</dt><dd>{fields.occupants}</dd></div>{/if}
                  {#if fields.bathroomType}<div><dt>Bathroom Type</dt><dd>{homeListingFieldOptions.bathroomType.find(option => option.value === fields.bathroomType)?.label ?? fields.bathroomType}</dd></div>{/if}
                </dl>
                {#if fields.roomFurnished}<p>Furnished</p>{/if}
              {/if}
              {#if fields.catFriendly || fields.dogFriendly}<ul class="preview-meetups">{#if fields.catFriendly}<li>Cat friendly</li>{/if}{#if fields.dogFriendly}<li>Dog friendly</li>{/if}</ul>{/if}
            {/if}
            <h4>Description</h4>
          {:else if vehicle}
            <h4>About this vehicle</h4>
            <dl class="preview-attributes">
              {#if fields.year}<div><dt>Year</dt><dd>{fields.year}</dd></div>{/if}
              {#if fields.make}<div><dt>Make</dt><dd dir="auto">{fields.make}</dd></div>{/if}
              {#if fields.model}<div><dt>Model</dt><dd dir="auto">{fields.model}</dd></div>{/if}
              {#if hasVehicleDetails || trailer}
                {#if detailedVehicle && fields.mileage}<div><dt>Mileage</dt><dd>{fields.mileage}</dd></div>{/if}
                {#if carTruck && fields.bodyStyle}<div><dt>Body style</dt><dd>{carOptions.bodyStyle.find(option => option.value === fields.bodyStyle)?.label ?? fields.bodyStyle}</dd></div>{/if}
                {#if fields.exteriorColor}<div><dt>Exterior color</dt><dd>{detailOptions.exteriorColor.find(option => option.value === fields.exteriorColor)?.label ?? fields.exteriorColor}</dd></div>{/if}
                {#if (carTruck || powersport || rvCamper || trailer || boat || commercialIndustrial) && fields.interiorColor}<div><dt>Interior color</dt><dd>{carOptions.interiorColor.find(option => option.value === fields.interiorColor)?.label ?? fields.interiorColor}</dd></div>{/if}
                {#if carTruck && fields.vehicleCondition}<div><dt>Vehicle condition</dt><dd>{carOptions.condition.find(option => option.value === fields.vehicleCondition)?.label ?? fields.vehicleCondition}</dd></div>{/if}
                {#if hasVehicleDetails && fields.fuelType}<div><dt>Fuel type</dt><dd>{detailOptions.fuelType.find(option => option.value === fields.fuelType)?.label ?? fields.fuelType}</dd></div>{/if}
                {#if detailedVehicle && fields.transmission}<div><dt>Transmission</dt><dd>{detailOptions.transmission.find(option => option.value === fields.transmission)?.label ?? fields.transmission}</dd></div>{/if}
              {/if}
            </dl>
            {#if carTruck && fields.cleanTitle}<p class="hint">This vehicle has a clean title.</p>{/if}
            <h4>Seller's description</h4>
          {:else}
          <h4>Details</h4>
          {#if fields.condition}<dl class="preview-attributes"><div><dt>Condition</dt><dd>{itemListingFieldOptions.condition.find(option => option.value === fields.condition)?.label ?? fields.condition}</dd></div></dl>{/if}
          {/if}
          <p class="preview-description" class:placeholder={!fields.description} dir="auto">{#if fields.description}{fields.description}{:else}Description will appear here.{/if}</p>
          {#if !home && fields.locationData}<ListingLocationPreview location={fields.locationData} onopen={openHelp} />{/if}
          {#if kind === 'items' && tags.length}
            <section class="preview-tags" aria-labelledby="preview-tags-heading">
              <h4 id="preview-tags-heading">Tags</h4>
              <ul>{#each tags as tag}<li dir="auto">{tag}</li>{/each}</ul>
            </section>
          {/if}
          {#if kind === 'items' && (fields.publicMeetup || fields.doorPickup || fields.doorDropoff)}
            <h4>Meetup preferences</h4>
            <ul class="preview-meetups">{#if fields.publicMeetup}<li>Public meetup</li>{/if}{#if fields.doorPickup}<li>Door pickup</li>{/if}{#if fields.doorDropoff}<li>Door dropoff</li>{/if}</ul>
          {/if}
          <h4>Seller information</h4>
          {#if profile}<div class="seller-line"><PhosphorIcon name="user-circle" size={40} /><strong>{accountLabel(profile)}</strong></div>{/if}
        </div>
      </div>
    </div>
  </section>
</div>

<GuidedTour scope="listing-editor" ready={tourReady} {kind} vehicleType={fields.vehicleType} homeOffer={fields.homeOffer} privateRoom={fields.privateRoom} />

<style>
  .listing-media, .photo-upload-block { min-width: 0; }
  .photo-label.invalid, .photo-label.invalid strong, .photo-label.invalid span { color: var(--danger); }
  .photo-limit-error { display: flex; align-items: flex-start; gap: .5rem; margin-top: .8rem; color: var(--danger); font-size: var(--ui-small-size); line-height: 1.5; }
  .video-thumbnail { display: grid; place-items: center; color: var(--text); }
  .home-fields { margin-top: 1.6rem; }
  .home-room-details { display: grid; gap: 1.6rem; padding-top: 1.6rem; border-top: 1px solid var(--border); }
  .home-property-heading { padding-top: 1.6rem; border-top: 1px solid var(--border); }
  .home-room-details h2, .home-property-heading { color: var(--text-strong); font-size: var(--ui-body-size); font-weight: var(--ui-title-weight); }
  .home-advanced { margin-top: 1.6rem; padding-top: 1.6rem; border-top: 1px solid var(--border); }
  .home-advanced-heading { display: flex; align-items: baseline; justify-content: space-between; gap: 1.2rem; margin-bottom: 1.6rem; }
  .home-advanced-heading h2 { color: var(--text-strong); font-size: var(--ui-body-size); font-weight: var(--ui-title-weight); }
  .vehicle-color { margin-top: 1.6rem; }
  .vehicle-type { margin-bottom: 1.6rem; }
  .photo-section-heading { margin-bottom: 1.2rem; font-size: var(--ui-body-size); font-weight: var(--ui-title-weight); color: var(--text-strong); }
  .vehicle-section { display: grid; gap: 1.6rem; margin-top: 1.6rem; padding-top: 1.6rem; border-top: 1px solid var(--border); }
  .vehicle-section h2 { font-size: var(--ui-body-size); font-weight: var(--ui-title-weight); color: var(--text-strong); }
  .item-listing-editor { display: grid; grid-template-columns: 36rem minmax(0, 1fr); height: 100%; min-height: 0; color: var(--text); }
  .form-pane { display: flex; flex-direction: column; min-height: 0; min-width: 0; border-inline-end: 1px solid var(--border); background: var(--surface); }
  .form-heading { display: flex; flex-shrink: 0; align-items: flex-start; justify-content: space-between; gap: 1rem; padding: 1.6rem; border-bottom: 1px solid var(--border); }
  h1 { margin-top: .6rem; color: var(--text-strong); font-size: var(--ui-title-size); font-weight: var(--ui-title-weight); line-height: 1.5; }
  h1:focus { outline: none; }
  .previous, .save-draft { display: inline-flex; align-items: center; gap: .4rem; min-height: 2.8rem; padding: 0; border: 0; background: transparent; color: var(--accent); font: inherit; cursor: pointer; }
  .save-draft { font-weight: var(--ui-label-weight); }
  .previous-icon { display: inline-flex; }
  :global([dir=rtl]) .previous-icon { transform: rotate(180deg); }
  button:disabled { opacity: .45; cursor: not-allowed; }
  button:focus-visible, summary:focus-visible { outline: 2px solid var(--focus); outline-offset: 2px; }
  .form-scroll { flex: 1; min-height: 0; overflow-y: auto; overscroll-behavior: contain; padding: 1.6rem; }
  .account-line, .seller-line { display: flex; align-items: center; gap: 1rem; overflow-wrap: anywhere; }
  .account-line { margin-bottom: 1.6rem; }
  .account-line p { margin-top: .3rem; color: var(--text-muted); font-size: var(--ui-small-size); }
  .listing-visibility { display: flex; flex-wrap: wrap; gap: .4rem .8rem; }
  fieldset { display: contents; }
  .photo-label { display: flex; align-items: center; gap: .8rem; }
  .photo-label span { font-size: var(--ui-small-size); color: var(--text-muted); font-variant-numeric: tabular-nums; }
  .hint { display: block; margin-top: .4rem; color: var(--text-muted); font-size: var(--ui-small-size); line-height: 1.6; font-weight: 400; }
  .file-input { display: none; }
  .photo-upload { margin-top: .8rem; border: 1px solid transparent; border-radius: var(--ui-input-radius); }
  .photo-upload.media-invalid { border-color: var(--danger); }
  .video-loading-status { display: grid; gap: .3rem; margin-bottom: .6rem; color: var(--text-muted); font-size: var(--ui-small-size); }
  .video-loading-status progress { width: 100%; height: .5rem; accent-color: var(--accent); }
  .photo-grid.empty-media { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .photo-grid.empty-media .add-photos { grid-column: 1; grid-row: 1; }
  .photo-grid.empty-media :global(.listing-video) { grid-column: 2; grid-row: 1; }
  .photo-upload.dragging { outline: 2px solid var(--focus); outline-offset: 3px; background: var(--accent-faint); }
  .add-photos { display: flex; flex-direction: column; align-items: center; justify-content: center; gap: .8rem; width: 100%; min-height: 15rem; padding: 1.6rem; border: 1px solid var(--border-strong); border-radius: var(--ui-input-radius); background: transparent; color: var(--text); font: inherit; cursor: pointer; }
  .add-photos:hover, .add-photo-tile:hover { background: var(--surface-hover); }
  .add-photos > span { color: var(--text-muted); }
  .photo-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: .6rem; }
  .photo-tile { position: relative; aspect-ratio: 1; min-width: 0; }
  .photo-select { display: block; width: 100%; height: 100%; padding: 0; border: 1px solid var(--border); border-radius: .5rem; overflow: hidden; background: var(--surface-subtle); cursor: pointer; }
  .photo-select[aria-pressed=true] { border-color: var(--accent); }
  .photo-select { cursor: grab; touch-action: none; user-select: none; -webkit-user-drag: none; }
  .photo-select img { pointer-events: none; }
  .photo-drag-preview { position: fixed; top: 0; left: 0; z-index: 1000; pointer-events: none; overflow: hidden; border-radius: .5rem; opacity: .9; box-shadow: 0 .6rem 1.8rem var(--border-strong); }
  .photo-drag-preview img { width: 100%; height: 100%; object-fit: cover; }
  .drag-source { opacity: .45; }
  .drop-before::after, .drop-after::after { content: ''; position: absolute; inset: 0; z-index: 1001; border: .4rem dashed var(--accent); border-radius: .5rem; background: color-mix(in srgb, var(--accent) 12%, transparent); pointer-events: none; }
  .reorder-help { margin-top: .8rem; }
  .sr-only { position: absolute; width: .1rem; height: .1rem; overflow: hidden; clip-path: inset(50%); white-space: nowrap; }
  .photo-select img { width: 100%; height: 100%; object-fit: cover; }
  .remove-photo { position: absolute; inset-block-start: .4rem; inset-inline-end: .4rem; display: grid; place-items: center; width: 2.8rem; height: 2.8rem; border: 1px solid var(--border); border-radius: 50%; background: var(--surface); color: var(--text); cursor: pointer; }
  .add-photo-tile { display: grid; place-items: center; aspect-ratio: 1; border: 1px solid var(--border-strong); border-radius: .5rem; background: transparent; color: var(--accent); cursor: pointer; }
  .required-heading { margin-top: 2rem; font-size: var(--ui-title-size); font-weight: var(--ui-title-weight); }
  .fields { display: flex; flex-direction: column; gap: 1.6rem; margin-top: 1.2rem; }
  .text-field { position: relative; }
  .text-field label { position: absolute; z-index: 1; inset-inline-start: 1.4rem; top: 1.8rem; max-width: calc(100% - 2.8rem); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: var(--text-muted); font-size: 1.4rem; font-weight: 400; line-height: 1.5; pointer-events: none; }
  .text-field.filled label, .text-field:focus-within label { top: .6rem; font-size: var(--ui-small-size); line-height: 1.3; }
  .ui-input { min-height: 5.8rem; padding: 2.4rem 1.4rem .8rem; border-radius: 1rem; font-size: 1.4rem; line-height: 1.5; font-weight: 400; }
  textarea { display: block; resize: vertical; min-height: 9rem; }
  .text-field:has(textarea) label { inset-inline-end: 1.4rem; background: var(--surface); }
  .more-details { margin-top: 1.6rem; }
  summary { display: flex; align-items: center; justify-content: space-between; gap: 1rem; padding-block: .8rem; cursor: pointer; list-style: none; }
  summary::-webkit-details-marker { display: none; }
  .details-caret { display: inline-flex; flex-shrink: 0; }
  .more-details[open] .details-caret { transform: rotate(180deg); }
  .meetup-preferences h2 { color: var(--text-strong); font-size: var(--ui-title-size); font-weight: var(--ui-title-weight); }
  .help-link { display: inline-block; color: var(--accent); font-size: var(--ui-small-size); line-height: 1.6; text-decoration: underline; text-underline-offset: .2em; }
  .help-link:focus-visible { outline: 2px solid var(--focus); outline-offset: 2px; }
  .meetup-options { display: flex; flex-direction: column; gap: 1.6rem; margin-top: 1.6rem; }
  .safety-link { align-self: flex-start; margin-inline-start: 3.2rem; margin-top: -1.2rem; }
  .listing-preferences { display: flex; flex-direction: column; gap: 2rem; margin-top: 2rem; padding-top: 2rem; border-top: 1px solid var(--border); }
  .listing-preferences > .help-link { align-self: flex-start; margin-top: -1.5rem; }
  .public-notice { margin-top: 2rem; color: var(--text-muted); font-size: var(--ui-small-size); line-height: 1.6; }
  .feedback { color: var(--success); font-size: var(--ui-small-size); line-height: 1.6; }
  .feedback.has-feedback { margin-bottom: 1rem; }
  .invalid { color: var(--danger); }
  .form-footer { flex-shrink: 0; padding: 1.6rem; border-top: 1px solid var(--border); }
  .form-footer button { width: 100%; }
  .footer-actions { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 1rem; }
  .listing-progress { display: flex; gap: .5rem; margin-bottom: 1.6rem; }
  .listing-progress span { flex: 1; height: .5rem; border-radius: .5rem; background: var(--surface-hover); }
  .listing-progress .complete { background: var(--accent); }
  .preview-pane { display: flex; min-width: 0; min-height: 0; overflow: auto; padding: 2.4rem; background: var(--bg); }
  .preview-card { display: flex; flex-direction: column; flex: 1; min-width: 0; min-height: 48rem; padding: 1.6rem; border: 1px solid var(--border); border-radius: var(--ui-panel-radius); background: var(--surface); }
  .preview-card > h2 { margin-bottom: 1.6rem; font-size: var(--ui-title-size); font-weight: var(--ui-title-weight); }
  .preview-body { display: grid; flex: 1; min-height: 0; grid-template-columns: minmax(0, 1.5fr) minmax(18rem, 1fr); }
  .preview-media { display: flex; flex-direction: column; min-width: 0; min-height: 0; background: var(--surface-subtle); }
  .preview-image-area { display: grid; flex: 1; place-items: center; position: relative; min-height: 28rem; }
  .preview-image { position: absolute; inset: 0; width: 100%; height: 100%; object-fit: contain; }
  .preview-empty { display: flex; flex-direction: column; align-items: center; gap: 1rem; padding: 2rem; text-align: center; color: var(--text-secondary); }
  .preview-empty h3 { font-size: 1.8rem; font-weight: var(--ui-title-weight); }
  .preview-empty p { max-width: 40ch; line-height: 1.6; }
  .gallery-controls { position: absolute; inset: 50% 1rem auto; display: flex; justify-content: space-between; pointer-events: none; transform: translateY(-50%); }
  .gallery-controls button { display: grid; place-items: center; width: 3.6rem; height: 3.6rem; border: 1px solid var(--border); border-radius: 50%; background: var(--surface); color: var(--text); cursor: pointer; pointer-events: auto; }
  .next-photo { transform: rotate(180deg); }
  :global([dir=rtl]) .gallery-controls button { transform: rotate(180deg); }
  :global([dir=rtl]) .gallery-controls .next-photo { transform: none; }
  .preview-thumbnails { display: flex; gap: .6rem; overflow-x: auto; padding: 1rem; }
  .preview-thumbnails button { flex: 0 0 4rem; height: 4rem; padding: 0; overflow: hidden; border: 2px solid transparent; border-radius: .4rem; background: var(--surface); cursor: pointer; }
  .preview-thumbnails button:first-child { margin-inline-start: auto; }
  .preview-thumbnails button:last-child { margin-inline-end: auto; }
  .preview-thumbnails button[aria-pressed=true] { border-color: var(--accent); }
  .preview-thumbnails img { width: 100%; height: 100%; object-fit: cover; filter: blur(1px); opacity: .55; }
  .preview-thumbnails button[aria-pressed=true] img { filter: none; opacity: 1; }
  .preview-details { min-width: 0; overflow-y: auto; padding: 1.6rem; border-inline-start: 1px solid var(--border); overflow-wrap: anywhere; }
  .preview-details h3 { color: var(--text-strong); font-size: 2rem; line-height: 1.4; font-weight: var(--ui-title-weight); }
  .preview-price { margin-top: .4rem; font-size: 1.5rem; }
  .preview-details h4 { margin-block: 2.4rem 1.2rem; font-size: var(--ui-title-size); font-weight: var(--ui-title-weight); }
  .preview-description { white-space: pre-wrap; line-height: 1.6; }
  .preview-attributes { margin-bottom: 1.2rem; }
  .preview-attributes div { display: flex; flex-wrap: wrap; gap: .4rem 1rem; line-height: 1.6; }
  .preview-attributes dt { color: var(--text-muted); }
  .preview-attributes dd { margin: 0; color: var(--text); }
  .location-status { display: grid; justify-items: start; gap: .6rem; }
  .location-status-action { padding: .4rem 0; border: 0; background: transparent; color: var(--accent); font: inherit; cursor: pointer; }
  .preview-tags h4 { margin-block: 1.6rem .8rem; font-size: var(--ui-body-size); font-weight: 400; }
  .preview-tags ul { display: flex; flex-wrap: wrap; gap: .8rem; list-style: none; padding: 0; margin: 0; }
  .preview-tags li { max-width: 100%; padding: .8rem 1.2rem; border-radius: 999px; background: var(--surface-active); color: var(--text); font-size: var(--ui-body-size); font-weight: var(--ui-label-weight); overflow-wrap: anywhere; }
  .preview-meetups { padding-inline-start: 1.8rem; line-height: 1.6; }
  .placeholder { color: var(--text-muted) !important; }
  @media (max-width: 1050px) {
    .preview-pane { padding: 1.6rem; }
    .preview-body { display: flex; flex-direction: column; }
    .preview-media { flex: 1; min-height: 30rem; }
    .preview-details { min-height: 18rem; overflow: visible; border-inline-start: 0; border-top: 1px solid var(--border); }
    .preview-card { min-height: 64rem; }
  }
  @media (max-width: 620px) {
    .item-listing-editor { display: flex; flex-direction: column; overflow-y: auto; }
    .form-pane { min-height: 60rem; flex-shrink: 0; border-inline-end: 0; }
    .form-scroll { overflow: visible; }
    .preview-pane { flex-shrink: 0; }
  }
</style>
