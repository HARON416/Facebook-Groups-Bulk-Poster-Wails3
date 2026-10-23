<script lang="ts">
  import { getLocale } from '../lib/i18n.svelte';
  import LocalizedFeedback from './LocalizedFeedback.svelte';
  import { hasLogDetails } from '../lib/log-message';
  import { logMessage, scheduledLogMessage } from './LogMessage.svelte';
  import { onMount, tick, untrack } from 'svelte';
  import { createAccountService } from '../lib/accounts';
  import type { AccountProfile } from '../../bindings/postika';
  import AccountDropdown from './AccountDropdown.svelte';
  import { accountLabel } from './AccountLabel.svelte';

  let { profile, profiles, onselect }: { profile: AccountProfile; profiles: AccountProfile[]; onselect: (id: string) => void } = $props();
  const GroupsService = untrack(() => createAccountService(profile.id));
  const displayProfiles = $derived(profiles.map((item) => ({ ...item, name: accountLabel(item) })));
  import AuxiliaryWindow from './AuxiliaryWindow.svelte';
  import { formatDuration, type Activity } from '../lib/posting';
  import { pollWhileWindowOpen } from '../lib/window-polling';

  type Entry = { id: number; level: string; outcome?: string; timestamp: string; message: string; hasResponse: boolean; diagnostic?: string };
  let entries = $state<Entry[]>([]);
  let total = $state(0);
  let page = $state(1);
  const pageSize = 50;
  let runId = 0;
  let followLatest = $state(true);
  let active = $state(false);
  let jobState = $state('idle');
  let pauseRequested = $state(false);
  let offline = $state(false);
  let message = $state(/* @wc-ignore */ 'Ready to post.');
  let error = $state('');
  let controlBusy = $state(false);
  let reading = false;
  let logScroller: HTMLElement;
  let copyingId = $state<number | null>(null);
  let copiedId = $state<number | null>(null);
  let copyFeedback = $state('');
  let navigationVersion = 0;
  let activity = $state<Activity | null>(null);
  const percentage = $derived(activity?.total ? Math.round(activity.processed / activity.total * 100) : 0);
  const pages = $derived(Math.max(1, Math.ceil(total / pageSize)));
  const pageNumbers = $derived.by(() => {
    const candidates = new Set([1, 2, 3, 4, pages - 3, pages - 2, pages - 1, pages, page - 1, page, page + 1]);
    const numbers = [...candidates].filter((n) => n >= 1 && n <= pages).sort((a, b) => a - b);
    return numbers.flatMap((n, i) => i && n - numbers[i - 1] > 1 ? [0, n] : [n]);
  });
  const stateLabel = $derived(jobState === 'scheduled' ? /* @wc-include */ 'Scheduled' : offline && active ? /* @wc-include */ 'Paused · Waiting for internet' : jobState === 'pausing' ? /* @wc-include */ 'Pausing after the current operation' : jobState === 'paused' ? /* @wc-include */ 'Paused' : jobState === 'stopping' ? /* @wc-include */ 'Stopping' : active ? /* @wc-include */ 'Posting' : jobState === 'complete' ? /* @wc-include */ 'Finished' : jobState === 'stopped' ? /* @wc-include */ 'Stopped' : /* @wc-include */ 'Ready');

  async function refresh() {
    if (reading) return;
    reading = true;
    const version = navigationVersion;
    try {
      const result = await GroupsService.GetPostingLogs(followLatest ? 2147483647 : page, pageSize);
      if (version !== navigationVersion) return;
      const hasNewEntries = result.runId !== runId || result.entries?.at(-1)?.id !== entries.at(-1)?.id;
      const nearLatest = !logScroller || logScroller.scrollHeight - logScroller.scrollTop - logScroller.clientHeight < 64;
      if (result.runId !== runId) { runId = result.runId; followLatest = true; copiedId = null; copyFeedback = ''; }
      total = result.total;
      page = result.page;
      entries = result.entries ?? [];
      active = result.status.active;
      jobState = result.status.state;
      pauseRequested = result.status.pauseRequested;
      offline = result.status.offline;
      message = result.status.message || /* @wc-ignore */ 'Ready to post.';
      activity = result.status as Activity;
      error = '';
      if (followLatest && hasNewEntries && nearLatest) {
        await tick();
        logScroller?.scrollTo({ top: logScroller.scrollHeight });
      }
    } catch (cause) { error = cause instanceof Error ? cause.message : String(cause); }
    finally { reading = false; }
  }
  async function goToPage(next: number) {
    page = Math.max(1, Math.min(pages, next));
    navigationVersion++;
    followLatest = page === pages;
    await refresh();
    await tick();
    logScroller?.scrollTo({ top: followLatest ? logScroller.scrollHeight : 0 });
  }
  async function copyResponse(entry: Entry) {
    if (copyingId !== null) return;
    const responseRun = runId;
    copyingId = entry.id;
    copyFeedback = '';
    error = '';
    try {
      await GroupsService.CopyPostingResponse(responseRun, entry.id);
      if (responseRun === runId) { copiedId = entry.id; copyFeedback = /* @wc-ignore */ 'Facebook response copied.'; }
    } catch (cause) {
      error = cause instanceof Error ? cause.message : String(cause);
    } finally { copyingId = null; }
  }
  async function control(action: 'pause' | 'resume' | 'stop') {
    if (controlBusy) return;
    controlBusy = true;
    try {
      if (action === 'pause') await GroupsService.PausePosting();
      else if (action === 'resume') await GroupsService.ResumePosting();
      else await GroupsService.CancelPosting();
      await refresh();
    } catch (cause) { error = cause instanceof Error ? cause.message : String(cause); }
    finally { controlBusy = false; }
  }
  function timestamp(value: string) {
    return new Intl.DateTimeFormat(getLocale(), { hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false }).format(new Date(value));
  }
  function levelLabel(level: string) {
    const labels: Record<string, string> = { warning: /* @wc-include */ 'WARN', info: /* @wc-include */ 'INFO', error: /* @wc-include */ 'ERROR', debug: /* @wc-include */ 'DEBUG' };
    return labels[level] ?? level.toUpperCase();
  }
  function entryLabel(entry: Entry) {
    const labels: Record<string, string> = { posted: /* @wc-include */ 'PUBLISHED', pending: /* @wc-include */ 'PENDING', failed: /* @wc-include */ 'FAILED', skipped: /* @wc-include */ 'SKIPPED', unknown: /* @wc-include */ 'UNCONFIRMED' };
    return labels[entry.outcome ?? ''] ?? levelLabel(entry.level);
  }
  function itemTime(value: string) {
    const match = value.match(/ · Item time (\d+)s$/);
    return match ? formatDuration(Number(match[1])) : '';
  }
  onMount(() => pollWhileWindowOpen(refresh, 500));
</script>

<AuxiliaryWindow title="Posting logs" account={accountLabel(profile)}>
{#snippet profileSwitcher()}
  <AccountDropdown profiles={displayProfiles} selected={profile.id} {onselect} compact />
{/snippet}
{#snippet actions()}
  <span class="run-state" dir="auto" class:running={active && !pauseRequested}><span class="state-text">{stateLabel}</span></span>
{/snippet}
<div class="logs-window ui-window">
  <header>
    <div class="heading"><div><p class="eyebrow">POSTIKA / ACTIVITY</p><h1 class="ui-title">Posting logs</h1></div></div>
    <section class="activity" aria-label="Posting progress">
      <div class="progress-label"><strong>{activity?.processed || 0}/{activity?.total || 0} groups processed</strong><span>{percentage}% complete</span></div>
      <progress max="100" value={percentage} aria-label="Groups processed">{percentage}%</progress>
      <div class="counts"><div class="published"><strong>{activity?.posted || 0}</strong><span>Published</span></div><div class="pending"><strong>{activity?.pending || 0}</strong><span>Awaiting approval</span></div><div class="failed"><strong>{activity?.failed || 0}</strong><span>Failed or skipped</span></div><div><strong>{activity?.remaining || 0}</strong><span>Remaining</span></div></div>
      <div class="current-activity">
        <div class="current-action">
          <p class="eyebrow">{active ? /* @wc-include */ 'CURRENT ACTIVITY' : /* @wc-include */ 'RUN STATUS'}</p>
          {#if activity?.currentGroup?.name}<p class="current-group">Group {activity.current}/{activity.total}: {activity.currentGroup.name}</p>{/if}
          {#if activity?.waitSeconds && active}<div class="countdown" class:paused={pauseRequested}>Next post in <strong>{formatDuration(activity.waitSeconds)}</strong>{pauseRequested ? /* @wc-include */ ' · Paused' : ''}</div>{:else}<p class="action-message" role="status">{jobState === 'scheduled' && activity?.scheduledAt ? scheduledLogMessage(activity.scheduledAt) : logMessage(message, true)}</p>{/if}
        </div>
        <div class="timing"><span>Elapsed <strong>{formatDuration(activity?.elapsedSeconds || 0)}</strong></span><span>Current group <strong>{formatDuration(activity?.itemSeconds || 0)}</strong></span></div>
      </div>
    </section>
    {#if error}<div class="error" role="alert"><LocalizedFeedback message={error} /></div>{/if}
  </header>
  <div class="feed-heading"><h2>Activity timeline <span>{total} logs</span></h2><button class="follow" class:following={followLatest} onclick={() => goToPage(pages)}>{followLatest ? 'Following latest' : 'Jump to latest'} <span aria-hidden="true">↓</span></button></div>
  <main aria-label="Posting activity" bind:this={logScroller}>
    <ol class="timeline">
        {#each entries as entry (entry.id)}
          <li class="timeline-item" class:warning={entry.level === 'warning' || entry.outcome === 'pending'} class:error={entry.level === 'error'} class:published={entry.outcome === 'posted'} class:debug={entry.level === 'debug'}>
            <span class="timeline-icon" aria-hidden="true"><svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" fill="currentColor" viewBox="0 0 256 256"><path d="M212,136a12,12,0,1,1-12-12A12,12,0,0,1,212,136Zm36,0c0,40.37-21.08,72-48,72H56c-26.92,0-48-31.63-48-72S29.08,64,56,64H92.69l37.65-37.66A8,8,0,0,1,136,24h32a8,8,0,0,1,0,16H139.31l-24,24H200C226.92,64,248,95.63,248,136ZM56,192H169.51a73.46,73.46,0,0,1-12.67-24H80a8,8,0,0,1,0-16h73.16A110.63,110.63,0,0,1,152,136c0-22.86,6.76-42.9,17.51-56H56c-12.47,0-23.55,13.26-28.8,32H104a8,8,0,0,1,0,16H24.35q-.34,3.93-.35,8C24,166.36,38.65,192,56,192Zm176-56c0-30.36-14.65-56-32-56s-32,25.64-32,56,14.65,56,32,56S232,166.36,232,136Z"></path></svg></span>
            <article class="log-card" aria-label={`${entryLabel(entry)} log`}>
              <div class="log-meta"><span class="level">{entryLabel(entry)}</span>{#if entry.outcome}<span class="severity">{levelLabel(entry.level)}</span>{/if}<time datetime={entry.timestamp} title={new Date(entry.timestamp).toLocaleString(getLocale())}>{timestamp(entry.timestamp)}</time>{#if itemTime(entry.message)}<span class="item-time">Took {itemTime(entry.message)}</span>{/if}</div>
              <div class="message-cell"><p class="log-title">{logMessage(entry.message, entry.level === 'debug' || entry.level === 'error')}</p>{#if entry.hasResponse}<button class="copy-response" disabled={copyingId !== null} aria-label="Copy Facebook response" title={copiedId === entry.id ? 'Response copied' : 'Copy Facebook response'} onclick={() => copyResponse(entry)}><svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" fill="currentColor" viewBox="0 0 256 256" aria-hidden="true"><path d="M216,32H88a8,8,0,0,0-8,8V80H40a8,8,0,0,0-8,8V216a8,8,0,0,0,8,8H168a8,8,0,0,0,8-8V176h40a8,8,0,0,0,8-8V40A8,8,0,0,0,216,32ZM160,208H48V96H160Zm48-48H176V88a8,8,0,0,0-8-8H96V48H208Z"></path></svg><span>{copiedId === entry.id ? 'Copied' : copyingId === entry.id ? 'Copying…' : 'Copy response'}</span></button>{/if}</div>
              {#if hasLogDetails(entry.message) || entry.hasResponse || entry.diagnostic}
                <details class="debug-details"><summary><span>Debug details<small>Original log message{entry.hasResponse ? /* @wc-include */ ' · Response available to copy' : ''}</small></span><span class="chevron" aria-hidden="true">⌄</span></summary><p dir="auto" class:technical={hasLogDetails(entry.message)}>{hasLogDetails(entry.message) ? entry.message : logMessage(entry.message)}</p>{#if entry.diagnostic}<p class="technical" dir="auto">{entry.diagnostic}</p>{/if}</details>
              {/if}
            </article>
          </li>
        {:else}
          <li class="empty"><strong>Your activity will appear here</strong><p>Start posting to follow uploads, results, and wait times.</p></li>
        {/each}
    </ol>
  </main>
  <footer>
    <div class="pagination">
      <p>{total ? (page - 1) * pageSize + 1 : 0}–{Math.min(page * pageSize, total)} of {total} logs</p>
      {#if copyFeedback}<span class="copy-feedback" role="status">{logMessage(copyFeedback)}</span>{/if}
      <nav aria-label="Log pages">
        <button aria-label="Previous page" disabled={page <= 1} onclick={() => goToPage(page - 1)}>← Prev</button>
        {#each pageNumbers as number, index (`${number}-${index}`)}
          {#if number === 0}<span class="ellipsis">…</span>{:else}<button class:current={number === page} aria-current={number === page ? 'page' : undefined} aria-label={`Page ${number}`} onclick={() => goToPage(number)}>{number}</button>{/if}
        {/each}
        <button aria-label="Next page" disabled={page >= pages} onclick={() => goToPage(page + 1)}>Next →</button>
      </nav>
    </div>
    <div class="controls">
      <button disabled={!active || offline || jobState === 'scheduled' || jobState === 'stopping' || controlBusy} onclick={() => control(pauseRequested ? 'resume' : 'pause')}>{pauseRequested ? 'Resume' : 'Pause'}</button>
      <button class="stop" disabled={!active || jobState === 'stopping' || controlBusy} onclick={() => control('stop')}>{jobState === 'stopping' ? 'Stopping…' : 'Stop'}</button>
    </div>
  </footer>
</div>
</AuxiliaryWindow>

<style>
  .logs-window {
    height:100%;
    width:100%;
    padding:var(--ui-space) var(--ui-space) 0;
    display:flex;
    flex-direction:column;
    overflow:hidden;
    overflow:clip;
    color:var(--text-strong);
    background:var(--surface);
  }
  header {
    flex:0 0 auto;
    max-height:55%;
    overflow-y:auto;
    overscroll-behavior:contain;
    padding-bottom:1.8rem;
  }
  .heading {
    display:flex;
    justify-content:space-between;
    align-items:center;
    gap:2rem;
  }
  .eyebrow {
    font-size:1rem;
    font-weight:650;
    letter-spacing:.13em;
    color:var(--text-muted);
  }
  h1 {
    margin-top:.5rem;
  }
  .run-state {
    display:inline-flex;
    min-width:0;
    max-width:100%;
    align-items:center;
    padding:.7rem 1.2rem;
    font-size:1.2rem;
    color:var(--text-secondary);
  }
  .state-text { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .run-state.running { color:var(--accent); }
  .activity {
    margin-top:1.8rem;
    font-size:1.2rem;
  }
  .progress-label {
    display:flex;
    justify-content:space-between;
    gap:1.5rem;
    color:var(--text-secondary);
  }
  .progress-label strong {
    font-weight:550;
  }
  progress {
    display:block;
    width:100%;
    height:.6rem;
    margin:.9rem 0 1.6rem;
    border:0;
    border-radius:1rem;
    overflow:hidden;
    appearance:none;
  }
  progress::-webkit-progress-bar {
    background:var(--border);
  }
  progress::-webkit-progress-value {
    background:var(--accent);
    border-radius:1rem;
  }
  progress::-moz-progress-bar {
    background:var(--accent);
  }
  .counts {
    display:grid;
    grid-template-columns:repeat(4,1fr);
    gap:1rem;
  }
  .counts div {
    padding:1.2rem 1.6rem;
    border:1px solid var(--border);
    border-radius:.8rem;
    background:var(--surface);
    display:flex;
    align-items:center;
    gap:1rem;
  }
  .counts strong {
    font-size:2.1rem;
    font-weight:600;
    font-variant-numeric:tabular-nums;
  }
  .counts span {
    color:var(--text-secondary);
    font-size:1.2rem;
  }
  .counts .published strong {
    color:var(--accent);
  }
  .counts .pending strong {
    color:var(--warning);
  }
  .counts .failed strong {
    color:var(--danger);
  }
  .current-activity {
    margin-top:1.2rem;
    padding:1.4rem 1.6rem;
    display:flex;
    justify-content:space-between;
    align-items:center;
    gap:2rem;
    border:1px solid var(--border-accent);
    border-inline-start:3px solid var(--accent);
    background:var(--surface-active);
    border-radius:.4rem;
  }
  .current-action {
    flex:1;
    min-width:0;
  }
  .current-group {
    margin-top:.5rem;
    font-size:1.3rem;
    line-height:1.5;
    font-weight:600;
    overflow-wrap:anywhere;
  }
  .action-message,.countdown {
    margin-top:.4rem;
    font-size:1.3rem;
    line-height:1.5;
    color:var(--text-secondary);
  }
  .countdown strong {
    color:var(--accent);
    font-variant-numeric:tabular-nums;
  }
  .countdown.paused strong {
    color:var(--warning);
  }
  .timing {
    display:flex;
    flex-shrink:0;
    gap:2.4rem;
  }
  .timing span {
    font-size:1rem;
    color:var(--text-secondary);
  }
  .timing strong {
    display:block;
    margin-top:.4rem;
    font-size:1.7rem;
    font-weight:550;
    color:var(--text-strong);
    font-variant-numeric:tabular-nums;
  }
  header>.error {
    margin-top:1rem;
    color:var(--danger);
    font-size:1.3rem;
  }
  .feed-heading {
    flex-shrink:0;
    display:flex;
    align-items:center;
    justify-content:space-between;
    gap:1rem;
    margin-bottom:1rem;
  }
  h2 {
    font-size:var(--ui-title-size);
    font-weight:var(--ui-title-weight);
  }
  h2 span {
    font-size:1.1rem;
    font-weight:400;
    margin-inline-start:.8rem;
    color:var(--text-muted);
  }
  .follow {
    min-height:3rem;
    padding:.4rem .9rem;
    font-size:1.1rem;
    color:var(--text-secondary);
    background:transparent;
  }
  .follow.following {
    color:var(--accent);
    border-color:var(--border-accent);
    background:var(--surface-active);
  }
  main {
    flex:1;
    min-height:0;
    overflow:auto;
    overscroll-behavior:contain;
    padding-block:.2rem 1rem;
    padding-inline:0 .8rem;
  }
  .timeline {
    list-style:none;
    padding-inline-start:4rem;
  }
  .timeline-item {
    --log-color:var(--accent);
    --log-tint:var(--surface-active);
    position:relative;
    padding-bottom:1rem;
  }
  .timeline-item.warning {
    --log-color:var(--warning);
    --log-tint:var(--warning-bg);
  }
  .timeline-item.error {
    --log-color:var(--danger);
    --log-tint:var(--danger-bg);
  }
  .timeline-item.published {
    --log-color: var(--success);
    --log-tint: var(--success-bg);
  }
  .timeline-item.debug {
    --log-color: var(--text-muted);
    --log-tint: var(--border);
  }
  .severity {
    font-size: 0.95rem;
    letter-spacing: 0.04em;
  }
  .timeline-item::before {
    content:'';
    position:absolute;
    inset-inline-start:-2.4rem;
    top:2.6rem;
    bottom:-2.6rem;
    width:1px;
    background:var(--border);
  }
  .timeline-item:last-child::before {
    display:none;
  }
  .timeline-icon {
    position:absolute;
    inset-inline-start:-4rem;
    top:1rem;
    width:3.2rem;
    height:3.2rem;
    display:grid;
    place-items:center;
    color:var(--log-color);
    background:var(--bg);
    border-radius:50%;
    box-shadow:0 0 0 4px var(--bg);
  }
  .log-card {
    background:var(--surface);
    border:1px solid var(--border);
    border-inline-start:3px solid var(--log-color);
    border-radius:.5rem;
    padding:1.2rem 1.5rem;
  }
  .log-meta {
    display:flex;
    align-items:center;
    gap:1rem;
    font-size:1.05rem;
    color:var(--text-muted);
    font-variant-numeric:tabular-nums;
  }
  .level {
    padding:.3rem .6rem;
    color:var(--log-color);
    background:var(--log-tint);
    border-radius:.3rem;
    font-size:.95rem;
    letter-spacing:.05em;
    font-weight:700;
  }
  .item-time {
    margin-inline-start:auto;
  }
  .message-cell {
    display:flex;
    align-items:center;
    justify-content:space-between;
    gap:1.5rem;
    margin-top:.6rem;
  }
  .log-title {
    flex:1;
    min-width:0;
    font-size:1.3rem;
    font-weight:500;
    line-height:1.6;
    overflow-wrap:anywhere;
  }
  .copy-response {
    display:inline-flex;
    align-items:center;
    justify-content:center;
    gap:.6rem;
    flex-shrink:0;
    min-height:3.2rem;
    padding:.5rem .8rem;
    color:var(--accent);
    font-size:1.1rem;
  }
  .debug-details {
    margin-top:1rem;
    border:1px solid var(--border);
    border-inline-start:2px solid var(--log-color);
    background:var(--surface-subtle);
  }
  .debug-details summary {
    list-style:none;
    display:flex;
    align-items:center;
    justify-content:space-between;
    gap:1rem;
    padding:.9rem 1.1rem;
    cursor:pointer;
    font-size:1.1rem;
    font-weight:600;
  }
  .debug-details summary::-webkit-details-marker {
    display:none;
  }
  .debug-details small {
    display:block;
    margin-top:.3rem;
    font-size:1rem;
    font-weight:400;
    color:var(--text-muted);
  }
  .chevron {
    display:grid;
    place-items:center;
    width:2.8rem;
    height:2.8rem;
    font-size:1.7rem;
    color:var(--log-color);
    background:var(--log-tint);
  }
  .debug-details[open] .chevron {
    transform:rotate(180deg);
  }
  .debug-details>p {
    border-top:1px solid var(--border);
    padding:1.1rem;
    font-size:1.2rem;
    color:var(--text-secondary);
    line-height:1.7;
    overflow-wrap:anywhere;
    white-space:pre-wrap;
    user-select:text;
  }
  .debug-details>p.technical {
    font-family:ui-monospace,monospace;
    font-size:1.1rem;
  }
  .empty {
    padding:5rem 2rem;
    text-align:center;
    color:var(--text-secondary);
    font-size:1.3rem;
  }
  .empty strong {
    color:var(--text-strong);
    font-size:1.5rem;
    font-weight:550;
  }
  .empty p {
    margin-top:.7rem;
  }
  footer {
    flex:0 0 auto;
    min-height:7rem;
    padding:1.2rem 0;
    border-top:1px solid var(--border);
    display:flex;
    align-items:center;
    justify-content:space-between;
    gap:2rem;
  }
  .pagination,nav,.controls {
    display:flex;
    align-items:center;
    gap:.8rem;
  }
  .pagination {
    gap:1.2rem;
    flex-wrap:wrap;
    min-width:0;
  }
  nav {
    gap:.3rem;
    max-width:100%;
    overflow-x:auto;
  }
  nav button,nav .ellipsis {
    flex:0 0 auto;
    min-height:3.4rem;
    min-width:3.4rem;
    padding:.5rem .9rem;
  }
  .pagination p,.copy-feedback {
    font-size:1.1rem;
    color:var(--text-secondary);
  }
  .pagination p {
    white-space:nowrap;
    font-variant-numeric:tabular-nums;
  }
  .copy-feedback {
    color:var(--accent);
  }
  button {
    min-height:4rem;
    min-width:4rem;
    padding:.9rem 1.6rem;
    border:1px solid var(--border);
    background:var(--surface);
    border-radius:.7rem;
    color:var(--text-secondary);
    font-size:var(--ui-body-size);
    font-weight:var(--ui-label-weight);
    cursor:pointer;
  }
  button:hover:enabled {
    background:var(--surface-active);
    border-color:var(--accent);
  }
  button.current {
    background:var(--accent);
    border-color:var(--accent);
    color: var(--on-accent);
  }
  button:disabled {
    opacity:.45;
    cursor:not-allowed;
  }
  button:focus-visible,summary:focus-visible,main:focus-visible {
    outline:2px solid var(--accent);
    outline-offset:2px;
  }
  .controls {
    flex-shrink:0;
  }
  .stop {
    color:var(--danger);
    border-color:var(--danger-border);
  }
  .ellipsis {
    display:flex;
    align-items:center;
    justify-content:center;
    font-size:1.2rem;
  }
  @media(max-width:900px) {
    .logs-window {
      padding:var(--ui-space) var(--ui-space) 0;
    }
    .counts div {
      display:block;
      padding:1rem 1.2rem;
    }
    .counts span {
      display:block;
      margin-top:.3rem;
      font-size:1.1rem;
    }
    .timing {
      gap:1.4rem;
    }
    footer {
      gap:1rem;
    }
  }
  @media(max-width:600px) {
    .current-activity {
      align-items:flex-start;
    }
    .counts {
      gap:.5rem;
    }
    .copy-response span {
      display:none;
    }
    .message-cell {
      gap:.7rem;
    }
    .timing {
      flex-direction:column;
      gap:.8rem;
    }
    footer {
      align-items:flex-end;
    }
    .pagination {
      gap:.5rem;
    }
  }
  @media (max-height: 600px) {
    .logs-window { padding-top: 1.2rem; }
    header { max-height: 48%; padding-bottom: 1rem; }
    .heading { position: absolute; width: 1px; height: 1px; overflow: hidden; clip-path: inset(50%); }
    .activity { margin-top: 0; }
    progress { margin: 0.6rem 0 1rem; }
    .counts div { padding: 0.7rem 1rem; }
    .counts strong { font-size: 1.8rem; }
    .current-activity { padding: 1rem 1.2rem; margin-top: 0.8rem; }
    .timing strong { font-size: 1.4rem; }
    .feed-heading { margin-bottom: 0.6rem; }
    footer { min-height: 6rem; padding: 0.8rem 0; }
  }
</style>
