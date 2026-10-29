<script lang="ts">
  import { onMount, tick } from 'svelte';
  import PhosphorIcon from './PhosphorIcon.svelte';
  import type { NotificationKind } from '../lib/notifications.svelte';

  let { kind = 'info', message, description = '', duration = 0, actionLabel = '', onaction, onclose }: {
    kind?: NotificationKind; message: string; description?: string; duration?: number;
    actionLabel?: string; onaction?: () => void; onclose?: () => void;
  } = $props();
  const icons = { info: 'info', success: 'check-circle', warning: 'warning-circle', error: 'x-circle' } as const;
  let bar: HTMLDivElement;
  let timer: ReturnType<typeof setTimeout> | undefined;
  let remaining = 0, started = 0, hovered = false, focused = false, windowHidden = false;
  let returnFocus: HTMLElement | null = null;
  function pause() {
    if (timer !== undefined) { clearTimeout(timer); timer = undefined; remaining = Math.max(0, remaining - (Date.now() - started)); }
  }
  function resume() {
    if (!onclose || duration <= 0 || timer !== undefined || hovered || focused || windowHidden || document.hidden) return;
    started = Date.now(); timer = setTimeout(() => { timer = undefined; onclose?.(); }, remaining);
  }
  async function dismiss() {
    const restore = bar.contains(document.activeElement);
    pause(); onclose?.(); await tick();
    if (restore && returnFocus?.isConnected) returnFocus.focus({ preventScroll: true });
  }
  onMount(() => {
    remaining = duration;
    returnFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    const hidden = () => { windowHidden = true; pause(); };
    const shown = () => { windowHidden = false; resume(); };
    const visibility = () => { if (document.hidden) pause(); else resume(); };
    document.addEventListener('visibilitychange', visibility);
    window.addEventListener('postika:window-hidden', hidden);
    window.addEventListener('postika:window-shown', shown);
    resume();
    return () => {
      pause(); document.removeEventListener('visibilitychange', visibility);
      window.removeEventListener('postika:window-hidden', hidden); window.removeEventListener('postika:window-shown', shown);
    };
  });
</script>

<div class="notification-bar" data-kind={kind} role={kind === 'error' ? 'alert' : 'status'} aria-atomic="true" bind:this={bar} onpointerenter={() => { hovered = true; pause(); }} onpointerleave={() => { hovered = false; resume(); }} onfocusin={() => { focused = true; pause(); }} onfocusout={(event) => { if (!bar.contains(event.relatedTarget as Node | null)) { focused = false; resume(); } }} onkeydown={(event) => { if (event.key === 'Escape' && onclose) { event.stopPropagation(); void dismiss(); } }}>
  <span class="indicator"><PhosphorIcon name={icons[kind]} size={20} /></span>
  <div class="copy"><p class="message">{message}</p>{#if description}<p class="description">{description}</p>{/if}{#if actionLabel && onaction}<button class="notification-action" type="button" onclick={onaction}>{actionLabel}</button>{/if}</div>
  {#if onclose}<button class="notification-close" type="button" aria-label="Close" onclick={dismiss}><PhosphorIcon name="x" size={16} /></button>{/if}
</div>

<style>
  .notification-bar { --notification-color: var(--accent); display: flex; align-items: flex-start; gap: 1rem; padding: 1.4rem 1.6rem; border: .2rem solid var(--notification-color); border-radius: .4rem; background: var(--surface); color: var(--text); box-shadow: 0 .6rem 2rem var(--shadow-strong); font-size: var(--ui-body-size); }
  .notification-bar[data-kind=success] { --notification-color: var(--success); }
  .notification-bar[data-kind=warning] { --notification-color: var(--warning); }
  .notification-bar[data-kind=error] { --notification-color: var(--danger); }
  .indicator { display: inline-flex; flex: 0 0 auto; margin-top: .1rem; color: var(--notification-color); }
  .copy { flex: 1; min-width: 0; overflow-wrap: anywhere; }
  .message { color: var(--text-strong); font-weight: var(--ui-label-weight); line-height: 1.5; }
  .description { margin-top: .4rem; color: var(--text-secondary); font-size: var(--ui-small-size); line-height: 1.6; }
  button { border: 0; background: transparent; font: inherit; cursor: pointer; }
  button:focus-visible { outline: 2px solid var(--focus); outline-offset: 2px; }
  .notification-close { display: grid; place-items: center; flex: 0 0 2.8rem; width: 2.8rem; height: 2.8rem; margin-top: -.4rem; margin-inline-end: -.6rem; padding: 0; border-radius: .4rem; color: var(--text-secondary); }
  .notification-close:hover { color: var(--text); background: var(--surface-hover); }
  .notification-action { margin-top: .6rem; padding: .4rem 0; color: var(--accent); font-weight: var(--ui-label-weight); }
</style>
