<script lang="ts">
  import { Browser, Call, System } from '@wailsio/runtime';
  import { getLocale } from '../lib/i18n.svelte';
  import { localeDirection } from '../lib/locale-preferences';

  let dialog: HTMLDialogElement;
  let trigger: HTMLButtonElement;
  let expanded = $state(false);
  let failed = $state(false);
  let selectedSubject = $state('');
  const email = 'haronkibetrutoh@gmail.com';
  const topics = [
    { label: 'Question', subject: 'Question' },
    { label: 'Feedback', subject: 'Feedback' },
    { label: 'Bug report', subject: 'Bug' },
    { label: 'Account help', subject: 'Account' },
    { label: 'Business inquiry', subject: 'Business' },
    { label: 'Other', subject: 'Other' },
  ];
  function open() { failed = false; dialog.showModal(); expanded = true; }
  function close() { dialog.close(); }
  function backdrop(event: MouseEvent) {
    if (event.target !== dialog) return;
    const bounds = dialog.getBoundingClientRect();
    if (event.clientX < bounds.left || event.clientX > bounds.right || event.clientY < bounds.top || event.clientY > bounds.bottom) close();
  }
  function mailto(subject?: string) {
    return `mailto:${email}${subject ? `?subject=${encodeURIComponent(`[Postika] [${subject}] `)}` : ''}`;
  }
  function gmail() {
    const query = new URLSearchParams({ view: 'cm', fs: '1', to: email, su: `[Postika] [${selectedSubject}] ` });
    return `https://mail.google.com/mail/?${query}`;
  }
  function isDesktop() { return System.IsWindows() || System.IsMac() || System.IsLinux(); }
  async function openGmail(event: MouseEvent) {
    if (!isDesktop()) return;
    event.preventDefault();
    try { await Browser.OpenURL(gmail()); }
    catch { failed = true; }
  }
  async function send(event: MouseEvent, subject?: string) {
    // Let a browser handle mailto directly. The desktop bridge is only
    // available inside Wails; a preview server cannot launch native apps.
    if (!isDesktop()) return;
    event.preventDefault();
    failed = false;
    selectedSubject = subject ?? 'Other';
    try { await Call.ByName('main.ContactService.OpenEmail', selectedSubject); }
    catch { failed = true; }
  }
</script>

<button class="contact-trigger" type="button" bind:this={trigger} aria-haspopup="dialog" aria-expanded={expanded} aria-controls="contact-dialog" onclick={open}>Contact</button>

<dialog class="ui-dialog" id="contact-dialog" bind:this={dialog} dir={localeDirection(getLocale())} aria-labelledby="contact-heading" onclick={backdrop} onclose={() => { expanded = false; trigger.focus(); }}>
  <header class="ui-dialog-heading">
    <h2 class="ui-title" id="contact-heading">Contact</h2>
    <button class="ui-dialog-close" type="button" onclick={close}>Close</button>
  </header>
  <div class="contact-content">
    <div class="contact-topics">
      {#each topics as topic}
        <a href={mailto(topic.subject)} onclick={(event) => send(event, topic.subject)}>
          <span>{topic.label}</span>
        </a>
      {/each}
    </div>
    {#if failed}<p class="contact-address" dir="ltr">{email}</p><p class="contact-error" role="alert">Could not open your email app. Copy the address above to send an email.</p><a class="gmail-link" href={gmail()} target="_blank" rel="noopener noreferrer" onclick={openGmail}>{/* @wc-ignore */ 'Gmail'}</a>{/if}
  </div>
</dialog>

<style>
  .contact-trigger { --wails-draggable: no-drag; display: inline-flex; align-items: center; justify-content: center; flex-shrink: 0; min-height: 3.2rem; padding: .6rem 1rem; margin-inline-end: .4rem; border: 0; border-radius: .4rem; background: transparent; color: var(--text-secondary); font-size: var(--ui-small-size); font-weight: 400; white-space: nowrap; cursor: pointer; }
  .contact-trigger:hover, .contact-trigger[aria-expanded=true] { background: var(--surface-hover); color: var(--text); }
  .contact-trigger:focus-visible, a:focus-visible { outline: 2px solid var(--focus); outline-offset: 2px; }
  dialog { width: min(52rem, calc(100vw - 3.2rem)); max-height: calc(100dvh - 3.2rem); overflow-y: auto; overscroll-behavior: contain; user-select: text; }
  .ui-dialog-heading { padding-bottom: 1.2rem; border-bottom: 1px solid var(--border); }
  .contact-content { padding: var(--ui-space); line-height: 1.6; }
  .contact-topics { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: var(--ui-field-gap); }
  .contact-topics a { display: flex; align-items: center; min-height: 4rem; padding: 1rem 1.2rem; border-radius: var(--ui-input-radius); background: var(--accent-faint); color: var(--text); text-decoration: none; font-size: var(--ui-body-size); font-weight: var(--ui-label-weight); }
  .contact-topics a:hover { background: var(--accent-soft); color: var(--text-strong); }
  .contact-address { margin-top: var(--ui-space); overflow-wrap: anywhere; }
  .contact-error { margin-top: .8rem; color: var(--danger); }
  .gmail-link { display: inline-flex; align-items: center; min-height: 4rem; color: var(--accent); text-underline-offset: .2em; }
  @media (max-width: 480px) { .contact-topics { grid-template-columns: 1fr; } }
</style>
