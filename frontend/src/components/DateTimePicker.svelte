<script lang="ts">
  import { getLocale } from '../lib/i18n.svelte';
  import { localeDirection } from '../lib/locale-preferences';
  import { onMount, tick } from 'svelte';

  let { value = $bindable(''), valid = $bindable(false) }: { value?: string; valid?: boolean } = $props();
  const pad = (number: number) => String(number).padStart(2, '0');
  const dayKey = (date: Date) => `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`;
  const parse = (text: string) => {
    const date = new Date(text);
    return Number.isFinite(date.getTime()) ? date : new Date();
  };
  const initialNow = new Date();
  let now = $state(initialNow);
  let month = $state(new Date(initialNow.getFullYear(), initialNow.getMonth(), 1));
  let calendar: HTMLDivElement;
  const selected = $derived(parse(value));
  const selectedDay = $derived(dayKey(selected));
  const today = $derived(dayKey(now));
  const hour = $derived(selected.getHours());
  const minute = $derived(selected.getMinutes());
  const isFuture = $derived(!!value && new Date(value).getTime() > now.getTime());
  const monthLabel = $derived(month.toLocaleDateString(getLocale(), { month: 'long', year: 'numeric' }));
  const previousDisabled = $derived(month.getFullYear() * 12 + month.getMonth() <= now.getFullYear() * 12 + now.getMonth());
  const days = $derived.by(() => {
    const start = new Date(month.getFullYear(), month.getMonth(), 1);
    const offset = (start.getDay() + 6) % 7;
    const count = new Date(month.getFullYear(), month.getMonth() + 1, 0).getDate();
    return Array.from({ length: Math.ceil((offset + count) / 7) * 7 }, (_, index) => {
      const date = new Date(month.getFullYear(), month.getMonth(), index - offset + 1);
      return { date, key: dayKey(date), outside: date.getMonth() !== month.getMonth() };
    });
  });
  const tabDay = $derived(days.some((day) => day.key === selectedDay) ? selectedDay : days.find((day) => !day.outside && day.key >= today)?.key);
  const weekdays = $derived(Array.from({ length: 7 }, (_, index) => {
    const date = new Date(now.getFullYear(), now.getMonth(), now.getDate() + (index - now.getDay() + 7) % 7, hour, minute);
    if (date.getTime() <= now.getTime()) date.setDate(date.getDate() + 7);
    return { label: date.toLocaleDateString(getLocale(), { weekday: 'narrow' }), index, date, description: date.toLocaleDateString(getLocale(), { weekday: 'long', month: 'short', day: 'numeric', year: 'numeric' }) };
  }));
  const presets = [{ label: 'Morning', hour: 9 }, { label: 'Afternoon', hour: 13 }, { label: 'Evening', hour: 18 }];
  $effect(() => { valid = isFuture; });
  $effect(() => {
    const date = parse(value);
    month = new Date(date.getFullYear(), date.getMonth(), 1);
  });
  onMount(() => {
    const timer = setInterval(() => { now = new Date(); }, 15000);
    return () => clearInterval(timer);
  });

  function update(date: Date) {
    value = `${dayKey(date)}T${pad(date.getHours())}:${pad(date.getMinutes())}`;
    now = new Date();
  }
  function selectDay(date: Date) {
    update(new Date(date.getFullYear(), date.getMonth(), date.getDate(), hour, minute));
  }
  function changeMonth(offset: number) {
    month = new Date(month.getFullYear(), month.getMonth() + offset, 1);
  }
  function setTime(hours: number, minutes: number) {
    const date = new Date(selected);
    date.setHours(hours, minutes, 0, 0);
    update(date);
  }
  function stepTime(part: 'hour' | 'minute', direction: number) {
    if (part === 'hour') setTime((hour + direction + 24) % 24, minute);
    else setTime(hour, (minute + direction + 60) % 60);
  }
  function inputTime(event: Event, part: 'hour' | 'minute') {
    const input = event.currentTarget as HTMLInputElement;
    const number = Number(input.value);
    const maximum = part === 'hour' ? 23 : 59;
    if (!/^\d{1,2}$/.test(input.value) || number > maximum) {
      input.value = pad(part === 'hour' ? hour : minute);
      return;
    }
    setTime(part === 'hour' ? number : hour, part === 'minute' ? number : minute);
    input.value = pad(number);
  }
  function presetPast(hours: number) {
    const date = new Date(selected);
    date.setHours(hours, 0, 0, 0);
    return date.getTime() <= now.getTime();
  }
  async function moveDay(event: KeyboardEvent, date: Date) {
    const next = new Date(date);
    const horizontal = localeDirection(getLocale()) === 'rtl' ? -1 : 1;
    const shifts: Record<string, number> = { ArrowLeft: -horizontal, ArrowRight: horizontal, ArrowUp: -7, ArrowDown: 7, Home: -((date.getDay() + 6) % 7), End: 6 - ((date.getDay() + 6) % 7) };
    if (event.key in shifts) next.setDate(next.getDate() + shifts[event.key]);
    else if (event.key === 'PageUp' || event.key === 'PageDown') {
      const day = next.getDate();
      next.setDate(1);
      next.setMonth(next.getMonth() + (event.key === 'PageUp' ? -1 : 1));
      next.setDate(Math.min(day, new Date(next.getFullYear(), next.getMonth() + 1, 0).getDate()));
    } else return;
    event.preventDefault();
    selectDay(dayKey(next) < today ? now : next);
    await tick();
    calendar?.querySelector<HTMLButtonElement>(`[data-date="${selectedDay}"]`)?.focus();
  }
</script>

<div class="date-time-picker" id="schedule-time" tabindex="-1" aria-label="Choose a date and time">
  <section class="date-section" aria-label="Select date">
    <div class="weekday-shortcuts" role="group" aria-label="Choose a weekday for this post">
      {#each weekdays as day (day.index)}
        <button type="button" class:chosen={selected.getDay() === day.index} aria-pressed={selected.getDay() === day.index} aria-label={`Select ${day.description}`} title={day.description} onclick={() => selectDay(day.date)}>{day.label}</button>
      {/each}
    </div>
    <div class="month-navigation">
      <button type="button" aria-label="Previous month" disabled={previousDisabled} onclick={() => changeMonth(-1)}><svg viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="m14 6-6 6 6 6" /></svg></button>
      <h3 aria-live="polite">{monthLabel}</h3>
      <button type="button" aria-label="Next month" onclick={() => changeMonth(1)}><svg viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="m10 6 6 6-6 6" /></svg></button>
    </div>
    <div class="weekdays" aria-hidden="true">{#each Array.from({ length: 7 }, (_, index) => new Date(2024, 0, index + 1).toLocaleDateString(getLocale(), { weekday: 'narrow' })) as day}<span>{day}</span>{/each}</div>
    <div class="calendar" bind:this={calendar} role="group" aria-label={monthLabel}>
      {#each days as day (day.key)}
        <button type="button" data-date={day.key} class:outside={day.outside} class:selected={day.key === selectedDay} class:today={day.key === today}
          disabled={day.key < today} tabindex={day.key === tabDay ? 0 : -1}
          aria-label={day.date.toLocaleDateString(getLocale(), { weekday: 'long', year: 'numeric', month: 'long', day: 'numeric' })}
          aria-pressed={day.key === selectedDay} aria-current={day.key === today ? 'date' : undefined}
          onclick={() => selectDay(day.date)} onkeydown={(event) => moveDay(event, day.date)}>{day.date.getDate()}</button>
      {/each}
    </div>
  </section>
  <section class="time-section" aria-label="Select time">
    <div class="time-heading"><h3>Select time</h3><span>24-hour clock</span></div>
    <div class="time-controls">
      <div class="time-unit">
        <button type="button" aria-label="Increase hour" onclick={() => stepTime('hour', 1)}>⌃</button>
        <input aria-label="Hour" type="text" inputmode="numeric" maxlength="2" onfocus={(event) => event.currentTarget.select()} value={pad(hour)} onchange={(event) => inputTime(event, 'hour')} onkeydown={(event) => { if (event.key === 'Enter') { event.preventDefault(); event.currentTarget.blur(); } else if (event.key === 'ArrowUp' || event.key === 'ArrowDown') { event.preventDefault(); stepTime('hour', event.key === 'ArrowUp' ? 1 : -1); } }} />
        <button type="button" aria-label="Decrease hour" onclick={() => stepTime('hour', -1)}>⌄</button>
        <span>Hour</span>
      </div>
      <span class="colon" aria-hidden="true">:</span>
      <div class="time-unit">
        <button type="button" aria-label="Increase minute" onclick={() => stepTime('minute', 1)}>⌃</button>
        <input aria-label="Minute" type="text" inputmode="numeric" maxlength="2" onfocus={(event) => event.currentTarget.select()} value={pad(minute)} onchange={(event) => inputTime(event, 'minute')} onkeydown={(event) => { if (event.key === 'Enter') { event.preventDefault(); event.currentTarget.blur(); } else if (event.key === 'ArrowUp' || event.key === 'ArrowDown') { event.preventDefault(); stepTime('minute', event.key === 'ArrowUp' ? 1 : -1); } }} />
        <button type="button" aria-label="Decrease minute" onclick={() => stepTime('minute', -1)}>⌄</button>
        <span>Minute</span>
      </div>
    </div>
    <div class="time-presets">
      {#each presets as preset}<button type="button" disabled={presetPast(preset.hour)} class:chosen={hour === preset.hour && minute === 0} onclick={() => setTime(preset.hour, 0)}><span>{preset.label}</span><strong>{pad(preset.hour)}:00</strong></button>{/each}
    </div>
  </section>
  <div class="selection-summary" aria-live="polite"><svg viewBox="0 0 24 24" fill="none" aria-hidden="true"><rect x="3" y="5" width="18" height="16" rx="3" /><path d="M7 3v4m10-4v4M3 11h18m-13 5h3" /></svg><span>{selected.toLocaleDateString(getLocale(), { weekday: 'short', month: 'short', day: 'numeric', year: 'numeric' })}<strong>{pad(hour)}:{pad(minute)}</strong></span></div>
  {#if !isFuture}<p class="validation" role="status">Choose a date and time in the future.</p>{/if}
</div>

<style>
  .date-time-picker { display: grid; grid-template-columns: minmax(0, 1.2fr) minmax(0, 1fr); border: 1px solid var(--border); border-radius: 1.2rem; background: var(--surface); box-shadow: 0 4px 16px var(--shadow-soft); overflow: hidden; }
  .date-time-picker:focus-visible { outline: 2px solid var(--focus); outline-offset: 3px; }
  section { padding: 1.8rem; min-width: 0; }
  .date-section { border-inline-end: 1px solid var(--border); }
  button { font: inherit; cursor: pointer; border: 0; color: var(--text-secondary); background: transparent; }
  button:disabled { opacity: .3; cursor: not-allowed; }
  button:focus-visible, input:focus-visible { outline: 2px solid var(--focus); outline-offset: 2px; }
  .weekday-shortcuts { display: flex; align-items: center; justify-content: space-between; gap: .6rem; margin-bottom: 1.6rem; }
  .weekday-shortcuts button { display: grid; place-items: center; width: 3rem; height: 3rem; flex: 0 1 3rem; aspect-ratio: 1; padding: 0; border: 1px solid var(--border); border-radius: 50%; font-size: var(--ui-small-size); }
  .weekday-shortcuts button:hover:not(.chosen) { background: var(--surface-hover); border-color: var(--accent); }
  .weekday-shortcuts .chosen { background: var(--accent); border-color: var(--accent); color: var(--on-accent); }
  .time-presets .chosen { background: var(--surface-active); border-color: var(--border-accent); color: var(--accent); }
  .month-navigation { display: flex; align-items: center; justify-content: space-between; margin-bottom: 1.2rem; }
  h3 { font-size: var(--ui-body-size); color: var(--text-strong); font-weight: 550; }
  .month-navigation button { display: grid; place-items: center; width: 3rem; height: 3rem; border-radius: .6rem; }
  .month-navigation button:hover:not(:disabled), .calendar button:hover:not(:disabled):not(.selected) { background: var(--surface-hover); }
  svg { width: 1.6rem; height: 1.6rem; stroke: currentColor; stroke-width: 1.6; stroke-linecap: round; stroke-linejoin: round; }
  .weekdays, .calendar { display: grid; grid-template-columns: repeat(7, minmax(0, 1fr)); gap: .4rem; text-align: center; }
  .weekdays { font-size: var(--ui-small-size); color: var(--text-muted); margin-bottom: .8rem; }
  .calendar button { min-height: 3.2rem; border-radius: .6rem; font-size: var(--ui-small-size); position: relative; }
  .calendar .outside { color: var(--text-faint); }
  .calendar .today::after { content: ''; position: absolute; bottom: .3rem; left: calc(50% - .15rem); width: .3rem; height: .3rem; border-radius: 50%; background: var(--accent); }
  .calendar .selected { background: var(--accent); color: var(--on-accent); font-weight: 600; box-shadow: 0 2px 5px var(--accent-soft); }
  .calendar .selected.today::after { background: var(--on-accent); }
  .time-heading { display: flex; align-items: center; justify-content: space-between; gap: .5rem; margin-top: .5rem; }
  .time-heading > span { font-size: var(--ui-small-size); color: var(--text-muted); }
  .time-controls { direction: ltr; display: flex; justify-content: center; align-items: center; gap: 1.4rem; margin: 2rem 0; padding: .8rem; border-radius: 1rem; background: var(--surface-hover); }
  .selection-summary strong { direction: ltr; }
  :global(html[dir="rtl"]) .month-navigation button svg { transform: scaleX(-1); }
  .time-unit { display: flex; flex-direction: column; align-items: center; gap: .4rem; }
  .time-unit button { width: 4.8rem; height: 2.4rem; border-radius: .5rem; font-size: 2rem; color: var(--text-muted); }
  .time-unit button:hover { background: var(--surface-hover); color: var(--accent); }
  .time-unit input { width: 5.2rem; height: 4.8rem; padding: 0; border: 1px solid var(--border); border-radius: .7rem; background: var(--surface); text-align: center; font: inherit; font-size: 2.6rem; font-weight: 550; font-variant-numeric: tabular-nums; color: var(--text-strong); }
  .time-unit > span { font-size: var(--ui-small-size); color: var(--text-muted); }
  .colon { font-size: 2rem; color: var(--text-muted); margin-top: -1.6rem; }
  .time-presets { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: .6rem; }
  .time-presets button { display: flex; flex-direction: column; align-items: center; gap: .5rem; padding: .9rem .3rem; border: 1px solid var(--border); border-radius: .6rem; }
  .time-presets span { font-size: var(--ui-small-size); color: var(--text-muted); }
  .time-presets strong { font-size: var(--ui-small-size); font-weight: 500; }
  .selection-summary { grid-column: 1 / -1; display: flex; gap: .8rem; align-items: center; padding: 1.3rem 1.8rem; background: var(--surface-active); border-top: 1px solid var(--border); color: var(--accent); font-size: var(--ui-small-size); }
  .selection-summary span { display: flex; flex: 1; justify-content: space-between; align-items: center; gap: 1rem; }
  .selection-summary strong { font-size: 1.4rem; font-weight: 600; font-variant-numeric: tabular-nums; }
  .validation { grid-column: 1 / -1; padding: 1rem 1.8rem; font-size: var(--ui-small-size); color: var(--danger); border-top: 1px solid var(--danger-border); }
  @media (max-width: 600px) { .date-time-picker { grid-template-columns: 1fr; } .date-section { border-inline-end: 0; border-bottom: 1px solid var(--border); } }
</style>
