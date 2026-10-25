<script lang="ts">
  import { getLocale } from '../lib/i18n.svelte';
  import { onMount } from 'svelte';
  import EditorIcon from './EditorIcon.svelte';
  import { ContentEditable, getEditor } from 'svelte-lexical';
  import { $getRoot as getRoot, $getSelection as getSelection, $isRangeSelection as isRangeSelection, $createParagraphNode as createParagraphNode, $createTextNode as createTextNode, FORMAT_TEXT_COMMAND, FORMAT_ELEMENT_COMMAND, HISTORY_PUSH_TAG, UNDO_COMMAND, REDO_COMMAND, CAN_UNDO_COMMAND, CAN_REDO_COMMAND, COMMAND_PRIORITY_LOW, COMMAND_PRIORITY_HIGH, PASTE_COMMAND, type TextFormatType } from 'lexical';
  import { $createHeadingNode as createHeadingNode, $createQuoteNode as createQuoteNode } from '@lexical/rich-text';
  import { $setBlocksType as setBlocksType } from '@lexical/selection';
  import { $isListNode as isListNode } from '@lexical/list';
  import { $generateHtmlFromNodes as generateHtmlFromNodes, $generateNodesFromDOM as generateNodesFromDOM } from '@lexical/html';
  import { applyPostList, removePostList } from '../lib/post-lists';
  import { cleanPostHTML } from '../lib/post-format';
  let { body = $bindable(''), html = $bindable(''), editorJSON = $bindable('') } = $props<{ body?: string; html?: string; editorJSON?: string }>();
  const editor = getEditor();
  let bold = $state(false), italic = $state(false), block = $state('paragraph'), canUndo = $state(false), canRedo = $state(false);
  function format(type: TextFormatType) { editor.dispatchCommand(FORMAT_TEXT_COMMAND, type); editor.focus(); }
  function setBlock(value: string) {
    if (value === 'bullet' || value === 'number') editor.update(() => {
      applyPostList(value);
    }, { tag: HISTORY_PUSH_TAG });
    else {
      editor.update(() => { removePostList(); const selection = getSelection(); if (isRangeSelection(selection)) setBlocksType(selection, () => value === 'quote' ? createQuoteNode() : value === 'h1' || value === 'h2' ? createHeadingNode(value) : createParagraphNode()); }, { tag: HISTORY_PUSH_TAG });
    }
    editor.focus();
  }
  onMount(() => {
    const cleanup = [editor.registerUpdateListener(({ editorState }) => {
      editorState.read(() => {
        body = getRoot().getTextContent();
        html = cleanPostHTML(generateHtmlFromNodes(editor));
        editorJSON = JSON.stringify(editorState.toJSON());
        const selection = getSelection();
        if (isRangeSelection(selection)) {
          bold = selection.hasFormat('bold'); italic = selection.hasFormat('italic');
          const node = selection.anchor.getNode().getTopLevelElement();
          block = node && isListNode(node) ? node.getListType() : node?.getType() === 'heading' ? (node as import('@lexical/rich-text').HeadingNode).getTag() : node?.getType() ?? 'paragraph';
        }
      });
    }), editor.registerCommand(FORMAT_TEXT_COMMAND, value => value !== 'bold' && value !== 'italic', COMMAND_PRIORITY_HIGH), editor.registerCommand(FORMAT_ELEMENT_COMMAND, () => true, COMMAND_PRIORITY_HIGH), editor.registerCommand(CAN_UNDO_COMMAND, value => { canUndo = value; return false; }, COMMAND_PRIORITY_LOW), editor.registerCommand(CAN_REDO_COMMAND, value => { canRedo = value; return false; }, COMMAND_PRIORITY_LOW), editor.registerCommand(PASTE_COMMAND, event => {
      const clipboard = event instanceof ClipboardEvent ? event.clipboardData : null;
      const pasted = clipboard?.getData('text/html');
      if (!clipboard || !event) return false;
      const selection = getSelection();
      if (!isRangeSelection(selection)) return false;
      event.preventDefault();
      if (pasted) {
        const document = new DOMParser().parseFromString(cleanPostHTML(pasted), 'text/html');
        selection.insertNodes(generateNodesFromDOM(editor, document));
      } else selection.insertRawText(clipboard.getData('text/plain'));
      return true;
    }, COMMAND_PRIORITY_HIGH)];
    if (!editorJSON && body) editor.update(() => { const paragraph = createParagraphNode(); paragraph.append(createTextNode(body)); getRoot().clear().append(paragraph); });
    return () => cleanup.forEach(dispose => dispose());
  });
</script>

<div class="toolbar" aria-label="Post editor controls">
  <div class="formatting" role="toolbar" aria-label="Text formatting">
    <button type="button" title="Bold (Ctrl+B)" aria-label="Bold" aria-pressed={bold} onmousedown={(event) => event.preventDefault()} onclick={() => format('bold')}><EditorIcon name="text-b" /></button>
    <button type="button" title="Italic (Ctrl+I)" aria-label="Italic" aria-pressed={italic} onmousedown={(event) => event.preventDefault()} onclick={() => format('italic')}><EditorIcon name="text-italic" /></button>
    <span class="divider"></span>
    {#each [{ type: 'h1', label: 'Heading 1', icon: 'text-h-one' }, { type: 'h2', label: 'Heading 2', icon: 'text-h-two' }, { type: 'quote', label: 'Quote', icon: 'quotes' }, { type: 'bullet', label: 'Bullet list', icon: 'list-bullets' }, { type: 'number', label: 'Numbered list', icon: 'list-numbers' }] as const as style}
      <button type="button" title={style.label} aria-label={style.label} aria-pressed={block === style.type} onmousedown={(event) => event.preventDefault()} onclick={() => setBlock(block === style.type ? 'paragraph' : style.type)}><EditorIcon name={style.icon} /></button>
    {/each}
    <span class="divider"></span>
    <button type="button" title="Undo (Ctrl+Z)" aria-label="Undo" disabled={!canUndo} onmousedown={(event) => event.preventDefault()} onclick={() => editor.dispatchCommand(UNDO_COMMAND, undefined)}><EditorIcon name="arrow-counter-clockwise" /></button>
    <button type="button" title="Redo (Ctrl+Shift+Z)" aria-label="Redo" disabled={!canRedo} onmousedown={(event) => event.preventDefault()} onclick={() => editor.dispatchCommand(REDO_COMMAND, undefined)}><EditorIcon name="arrow-clockwise" /></button>
  </div>
</div>
<div class="writing" dir="auto">
  <ContentEditable id="post-body" className="post-editable post-rich-content" ariaLabel="Post body" ariaMultiline={true} ariaRequired={true} />
  {#if !body}<span class="placeholder">What would you like to share?</span>{/if}
</div>
<div class="editor-footer"><span>Bold, italic, headings, quotes, and lists · Markdown shortcuts supported</span><span>{body.length.toLocaleString(getLocale())} characters</span></div>

<style>
  .toolbar { display: flex; align-items: center; flex-wrap: wrap; gap: .8rem; padding: .8rem 1rem; border-bottom: 1px solid var(--border); background: var(--surface-subtle); }
  .formatting { display: flex; align-items: center; gap: .4rem; }
  .formatting { flex-wrap: wrap; }
  button { min-height: 3.4rem; border: 0; border-radius: .5rem; background: transparent; color: var(--text-secondary); font-size: var(--ui-body-size); cursor: pointer; padding: .6rem 1rem; }
  .formatting button { display: inline-flex; align-items: center; justify-content: center; min-width: 3.4rem; font-size: 1.7rem; }
  button:hover:not(:disabled) { background: var(--surface-hover); }
  button[aria-pressed=true] { background: var(--surface-active); color: var(--accent); }
  button:disabled { opacity: .35; cursor: default; }
  button:focus-visible { outline: 2px solid var(--focus); outline-offset: 2px; }
  .divider { height: 2rem; border-left: 1px solid var(--border); margin: 0 .5rem; }
  .writing { position: relative; }
  :global(.post-editable) { min-height: 22rem; padding: 1.8rem; outline: none; }
  .placeholder { position: absolute; inset-inline-start: 1.8rem; top: 1.8rem; color: var(--text-muted); font-size: 1.5rem; pointer-events: none; }
  .editor-footer { display: flex; justify-content: space-between; flex-wrap: wrap; gap: 1rem; padding: 1.2rem 1.8rem; border-top: 1px solid var(--border); color: var(--text-muted); font-size: var(--ui-small-size); }
</style>
