import { $getRoot, $getSelection, $isRangeSelection, $isElementNode, $isTextNode, $isLineBreakNode, $createLineBreakNode, $splitNode, createState, $getState, $setState, type ElementNode } from 'lexical';
import { $insertList, $removeList, $isListNode } from '@lexical/list';

// This state survives list conversion, undo, and saved editor JSON. It records
// which lines belonged to the same paragraph before splitting for a list.
const lineGroup = createState('postikaLineGroup', { parse: value => typeof value === 'string' ? value : '' });
let groupSequence = 0;

function blocksInOrder(): ElementNode[] {
  const result: ElementNode[] = [];
  function visit(node: ElementNode) {
    if (['paragraph', 'heading', 'quote', 'listitem'].includes(node.getType())) result.push(node);
    for (const child of node.getChildren()) if ($isElementNode(child)) visit(child);
  }
  visit($getRoot());
  return result;
}

// List conversion creates replacement blocks but keeps their document order.
// Transfer provenance to those replacements, including intentionally empty lines.
function retainLineGroups(convert: () => void) {
  const groups = blocksInOrder().map(node => $getState(node, lineGroup));
  convert();
  const after = blocksInOrder();
  if (after.length === groups.length) after.forEach((node, index) => $setState(node, lineGroup, groups[index]));
}

export function applyPostList(type: 'bullet' | 'number') {
  splitSelectedLines();
  retainLineGroups(() => $insertList(type));
}

export function removePostList() {
  retainLineGroups(() => $removeList());
  for (const node of blocksInOrder()) {
    if (!node.isAttached() || node.getType() !== 'paragraph') continue;
    const previous = node.getPreviousSibling();
    const group = $getState(node, lineGroup);
    if (group && $isElementNode(previous) && previous.getType() === 'paragraph' && $getState(previous, lineGroup) === group) {
      previous.append($createLineBreakNode(), ...node.getChildren());
      node.remove();
    }
  }
}

// Lexical lists operate on blocks. Pasted text and Shift+Enter can leave many
// lines in one block, so split those lines before applying the list command.
// Moving the existing nodes retains bold/italic and the selection's text keys.
export function splitSelectedLines() {
  const selection = $getSelection();
  if (!$isRangeSelection(selection)) return;
  const blocks = new Map<string, ElementNode>();
  for (const selected of selection.getNodes()) {
    let node = selected;
    while (node.getParent() && !['paragraph', 'heading', 'quote', 'listitem'].includes(node.getType())) node = node.getParent()!;
    if ($isElementNode(node) && ['paragraph', 'heading', 'quote', 'listitem'].includes(node.getType())) blocks.set(node.getKey(), node);
  }
  for (const block of blocks.values()) {
    for (const child of block.getChildren()) {
      if (!$isTextNode(child) || !/[\r\n]/.test(child.getTextContent())) continue;
      const offsets = Array.from(child.getTextContent().matchAll(/\r\n|\r|\n/g)).flatMap(match => [match.index!, match.index! + match[0].length]);
      for (const part of child.splitText(...offsets)) {
        if (/^(\r\n|\r|\n)$/.test(part.getTextContent())) part.replace($createLineBreakNode());
      }
    }
    // Snapshot the breaks: later splits move nodes into their new blocks.
    const breaks = block.getChildren().filter($isLineBreakNode);
    const group = breaks.length ? $getState(block, lineGroup) || `${Date.now()}-${++groupSequence}` : '';
    if (group) $setState(block, lineGroup, group);
    for (const lineBreak of breaks) {
      const parent = lineBreak.getParentOrThrow();
      const [, rightTree] = $splitNode(parent, lineBreak.getIndexWithinParent() + 1);
      let right = rightTree;
      while ($isListNode(right)) right = right.getFirstChildOrThrow() as ElementNode;
      $setState(right, lineGroup, group);
      lineBreak.remove();
    }
  }
}
