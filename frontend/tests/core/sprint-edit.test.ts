import { describe, expect, it } from 'vitest';

import { atDay, localDay, sprintEdit } from '../../src/core/board.js';
import type { Sprint } from '../../src/core/board-types.js';

// Local wall-clock dates, so the tests read the same in every time zone.
const at = (y: number, m: number, d: number, hh = 12, mm = 0) =>
  new Date(y, m - 1, d, hh, mm).toISOString();
const NOW = new Date(2026, 9, 14, 15, 0);

const future: Sprint = {
  id: 1,
  name: 'Sprint 12',
  state: 'future',
  goal: null,
  start: null,
  end: null,
  completed: null,
};
const active: Sprint = {
  ...future,
  id: 2,
  state: 'active',
  goal: 'Paiement',
  start: at(2026, 10, 12, 9, 30),
  end: at(2026, 10, 23, 17, 45),
};

const form = (s: Sprint) => ({
  name: s.name,
  goal: s.goal ?? '',
  start: localDay(s.start),
  end: localDay(s.end),
});

describe('localDay and atDay', () => {
  it('reads the local day of an instant', () => {
    expect(localDay(at(2026, 10, 14, 23, 30))).toBe('2026-10-14');
    expect(localDay(null)).toBe('');
    expect(localDay('not a date')).toBe('');
  });

  it('puts a day at the time an instant had, or at the hour given', () => {
    const d = new Date(atDay('2026-11-02', at(2026, 10, 12, 9, 30), 17) ?? '');
    expect([d.getMonth(), d.getDate(), d.getHours(), d.getMinutes()]).toEqual([10, 2, 9, 30]);
    const e = new Date(atDay('2026-11-02', null, 17, 15) ?? '');
    expect([e.getDate(), e.getHours(), e.getMinutes()]).toEqual([2, 17, 15]);
    expect(atDay('02/11/2026', null, 9)).toBeNull();
  });
});

describe('sprintEdit', () => {
  it('has nothing to send when nothing changed', () => {
    expect(sprintEdit(active, form(active), NOW)).toEqual({ ok: true, patch: {} });
    expect(sprintEdit(future, form(future), NOW)).toEqual({ ok: true, patch: {} });
  });

  it('renames and sets the goal, trimmed; an emptied goal is cleared', () => {
    expect(
      sprintEdit(future, { ...form(future), name: '  Sprint 13 ', goal: ' Livrer ' }, NOW),
    ).toEqual({
      ok: true,
      patch: { name: 'Sprint 13', goal: 'Livrer' },
    });
    expect(sprintEdit(active, { ...form(active), goal: '  ' }, NOW)).toEqual({
      ok: true,
      patch: { goal: null },
    });
  });

  it('refuses an empty name', () => {
    expect(sprintEdit(future, { ...form(future), name: '  ' }, NOW)).toEqual({
      ok: false,
      error: 'name',
    });
  });

  it('plans dates on a future sprint, and clears them', () => {
    const planned = sprintEdit(
      future,
      { ...form(future), start: '2026-11-02', end: '2026-11-13' },
      NOW,
    );
    expect(planned.ok).toBe(true);
    if (planned.ok) {
      expect(localDay(planned.patch.start ?? null)).toBe('2026-11-02');
      expect(localDay(planned.patch.end ?? null)).toBe('2026-11-13');
    }
    const withDates: Sprint = { ...future, start: at(2026, 11, 2, 9), end: at(2026, 11, 13, 17) };
    expect(sprintEdit(withDates, { ...form(withDates), start: '', end: '' }, NOW)).toEqual({
      ok: true,
      patch: { start: null, end: null },
    });
  });

  it('keeps the time of a date that is moved, not one that is not', () => {
    const r = sprintEdit(active, { ...form(active), end: '2026-10-30' }, NOW);
    expect(r.ok).toBe(true);
    if (r.ok) {
      expect(r.patch.start).toBeUndefined();
      const end = new Date(r.patch.end ?? '');
      expect([end.getDate(), end.getHours(), end.getMinutes()]).toEqual([30, 17, 45]);
    }
  });

  it('wants the end after the start', () => {
    expect(
      sprintEdit(future, { ...form(future), start: '2026-11-13', end: '2026-11-02' }, NOW),
    ).toEqual({ ok: false, error: 'order' });
    expect(sprintEdit(active, { ...form(active), end: '2026-10-11' }, NOW)).toEqual({
      ok: false,
      error: 'order',
    });
    // The same day is fine when the end comes later in it: 9:00 to 17:00.
    expect(
      sprintEdit(future, { ...form(future), start: '2026-11-02', end: '2026-11-02' }, NOW).ok,
    ).toBe(true);
  });

  it('keeps both dates of a sprint that has begun, none of them in the future', () => {
    expect(sprintEdit(active, { ...form(active), start: '' }, NOW)).toEqual({
      ok: false,
      error: 'start',
    });
    expect(sprintEdit(active, { ...form(active), end: '' }, NOW)).toEqual({
      ok: false,
      error: 'end',
    });
    expect(sprintEdit(active, { ...form(active), start: '2026-10-15' }, NOW)).toEqual({
      ok: false,
      error: 'startFuture',
    });
    expect(sprintEdit(active, { ...form(active), start: '2026-10-14' }, NOW).ok).toBe(true);
  });
});
