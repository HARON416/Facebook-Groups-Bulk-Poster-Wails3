import test from 'node:test';
import assert from 'node:assert/strict';
import { createTourChoice, placeTourCard, tourStorageKey, tourKey } from '../src/lib/tour.ts';

test('tour choices persist without touching drafts, theme or language', () => {
  const data = new Map([['postika:language', 'ar'], ['draft', 'saved message']]);
  const storage = () => ({ getItem: key => data.get(key) ?? null, setItem: (key, value) => data.set(key, value) });
  const first = createTourChoice(storage);
  assert.equal(first.hasSeen(), false);
  assert.equal(data.size, 2);
  first.remember();
  assert.equal(first.hasSeen(), true);
  assert.equal(createTourChoice(storage).hasSeen(), true);
  assert.deepEqual([...data], [['postika:language', 'ar'], ['draft', 'saved message'], [tourStorageKey, 'seen']]);
});

test('blocked local storage still respects Skip for the rest of the session', () => {
  const choice = createTourChoice(() => { throw new Error('storage disabled'); });
  assert.equal(choice.hasSeen(), false);
  choice.remember();
  assert.equal(choice.hasSeen(), true);
});

test('Marketplace tour dismissals stay separate from groups and from each other', () => {
  const data = new Map();
  const storage = () => ({ getItem: key => data.get(key) ?? null, setItem: (key, value) => data.set(key, value) });
  assert.equal(tourKey('groups'), tourStorageKey);
  const scopes = ['groups', 'marketplace', 'listing-types', 'listing-editor'];
  for (const scope of scopes) {
    const choice = createTourChoice(storage, tourKey(scope));
    assert.equal(choice.hasSeen(), false);
    choice.remember();
    assert.equal(createTourChoice(storage, tourKey(scope)).hasSeen(), true);
    for (const unseen of scopes.slice(scopes.indexOf(scope) + 1)) assert.equal(createTourChoice(storage, tourKey(unseen)).hasSeen(), false);
  }
  assert.equal(data.size, 4);
});

test('tour cards stay on screen and avoid the highlighted target when space permits', () => {
  for (const [viewport, target, width, height] of [
    [{ width: 1440, height: 900 }, { left: 1200, right: 1400, top: 12, bottom: 46 }, 360, 220],
    [{ width: 1440, height: 900 }, { left: 400, right: 1100, top: 800, bottom: 884 }, 360, 220],
    [{ width: 390, height: 844 }, { left: 16, right: 260, top: 16, bottom: 48 }, 358, 300],
    [{ width: 320, height: 568 }, null, 288, 360],
    [{ width: 1440, height: 500 }, { left: 16, right: 200, top: 16, bottom: 480 }, 360, 300],
  ]) {
    const { x, y } = placeTourCard(target, width, height, viewport);
    assert.ok(x >= 16 && y >= 16);
    assert.ok(x + width <= viewport.width - 16 && y + height <= viewport.height - 16);
    if (target) assert.ok(y >= target.bottom || y + height <= target.top || x >= target.right || x + width <= target.left);
  }
});
