<script lang="ts">
  import AccountDropdown from './AccountDropdown.svelte';
  import { languages } from '../locales/languages.js';
  import { i18n, setLocale } from '../lib/i18n.svelte';
  let error = $state('');
  const options = languages.map(({ code, name, flag }) => ({ id: code, name, flag }));
  async function select(locale: string) {
    error = '';
    try { await setLocale(locale); }
    catch { error = 'Could not change the language. Please try again.'; }
  }
</script>

<div class="language-menu">
  <AccountDropdown id="language" language compact profiles={options} selected={i18n.locale} disabled={i18n.changing} onselect={select} />
  {#if error}<p role="alert">{error}</p>{/if}
</div>

<style>
  .language-menu { --wails-draggable: no-drag; position: relative; }
  .language-menu :global(.account-dropdown) { width: 15rem; }
  .language-menu :global(.account-menu) { width: 23rem; }
  p { position: absolute; z-index: 60; right: 0; top: calc(100% + .5rem); width: 24rem; padding: 1rem; border: 1px solid var(--border); border-radius: .6rem; background: var(--surface); color: var(--danger); font-size: 1.2rem; }
</style>
