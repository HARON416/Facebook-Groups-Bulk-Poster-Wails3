<script lang="ts">
  import { onMount, tick } from 'svelte';
  import { getLocale } from '../lib/i18n.svelte';
  import { localeDirection } from '../lib/locale-preferences';
  import { placeTourCard, tourChoice, type TourRect } from '../lib/tour';

  let { ready, onpage, onactive }: { ready: boolean; onpage: (page: number | null) => void; onactive?: (active: boolean) => void } = $props();
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
  const steps = $derived([
    { selector: '[data-tour="accounts"]', page: null, title: 'Accounts', description: 'Separate Facebook sessions, groups, drafts, settings, and queues for each account.' },
    { selector: '[data-tour="compose"]', page: 0, title: 'Create post', description: 'Write your message and add at least one image; both are required. Use Next to move forward and Previous to edit an earlier step.' },
    { selector: '[data-tour="import"]', page: 0, title: 'Fetch posts from a Facebook page', description: 'Review the posts fetched from your Facebook page.' },
    { selector: '[data-tour="groups"]', page: 1, title: 'Select groups', description: 'Select Load groups and finish Facebook login or verification in Chrome. Search the loaded list, select the groups you want, then choose Next.' },
    { selector: '[data-tour="timing"]', page: 2, title: 'Interval between posts', description: 'At least 60 seconds. Use equal values for a fixed interval.' },
    { selector: '[data-tour="schedule"]', page: 2, title: 'Post or schedule', description: 'Choose Schedule, select a future date and time in the displayed time zone, then Confirm schedule. Facebook may require login when the queue starts.' },
    { selector: '[data-tour="logs"]', page: null, title: 'Posting logs', description: 'Open Posting logs to follow progress. Its account selector lets you inspect other queues without switching the account in the main workspace.' },
    { selector: '[data-tour="language"]', page: null, title: 'Languages', description: 'Use Language and Themes in the top bar. Your choices apply across windows.' },
    { selector: '[data-tour="themes"]', page: null, title: 'Themes', description: 'Language and theme choices sync across windows.' },
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
    card?.querySelector<HTMLButtonElement>('[data-tour-next]')?.focus({ preventScroll: true });
  }
  function rememberPosition() {
    previousFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    savedScroll = [...document.querySelectorAll<HTMLElement>('[data-step-scroll]')].map(element => ({ element, top: element.scrollTop, left: element.scrollLeft }));
  }
  export async function start() {
    if (!ready || mode === 'tour' || document.querySelector('dialog[open]')) return;
    rememberPosition();
    tourChoice.remember();
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
    if (remember) tourChoice.remember();
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
    window.addEventListener('resize', measure);
    document.addEventListener('scroll', measure, true);
    return () => {
      observer?.disconnect();
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
    if (mode !== 'closed' || tourChoice.hasSeen()) return;
    const timer = window.setTimeout(async () => {
      if (!ready || document.querySelector('dialog[open]') || tourChoice.hasSeen()) return;
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
        <h2 class="ui-title" id="tour-heading">{mode === 'invite' ? 'Take a quick tour of Postika?' : steps[index].title}</h2>
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
  @media (prefers-reduced-motion: no-preference) { .tour-card { animation: tour-enter .15s ease-out; } @keyframes tour-enter { from { opacity: 0; } to { opacity: 1; } } }
</style>
