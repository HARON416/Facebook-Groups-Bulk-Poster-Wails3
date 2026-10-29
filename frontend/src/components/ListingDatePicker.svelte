<script lang="ts">
  import { tick } from 'svelte';
  import { getLocale } from '../lib/i18n.svelte';
  import { localeDirection } from '../lib/locale-preferences';
  import { calendarDays, dateKey, parseDateKey, shiftCalendarMonth } from '../lib/calendar-date';
  import PhosphorIcon from './PhosphorIcon.svelte';

  let { id, label, value = $bindable('') }: { id: string; label: string; value?: string } = $props();
  let root: HTMLDivElement;
  let trigger: HTMLButtonElement;
  let panel = $state<HTMLDivElement>();
  let expanded = $state(false), above = $state(false), maxHeight = $state(340);
  let today = $state(dateKey(new Date()));
  let month = $state(new Date());
  let focused = $state(dateKey(new Date()));
  const selected = $derived(parseDateKey(value));
  const display = $derived(selected?.toLocaleDateString(getLocale(), { year: 'numeric', month: 'short', day: 'numeric' }) ?? '');
  const monthLabel = $derived(month.toLocaleDateString(getLocale(), { month: 'long', year: 'numeric' }));
  const days = $derived(calendarDays(month));
  const weekdays = $derived(Array.from({ length: 7 }, (_, index) => new Date(2026, 10, index + 1).toLocaleDateString(getLocale(), { weekday: 'short' })));
  const previousDisabled = $derived(dateKey(new Date(month.getFullYear(), month.getMonth() + 1, 0, 12)) < today || month.getFullYear() === Number(today.slice(0, 4)) && month.getMonth() === Number(today.slice(5, 7)) - 1);

  function place() {
    const bounds = root.closest('.form-scroll')?.getBoundingClientRect();
    const top = Math.max(0, bounds?.top ?? 0), bottom = Math.min(window.innerHeight, bounds?.bottom ?? window.innerHeight);
    const field = root.getBoundingClientRect();
    const below = bottom - field.bottom - 6, roomAbove = field.top - top - 6;
    above = below < 340 && roomAbove > below;
    maxHeight = Math.max(60, Math.min(340, above ? roomAbove : below));
  }
  async function open() {
    today = dateKey(new Date());
    const initial = selected && value >= today ? selected : parseDateKey(today)!;
    focused = dateKey(initial);
    month = new Date(initial.getFullYear(), initial.getMonth(), 1, 12);
    place(); expanded = true;
    await tick();
    focusDay();
  }
  function focusDay() { panel?.querySelector<HTMLButtonElement>(`[data-date="${focused}"]`)?.focus({ preventScroll: true }); }
  function close(restore = false) { expanded = false; if (restore) trigger?.focus({ preventScroll: true }); }
  function select(key: string) { value = key; close(true); }
  function changeMonth(offset: number) {
    month = new Date(month.getFullYear(), month.getMonth() + offset, 1, 12);
    focused = days.find(day => !day.outside && day.key >= today)?.key ?? today;
  }
  async function keydown(event: KeyboardEvent, date: Date) {
    const next = new Date(date);
    const direction = localeDirection(getLocale()) === 'rtl' ? -1 : 1;
    const shifts: Record<string, number> = { ArrowLeft: -direction, ArrowRight: direction, ArrowUp: -7, ArrowDown: 7, Home: -date.getDay(), End: 6 - date.getDay() };
    let target: Date;
    if (event.key in shifts) { next.setDate(next.getDate() + shifts[event.key]); target = next; }
    else if (event.key === 'PageUp' || event.key === 'PageDown') target = shiftCalendarMonth(date, (event.key === 'PageUp' ? -1 : 1) * (event.shiftKey ? 12 : 1));
    else return;
    event.preventDefault();
    if (dateKey(target) < today) target = parseDateKey(today)!;
    focused = dateKey(target);
    month = new Date(target.getFullYear(), target.getMonth(), 1, 12);
    await tick(); focusDay();
  }
</script>

<svelte:window onpointerdown={(event) => { if (!root?.contains(event.target as Node)) close(); }} onresize={() => { if (expanded) place(); }}
  onkeydown={(event) => { if (expanded && event.key === 'Escape' && root?.contains(event.target as Node)) { event.preventDefault(); event.stopPropagation(); close(true); } }} />

<div class="date-field" role="group" aria-label={label} class:filled={Boolean(display)} class:expanded bind:this={root}
  onfocusout={(event) => { if (!root.contains(event.relatedTarget as Node | null)) close(); }}>
  <span class="calendar-icon"><PhosphorIcon name="calendar-blank" size={20} /></span>
  <label for={id}>{label}</label>
  <button class="date-trigger" type="button" {id} bind:this={trigger} aria-haspopup="dialog" aria-expanded={expanded} aria-controls={expanded ? `${id}-calendar` : undefined}
    onclick={() => { if (expanded) close(); else void open(); }}
    onkeydown={(event) => { if (event.key === 'ArrowDown' || event.key === 'ArrowUp') { event.preventDefault(); void open(); } }}>{display || '\u00a0'}</button>
  {#if expanded}
    <div class="calendar-panel" class:above role="dialog" aria-modal="false" aria-label={label} id={`${id}-calendar`} bind:this={panel} style:max-height={`${maxHeight}px`}>
      <div class="month-navigation">
        <button type="button" aria-label="Previous month" disabled={previousDisabled} onclick={() => changeMonth(-1)}><span class="previous-icon"><PhosphorIcon name="caret-down" size={16} /></span></button>
        <h3 aria-live="polite">{monthLabel}</h3>
        <button type="button" aria-label="Next month" onclick={() => changeMonth(1)}><span class="next-icon"><PhosphorIcon name="caret-down" size={16} /></span></button>
      </div>
      <div class="weekdays" aria-hidden="true">{#each weekdays as day}<span>{day}</span>{/each}</div>
      <div class="calendar-days" role="group" aria-label={monthLabel}>
        {#each days as day (day.key)}
          <button type="button" data-date={day.key} class:outside={day.outside} class:selected={day.key === value} class:today={day.key === today} disabled={day.key < today}
            tabindex={day.key === focused ? 0 : -1} aria-pressed={day.key === value} aria-current={day.key === today ? 'date' : undefined}
            aria-label={day.date.toLocaleDateString(getLocale(), { weekday: 'long', year: 'numeric', month: 'long', day: 'numeric' })}
            onfocus={() => { focused = day.key; }} onclick={() => select(day.key)} onkeydown={(event) => { void keydown(event, day.date); }}>{day.date.getDate()}</button>
        {/each}
      </div>
      {#if value}<button type="button" class="clear-date" onclick={() => select('')}>Clear date</button>{/if}
    </div>
  {/if}
</div>

<style>
  .date-field { position: relative; min-width: 0; }
  label { position: absolute; z-index: 1; inset-inline-start: 4rem; top: 1.8rem; max-width: calc(100% - 5.4rem); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: var(--text-muted); font-size: 1.4rem; line-height: 1.5; pointer-events: none; }
  .filled label, .expanded label { top: .6rem; font-size: var(--ui-small-size); line-height: 1.3; }
  .expanded label { color: var(--accent); }
  .calendar-icon { position: absolute; z-index: 1; inset-inline-start: 1.4rem; top: 50%; transform: translateY(-50%); color: var(--text-muted); pointer-events: none; }
  .date-trigger { width: 100%; min-height: 5.8rem; padding: 2.4rem 1.4rem .8rem 4rem; padding-inline: 4rem 1.4rem; border: 1px solid var(--border-strong); border-radius: 1rem; background: var(--surface); color: var(--text); font: inherit; font-size: 1.4rem; line-height: 1.5; text-align: start; cursor: pointer; }
  .date-trigger:hover { background: var(--surface-hover); }
  .expanded .date-trigger { border-color: var(--accent); }
  button:focus-visible { outline: 2px solid var(--focus); outline-offset: 2px; }
  .calendar-panel { position: absolute; z-index: 40; top: calc(100% + .5rem); inset-inline-start: 0; width: min(30rem, 100%); padding: 1.2rem; overflow-y: auto; border: 1px solid var(--border-strong); border-radius: 1.2rem; background: var(--bg); box-shadow: 0 .6rem 1.8rem var(--shadow-soft); }
  .calendar-panel.above { top: auto; bottom: calc(100% + .5rem); }
  .month-navigation { display: flex; align-items: center; justify-content: space-between; gap: .4rem; margin-bottom: .8rem; }
  h3 { color: var(--text-strong); font-size: var(--ui-body-size); font-weight: var(--ui-title-weight); }
  .calendar-panel button { border: 0; border-radius: .5rem; background: transparent; color: var(--text); font: inherit; cursor: pointer; }
  .month-navigation button { display: grid; place-items: center; width: 3rem; height: 3rem; flex-shrink: 0; }
  .previous-icon { transform: rotate(90deg); }
  .next-icon { transform: rotate(-90deg); }
  :global([dir=rtl]) .previous-icon { transform: rotate(-90deg); }
  :global([dir=rtl]) .next-icon { transform: rotate(90deg); }
  .weekdays, .calendar-days { display: grid; grid-template-columns: repeat(7, minmax(0, 1fr)); text-align: center; }
  .weekdays { padding-bottom: .6rem; margin-bottom: .5rem; border-bottom: 1px solid var(--border); color: var(--text-muted); font-size: var(--ui-small-size); }
  .calendar-days button { min-height: 3rem; font-size: var(--ui-small-size); font-variant-numeric: tabular-nums; }
  .calendar-panel button:hover:not(:disabled) { background: var(--surface-hover); }
  .calendar-panel button:disabled, .calendar-days .outside { color: var(--text-muted); }
  .calendar-panel button:disabled { opacity: .45; cursor: default; }
  .calendar-days .today { font-weight: 700; }
  .calendar-days .selected, .calendar-days .selected:hover { background: var(--accent); color: var(--on-accent); }
  .calendar-panel .clear-date { display: block; margin-top: .8rem; padding: .5rem .8rem; color: var(--accent); font-size: var(--ui-small-size); }
</style>
