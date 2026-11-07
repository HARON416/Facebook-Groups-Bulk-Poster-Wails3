export const tourStorageKey = 'postika:guided-tour:v1';

export type TourScope = 'groups' | 'marketplace' | 'listing-types' | 'listing-editor';
export const tourKey = (scope: TourScope) => scope === 'groups' ? tourStorageKey : `postika:guided-tour:${scope}:v1`;

export function createTourChoice(storage: () => Pick<Storage, 'getItem' | 'setItem'> = () => localStorage, key = tourStorageKey) {
  let seen = false;
  return {
    hasSeen() {
      try { return seen || storage().getItem(key) === 'seen'; }
      catch { return seen; }
    },
    remember() {
      seen = true;
      try { storage().setItem(key, 'seen'); }
      catch { /* Still respect the choice for this window when storage is unavailable. */ }
    },
  };
}

export const tourChoice = createTourChoice();
const choices = new Map<TourScope, ReturnType<typeof createTourChoice>>([['groups', tourChoice]]);
export function tourChoiceFor(scope: TourScope) {
  let choice = choices.get(scope);
  if (!choice) { choice = createTourChoice(undefined, tourKey(scope)); choices.set(scope, choice); }
  return choice;
}

export interface TourRect { left: number; top: number; right: number; bottom: number }
export function placeTourCard(target: TourRect | null, width: number, height: number, viewport: { width: number; height: number }) {
  const margin = 16, gap = 12;
  const clampX = (x: number) => Math.max(margin, Math.min(x, viewport.width - width - margin));
  const clampY = (y: number) => Math.max(margin, Math.min(y, viewport.height - height - margin));
  if (!target) return { x: clampX((viewport.width - width) / 2), y: clampY((viewport.height - height) / 2) };
  const x = clampX(target.left);
  if (target.bottom + gap + height <= viewport.height - margin) return { x, y: target.bottom + gap };
  if (target.top - gap - height >= margin) return { x, y: target.top - gap - height };
  if (target.right + gap + width <= viewport.width - margin) return { x: target.right + gap, y: clampY(target.top) };
  if (target.left - gap - width >= margin) return { x: target.left - gap - width, y: clampY(target.top) };
  return { x, y: clampY(viewport.height - height - margin) };
}
