<script lang="ts">
  import { onDestroy, tick } from 'svelte';
  import { logMessage } from './LogMessage.svelte';
  import LocalizedFeedback from './LocalizedFeedback.svelte';
  import PostEditor from './PostEditor.svelte';
  import FacebookPageDialog from './FacebookPageDialog.svelte';
  import FetchedPostCard from './FetchedPostCard.svelte';
  import { reorder } from '../lib/reorder';
  import { snapshotImage, type DraftImage } from '../lib/drafts';
  import type { FacebookPagePost } from '../../bindings/postika/models';

  export interface PostImage extends DraftImage {
    url: string;
  }

  interface Props {
    profileID: string;
    message: string | string[];
    body: string;
    bodyHTML: string;
    bodyState: string;
    images: PostImage[];
    fetchedPosts?: FacebookPagePost[] | null;
  }

  let { profileID, message = $bindable(), body = $bindable(), bodyHTML = $bindable(), bodyState = $bindable(), images = $bindable(), fetchedPosts = $bindable(null) }: Props = $props();
  let fileInput = $state<HTMLInputElement>();
  let entranceBatch = $state(0);
  let composer = $state<HTMLDivElement>();

  async function deleteFetchedPost(id: string) {
    if (!fetchedPosts) return;
    const index = fetchedPosts.findIndex(post => post.id === id);
    if (index < 0) return;
    const remaining = fetchedPosts.filter(post => post.id !== id);
    fetchedPosts = remaining.length ? remaining : null;
    await tick();
    if (!remaining.length) {
      const pane = composer?.closest<HTMLElement>('[data-step-scroll]');
      pane?.querySelector<HTMLElement>('#step-heading')?.focus({ preventScroll: true });
      pane?.scrollTo({ top: 0, behavior: 'instant' });
    }
  }

  let draggedId = $state<string | null>(null);
  let dropTarget = $state<string | null>(null);
  let dropSide = $state<'before' | 'after'>('before');
  let reorderMessage = $state('');
  let dragPreview = $state<HTMLDivElement>();
  let preview = $state<{ url: string; size: number; x: number; y: number } | null>(null);
  let frame = 0;
  let pointer: {
    id: number; imageId: string; button: HTMLButtonElement;
    startX: number; startY: number; x: number; y: number;
    offsetX: number; offsetY: number; size: number;
  } | null = null;

  onDestroy(resetDrag);
  const acceptedTypes = new Set(['image/jpeg', 'image/png', 'image/webp', 'image/gif']);

  async function addImages(files: FileList | File[] | null) {
    if (!files) return;
    const rejected: string[] = [];
    const unreadable: string[] = [];
    for (const file of Array.from(files)) {
      if (!acceptedTypes.has(file.type)) {
        rejected.push(file.name);
        continue;
      }
      if (images.some((image) => !image.unavailable && image.file.name === file.name &&
        image.file.size === file.size && image.file.lastModified === file.lastModified)) continue;
      try {
        const copy = await snapshotImage(file);
        const replacement = images.findIndex(image => image.unavailable && image.file.name === file.name);
        const image = { id: replacement < 0 ? crypto.randomUUID() : images[replacement].id, file: copy, url: URL.createObjectURL(copy) };
        images = replacement < 0 ? [...images, image] : images.map((entry, i) => i === replacement ? image : entry);
      } catch { unreadable.push(file.name); }
    }
    message = [
      ...(rejected.length ? [/* @wc-ignore */ `Unsupported files: ${rejected.join(', ')}. Choose JPG, PNG, WebP, or GIF images.`] : []),
      ...(unreadable.length ? [/* @wc-ignore */ `Could not read ${unreadable.join(', ')}`] : []),
    ];
  }


  function removeImage(id: string) {
    resetDrag();
    const image = images.find((image) => image.id === id);
    if (image) URL.revokeObjectURL(image.url);
    images = images.filter((image) => image.id !== id);
  }

  function imageFailed(id: string) {
    const image = images.find((image) => image.id === id);
    if (image) message = /* @wc-ignore */ `Could not preview ${image.file.name}. Please choose another image.`;
    removeImage(id);
  }

  function resetDrag() {
    cancelAnimationFrame(frame);
    frame = 0;
    const previous = pointer;
    pointer = null;
    if (previous?.button.hasPointerCapture(previous.id)) {
      previous.button.releasePointerCapture(previous.id);
    }
    draggedId = null;
    dropTarget = null;
    preview = null;
  }

  function startPointer(event: PointerEvent, imageId: string) {
    if (!event.isPrimary || event.button !== 0 || images.length < 2) return;
    if (images.find(image => image.id === imageId)?.unavailable) return;
    resetDrag();
    const button = event.currentTarget as HTMLButtonElement;
    const bounds = button.getBoundingClientRect();
    pointer = {
      id: event.pointerId, imageId, button,
      startX: event.clientX, startY: event.clientY,
      x: event.clientX, y: event.clientY,
      offsetX: event.clientX - bounds.left, offsetY: event.clientY - bounds.top,
      size: bounds.width,
    };
    button.setPointerCapture(event.pointerId);
  }

  function updateTarget() {
    if (!pointer || !draggedId) return;
    const tile = document.elementFromPoint(pointer.x, pointer.y)?.closest<HTMLElement>('[data-image-target]');
    const targetId = tile?.dataset.imageTarget;
    if (!tile || !targetId || targetId === draggedId) {
      dropTarget = null;
      return;
    }
    const bounds = tile.getBoundingClientRect();
    dropTarget = targetId;
    dropSide = tile.dataset.imageEnd !== undefined || pointer.x > bounds.left + bounds.width / 2 ? 'after' : 'before';
  }

  function renderDrag() {
    if (!pointer || !draggedId) return;
    if (dragPreview) {
      dragPreview.style.transform = `translate3d(${pointer.x - pointer.offsetX}px, ${pointer.y - pointer.offsetY}px, 0)`;
    }
    // Scroll the step content while keeping the progress header in place.
    const edge = 64;
    const scroller = pointer.button.closest<HTMLElement>('[data-step-scroll]');
    if (scroller) {
      const bounds = scroller.getBoundingClientRect();
      const inside = pointer.x >= bounds.left && pointer.x <= bounds.right &&
        pointer.y >= bounds.top && pointer.y <= bounds.bottom;
      const scroll = !inside ? 0 : pointer.y < bounds.top + edge ? -10 : pointer.y > bounds.bottom - edge ? 10 : 0;
      if (scroll) scroller.scrollBy({ top: scroll, behavior: 'instant' });
    }
    updateTarget();
    frame = requestAnimationFrame(renderDrag);
  }

  function movePointer(event: PointerEvent) {
    if (!pointer || pointer.id !== event.pointerId) return;
    pointer.x = event.clientX;
    pointer.y = event.clientY;
    if (!draggedId && Math.hypot(pointer.x - pointer.startX, pointer.y - pointer.startY) >= 5) {
      const image = images.find((image) => image.id === pointer?.imageId);
      if (!image) return resetDrag();
      draggedId = image.id;
      preview = {
        url: image.url, size: pointer.size,
        x: pointer.x - pointer.offsetX, y: pointer.y - pointer.offsetY,
      };
      frame = requestAnimationFrame(renderDrag);
    }
    if (draggedId) event.preventDefault();
  }

  function announceMove(id: string) {
    const index = images.findIndex((image) => image.id === id);
    if (index >= 0) reorderMessage = /* @wc-ignore */ `${images[index].file.name} moved to position ${index + 1} of ${images.length}.`;
  }

  async function endPointer(event: PointerEvent) {
    if (!pointer || pointer.id !== event.pointerId) return;
    const button = pointer.button;
    pointer.x = event.clientX;
    pointer.y = event.clientY;
    updateTarget();
    if (draggedId && dropTarget) {
      images = reorder(images, draggedId, dropTarget, dropSide);
      announceMove(draggedId);
    }
    resetDrag();
    await tick();
    button.focus({ preventScroll: true });
  }

  async function keyboardMove(event: KeyboardEvent, id: string) {
    if (!event.altKey || !['ArrowLeft', 'ArrowRight'].includes(event.key)) return;
    event.preventDefault();
    const direction = event.key === 'ArrowRight' ? 1 : -1;
    const index = images.findIndex((image) => image.id === id);
    const target = images[index + direction];
    if (!target) return;
    const button = event.currentTarget as HTMLButtonElement;
    images = reorder(images, id, target.id, direction === 1 ? 'after' : 'before');
    announceMove(id);
    await tick();
    button.focus();
  }

</script>

<svelte:window
  onpointermove={movePointer}
  onpointerup={endPointer}
  onpointercancel={resetDrag}
  onblur={resetDrag}
  onkeydown={(event) => { if (event.key === 'Escape') resetDrag(); }}
/>

<div bind:this={composer} class="composer">
  {#if fetchedPosts === null}
  <div class="field">
    <div class="field-heading">
      <span class="field-label">Post body</span>
      <span>Required</span>
    </div>
    <div class="editor">
      <PostEditor bind:body bind:html={bodyHTML} bind:editorJSON={bodyState} />
    </div>
  </div>

  <div class="field image-drop-zone" id={`post-images-${profileID}`} data-file-drop-target>
    <div class="field-heading">
      <span class="field-label" id="images-label">Images</span>
      <span>Required</span>
    </div>
    <input
      bind:this={fileInput}
      class="file-input"
      type="file"
      accept="image/jpeg,image/png,image/webp,image/gif"
      multiple
      aria-label="Choose images"
      onchange={(event) => { addImages(event.currentTarget.files); event.currentTarget.value = ''; }}
    />
    {#if images.length === 0}
    <button
      type="button"
      class="image-selector"
      aria-labelledby="images-label browse-instruction"
      aria-describedby="image-formats"
      onclick={() => fileInput?.click()}
    >
      <span class="image-icon" aria-hidden="true">
        <svg viewBox="0 0 24 24" fill="none">
          <rect x="3" y="3" width="18" height="18" rx="3" />
          <circle cx="8.5" cy="8.5" r="1.5" />
          <path d="m21 15-5-5L5 21" />
        </svg>
      </span>
      <span id="browse-instruction">Drop images here or <span class="browse">browse images</span></span>
      <span id="image-formats" class="formats">JPG, PNG, WebP, or GIF · Select multiple images</span>
    </button>
    {/if}
    <div class="file-message" role="status">{#if Array.isArray(message)}{#each message as item, index}{#if index}<br />{/if}<LocalizedFeedback message={item} technical={false} />{/each}{:else}{logMessage(message)}{/if}</div>

    {#if images.length}
      <div class="preview-heading">
        <span>{images.length === 1 ? /* @wc-include */ '1 image selected' : `${images.length} images selected`}</span>
        <button type="button" class="add-images" onclick={() => fileInput?.click()}>Add more</button>
      </div>
      <p class="drop-help">Drop more images here to add them.</p>
      {#if images.length > 1}
        <p class="reorder-help" id="reorder-help">Drag images to rearrange. Or focus an image and use Alt + Left / Right.</p>
      {/if}
      <p class="sr-only" role="status">{logMessage(reorderMessage)}</p>
      <ul class="previews" aria-label="Selected images">
        {#each images as image (image.id)}
          <li
            data-image-target={image.id}
            class:drag-source={draggedId === image.id}
            class:drop-before={dropTarget === image.id && dropSide === 'before'}
            class:drop-after={dropTarget === image.id && dropSide === 'after'}
          >
            <div class="thumbnail">
              <button
                type="button"
                class="image-drag"
                class:unavailable={image.unavailable}
                aria-label={`${image.file.name}, image ${images.indexOf(image) + 1} of ${images.length}`}
                aria-describedby={images.length > 1 ? 'reorder-help' : undefined}
                onpointerdown={(event) => startPointer(event, image.id)}
                onlostpointercapture={() => { if (pointer) resetDrag(); }}
                ondragstart={(event) => event.preventDefault()}
                onkeydown={(event) => keyboardMove(event, image.id)}
                onclick={() => { if (image.unavailable) fileInput?.click(); }}
              >
                {#if image.unavailable}<span class="unavailable-image">Could not read {image.file.name}</span>{:else}<img src={image.url} alt={image.file.name} draggable="false" onerror={() => imageFailed(image.id)} />{/if}
              </button>
              <button type="button" class="remove-image" aria-label={`Remove ${image.file.name}`} onclick={() => removeImage(image.id)}>
                <svg viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="m6 6 12 12M18 6 6 18" /></svg>
              </button>
            </div>
          </li>
        {/each}
        {#each Array(Math.max(0, 4 - images.length)) as _}
          <li
            class="preview-placeholder"
            aria-hidden="true"
            data-image-target={images[images.length - 1].id}
            data-image-end
          ></li>
        {/each}
      </ul>
    {/if}
  </div>
  {/if}
  <div class="facebook-source" class:has-posts={fetchedPosts !== null}>
    <FacebookPageDialog {profileID} onfetched={(posts) => { resetDrag(); entranceBatch++; fetchedPosts = posts; }} />
  </div>
  {#if fetchedPosts !== null}
    <section class="fetched-section" aria-labelledby={`fetched-posts-heading-${profileID}`}>
      <div class="fetched-heading">
        <h2 id={`fetched-posts-heading-${profileID}`}>Fetched posts <span>{fetchedPosts.length}</span></h2>
        <button type="button" class="write-instead" onclick={() => { fetchedPosts = null; }}>Write a post instead</button>
      </div>
      {#if fetchedPosts.length}
        {#key entranceBatch}
        <ol class="fetched-posts" aria-label="Fetched posts">
          {#each fetchedPosts as post, index (post.id)}
            <li class="fetched-post-card">
              <FetchedPostCard {post} {index} onsave={(updated) => { fetchedPosts = fetchedPosts!.map(entry => entry.id === updated.id ? updated : entry); }} ondelete={() => deleteFetchedPost(post.id)} />
            </li>
          {/each}
        </ol>
        {/key}
      {:else}
        <p class="no-fetched-posts">No posts to show. Fetch posts or write a post instead.</p>
      {/if}
    </section>
  {/if}
</div>

{#if preview}
  <div
    bind:this={dragPreview}
    class="drag-preview"
    aria-hidden="true"
    style:width={`${preview.size}px`}
    style:height={`${preview.size}px`}
    style:transform={`translate3d(${preview.x}px, ${preview.y}px, 0)`}
  >
    <img src={preview.url} alt="" draggable="false" />
  </div>
{/if}

<style>
  .unavailable-image { display: flex; height: 100%; align-items: center; padding: 1rem; border: 1px dashed var(--danger); border-radius: .7rem; color: var(--danger); background: var(--danger-bg); font-size: 1.1rem; line-height: 1.5; overflow-wrap: anywhere; }
  .composer { display: grid; gap: 2.8rem; }
  .facebook-source { display: grid; margin-top: -1.2rem; }
  .facebook-source.has-posts { margin-top: 0; }
  .fetched-section { min-width: 0; }
  .fetched-heading { display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 1rem; margin-bottom: var(--ui-space); }
  .fetched-heading h2 { color: var(--text-strong); font-size: var(--ui-title-size); font-weight: var(--ui-title-weight); line-height: 1.5; }
  .fetched-heading h2 span { margin-inline-start: .8rem; color: var(--text-muted); font-size: var(--ui-small-size); font-weight: 400; }
  .write-instead { padding: .4rem 0; border: 0; background: transparent; color: var(--accent); font-size: var(--ui-small-size); cursor: pointer; }
  .write-instead:hover { text-decoration: underline; }
  .fetched-posts { display: grid; gap: var(--ui-space); list-style: none; }
  .fetched-post-card { min-width: 0; }
  .no-fetched-posts { color: var(--text-secondary); font-size: var(--ui-body-size); line-height: 1.6; }
  .image-drop-zone { border-radius: 1rem; transition: background-color 0.2s, outline-color 0.2s; }
  .image-drop-zone:global(.file-drop-target-active) { outline: 0.2rem dashed var(--accent); outline-offset: 0.7rem; background: var(--surface-active); }
  .image-drop-zone:global(.file-drop-target-active) .image-selector { border-color: var(--accent); background: var(--surface-active); }
  .drop-help { color: var(--text-muted); font-size: 1.1rem; margin-bottom: 1rem; }
  .field-heading, .preview-heading { display: flex; align-items: center; justify-content: space-between; gap: 1.2rem; }
  .field-heading { margin-bottom: 1rem; font-size: 1.3rem; color: var(--text-muted); }
  .field-label { font-size: 1.4rem; font-weight: 550; color: var(--text); }
  .editor { overflow: hidden; border: 0.2rem solid var(--border-strong); border-radius: 1rem; background: var(--surface); transition: border-color 0.2s, box-shadow 0.2s; }
  .editor:focus-within { border-color: var(--accent); box-shadow: 0 0 0 0.3rem var(--accent-faint); }
  .file-input { display: none; }
  .image-selector { display: flex; width: 100%; min-height: 15rem; padding: 2.4rem; align-items: center; justify-content: center; flex-direction: column; gap: 0.9rem; border: 0.2rem dashed var(--border-strong); border-radius: 1rem; background: var(--surface); color: var(--text-secondary); cursor: pointer; font-size: 1.3rem; transition: border-color 0.2s, background-color 0.2s; }
  .image-selector:hover { border-color: var(--accent); background: var(--surface-active); }
  button:focus-visible { outline: 0.2rem solid var(--accent); outline-offset: 0.4rem; }
  .image-icon { display: grid; place-items: center; width: 3.6rem; height: 3.6rem; margin-bottom: 0.3rem; border-radius: 50%; background: var(--surface-active); color: var(--accent); }
  svg { width: 1.8rem; height: 1.8rem; stroke: currentColor; stroke-width: 1.7; stroke-linecap: round; stroke-linejoin: round; }
  .browse { color: var(--accent); font-weight: 600; }
  .formats { color: var(--text-muted); font-size: 1.1rem; }
  .file-message { color: var(--danger); font-size: 1.2rem; line-height: 1.5; }
  .file-message:not(:empty) { margin-top: 1rem; }
  .preview-heading { margin: 1.6rem 0 1rem; font-size: 1.2rem; color: var(--text-muted); }
  .add-images { border: 0; background: transparent; color: var(--accent); font-size: 1.2rem; font-weight: 550; cursor: pointer; }
  .previews { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 1.6rem; list-style: none; }
  .previews li { position: relative; min-width: 0; }
  .previews li.drag-source { opacity: 0.45; }
  .previews li.drop-before::before, .previews li.drop-after::after { position: absolute; top: 0; bottom: 0; width: 0.3rem; border-radius: 0.2rem; background: var(--accent); content: ''; }
  .previews li.drop-before::before { left: -0.95rem; }
  .previews li.drop-after::after { right: -0.95rem; }
  .image-drag { display: block; width: 100%; height: 100%; border: 0; background: transparent; cursor: grab; touch-action: none; user-select: none; }
  .drag-preview { position: fixed; top: 0; left: 0; z-index: 1000; overflow: hidden; border-radius: 1.2rem; pointer-events: none; will-change: transform; box-shadow: 0 1rem 3rem var(--shadow-strong); }
  .image-drag:active { cursor: grabbing; }
  .image-drag.unavailable { cursor: pointer; }
  .image-drag:focus-visible { outline-offset: -0.4rem; }
  .reorder-help { margin: 0 0 1.2rem; color: var(--text-muted); font-size: 1.2rem; line-height: 1.5; }
  .sr-only { position: absolute; width: 0.1rem; height: 0.1rem; overflow: hidden; clip-path: inset(50%); white-space: nowrap; }
  .thumbnail { position: relative; aspect-ratio: 1; overflow: hidden; border-radius: 1.2rem; background: var(--surface-hover); }
  .preview-placeholder { aspect-ratio: 1; border-radius: 1.2rem; background: var(--surface-hover); }
  img { display: block; width: 100%; height: 100%; object-fit: cover; }
  .remove-image { position: absolute; top: 0.8rem; right: 0.8rem; display: grid; place-items: center; width: 2.8rem; height: 2.8rem; border: 0; border-radius: 0.5rem; background: var(--image-overlay); backdrop-filter: blur(0.6rem); color: var(--on-image); cursor: pointer; transition: background-color 0.2s; }
  .remove-image:hover, .remove-image:focus-visible { background: var(--danger-solid); }
  .remove-image:focus-visible { outline-offset: -0.3rem; }
  @media (max-width: 540px) {
    .image-selector { padding: 2rem 1.2rem; }
    .formats { text-align: center; }
    .previews { grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 1.2rem; }
  }
  @media (prefers-reduced-motion: reduce) { .editor, .image-selector, .remove-image { transition: none; } }
</style>
