<script lang="ts">
  import { tick } from 'svelte';
  import UsageGuide from './UsageGuide.svelte';
  import { languages } from '../locales/languages.js';
  import { themeOptions } from '../lib/theme-options';
  import { getLocale } from '../lib/i18n.svelte';

  let dialog: HTMLDialogElement;
  let trigger: HTMLButtonElement;
  let expanded = $state(false);
  let view = $state<'features' | 'guide'>('features');

  function open() {
    view = 'features';
    dialog.showModal();
    expanded = true;
    dialog.querySelector('.feature-content')?.scrollTo(0, 0);
  }
  function close() { dialog.close(); }
  async function showView(next: 'features' | 'guide') {
    view = next;
    await tick();
    dialog.querySelector('.feature-content')?.scrollTo(0, 0);
  }
  function backdrop(event: MouseEvent) {
    if (event.target !== dialog) return;
    const bounds = dialog.getBoundingClientRect();
    if (event.clientX < bounds.left || event.clientX > bounds.right || event.clientY < bounds.top || event.clientY > bounds.bottom) close();
  }
</script>

<button class="features-trigger" type="button" bind:this={trigger} data-features-card aria-haspopup="dialog" aria-expanded={expanded} aria-controls="features-dialog" onclick={open}>
  <svg aria-hidden="true" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><path d="m12 3 2.6 6.4L21 12l-6.4 2.6L12 21l-2.6-6.4L3 12l6.4-2.6L12 3Z" /></svg>
  Features
</button>

<dialog id="features-dialog" bind:this={dialog} dir={getLocale() === 'ar' ? 'rtl' : 'ltr'} aria-labelledby="features-heading" onclick={backdrop} onclose={() => { expanded = false; trigger.focus(); }}>
  <header class="feature-header">
    <h1 id="features-heading">Features &amp; How to Use</h1>
    <button class="close" type="button" aria-label="Close" onclick={close}><svg aria-hidden="true" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"><path d="m6 6 12 12M18 6 6 18" /></svg></button>
  </header>
  <div class="view-switch" role="group" aria-label="Features">
    <button type="button" aria-pressed={view === 'features'} aria-controls="features-content" onclick={() => showView('features')}>Features</button>
    <button type="button" data-usage-guide aria-pressed={view === 'guide'} aria-controls="features-content" onclick={() => showView('guide')}>How to use</button>
  </div>
  <!-- The scroll pane needs a tab stop so keyboard users can scroll the full feature list. -->
  <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
  <div id="features-content" class="feature-content" tabindex="0" role="region" aria-labelledby="features-heading">
    {#if view === 'features'}
    <p class="intro">Post text and images to multiple Facebook groups.</p>

    <section aria-labelledby="features-create"><h2 id="features-create">Create post</h2><ul>
      <li>Bold, italic, headings, quotes, and lists · Markdown shortcuts supported</li>
      <li>JPG, PNG, WebP, or GIF · Select multiple images</li>
      <li>Browse or drop images, preview attachments, and remove images.</li>
      <li>Drag images to rearrange. Or focus an image and use Alt + Left / Right.</li>
      <li>Draft text and image copies are saved automatically for each account.</li>
    </ul></section>

    <section aria-labelledby="features-accounts"><h2 id="features-accounts">Accounts</h2><ul>
      <li>Separate Facebook sessions, groups, drafts, settings, and queues for each account.</li>
      <li>Switch accounts while other queues keep running.</li>
    </ul></section>

    <section aria-labelledby="features-groups"><h2 id="features-groups">Select groups</h2><ul>
      <li>Choose the groups you want to share your post with.</li>
      <li><span>Search groups</span> · <span>Select all groups</span></li>
      <li>Loaded groups stay available between app sessions.</li>
    </ul></section>

    <section aria-labelledby="features-posting"><h2 id="features-posting">Posting settings</h2><ul>
      <li><span>Post now</span> · <span>Schedule your queue</span></li>
      <li><strong>Interval between posts</strong><p>Minimum and maximum delays between posts.</p></li>
      <li><strong>Rewrite each post body with Gemini</strong><p>Custom instructions and a configurable Gemini model.</p><p class="muted">Your post text and instructions are sent to Google Gemini. Images are not sent. Gemini rewrites use plain text.</p></li>
      <li>Use saved defaults</li>
    </ul></section>

    <section aria-labelledby="features-logs"><h2 id="features-logs">Posting logs</h2><ul>
      <li><strong>Posting progress</strong><p><span>Pause</span> · <span>Resume</span> · <span>Stop</span></p></li>
      <li><span>PUBLISHED</span> · <span>PENDING</span> · <span>FAILED</span> · <span>SKIPPED</span> · <span>UNCONFIRMED</span></li>
      <li><span>Debug details</span> · <span>Copy Facebook response</span></li>
      <li>Queues pause when offline and resume when the connection returns.</li>
    </ul></section>

    <section aria-labelledby="features-preferences"><h2 id="features-preferences">Settings</h2><ul>
      <li>{languages.length} languages · {themeOptions.length} themes</li>
      <li>Language and theme choices sync across windows.</li>
      <li>Windows and Linux</li>
    </ul></section>

    <p class="session-note">Keep Postika open and your computer awake. Queues and logs are not restored after quitting.</p>
    {:else}
      <UsageGuide />
    {/if}
  </div>
</dialog>

<style>
  .features-trigger { --wails-draggable: no-drag; display: inline-flex; align-items: center; gap: .6rem; flex-shrink: 0; padding: .6rem .9rem; margin-inline: .4rem; border: 1px solid var(--border-accent); border-radius: .6rem; background: var(--accent-faint); color: var(--accent); font-size: 1.2rem; cursor: pointer; white-space: nowrap; }
  .features-trigger:hover, .features-trigger[aria-expanded=true] { background: var(--surface-active); border-color: var(--accent); }
  dialog { --wails-draggable: no-drag; position: fixed; inset: 0; margin: auto; padding: 0; width: min(88rem, calc(100vw - 4rem)); max-width: none; max-height: min(86rem, calc(100dvh - 6rem)); border: 1px solid var(--border-strong); border-radius: 1.6rem; background: var(--surface); color: var(--text); box-shadow: 0 2.4rem 8rem #0005; overflow: hidden; }
  dialog[open] { display: flex; flex-direction: column; }
  dialog::backdrop { background: #0009; backdrop-filter: blur(4px); }
  .feature-header { display: flex; align-items: center; justify-content: space-between; gap: 2rem; flex-shrink: 0; padding: 2rem 3.2rem; border-bottom: 1px solid var(--border); }
  h1 { color: var(--accent); font-size: 2.4rem; line-height: 1.2; font-weight: 500; }
  .close { display: grid; place-items: center; flex-shrink: 0; width: 3.6rem; height: 3.6rem; border: 1px solid var(--border); border-radius: .7rem; background: transparent; color: var(--text-secondary); cursor: pointer; }
  .close:hover { background: var(--surface-hover); color: var(--text); }
  .view-switch { display: flex; flex-shrink: 0; gap: .6rem; padding: 1.2rem 3.2rem; border-bottom: 1px solid var(--border); }
  .view-switch button { padding: .7rem 1.2rem; border: 1px solid transparent; border-radius: .6rem; background: transparent; color: var(--text-secondary); font-size: 1.3rem; cursor: pointer; }
  .view-switch button:hover { background: var(--surface-hover); color: var(--text); }
  .view-switch button[aria-pressed=true] { border-color: var(--border-accent); background: var(--accent-faint); color: var(--accent); }
  .feature-content { min-height: 0; padding: 2.4rem 3.2rem 3.2rem; overflow-y: auto; overscroll-behavior: contain; font-size: 1.4rem; line-height: 1.8; overflow-wrap: anywhere; }
  .intro { color: var(--text-strong); font-size: 1.6rem; }
  section { margin-top: 2.8rem; }
  h2 { margin-bottom: .8rem; color: var(--text-muted); font-size: 1.5rem; font-weight: 500; }
  ul { list-style: none; }
  li { position: relative; padding-inline-start: 1.8rem; margin-block: .6rem; }
  li::before { content: '–'; position: absolute; inset-inline-start: 0; color: var(--accent); }
  strong { color: var(--text-strong); font-weight: 600; }
  .muted { margin-top: .4rem; color: var(--text-muted); font-size: 1.2rem; }
  .session-note { margin-top: 3rem; padding-top: 1.8rem; border-top: 1px solid var(--border); color: var(--text-secondary); font-size: 1.2rem; }
  button:focus-visible, .feature-content:focus-visible { outline: 2px solid var(--focus); outline-offset: -3px; }
  @media (max-width: 600px) {
    dialog { width: calc(100vw - 2rem); max-height: calc(100dvh - 2rem); border-radius: 1rem; }
    .feature-header { padding: 2rem; }
    .view-switch { padding-inline: 2rem; }
    .feature-content { padding: 2rem; font-size: 1.3rem; }
  }
</style>
