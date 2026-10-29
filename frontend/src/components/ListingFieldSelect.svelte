<script lang="ts">
  import type { ComponentProps } from 'svelte';
  import type { ListingFieldOption } from '../lib/marketplace-fields';
  import PhosphorIcon from './PhosphorIcon.svelte';

  let { id, label, options, value = $bindable(''), placeholder = '', required = false, searchable = false, leadingIcon, onfocus, query = $bindable(''), remote = false, showResults = false, loading = false, emptyText = '', invalid = false, errorMessage = '', onblur, onquery, onselect }: {
    invalid?: boolean; errorMessage?: string; onblur?: () => void;
    id: string; label: string; options: readonly ListingFieldOption[];
    value?: string; placeholder?: string; required?: boolean; searchable?: boolean; leadingIcon?: ComponentProps<typeof PhosphorIcon>['name']; onfocus?: () => void; query?: string; remote?: boolean; showResults?: boolean; loading?: boolean; emptyText?: string; onquery?: (query: string) => void; onselect?: (option: ListingFieldOption) => void;
  } = $props();
  let expanded = $state(false), highlighted = $state(0), above = $state(false);
  let menuHeight = $state(300);
  let menu = $state<HTMLDivElement>();
  let root: HTMLDivElement;
  let trigger = $state<HTMLButtonElement | HTMLInputElement>();
  const menuOpen = $derived(expanded && (remote ? showResults && query.trim() !== '' : !searchable || options.length > 0));
  const selected = $derived(options.find(option => option.value === value));
  const filteredOptions = $derived(searchable && !remote && query.trim() ? options.filter(option => option.label.toLocaleLowerCase().includes(query.trim().toLocaleLowerCase())) : options);
  const sections = $derived.by(() => {
    const result: { id: string; group?: ListingFieldOption['group']; items: { option: ListingFieldOption; index: number }[] }[] = [];
    filteredOptions.forEach((option, index) => {
      const sectionID = option.group?.id ?? `ungrouped-${index}`;
      const last = result.at(-1);
      if (last?.id === sectionID) last.items.push({ option, index });
      else result.push({ id: sectionID, group: option.group, items: [{ option, index }] });
    });
    return result;
  });
  $effect(() => {
    if (!expanded || !menu) return;
    const option = menu.querySelector(`#${id}-option-${highlighted}`);
    if (!option) return;
    const bounds = menu.getBoundingClientRect(), item = option.getBoundingClientRect();
    if (item.top < bounds.top) menu.scrollBy({ top: item.top - bounds.top });
    else if (item.bottom > bounds.bottom) menu.scrollBy({ top: item.bottom - bounds.bottom });
  });
  function open() {
    if (!trigger) return;
    if (searchable && !remote) query = selected?.label ?? '';
    highlighted = Math.max(0, filteredOptions.findIndex(option => option.value === value));
    placeMenu();
    expanded = true;
  }
  function placeMenu() {
    if (!trigger || !root) return;
    const bounds = root.closest('.form-scroll')?.getBoundingClientRect() ?? { top: 0, bottom: window.innerHeight };
    const below = bounds.bottom - trigger.getBoundingClientRect().bottom - 6;
    const top = root.getBoundingClientRect().top - bounds.top - 6;
    const desired = Math.min(Math.max(filteredOptions.length, 1) * 44 + 60, 300);
    above = below < desired && top > below;
    menuHeight = Math.max(60, Math.min(desired, above ? top : below));
  }
  $effect(() => { filteredOptions.length; loading; if (expanded) placeMenu(); });
  function choose(index: number) {
    if (!filteredOptions[index]) return;
    const option = filteredOptions[index];
    value = option.value;
    if (remote) query = option.label;
    onselect?.(option);
    expanded = false;
    trigger?.focus();
  }
  function keydown(event: KeyboardEvent) {
    if (event.isComposing) return;
    if (event.key === 'Tab') { expanded = false; return; }
    if (event.key === 'Escape' && expanded) { event.preventDefault(); event.stopPropagation(); expanded = false; return; }
    if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
      event.preventDefault();
      if (!expanded) open();
      else if (filteredOptions.length) highlighted = (highlighted + (event.key === 'ArrowDown' ? 1 : -1) + filteredOptions.length) % filteredOptions.length;
    } else if (!searchable && (event.key === 'Home' || event.key === 'End')) {
      event.preventDefault(); if (!expanded) open(); highlighted = event.key === 'Home' ? 0 : Math.max(0, filteredOptions.length - 1);
    } else if ((event.key === 'Enter' || (!searchable && event.key === ' ')) && expanded) {
      event.preventDefault(); choose(highlighted);
    }
  }
</script>

<svelte:window onpointerdown={(event) => { if (!root?.contains(event.target as Node)) { if (expanded) onblur?.(); expanded = false; } }} onfocusin={(event) => { if (!root?.contains(event.target as Node)) expanded = false; }} />

<div class="field-control">
<div class="field-select" class:invalid onfocusout={(event) => { if (!root.contains(event.relatedTarget as Node | null)) { expanded = false; onblur?.(); } }} class:filled={Boolean(value) || remote && query.trim() !== ''} class:remote class:searchable class:with-icon={Boolean(leadingIcon)} class:expanded bind:this={root}>
  {#if leadingIcon}<span class="leading-icon" aria-hidden="true"><PhosphorIcon name={leadingIcon} size={20} /></span>{/if}
  <label for={id} title={label}>{label}</label>
  {#if searchable}
    <input class="field-trigger" {id} type="text" {placeholder} role="combobox" autocomplete="off" aria-autocomplete="list" aria-haspopup="listbox" aria-expanded={menuOpen} aria-controls={`${id}-options`} aria-required={required} aria-invalid={invalid} aria-describedby={invalid && errorMessage ? `${id}-error` : undefined} aria-activedescendant={menuOpen && filteredOptions[highlighted] ? `${id}-option-${highlighted}` : undefined} value={remote ? query : expanded ? query : selected?.label ?? ''} bind:this={trigger} onkeydown={keydown} onfocus={(event) => { open(); if (!remote) event.currentTarget.select(); onfocus?.(); }} onclick={() => { if (!expanded) open(); }} oninput={(event) => { query = event.currentTarget.value; expanded = true; highlighted = 0; onquery?.(query); }} />
  {:else}
  <button class="field-trigger" id={id} type="button" role="combobox" aria-haspopup="listbox" aria-expanded={menuOpen} aria-controls={`${id}-options`} aria-required={required} aria-invalid={invalid} aria-describedby={invalid && errorMessage ? `${id}-error` : undefined} aria-activedescendant={menuOpen && filteredOptions[highlighted] ? `${id}-option-${highlighted}` : undefined} bind:this={trigger} onkeydown={keydown} onclick={() => { if (expanded) expanded = false; else open(); }}>
    <span class:placeholder={!selected && !value}>{selected?.label ?? placeholder ?? ''}</span>
    <PhosphorIcon name="caret-down" size={16} />
  </button>
  {/if}
  {#if invalid}<span class="error-icon" aria-hidden="true"><PhosphorIcon name="warning" size={20} /></span>{/if}
  {#if menuOpen}
    <div class="field-options" class:above id={`${id}-options`} role="listbox" aria-label={label} aria-busy={loading} style:max-height={`${menuHeight}px`} bind:this={menu}>
      {#each sections as section (section.id)}
        <div role={section.group ? 'group' : undefined} aria-labelledby={section.group ? `${id}-group-${section.id}` : undefined}>
          {#if section.group}
            <div class="group-heading" id={`${id}-group-${section.id}`}>
              {#if section.group.icon}<span class="group-icon"><PhosphorIcon name={section.group.icon} /></span>{/if}
              <span>{section.group.label}</span>
            </div>
          {/if}
          {#each section.items as { option, index } (option.value)}
            <button id={`${id}-option-${index}`} data-value={option.value} type="button" role="option" aria-selected={value === option.value} aria-labelledby={`${id}-label-${index}`} aria-describedby={option.description ? `${id}-hint-${index}` : undefined} class:grouped={Boolean(section.group)} class:highlighted={highlighted === index} tabindex="-1" onpointermove={() => { highlighted = index; }} onmousedown={(event) => event.preventDefault()} onclick={() => choose(index)}>
              {#if option.icon}<span class="group-icon"><PhosphorIcon name={option.icon} /></span>{/if}
              <span class="option-copy"><span id={`${id}-label-${index}`} dir="auto">{option.label}</span>{#if option.description}<span class="option-hint" id={`${id}-hint-${index}`} dir="auto">{option.description}</span>{/if}</span>
              {#if value === option.value}<PhosphorIcon name="check" size={16} />{/if}
            </button>
          {/each}
        </div>
      {/each}
      {#if loading}<p class="no-options" role="status">Searching…</p>{:else if !filteredOptions.length}<p class="no-options" role="status">{#if remote}{emptyText}{:else if !options.length}Options will be added later.{:else}Try a different name.{/if}</p>{/if}
    </div>
  {/if}
</div>
{#if errorMessage}<p class="field-error" class:inactive={!invalid} id={`${id}-error`} aria-hidden={!invalid} role={invalid ? 'alert' : undefined}>{errorMessage}</p>{/if}
</div>

<style>
  .field-control { min-width: 0; }
  .field-error.inactive { visibility: hidden; }
  .field-error { margin-top: .4rem; color: var(--text-muted); font-size: var(--ui-small-size); line-height: 1.5; }
  .error-icon { position: absolute; inset-inline-end: 1.4rem; top: 50%; transform: translateY(-50%); display: inline-flex; pointer-events: none; color: var(--danger); }
  .field-select.invalid .field-trigger { border-color: var(--danger); padding-inline-end: 4.2rem; }
  .field-select.invalid .field-trigger:focus-visible { outline-color: var(--danger); }
  .field-select { position: relative; min-width: 0; }
  label { position: absolute; z-index: 1; inset-inline-start: 1.4rem; top: 1.8rem; max-width: calc(100% - 5rem); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: var(--text-muted); font-size: 1.4rem; font-weight: 400; line-height: 1.5; pointer-events: none; }
  .filled label, .expanded label, .searchable:focus-within label { top: .6rem; font-size: var(--ui-small-size); line-height: 1.3; }
  .field-trigger { position: relative; display: flex; align-items: center; width: 100%; min-height: 5.8rem; padding: 2.4rem 1.4rem .8rem; padding-inline-end: 4.2rem; border: 1px solid var(--border-strong); border-radius: 1rem; background: var(--surface); color: var(--text); font: inherit; font-size: 1.4rem; line-height: 1.5; text-align: start; cursor: pointer; }
  .field-trigger span { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .field-trigger :global(svg) { position: absolute; inset-inline-end: 1.4rem; top: 50%; transform: translateY(-50%); }
  .leading-icon { position: absolute; z-index: 1; inset-inline-start: 1.4rem; top: 50%; transform: translateY(-50%); display: inline-flex; pointer-events: none; color: var(--text-muted); }
  .with-icon label { inset-inline-start: 4rem; }
  .with-icon .field-trigger { padding-inline-start: 4rem; }
  .expanded label, .searchable:focus-within label { color: var(--accent); }
  input.field-trigger { padding-inline-end: 1.4rem; }
  input.field-trigger::placeholder { color: var(--text-muted); opacity: 0; }
  .field-select:focus-within input.field-trigger::placeholder { opacity: 1; }
  input.field-trigger { cursor: text; }
  .placeholder { visibility: hidden; }
  .field-trigger:focus-visible { outline: 2px solid var(--focus); outline-offset: 2px; }
  .field-trigger:hover, .field-trigger[aria-expanded=true] { background: var(--surface-hover); }
  .field-trigger[aria-expanded=true] { border-color: var(--accent); }
  .field-options { position: absolute; z-index: 40; inset-inline: 0; top: calc(100% + .5rem); max-height: 24rem; overflow-y: auto; padding: .4rem; border: 1px solid var(--border-strong); border-radius: .7rem; background: var(--bg); }
  .field-options.above { top: auto; bottom: calc(100% + .5rem); }
  .field-options button { display: flex; align-items: center; justify-content: space-between; gap: .8rem; width: 100%; min-height: 4rem; padding: .8rem; border: 0; border-radius: .4rem; background: transparent; color: var(--text); font: inherit; text-align: start; cursor: pointer; }
  .field-options button.highlighted { background: var(--surface-hover); }
  .field-options button[aria-selected=true] { color: var(--accent); }
  .field-options button.grouped { padding-inline-start: 5.2rem; }
  .group-heading { display: flex; align-items: center; gap: 1.2rem; padding: .8rem; color: var(--text-strong); font-weight: var(--ui-label-weight); }
  .group-icon { display: grid; place-items: center; flex: 0 0 3.2rem; width: 3.2rem; height: 3.2rem; border-radius: 50%; background: var(--accent-faint); color: var(--accent); }
  .option-copy { display: flex; flex: 1; flex-direction: column; min-width: 0; gap: .3rem; overflow-wrap: anywhere; }
  .option-hint { color: var(--text-muted); font-size: var(--ui-small-size); font-style: italic; line-height: 1.5; }
  .remote .option-hint { font-style: normal; }
  .no-options { padding: 1rem .8rem; color: var(--text-muted); line-height: 1.6; }
</style>
