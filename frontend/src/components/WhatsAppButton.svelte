<script lang="ts">
  import { Browser } from '@wailsio/runtime';

  let { onerror }: { onerror: (message: string) => void } = $props();
  const url = 'https://wa.me/254718448461';

  function open(event: MouseEvent) {
    event.preventDefault();
    void Browser.OpenURL(url).catch((cause) => {
      onerror(cause instanceof Error ? cause.message : String(cause));
    });
  }
</script>

<a href={url} target="_blank" rel="noopener noreferrer" onclick={open}>{/* @wc-ignore */ 'WhatsApp'}</a>

<style>
  a { --wails-draggable: no-drag; display: inline-flex; align-items: center; justify-content: center; flex-shrink: 0; min-height: 3.2rem; padding: .6rem 1rem; margin-right: .4rem; border-radius: .4rem; color: var(--accent); font-size: 1.2rem; font-weight: 500; text-decoration: none; white-space: nowrap; }
  a:hover { background: var(--surface-hover); }
  a:focus-visible { outline: 2px solid var(--focus); outline-offset: -2px; }
</style>
