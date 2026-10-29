import { test } from 'node:test';
import assert from 'node:assert/strict';
import { overlappingTarget } from '../src/lib/drag-target.ts';
import { reorder } from '../src/lib/reorder.ts';

const rect = (left, top, width = 100, height = 100) => ({ left, top, right: left + width, bottom: top + height });
const clip = rect(0, 0, 500, 500);
const target = (id, bounds) => ({ id, bounds });

test('a one-pixel overlap accepts a tile even with the pointer outside it', () => {
  const tile = target('b', rect(110, 0));
  assert.equal(overlappingTarget(rect(11, 0), [tile], clip), tile);
  assert.equal(overlappingTarget(rect(10, 0), [tile], clip), tile, 'edge contact is accepted');
  assert.equal(overlappingTarget(rect(9, 0), [tile], clip), undefined, 'a gap is not accepted');
});

test('overlap works from every side and across grid rows', () => {
  const tile = target('b', rect(110, 110));
  for (const dragged of [rect(11, 110), rect(209, 110), rect(110, 11), rect(110, 209), rect(11, 11)]) {
    assert.equal(overlappingTarget(dragged, [tile], clip), tile);
  }
});

test('the largest overlapping area wins; distance breaks equal-area ties', () => {
  const a = target('a', rect(0, 0)), b = target('b', rect(110, 0));
  assert.equal(overlappingTarget(rect(60, 0), [a, b], clip), b);
  const far = target('far', rect(0, 0, 200, 100));
  const near = target('near', rect(100, 0));
  assert.equal(overlappingTarget(rect(100, 0), [far, near], clip), near);
});

test('offscreen portions and tiles outside the scroll area cannot receive a drop', () => {
  const tile = target('b', rect(110, 110));
  const cropped = rect(0, 0, 160, 160);
  assert.equal(overlappingTarget(rect(161, 110), [tile], cropped), undefined);
  assert.equal(overlappingTarget(rect(110, 161), [tile], cropped), undefined);
  assert.equal(overlappingTarget(rect(110, 110), [tile], rect(0, 0, 100, 100)), undefined);
  assert.equal(overlappingTarget(rect(60, 60), [tile], cropped), tile);
});

test('edge targets reorder adjacent images in both directions', () => {
  const items = [{ id: 'a' }, { id: 'b' }, { id: 'c' }];
  const b = overlappingTarget(rect(11, 0), [target('b', rect(110, 0))], clip);
  assert.deepEqual(reorder(items, 'a', b.id, 'after').map(item => item.id), ['b', 'a', 'c']);
  const a = overlappingTarget(rect(99, 0), [target('a', rect(0, 0))], clip);
  assert.deepEqual(reorder(items, 'b', a.id, 'before').map(item => item.id), ['b', 'a', 'c']);
});
