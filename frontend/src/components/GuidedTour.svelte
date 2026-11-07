<script lang="ts">
  import { onMount, tick } from 'svelte';
  import { getLocale } from '../lib/i18n.svelte';
  import { localeDirection } from '../lib/locale-preferences';
  import { placeTourCard, tourChoiceFor, type TourRect, type TourScope } from '../lib/tour';
  import type { MarketplaceListingKind } from '../lib/marketplace-drafts';

  let { ready, scope = 'groups', kind = 'items', vehicleType = '', homeOffer = '', privateRoom = false, onpage = () => {}, onactive }: { ready: boolean; scope?: TourScope; kind?: MarketplaceListingKind; vehicleType?: string; homeOffer?: string; privateRoom?: boolean; onpage?: (page: number | null) => void; onactive?: (active: boolean) => void } = $props();
  const rental = $derived(kind === 'homes' && homeOffer === 'rent');
  const room = $derived(rental && privateRoom);
  const choice = $derived(tourChoiceFor(scope));
  let dialog: HTMLDialogElement;
  let card = $state<HTMLElement>();
  let mode = $state<'closed' | 'invite' | 'tour'>('closed');
  let index = $state(0), moving = $state(false);
  let target = $state<TourRect | null>(null);
  let position = $state({ x: 16, y: 16 });
  let savedScroll: { element: HTMLElement; top: number; left: number }[] = [];
  let previousFocus: HTMLElement | null = null;
  let generation = 0;
  let observer: ResizeObserver | undefined;
  const steps = $derived(scope === 'marketplace' ? [
    { selector: '[data-tour="accounts"]', page: null, title: 'Accounts', description: 'Marketplace listings and Facebook sessions stay separate for each account. Check the selected account before publishing.' },
    { selector: '.create-listing', page: null, title: 'Create new listing', description: 'Create an item, vehicle, or home listing. A separate window lets you prepare and save its details.' },
    { selector: '.listing-filter', page: null, title: 'Listing type', description: 'Filter saved listings by type. You can select items, vehicles, and homes together for publishing.' },
    { selector: '.listing-content .search', page: null, title: 'Search listings', description: 'Search saved listings by title or location.' },
    { selector: '.listing-content', page: null, title: 'Saved listings', description: 'Save as many listings as you need for future use. Publishing keeps your saved copies, so you can edit and reuse them later. Select the listings you want using their checkboxes.' },
    { selector: '.marketplace-toolbar .action-button--primary', page: null, title: 'Publish selected', description: 'Each listing opens the matching Facebook form automatically. Publish submits one listing at a time. Review the result in Chrome, then choose Next listing. Check uncertain submissions on Facebook before retrying.' },
  ] : scope === 'listing-types' ? [
    { selector: '[data-tour="accounts"]', page: null, title: 'Accounts', description: 'Marketplace listings and Facebook sessions stay separate for each account. Check the selected account before publishing.' },
    { selector: '#create-listing-items', page: null, title: 'Item for sale', description: 'Choose this for items such as furniture, electronics, or clothing.' },
    { selector: '#create-listing-vehicles', page: null, title: 'Vehicle for sale', description: 'Choose this for cars, motorcycles, boats, and other supported vehicle types.' },
    { selector: '#create-listing-homes', page: null, title: 'Home for sale or rent', description: 'Choose this for a property sale, a whole-property rental, or a private room in a shared property. After choosing a type, reopen the tour from How to use for help with the form.' },
  ] : scope === 'listing-editor' ? [
    ...(kind === 'vehicles' ? [
      { selector: '.vehicle-type', page: null, title: 'Vehicle type', description: 'Select the vehicle type first. The available fields and required details depend on this choice.' },
    ] : kind === 'homes' ? [
      { selector: '#listing-home-offer', page: null, title: 'Home for sale or rent', description: 'Choose whether you are selling or renting this property.' },
      ...(rental ? [{ selector: 'label[for="listing-private-room"]', page: null, title: 'This is a private room in a shared property.', description: 'Turn this on only when renting a private room in a shared property. Leave it off when renting the whole property.' }] : []),
    ] : []),
    { selector: '.listing-media', page: null, title: 'Photos and video', description: kind === 'items'
      ? 'Add at least one photo or video. Items allow up to 10 photos and one video. Rearrange photos to choose their display order.'
      : kind === 'vehicles' ? 'Add at least one photo or video. Vehicles allow up to 20 photos and one video. Rearrange photos to choose their display order.'
      : 'Add at least one photo or video. Homes allow up to 50 photos and one video. Rearrange photos to choose their display order.' },
    { selector: kind === 'items' ? '[data-tour="item-details"]' : kind === 'vehicles' ? '[data-tour="vehicle-details"]' : '.home-fields', page: null,
      title: kind === 'items' ? 'Item details' : kind === 'vehicles' ? 'About this vehicle' : room ? 'Room Details' : 'Property Details',
      description: kind === 'items' ? 'Enter a title, choose a category and condition, and describe the item you are selling.'
        : kind === 'vehicles' ? ['car-truck', 'motorcycle'].includes(vehicleType)
          ? 'Enter the year, make, model, and mileage for this vehicle.' : 'Enter the year, make, and model for this vehicle.'
        : room ? 'Enter the number of occupants, bedrooms, and bathrooms. Choose the bathroom type and furnished setting, then describe the room.'
        : rental ? 'Choose the rental type and enter the number of bedrooms, bathrooms, and a rental description.'
        : 'Choose the property type and enter the number of bedrooms, bathrooms, and a property description.' },
    { selector: room ? '#listing-room-price' : '#listing-price', page: null, title: rental ? 'Price per month' : 'Price',
      description: kind === 'items' ? 'Enter the asking price for your item as a whole number.'
        : kind === 'vehicles' ? 'Enter the asking price for this vehicle as a whole number.'
        : room ? 'Enter the monthly rent for this room as a whole number.'
        : rental ? 'Enter the monthly rent for the whole property as a whole number.'
        : homeOffer === 'sale' ? 'Enter the total sale price for the property as a whole number.'
        : 'Enter the asking price for this property as a whole number.' },
    { selector: kind === 'items' ? '.more-details summary' : '#listing-location', page: null, title: kind === 'items' ? 'More details' : kind === 'homes' ? 'Address' : 'Location',
      description: kind === 'items' ? 'Use More details for optional location, availability, tags, SKU, and meetup preferences. Choose a location suggestion and complete Facebook login in Chrome if prompted.'
        : kind === 'vehicles' ? 'Search for the town or city where this vehicle is located and select a suggestion. Complete Facebook login in Chrome if prompted.'
        : 'Search for the property address and select a suggestion. Complete Facebook login in Chrome if prompted.' },
    ...(kind === 'homes' ? [{ selector: '.home-advanced', page: null, title: 'Advanced Details', description: rental
      ? 'Optionally add the property area, date available, laundry, parking, air conditioning, heating, and cat or dog permissions.'
      : 'Optionally add the property area, laundry, parking, air conditioning, and heating.' }] : []),
    { selector: '.listing-preferences', page: null, title: 'Listing preferences', description: kind === 'items'
      ? 'Set promotion and Hide from friends preferences for this item. Hiding from friends does not make the listing private.'
      : kind === 'vehicles' ? 'Choose whether to add a promotion step after this vehicle listing is published.'
      : 'Choose whether to add a promotion step after this property listing is published.' },
    { selector: '.preview-pane', page: null, title: 'Preview', description: 'Review your photos and listing details here before saving or testing.' },
    { selector: '[data-tour="listing-test"]', page: null, title: 'Test', description: 'Test fills the Facebook form and stops before Publish. Review the result in Chrome. Test does not save your draft.' },
    { selector: '[data-tour="listing-save"]', page: null, title: 'Save listing', description: 'Save this listing and its media to this account for future use. Saved copies remain available after publishing, so you can edit and reuse them later.' },
  ] : [
    { selector: '[data-tour="accounts"]', page: null, title: 'Accounts', description: 'Separate Facebook sessions, groups, drafts, settings, and queues for each account.' },
    { selector: '[data-tour="compose"]', page: 0, title: 'Create post', description: 'Write your message and add at least one image; both are required. Use Next to move forward and Previous to edit an earlier step.' },
    { selector: '[data-tour="import"]', page: 0, title: 'Fetch posts from a Facebook page', description: 'Review the posts fetched from your Facebook page.' },
    { selector: '[data-tour="groups"]', page: 1, title: 'Select groups', description: 'Select Load groups and finish Facebook login or verification in Chrome. Search the loaded list, select the groups you want, then choose Next.' },
    { selector: '[data-tour="timing"]', page: 2, title: 'Interval between posts', description: 'At least 60 seconds. Use equal values for a fixed interval.' },
    { selector: '[data-tour="schedule"]', page: 2, title: 'Post or schedule', description: 'Choose Schedule, select a future date and time in the displayed time zone, then Confirm schedule. Facebook may require login when the queue starts.' },
    { selector: '[data-tour="logs"]', page: null, title: 'Posting logs', description: 'Open Posting logs to follow progress. Its account selector lets you inspect other queues without switching the account in the main workspace.' },
    { selector: '[data-tour="marketplace"]', page: null, title: 'Marketplace', description: 'Click Marketplace in the header to create and publish items, vehicles, and homes for sale or rent on Facebook Marketplace. Use How to use in that window to start the Marketplace tour.' },
    { selector: '[data-tour="language"]', page: null, title: 'Languages', description: 'Choose the language used for menus, buttons, and instructions.' },
    { selector: '[data-tour="themes"]', page: null, title: 'Themes', description: 'Choose a light or dark color theme for the app.' },
  ]);

  function elementForStep() {
    return [...document.querySelectorAll<HTMLElement>(steps[index].selector)].find(element => element.getClientRects().length > 0);
  }
  function measure() {
    if (mode === 'closed' || !card) return;
    const element = mode === 'tour' ? elementForStep() : undefined;
    const bounds = element?.getBoundingClientRect();
    target = bounds && bounds.right > 0 && bounds.left < innerWidth && bounds.bottom > 0 && bounds.top < innerHeight ? {
      left: Math.max(8, bounds.left - 6), top: Math.max(8, bounds.top - 6),
      right: Math.min(innerWidth - 8, bounds.right + 6), bottom: Math.min(innerHeight - 8, bounds.bottom + 6),
    } : null;
    position = placeTourCard(target, card.offsetWidth, card.offsetHeight, { width: innerWidth, height: innerHeight });
  }
  function keyboard(event: KeyboardEvent) {
    if (event.key !== 'Tab' || !card) return;
    const buttons = [...card.querySelectorAll<HTMLButtonElement>('button:not(:disabled)')];
    const first = buttons[0], last = buttons.at(-1);
    if (!first || !last) return;
    if (!card.contains(document.activeElement)) {
      event.preventDefault(); (event.shiftKey ? last : first).focus();
    } else if (event.shiftKey && document.activeElement === first) {
      event.preventDefault(); last.focus();
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault(); first.focus();
    }
  }
  async function showStep(next: number) {
    const current = ++generation;
    moving = true;
    index = next;
    onpage(steps[index].page);
    await tick();
    if (current !== generation || mode !== 'tour') return;
    const element = elementForStep();
    element?.scrollIntoView({ block: 'center', inline: 'nearest', behavior: 'instant' });
    observer?.disconnect();
    if (card) observer?.observe(card);
    if (element) observer?.observe(element);
    const scrollPane = element?.closest<HTMLElement>('[data-step-scroll]');
    if (scrollPane) observer?.observe(scrollPane);
    measure();
    moving = false;
    // Svelte must remove disabled before the browser can focus Next.
    await tick();
    if (current === generation && mode === 'tour') card?.querySelector<HTMLButtonElement>('[data-tour-next]')?.focus({ preventScroll: true });
  }
  function rememberPosition() {
    previousFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    savedScroll = [...document.querySelectorAll<HTMLElement>('[data-step-scroll]')].map(element => ({ element, top: element.scrollTop, left: element.scrollLeft }));
  }
  export async function start() {
    if (!ready || mode === 'tour' || document.querySelector('dialog[open]')) return;
    rememberPosition();
    choice.remember();
    onactive?.(true);
    mode = 'tour';
    await tick();
    if (!ready || mode !== 'tour') return;
    dialog.showModal();
    await showStep(0);
  }
  async function accept() {
    dialog.close();
    mode = 'closed';
    await tick();
    await start();
  }
  async function close(remember = true) {
    if (remember) choice.remember();
    generation++;
    mode = 'closed';
    moving = false;
    target = null;
    observer?.disconnect();
    dialog.close();
    onpage(null);
    onactive?.(false);
    await tick();
    for (const { element, top, left } of savedScroll) element.scrollTo({ top, left, behavior: 'instant' });
    const focus = previousFocus?.isConnected && !previousFocus.closest('dialog') ? previousFocus : document.querySelector<HTMLElement>('[data-features-card]');
    focus?.focus({ preventScroll: true });
  }
  onMount(() => {
    observer = new ResizeObserver(measure);
    const replay = () => { if (scope !== 'groups') void start(); };
    window.addEventListener('postika:marketplace-tour', replay);
    window.addEventListener('resize', measure);
    document.addEventListener('scroll', measure, true);
    return () => {
      observer?.disconnect();
      window.removeEventListener('postika:marketplace-tour', replay);
      window.removeEventListener('resize', measure);
      document.removeEventListener('scroll', measure, true);
      onpage(null);
      onactive?.(false);
    };
  });
  $effect(() => {
    if (!ready) {
      if (mode !== 'closed') void close(false);
      return;
    }
    if (mode !== 'closed' || choice.hasSeen()) return;
    const timer = window.setTimeout(async () => {
      if (!ready || document.querySelector('dialog[open]') || choice.hasSeen()) return;
      rememberPosition();
      mode = 'invite';
      await tick();
      if (!ready || mode !== 'invite') return;
      dialog.showModal();
      if (card) observer?.observe(card);
      measure();
    }, 700);
    return () => clearTimeout(timer);
  });
</script>

<dialog bind:this={dialog} class="tour-overlay" dir={localeDirection(getLocale())} aria-labelledby="tour-heading" aria-describedby="tour-description" onkeydown={keyboard} oncancel={(event) => { event.preventDefault(); void close(); }}>
  {#if mode !== 'closed'}
    {#if target}<div class="spotlight" aria-hidden="true" style:left={`${target.left}px`} style:top={`${target.top}px`} style:width={`${Math.max(0, target.right - target.left)}px`} style:height={`${Math.max(0, target.bottom - target.top)}px`}></div>{:else}<div class="shade" aria-hidden="true"></div>{/if}
    <section bind:this={card} class="tour-card" style:left={`${position.x}px`} style:top={`${position.y}px`}>
      <header class="ui-dialog-heading">
        <h2 class="ui-title" id="tour-heading">{mode === 'invite' ? scope === 'groups' ? 'Take a quick tour of Postika?' : 'Take a quick tour of Marketplace?' : steps[index].title}</h2>
        <button class="ui-dialog-close" type="button" onclick={() => close()}>Close</button>
      </header>
      <div class="tour-copy" aria-live={mode === 'tour' ? 'polite' : 'off'} aria-atomic="true">
        {#if mode === 'tour'}<p class="progress"><bdi dir="ltr">{index + 1} / {steps.length}</bdi></p>{/if}
        <p id="tour-description">{mode === 'invite' ? 'You can reopen the tour from How to use.' : steps[index].description}</p>
      </div>
      <footer class="ui-dialog-footer">
        {#if mode === 'invite'}
          <button type="button" onclick={() => close()}>Skip</button>
          <button type="button" class="primary" onclick={accept}>Start guided tour</button>
        {:else}
          <button type="button" disabled={moving || index === 0} onclick={() => showStep(index - 1)}>Previous</button>
          <button type="button" class="primary" data-tour-next disabled={moving} onclick={() => index === steps.length - 1 ? close() : showStep(index + 1)}>{index === steps.length - 1 ? 'Finish' : 'Next'}</button>
        {/if}
      </footer>
    </section>
  {/if}
</dialog>

<style>
  .tour-overlay { --wails-draggable: no-drag; position: fixed; inset: 0; width: 100vw; height: 100dvh; max-width: none; max-height: none; margin: 0; padding: 0; border: 0; background: transparent; color: var(--text); overflow: hidden; }
  .tour-overlay::backdrop { background: transparent; }
  .shade { position: fixed; inset: 0; background: rgb(0 0 0 / 65%); }
  .spotlight { position: fixed; border: 2px solid var(--accent); border-radius: .6rem; box-shadow: 0 0 0 200vmax rgb(0 0 0 / 65%); pointer-events: none; }
  .tour-card { position: fixed; display: flex; flex-direction: column; width: min(36rem, calc(100vw - 3.2rem)); max-height: calc(100dvh - 3.2rem); border: 1px solid var(--border-strong); border-radius: var(--ui-panel-radius); background: var(--surface); color: var(--text); box-shadow: 0 .8rem 2.4rem var(--shadow-strong); font-size: var(--ui-body-size); }
  .ui-dialog-heading { border-bottom: 1px solid var(--border); align-items: flex-start; }
  .ui-title { min-width: 0; overflow-wrap: anywhere; }
  .tour-copy { padding: var(--ui-space); overflow: auto; min-height: 0; line-height: 1.6; overflow-wrap: anywhere; }
  .progress { margin-bottom: .6rem; color: var(--text-muted); font-variant-numeric: tabular-nums; }
  .ui-dialog-footer { padding: 1.2rem var(--ui-space); }
  footer button { min-height: 3.2rem; padding: .7rem 1rem; border: 1px solid var(--border); border-radius: .5rem; background: var(--surface); color: var(--text); font: inherit; cursor: pointer; }
  footer .primary { background: var(--accent); border-color: var(--accent); color: var(--on-accent); }
  footer button:hover:not(:disabled) { filter: brightness(.94); }
  footer button:disabled { opacity: .5; cursor: default; }
  footer button:focus-visible { outline: 2px solid var(--focus); outline-offset: 2px; }
  @media (prefers-reduced-motion: no-preference) { .tour-card { animation: tour-enter var(--motion-fast) var(--motion-ease); } @keyframes tour-enter { from { opacity: 0; } to { opacity: 1; } } }
</style>
