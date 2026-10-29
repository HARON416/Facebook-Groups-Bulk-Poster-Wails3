<script lang="ts">
  import { onDestroy, tick } from 'svelte';
  import PhosphorIcon from './PhosphorIcon.svelte';
  import { inspectListingVideo } from '../lib/listing-video';

  let { file = $bindable<File | null>(null), busy = $bindable(false), error = $bindable(false), disabled = false, selected = false, thumbnail = '', thumbnailBusy = false, onselect }: { file?: File | null; busy?: boolean; error?: boolean; disabled?: boolean; selected?: boolean; thumbnail?: string; thumbnailBusy?: boolean; onselect?: () => void } = $props();
  let input: HTMLInputElement;
  let root: HTMLDivElement;
  let dragging = $state(false);
  let controller: AbortController | undefined;
  let generation = 0, destroyed = false;
  export async function chooseFiles(files: FileList | File[] | null) {
    if (!files?.length || disabled) return;
    const current = ++generation;
    controller?.abort(); controller = new AbortController();
    error = false; busy = true;
    try {
      if (files.length !== 1) throw new Error('invalid-video');
      const next = files[0];
      await inspectListingVideo(next, controller.signal);
      if (!destroyed && current === generation) file = next;
    } catch { if (!destroyed && current === generation && !controller.signal.aborted) error = true; }
    finally { if (!destroyed && current === generation) busy = false; }
  }
  function cancelInspection() { generation++; controller?.abort(); busy = false; error = false; }
  async function remove() {
    if (disabled) return;
    cancelInspection(); file = null; input.value = '';
    await tick();
    if (!destroyed) root?.querySelector<HTMLButtonElement>('.add-video')?.focus({ preventScroll: true });
  }
  onDestroy(() => { destroyed = true; generation++; controller?.abort(); busy = false; });
</script>

<div class="listing-video" class:dragging bind:this={root} role="group" aria-label="Videos"
  ondragover={(event) => { event.stopPropagation(); if (!disabled) { event.preventDefault(); event.dataTransfer!.dropEffect = 'copy'; dragging = true; } }}
  ondragleave={(event) => { if (!event.currentTarget.contains(event.relatedTarget as Node | null)) dragging = false; }}
  ondrop={(event) => { event.preventDefault(); event.stopPropagation(); dragging = false; void chooseFiles(event.dataTransfer?.files ?? null); }}>
  <input id="listing-video" type="file" accept="video/*" bind:this={input} {disabled} aria-label="Add video" onchange={(event) => { void chooseFiles(event.currentTarget.files); event.currentTarget.value = ''; }} />
  {#if busy || thumbnailBusy}
    <div class="loading-video" role="status"><PhosphorIcon name="video-camera" size={24} /><span>Loading video…</span></div>
    <button class="remove-video" type="button" {disabled} aria-label="Remove video" title="Remove video" onclick={remove}><PhosphorIcon name="x" size={14} /></button>
  {:else if file}
    <button class="video-select" type="button" {disabled} aria-label="Video preview" aria-pressed={selected} onclick={onselect}>
      {#if thumbnail}<img src={thumbnail} alt={file.name} />{:else}<PhosphorIcon name="video-camera" size={28} />{/if}
      <span class="video-mark"><PhosphorIcon name="video-camera" size={20} /></span>
    </button>
    <button class="remove-video" type="button" {disabled} aria-label="Remove video" title="Remove video" onclick={remove}><PhosphorIcon name="x" size={14} /></button>
  {:else}
    <button class="add-video" type="button" {disabled} onclick={() => input.click()}><PhosphorIcon name="video-camera" size={24} /><span>Add video</span><span>(1 minute max)</span></button>
  {/if}
</div>

<style>
  .listing-video { position: relative; min-width: 0; aspect-ratio: 1; border-radius: var(--ui-input-radius); background: var(--surface); }
  .listing-video.dragging { outline: 2px solid var(--focus); outline-offset: 2px; }
  input { display: none; }
  .add-video, .video-select, .loading-video { display: flex; flex-direction: column; align-items: center; justify-content: center; gap: .5rem; width: 100%; height: 100%; min-height: 10rem; padding: .6rem; border: 1px solid var(--border-strong); border-radius: inherit; background: transparent; color: var(--text-muted); font: inherit; font-size: var(--ui-small-size); text-align: center; }
  button { cursor: pointer; }
  .add-video:hover { background: var(--surface-hover); }
  button:focus-visible { outline: 2px solid var(--focus); outline-offset: 2px; }
  button:disabled { opacity: .45; cursor: not-allowed; }
  .video-select { position: absolute; inset: 0; overflow: hidden; padding: 0; }
  .video-select[aria-pressed=true] { border-color: var(--accent); }
  img { width: 100%; height: 100%; object-fit: contain; background: var(--surface-subtle); pointer-events: none; }
  .video-mark { position: absolute; bottom: .6rem; inset-inline-start: .6rem; display: grid; place-items: center; padding: .3rem; border-radius: .4rem; background: var(--bg); color: var(--text); }
  .remove-video { position: absolute; top: .6rem; inset-inline-end: .6rem; display: grid; place-items: center; width: 2.8rem; height: 2.8rem; padding: 0; border: 1px solid var(--border); border-radius: 50%; background: var(--bg); color: var(--text); }
  .remove-video:hover:not(:disabled) { background: var(--surface-hover); }
</style>
