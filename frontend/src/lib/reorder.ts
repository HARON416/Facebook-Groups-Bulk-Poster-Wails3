// Move an item to the indicated side of another item without changing its data.
export function reorder<T extends { id: string }>(
  items: T[], id: string, targetId: string, side: 'before' | 'after',
): T[] {
  const from = items.findIndex((item) => item.id === id);
  const target = items.findIndex((item) => item.id === targetId);
  if (from < 0 || target < 0 || from === target) return items;
  let insertion = target + (side === 'after' ? 1 : 0);
  if (from < insertion) insertion--;
  if (insertion === from) return items;
  const result = [...items];
  const [item] = result.splice(from, 1);
  result.splice(insertion, 0, item);
  return result;
}
