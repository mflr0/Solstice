import { describe, expect, it } from 'vitest';

import { jiraDuration, parseDuration } from '../../src/core/dates.js';
import {
  estimateChanges,
  estimateText,
  readEstimate,
  worklogEdit,
  worklogStart,
} from '../../src/core/estimates.js';

const DAY = 8 * 3600;
const WEEK = 5 * DAY;

describe('weeks in durations', () => {
  it.each([
    ['1w', WEEK],
    ['1w 2d', WEEK + 2 * DAY],
    ['1w 2d 3h 30m', WEEK + 2 * DAY + 3 * 3600 + 1800],
    ['0.5w', 2.5 * DAY],
  ])('reads %s', (text, seconds) => {
    expect(parseDuration(text)).toBe(seconds);
  });
});

describe('jiraDuration', () => {
  it.each([
    [0, '0m'],
    [59, '0m'],
    [60, '1m'],
    [5400, '1h 30m'],
    [DAY, '1d'],
    [WEEK + 3600, '1w 1h'],
    [9 * DAY + 1800, '1w 4d 30m'],
  ])('writes %i seconds as %s', (seconds, text) => {
    expect(jiraDuration(seconds)).toBe(text);
  });

  it('is read back as what it writes', () => {
    for (const seconds of [60, 5400, DAY, WEEK + 2 * DAY + 3 * 3600 + 1800, 123 * 60]) {
      expect(parseDuration(jiraDuration(seconds))).toBe(seconds);
    }
  });
});

describe('readEstimate', () => {
  it('clears on an empty field', () => {
    expect(readEstimate('')).toEqual({ ok: true, seconds: null });
    expect(readEstimate('   ')).toEqual({ ok: true, seconds: null });
  });

  it('reads durations, including none left', () => {
    expect(readEstimate('1d 2h')).toEqual({ ok: true, seconds: 10 * 3600 });
    expect(readEstimate('0m')).toEqual({ ok: true, seconds: 0 });
    expect(readEstimate('0')).toEqual({ ok: true, seconds: 0 });
    expect(readEstimate(' 0h ')).toEqual({ ok: true, seconds: 0 });
  });

  it.each(['soon', '1x', '2d 4 5', 'h', '-1h'])('refuses %s', (text) => {
    expect(readEstimate(text)).toEqual({ ok: false });
  });
});

describe('estimateText', () => {
  it('shows an estimate as it can be typed back', () => {
    expect(estimateText(null)).toBe('');
    expect(estimateText(0)).toBe('0m');
    expect(estimateText(WEEK + DAY)).toBe('1w 1d');
  });
});

describe('estimateChanges', () => {
  const current = { original: DAY, remaining: 4 * 3600 };

  it('sends nothing when nothing was touched', () => {
    expect(estimateChanges({ original: '1d', remaining: '4h' }, current)).toEqual({
      fields: {},
      invalid: null,
    });
  });

  it('sends only what changed, written for the server', () => {
    expect(estimateChanges({ original: '2d', remaining: '4h' }, current).fields).toEqual({
      originalEstimate: '2d',
    });
    expect(estimateChanges({ original: '1d', remaining: '1d 30m' }, current).fields).toEqual({
      remainingEstimate: '1d 30m',
    });
  });

  it('does not send a field again for another way of writing the same time', () => {
    expect(estimateChanges({ original: '8h', remaining: '240m' }, current).fields).toEqual({});
  });

  it('clears a field emptied', () => {
    expect(estimateChanges({ original: '', remaining: '4h' }, current).fields).toEqual({
      originalEstimate: null,
    });
    expect(
      estimateChanges({ original: '1d', remaining: '' }, { original: DAY, remaining: null }).fields,
    ).toEqual({});
  });

  it('keeps seconds its text cannot show', () => {
    const odd = { original: 3630, remaining: null };
    expect(estimateChanges({ original: '1h', remaining: '' }, odd).fields).toEqual({});
  });

  it('names the first field it cannot read, and still reads the other', () => {
    const r = estimateChanges({ original: 'soon', remaining: '1h' }, current);
    expect(r.invalid).toBe('original');
    expect(r.fields).toEqual({ remainingEstimate: '1h' });
    expect(estimateChanges({ original: '1d', remaining: '??' }, current).invalid).toBe('remaining');
  });

  it('sets estimates on an issue that had none', () => {
    expect(
      estimateChanges({ original: '3h', remaining: '' }, { original: null, remaining: null })
        .fields,
    ).toEqual({ originalEstimate: '3h' });
  });
});

describe('worklogEdit', () => {
  const started = new Date(2026, 9, 14, 12, 0, 0).toISOString();
  const log = { started, seconds: 5400, body: 'Analyse' };

  it('sends nothing when nothing changed', () => {
    expect(worklogEdit(log, { duration: '1h 30m', day: '2026-10-14', body: 'Analyse' })).toEqual({
      ok: true,
      patch: {},
    });
  });

  it('sends only what changed', () => {
    expect(worklogEdit(log, { duration: '2h', day: '2026-10-14', body: 'Analyse' })).toEqual({
      ok: true,
      patch: { seconds: 7200 },
    });
    const moved = worklogEdit(log, { duration: '1h 30m', day: '2026-10-13', body: 'Revue' });
    expect(moved).toEqual({
      ok: true,
      patch: { started: worklogStart('2026-10-13'), body: 'Revue' },
    });
  });

  it('can empty the text', () => {
    expect(worklogEdit(log, { duration: '1h 30m', day: '2026-10-14', body: '' })).toEqual({
      ok: true,
      patch: { body: '' },
    });
    expect(
      worklogEdit({ ...log, body: null }, { duration: '1h 30m', day: '2026-10-14', body: ' ' }),
    ).toEqual({ ok: true, patch: {} });
  });

  it('refuses what is not a duration, or has no day', () => {
    expect(worklogEdit(log, { duration: '', day: '2026-10-14', body: '' })).toEqual({ ok: false });
    expect(worklogEdit(log, { duration: '0', day: '2026-10-14', body: '' })).toEqual({ ok: false });
    expect(worklogEdit(log, { duration: 'lots', day: '2026-10-14', body: '' })).toEqual({
      ok: false,
    });
    expect(worklogEdit(log, { duration: '1h', day: '', body: '' })).toEqual({ ok: false });
  });

  it('puts a work log at noon of its day', () => {
    const d = new Date(worklogStart('2026-10-14'));
    expect([d.getFullYear(), d.getMonth(), d.getDate(), d.getHours()]).toEqual([2026, 9, 14, 12]);
  });
});
