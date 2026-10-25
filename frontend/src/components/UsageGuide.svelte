<div class="usage-guide">
  <p class="introduction">You can use the default account without adding another one. Sign in to Facebook when you choose Load groups in step 2.</p>

  <section><h2>Create post</h2>
    <p>Write your message and add at least one image; both are required. Use Next to move forward and Previous to edit an earlier step.</p>
    <p>Bold, italic, headings, quotes, and lists · Markdown shortcuts supported</p>
    <h3>Images</h3>
    <p>Browse or drop images, preview attachments, and remove images.</p>
    <p>JPG, PNG, WebP, or GIF · Select multiple images</p>
    <p>Drag images to rearrange. Or focus an image and use Alt + Left / Right.</p>
    <p class="tip">Draft text and image copies are saved automatically for each account.</p>
  </section>

  <section><h2>Select groups</h2>
    <p>Select Load groups and finish Facebook login or verification in Chrome. Search the loaded list, select the groups you want, then choose Next.</p>
    <p class="tip">Loaded groups stay available between app sessions.</p>
  </section>

  <section><h2>Post or schedule</h2>
    <h3>Interval between posts</h3>
    <p>Choose how long to wait before posting to the next group.</p>
    <p>At least 60 seconds. Use equal values for a fixed interval.</p>
    <h3>Post now</h3>
    <p>Review the account, message, image order, selected groups, and posting intervals. Choose Post now and complete any Facebook login or verification prompts in Chrome.</p>
    <p class="tip">Switch accounts while other queues keep running.</p>
    <h3>Schedule your queue</h3>
    <p>Choose Schedule, select a future date and time in the displayed time zone, then Confirm schedule. Facebook may require login when the queue starts.</p>
    <p class="tip">Keep Postika open and your computer awake. Queues and logs are not restored after quitting.</p>
  </section>

  <aside class="additional-accounts"><h2>Add account</h2>
    <p>Adding another account is optional. Use it only when you want a separate Facebook login and workspace.</p>
    <p>Choose Add account, enter an account name, and select Create & sign in. Use the account switcher to return to an existing account.</p>
    <p>Sign in to Facebook in the new account’s browser window.</p>
    <p class="tip">Separate Facebook sessions, groups, drafts, settings, and queues for each account.</p>
  </aside>

  <aside><h2>Posting logs</h2>
    <p>Open Posting logs to follow progress. Its account selector lets you inspect other queues without switching the account in the main workspace.</p>
    <p>Pause finishes the current operation before waiting. Resume continues the queue; Stop ends the remaining queue. Queues paused manually need Resume even after internet returns.</p>
    <h3>Posting progress</h3>
    <p>Published means Facebook confirmed publication. Pending means admin approval is needed. Review failed or skipped entries; check unconfirmed submissions on Facebook before retrying to avoid duplicates.</p>
  </aside>

  <aside><h2>Debug details</h2>
    <p>For problems, expand Debug details or Copy Facebook response in Posting logs. For expired logins, use Load groups and finish verification before trying again.</p>
    <p class="tip">This post was submitted, but confirmation is uncertain. Check Facebook before retrying.</p>
  </aside>

  <aside><h2>Themes</h2>
    <p>Use Language and Themes in the top bar. Your choices apply across windows.</p>
    <p class="tip">Draft text and image copies are saved automatically for each account.</p>
  </aside>
</div>

<style>
  .usage-guide { counter-reset: step; }
  section { position: relative; padding-inline-start: 4rem; margin-top: 2.8rem; }
  .introduction { color: var(--text-secondary); }
  aside { margin-top: 2.8rem; padding-top: 2rem; border-top: 1px solid var(--border); }
  section::before { counter-increment: step; content: counter(step, decimal-leading-zero); position: absolute; inset-inline-start: 0; top: .1rem; color: var(--accent); font-size: var(--ui-small-size); font-weight: var(--ui-title-weight); font-variant-numeric: tabular-nums; }
  h2 { margin-bottom: .8rem; color: var(--text-strong); font-size: var(--ui-title-size); font-weight: var(--ui-title-weight); }
  h3 { margin-top: 1.6rem; margin-bottom: .6rem; color: var(--text-secondary); font-size: var(--ui-body-size); font-weight: var(--ui-title-weight); }
  p + p { margin-top: .8rem; }
  .tip { color: var(--text-muted); padding-inline-start: 1.2rem; border-inline-start: 2px solid var(--border-accent); font-size: var(--ui-body-size); }
  @media (max-width: 600px) { section { padding-inline-start: 3rem; } }
</style>
