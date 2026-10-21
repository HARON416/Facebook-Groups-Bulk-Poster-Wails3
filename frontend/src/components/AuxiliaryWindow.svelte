<script lang="ts">
  import type { Snippet } from 'svelte';
  import { Window } from '@wailsio/runtime';
  import { windowProfileID, windowAccountName } from '../lib/accounts';
  import { accountLabel } from './AccountLabel.svelte';
  import WindowControls from './WindowControls.svelte';
  import ThemeMenu from './ThemeMenu.svelte';
  import WhatsAppButton from './WhatsAppButton.svelte';

  let { title, children, actions, profileSwitcher, account: accountOverride }: { title: string; children: Snippet; actions?: Snippet; profileSwitcher?: Snippet; account?: string } = $props();
  let controls = $state<WindowControls>();
  let error = $state('');
  const account = $derived(accountOverride ?? accountLabel({ id: windowProfileID, name: windowAccountName }));
  const caption = $derived([title, account, /* @wc-ignore */ 'Postika'].join(' · '));

  $effect(() => {
    document.title = caption;
    // Also update the taskbar / window switcher title in the desktop app.
    void Window.SetTitle(caption).catch(() => {});
  });
</script>

<div class="auxiliary-window">
  <header class="auxiliary-title-bar">
    {#if profileSwitcher}<div class="window-profile">{@render profileSwitcher()}</div>{/if}
    <div class="window-caption" role="presentation" ondblclick={() => controls?.toggleMaximise()}>
      <div class="caption-text" dir="auto">
        <span class="window-label">{title}</span>
        {#if !profileSwitcher}<span class="caption-separator" aria-hidden="true">/</span>
        <span class="window-account" dir="auto">{account}</span>{/if}
      </div>
    </div>
    {#if actions}<div class="window-actions">{@render actions()}</div>{/if}
    <ThemeMenu />
    <WhatsAppButton onerror={(message) => { error = message; }} />
    <WindowControls bind:this={controls} onerror={(message) => { error = message; }} />
  </header>
  {#if error}<p class="window-error" role="alert">{error}</p>{/if}
  <div class="auxiliary-content">{@render children()}</div>
</div>

<style>
  .auxiliary-window { display: flex; flex-direction: column; height: 100%; min-height: 0; overflow: hidden; overflow: clip; border: .2rem solid var(--window-border); }
  .auxiliary-title-bar { direction: ltr; display: flex; flex: 0 0 4.8rem; min-width: 0; background: var(--surface); border-bottom: 1px solid var(--border); user-select: none; }
  .window-profile { --wails-draggable: no-drag; display: flex; align-items: center; padding: 0 1rem; }
  .window-caption { --wails-draggable: drag; display: flex; align-items: center; flex: 1; min-width: 0; padding: 0 1.6rem; }
  .caption-text { display: flex; align-items: center; gap: 1rem; min-width: 0; font-size: 1.2rem; line-height: 1.4; }
  .window-label { flex-shrink: 0; max-width: 100%; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: var(--text-strong); font-weight: 600; }
  .caption-separator { color: var(--border-strong); }
  .window-account { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: var(--text-secondary); }
  .window-actions { --wails-draggable: no-drag; display: flex; align-items: center; flex-shrink: 0; max-width: 28%; min-width: 0; padding: 0 1rem; }
  .auxiliary-content { flex: 1; min-height: 0; overflow: auto; overscroll-behavior: contain; }
  .window-error { padding: 1rem 1.6rem; color: var(--danger); font-size: 1.2rem; background: var(--danger-bg); }
</style>
