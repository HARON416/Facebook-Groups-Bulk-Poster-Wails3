<div class="theme-menu" data-tour="themes">
  {#await import('./ThemePicker.svelte') then { default: ThemePicker }}<ThemePicker />{/await}
</div>

<style>
  .theme-menu { --wails-draggable: no-drag; display: flex; align-items: center; flex-shrink: 0; padding: 0 .6rem; }
</style>
