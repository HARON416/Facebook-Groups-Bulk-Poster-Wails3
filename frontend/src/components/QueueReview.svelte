<script lang="ts">
  import type { AccountProfile, FacebookPagePost } from '../../bindings/postika';
  import { accountLabel } from './AccountLabel.svelte';

  let { profile, body, imageCount, fetchedPosts, groups, disabled = false, oneditpost, oneditgroups } = $props<{
    profile: AccountProfile;
    body: string;
    imageCount: number;
    fetchedPosts: FacebookPagePost[] | null;
    groups: { id: string; name: string }[];
    disabled?: boolean;
    oneditpost: () => void;
    oneditgroups: () => void;
  }>();
</script>

<section class="queue-review" aria-label="Review your queue">
  <h2>Review your queue</h2>
  <p class="account">Account: {accountLabel(profile)}</p>
  <div class="review-part">
    <div class="review-heading"><h3>Post text</h3><button type="button" {disabled} aria-label="Create post" onclick={oneditpost}>Edit</button></div>
    {#if fetchedPosts !== null}
      <details>
        <summary>Fetched posts <span>({fetchedPosts.length})</span></summary>
        <ol class="review-list">
          {#each fetchedPosts as post (post.id)}
            <li><p class="post-text" dir="auto">{post.text}</p><dl class="image-count"><dt>Images</dt><dd>{(post.images ?? []).length}</dd></dl></li>
          {/each}
        </ol>
      </details>
    {:else}
      <p class="post-text" dir="auto">{body}</p>
      <dl class="image-count"><dt>Images</dt><dd>{imageCount}</dd></dl>
    {/if}
  </div>
  <div class="review-part">
    <div class="review-heading"><h3>Groups</h3><button type="button" {disabled} aria-label="Select groups" onclick={oneditgroups}>Edit</button></div>
    <details>
      <summary>{groups.length} selected</summary>
      <ul class="review-list">{#each groups as group (group.id)}<li><bdi>{group.name}</bdi></li>{/each}</ul>
    </details>
  </div>
  <p class="queue-note">Keep Postika open and your computer awake. Queues and logs are not restored after quitting.</p>
</section>

<style>
  .queue-review { margin-block: 2rem 2.4rem; color: var(--text); font-size: var(--ui-body-size); line-height: 1.6; overflow-wrap: anywhere; }
  h2 { font-size: var(--ui-title-size); font-weight: var(--ui-title-weight); color: var(--text-strong); }
  .account { margin-top: .6rem; color: var(--text-secondary); }
  .review-part { margin-top: 1.6rem; padding-top: 1.2rem; border-top: 1px solid var(--border); }
  .review-heading { display: flex; align-items: center; justify-content: space-between; gap: 1.2rem; margin-bottom: .6rem; }
  h3 { font-size: var(--ui-body-size); font-weight: var(--ui-label-weight); }
  button { padding: .5rem .8rem; border: 1px solid var(--border); border-radius: var(--ui-input-radius); background: var(--surface); color: var(--accent); font: inherit; cursor: pointer; }
  button:hover:not(:disabled) { background: var(--surface-hover); }
  button:disabled { opacity: .6; cursor: default; }
  button:focus-visible, summary:focus-visible { outline: 2px solid var(--focus); outline-offset: 2px; }
  summary { width: fit-content; max-width: 100%; cursor: pointer; color: var(--text-secondary); }
  .review-list { max-height: 20rem; overflow-y: auto; overscroll-behavior: contain; margin-top: .8rem; padding-inline-start: 2rem; }
  li + li { margin-top: .6rem; }
  .post-text { white-space: pre-wrap; }
  .image-count { display: flex; flex-wrap: wrap; gap: .6rem; margin-top: .4rem; color: var(--text-secondary); }
  .queue-note { margin-top: 1.6rem; color: var(--text-secondary); }
</style>
