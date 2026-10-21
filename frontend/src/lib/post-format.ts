// Keep exported/pasted content limited to the formatting offered by our toolbar.
// Attributes, images, scripts, and embedded content never enter the preview.
export function cleanPostHTML(html: string): string {
  const source = new DOMParser().parseFromString(html, 'text/html');
  const result = document.createElement('div');
  const allowed = new Set(['p', 'br', 'strong', 'em', 'h1', 'h2', 'blockquote', 'ul', 'ol', 'li']);
  function copy(node: Node, into: Node) {
    if (node.nodeType === Node.TEXT_NODE) { into.appendChild(document.createTextNode(node.textContent ?? '')); return; }
    if (!(node instanceof Element)) return;
    const tag = node.tagName.toLowerCase();
    if (['script', 'style', 'iframe', 'object', 'svg', 'img', 'video', 'audio'].includes(tag)) return;
    const mapped = tag === 'b' ? 'strong' : tag === 'i' ? 'em' : tag === 'div' ? 'p' : tag;
    const target = allowed.has(mapped) ? document.createElement(mapped) : into;
    // Word and other editors can represent supported emphasis as span styles.
    // Convert only bold/italic into semantic tags, dropping all other styles.
    let content = target;
    const style = (node as HTMLElement).style;
    if (mapped !== 'strong' && (style?.fontWeight === 'bold' || Number(style?.fontWeight) >= 600)) {
      const strong = document.createElement('strong'); content.appendChild(strong); content = strong;
    }
    if (mapped !== 'em' && style?.fontStyle === 'italic') {
      const em = document.createElement('em'); content.appendChild(em); content = em;
    }
    for (const child of node.childNodes) copy(child, content);
    if (target !== into) into.appendChild(target);
  }
  for (const child of source.body.childNodes) copy(child, result);
  return result.innerHTML;
}
