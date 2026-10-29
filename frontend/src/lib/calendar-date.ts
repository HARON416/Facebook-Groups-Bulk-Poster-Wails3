export function dateKey(date: Date): string {
  return `${String(date.getFullYear()).padStart(4, '0')}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`;
}

// Date-only values use local noon, avoiding UTC parsing and daylight-saving gaps.
export function parseDateKey(value: string): Date | null {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(value)) return null;
  const [year, month, day] = value.split('-').map(Number);
  const date = new Date(2000, month - 1, day, 12);
  date.setFullYear(year);
  return dateKey(date) === value ? date : null;
}

export function calendarDays(month: Date) {
  const first = new Date(month.getFullYear(), month.getMonth(), 1, 12);
  const count = new Date(month.getFullYear(), month.getMonth() + 1, 0).getDate();
  return Array.from({ length: Math.ceil((first.getDay() + count) / 7) * 7 }, (_, index) => {
    const date = new Date(month.getFullYear(), month.getMonth(), index - first.getDay() + 1, 12);
    return { date, key: dateKey(date), outside: date.getMonth() !== month.getMonth() };
  });
}

export function shiftCalendarMonth(date: Date, offset: number): Date {
  const month = new Date(date.getFullYear(), date.getMonth() + offset, 1, 12);
  month.setDate(Math.min(date.getDate(), new Date(month.getFullYear(), month.getMonth() + 1, 0).getDate()));
  return month;
}
