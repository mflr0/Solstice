import { afterAll, beforeAll, describe, expect, it } from 'vitest';

import {
  ago,
  isOverdue,
  parseDay,
  parseDuration,
  splitDuration,
  todayISO,
} from '../../src/core/dates.js';
import {
  formatDay,
  formatDuration,
  formatSize,
  relative,
  renderDay,
  useMessages,
} from '../../src/i18n/t.js';
import { en } from '../../src/i18n/en.js';
import { fr } from '../../src/i18n/fr.js';

// Local wall-clock dates, so the tests read the same in every time zone.
const NOW = new Date(2026, 2, 12, 15, 0, 0);
const at = (y: number, m: number, d: number, hh = 12, mm = 0, ss = 0) =>
  new Date(y, m - 1, d, hh, mm, ss).toISOString();

describe('ago', () => {
  it('says "now" for the last minute, and for clock skew', () => {
    expect(ago(at(2026, 3, 12, 14, 59, 30), NOW)).toEqual({ kind: 'now' });
    expect(ago(at(2026, 3, 12, 15, 5), NOW)).toEqual({ kind: 'now' });
  });

  it('counts minutes, then hours while still today', () => {
    expect(ago(at(2026, 3, 12, 14, 12), NOW)).toEqual({ kind: 'minutes', n: 48 });
    expect(ago(at(2026, 3, 12, 9, 30), NOW)).toEqual({ kind: 'hours', n: 5 });
  });

  it('uses calendar days: last night is "yesterday", not hours', () => {
    expect(ago(at(2026, 3, 11, 23, 50), NOW)).toEqual({ kind: 'yesterday' });
    expect(ago(at(2026, 3, 9, 10), NOW)).toEqual({ kind: 'days', n: 3 });
  });

  it('gives the date after a week', () => {
    expect(ago(at(2026, 3, 5, 10), NOW)).toEqual({ kind: 'date', day: { y: 2026, m: 3, d: 5 } });
  });
});

describe('French wording', () => {
  beforeAll(() => {
    useMessages(fr);
  });

  afterAll(() => {
    useMessages(en);
  });

  it('renders relative times', () => {
    expect(relative(at(2026, 3, 12, 14, 59, 50), NOW)).toBe('à l’instant');
    expect(relative(at(2026, 3, 12, 12, 0), NOW)).toBe('il y a 3 h');
    expect(relative(at(2026, 3, 11, 9, 0), NOW)).toBe('hier');
    expect(relative(at(2026, 2, 1, 9, 0), NOW)).toBe('1er février 2026');
  });

  it('renders calendar days without shifting them', () => {
    expect(renderDay({ y: 2026, m: 3, d: 12 })).toBe('12 mars 2026');
    expect(formatDay('2026-08-01')).toBe('1er août 2026');
    expect(formatDay('not a date')).toBe('not a date');
  });

  it('renders durations and sizes', () => {
    expect(formatDuration(9000)).toBe('2 h 30');
    expect(formatDuration(7200)).toBe('2 h');
    expect(formatDuration(2700)).toBe('45 min');
    expect(formatDuration(3900)).toBe('1 h 05');
    expect(formatSize(820)).toBe('820 o');
    expect(formatSize(12 * 1024)).toBe('12 Ko');
    expect(formatSize(1.4 * 1024 * 1024)).toBe('1,4 Mo');
  });
});

describe('parseDuration', () => {
  it.each([
    ['2h30', 9000],
    ['2 h 30', 9000],
    ['2h 30m', 9000],
    ['2h30min', 9000],
    ['1,5 h', 5400],
    ['1.5h', 5400],
    ['45 min', 2700],
    ['45m', 2700],
    ['2', 7200],
    ['1j', 8 * 3600],
    ['1j 2h', 10 * 3600],
  ])('reads %s', (text, seconds) => {
    expect(parseDuration(text)).toBe(seconds);
  });

  it.each(['', 'abc', '0', '0h', '2x', 'h', '30 30'])('refuses %s', (text) => {
    expect(parseDuration(text)).toBeNull();
  });
});

describe('days', () => {
  it('parses YYYY-MM-DD', () => {
    expect(parseDay('2026-03-12')).toEqual({ y: 2026, m: 3, d: 12 });
    expect(parseDay('12/03/2026')).toBeNull();
  });

  it('knows today and overdue dates', () => {
    expect(todayISO(NOW)).toBe('2026-03-12');
    expect(isOverdue('2026-03-11', NOW)).toBe(true);
    expect(isOverdue('2026-03-12', NOW)).toBe(false);
  });

  it('splits durations into hours and minutes', () => {
    expect(splitDuration(5430)).toEqual({ h: 1, m: 31 });
    expect(splitDuration(-5)).toEqual({ h: 0, m: 0 });
  });
});
