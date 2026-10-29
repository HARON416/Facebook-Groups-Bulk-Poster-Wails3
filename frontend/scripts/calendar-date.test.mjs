import { test } from 'node:test';
import assert from 'node:assert/strict';
import { calendarDays, dateKey, parseDateKey, shiftCalendarMonth } from '../src/lib/calendar-date.ts';

test('date-only parsing round trips without UTC conversion and rejects invalid dates', () => {
  for (const key of ['2026-10-29', '2024-02-29', '2026-01-01', '2026-12-31']) {
    assert.equal(dateKey(parseDateKey(key)), key);
    assert.equal(parseDateKey(key).getHours(), 12);
  }
  for (const key of ['', '2026-02-29', '2026-13-01', '2026-04-31', '2026-00-01', '2026-1-1', '2026-10-29T00:00:00Z']) assert.equal(parseDateKey(key), null);
});

test('October 2026 matches the Sunday-first reference calendar', () => {
  const days = calendarDays(parseDateKey('2026-10-01'));
  assert.equal(days.length, 35);
  assert.equal(days[0].key, '2026-09-27');
  assert.equal(days[0].outside, true);
  assert.equal(days[4].key, '2026-10-01');
  assert.equal(days.at(-1).key, '2026-10-31');
  assert.equal(days.filter(day => !day.outside).length, 31);
});

test('calendar handles six-row months and leap years', () => {
  assert.equal(calendarDays(parseDateKey('2026-08-01')).length, 42);
  assert.equal(calendarDays(parseDateKey('2024-02-01')).filter(day => !day.outside).length, 29);
});

test('keyboard month/year navigation clamps the day at month boundaries', () => {
  assert.equal(dateKey(shiftCalendarMonth(parseDateKey('2026-01-31'), 1)), '2026-02-28');
  assert.equal(dateKey(shiftCalendarMonth(parseDateKey('2024-02-29'), 12)), '2025-02-28');
  assert.equal(dateKey(shiftCalendarMonth(parseDateKey('2026-01-01'), -1)), '2025-12-01');
});
