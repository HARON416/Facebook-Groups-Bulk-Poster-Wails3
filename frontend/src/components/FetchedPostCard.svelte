<script lang="ts">
  import 'animate.css';
  import { onDestroy, onMount, tick } from 'svelte';
  import type { FacebookPagePost } from '../../bindings/postika/models';

  let { post, index, onsave, ondelete }: {
    post: FacebookPagePost;
    index: number;
    onsave: (post: FacebookPagePost) => void;
    ondelete: () => void;
  } = $props();
  let editing = $state(false);
  let text = $state('');
  let images = $state<string[]>([]);
  let textarea = $state<HTMLTextAreaElement>();
  let editButton = $state<HTMLButtonElement>();
  let card = $state<HTMLElement>();
  let deleting = $state(false);
  let disposed = false;
  onDestroy(() => { disposed = true; });
  onMount(() => { void enter(); });

  async function enter() {
    if (!card || window.matchMedia('(prefers-reduced-motion: reduce)').matches) return;
    const element = card;
    element.style.animationDelay = `${index * 80}ms`;
    element.classList.add('animate__animated', 'animate__slideInLeft');
    await Promise.allSettled(element.getAnimations().map(animation => animation.finished));
    element.classList.remove('animate__slideInLeft');
    if (!deleting) element.classList.remove('animate__animated');
    element.style.removeProperty('animation-delay');
  }

  async function remove() {
    if (deleting || !card) return;
    const original = post;
    const element = card;
    deleting = true;
    // A delete during a delayed entrance starts its exit immediately.
    element.classList.remove('animate__slideInLeft');
    element.style.removeProperty('animation-delay');
    if (!window.matchMedia('(prefers-reduced-motion: reduce)').matches) {
      element.classList.add('animate__animated', 'animate__fadeOutLeft');
      // Wait for the actual CSS animation, rather than a matching timeout.
      // Cancellation/unmount also settles the promise without removing a new card.
      await Promise.allSettled(element.getAnimations().map(animation => animation.finished));
    }
    element.classList.remove('animate__animated', 'animate__fadeOutLeft');
    if (!disposed && post === original) ondelete();
    deleting = false;
  }

  async function edit() {
    text = post.text;
    images = [...post.images ?? []];
    editing = true;
    await tick();
    textarea?.focus();
  }

  async function finish(save: boolean) {
    if (save) onsave({ ...post, text, images: [...images] });
    editing = false;
    await tick();
    editButton?.focus();
  }
</script>

<article bind:this={card} class="post-card" aria-label={`Post ${index + 1}`} aria-busy={deleting}>
  <header>
    <div class="card-actions">
      {#if !editing}
        <button bind:this={editButton} type="button" class="card-action" disabled={deleting} aria-label={`Edit post ${index + 1}`} onclick={edit}>Edit</button>
      {/if}
      <button type="button" class="card-action delete" disabled={deleting} aria-label={`Delete post ${index + 1}`} onclick={remove}>Delete</button>
    </div>
  </header>
  {#if editing}
    <form onsubmit={(event) => { event.preventDefault(); void finish(true); }}>
      <label>
        <span>Post text</span>
        <textarea bind:this={textarea} class="ui-input" bind:value={text} disabled={deleting} rows="6"></textarea>
      </label>
      {#if images.length}
        <div class="post-images">
          {#each images as source, imageIndex}
            <div class="image-preview">
              <img src={source} alt="" loading="lazy" referrerpolicy="no-referrer" />
              <button type="button" class="remove-image" disabled={deleting} aria-label={`Remove image ${imageIndex + 1} from post ${index + 1}`} onclick={() => { images = images.filter((_, i) => i !== imageIndex); }}>
                <svg viewBox="0 0 24 24" aria-hidden="true"><path d="m6 6 12 12M18 6 6 18" /></svg>
              </button>
            </div>
          {/each}
        </div>
      {/if}
      <footer>
        <button type="button" class="action-button action-button--secondary" disabled={deleting} onclick={() => finish(false)}>Cancel</button>
        <button type="submit" class="action-button action-button--primary" disabled={deleting}>Save changes</button>
      </footer>
    </form>
  {:else}
    <p class="post-text">{post.text || /* @wc-include */ 'Post without text'}</p>
    {#if post.images?.length}
      <div class="post-images">
        {#each post.images as source}<img src={source} alt="" loading="lazy" referrerpolicy="no-referrer" />{/each}
      </div>
    {/if}
  {/if}
</article>

<style>
  .post-card { --animate-duration: 400ms; min-width: 0; padding: var(--ui-space); border: 1px solid var(--border); border-radius: var(--ui-panel-radius); background: var(--surface); }
  button:disabled { opacity: .45; cursor: wait; }
  header { display: flex; align-items: center; justify-content: flex-end; flex-wrap: wrap; gap: 1rem; margin-bottom: 1rem; }
  .card-actions { display: flex; flex-wrap: wrap; gap: .6rem; }
  .card-action { min-height: 3.2rem; padding: .5rem .9rem; border: 1px solid var(--border); border-radius: var(--ui-input-radius); background: transparent; color: var(--accent); font-size: var(--ui-small-size); cursor: pointer; }
  .card-action:hover { background: var(--surface-hover); }
  .delete { color: var(--danger); }
  .delete:hover { border-color: var(--danger-border); background: var(--danger-bg); }
  .post-text { color: var(--text); font-size: var(--ui-body-size); white-space: pre-wrap; overflow-wrap: anywhere; line-height: 1.6; }
  label { display: grid; gap: var(--ui-field-gap); color: var(--text); font-size: var(--ui-body-size); font-weight: var(--ui-label-weight); }
  textarea { resize: vertical; min-height: 12rem; line-height: 1.6; font-weight: 400; }
  .post-images { display: grid; grid-template-columns: repeat(auto-fill, minmax(min(10rem, 100%), 1fr)); gap: .8rem; margin-top: var(--ui-space); }
  .post-images img { display: block; width: 100%; aspect-ratio: 1; border-radius: var(--ui-input-radius); object-fit: cover; }
  .image-preview { position: relative; min-width: 0; }
  .remove-image { position: absolute; inset-block-start: .5rem; inset-inline-end: .5rem; display: grid; place-items: center; width: 2.8rem; height: 2.8rem; border: 1px solid var(--border); border-radius: .5rem; background: var(--surface); color: var(--danger); cursor: pointer; }
  .remove-image:hover { background: var(--danger-bg); border-color: var(--danger-border); }
  svg { width: 1.6rem; height: 1.6rem; fill: none; stroke: currentColor; stroke-width: 1.5; stroke-linecap: round; }
  footer { display: flex; justify-content: flex-end; flex-wrap: wrap; gap: 1rem; margin-top: var(--ui-space); }
  button:focus-visible { outline: 2px solid var(--focus); outline-offset: 2px; }
</style>
