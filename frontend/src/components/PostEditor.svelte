<script lang="ts">
  import { logMessage } from './LogMessage.svelte';
  import { untrack } from 'svelte';
  import { Composer, RichTextPlugin, HistoryPlugin, ListPlugin, HeadingNode, QuoteNode, ListNode, ListItemNode, MarkdownShortcutPlugin, BOLD_STAR, ITALIC_STAR, HEADING, QUOTE, UNORDERED_LIST, ORDERED_LIST } from 'svelte-lexical';
  import PostEditorContent from './PostEditorContent.svelte';
  let { body = $bindable(''), html = $bindable(''), editorJSON = $bindable('') } = $props<{ body?: string; html?: string; editorJSON?: string }>();
  let error = $state('');
  const facebookHeading = { ...HEADING, regExp: /^(#{1,2})\s/ };
  const initialConfig = untrack(() => ({
    namespace: 'postika-post', nodes: [HeadingNode, QuoteNode, ListNode, ListItemNode],
    theme: { paragraph: 'post-paragraph', heading: { h1: 'post-h1', h2: 'post-h2' }, quote: 'post-quote', list: { ul: 'post-ul', ol: 'post-ol', listitem: 'post-li' }, text: { bold: 'post-bold', italic: 'post-italic' } },
    editorState: editorJSON || undefined, onError: (cause: Error) => { error = cause.message; }
  }));
</script>
<Composer {initialConfig}>
  <PostEditorContent bind:body bind:html bind:editorJSON />
  <RichTextPlugin /><HistoryPlugin /><ListPlugin />
  <MarkdownShortcutPlugin transformers={[BOLD_STAR, ITALIC_STAR, facebookHeading, QUOTE, UNORDERED_LIST, ORDERED_LIST]} />
</Composer>
{#if error}<p class="editor-error" role="alert">Could not edit the post: {logMessage(error, true)}</p><details><summary>Debug details</summary><p dir="auto">{error}</p></details>{/if}
<style>
  .editor-error { padding: 1rem 1.8rem; color: var(--danger); font-size: 1.3rem; }
</style>
