/**
 * Time estimates and work logs typed in a form: reading the text, and
 * working out what to send. Pure functions, tested in
 * tests/core/estimates.test.ts.
 */

import { isoDay, jiraDuration, parseDuration } from './dates.js';
import type { TimeTracking, Worklog, WorklogPatch } from './types.js';

/** A typed estimate: nothing (it is cleared), a duration in seconds, or not understood. */
export type EstimateRead = { ok: true; seconds: number | null } | { ok: false };

/** An estimate of no time at all: "0", "0h", "0m"… */
const ZERO = /^\s*0+(?:[.,]0*)?\s*(?:min|mn|m|h|j|d|w)?\s*$/i;

/** Reads an estimate: empty clears it, "0m" is nothing left to do, else a duration. */
export function readEstimate(text: string): EstimateRead {
  if (text.trim() === '') return { ok: true, seconds: null };
  if (ZERO.test(text)) return { ok: true, seconds: 0 };
  const seconds = parseDuration(text);
  return seconds === null ? { ok: false } : { ok: true, seconds };
}

/** An estimate as it is written in a field: "1d 2h", or nothing. */
export function estimateText(seconds: number | null): string {
  return seconds === null ? '' : jiraDuration(seconds);
}

/** The fields of an issue that hold its estimates. */
export interface EstimateFields {
  originalEstimate?: string | null;
  remainingEstimate?: string | null;
}

export interface EstimateChanges {
  /** What to send: only the estimates that changed. */
  fields: EstimateFields;
  /** The first field that could not be read. */
  invalid: 'original' | 'remaining' | null;
}

/**
 * What typing new estimates changes. A field left as it was shown (even
 * with seconds that its text cannot show) is not sent again.
 */
export function estimateChanges(
  form: { original: string; remaining: string },
  current: Pick<TimeTracking, 'original' | 'remaining'>,
): EstimateChanges {
  const out: EstimateChanges = { fields: {}, invalid: null };
  for (const [name, typed, now, key] of [
    ['original', form.original, current.original, 'originalEstimate'],
    ['remaining', form.remaining, current.remaining, 'remainingEstimate'],
  ] as const) {
    if (typed.trim() === estimateText(now)) continue;
    const read = readEstimate(typed);
    if (!read.ok) {
      out.invalid ??= name;
      continue;
    }
    if (read.seconds === now) continue;
    out.fields[key] = read.seconds === null ? null : jiraDuration(read.seconds);
  }
  return out;
}

/** A work log being edited, as typed. */
export interface WorklogForm {
  duration: string;
  /** "YYYY-MM-DD". */
  day: string;
  body: string;
}

export type WorklogEdit = { ok: true; patch: WorklogPatch } | { ok: false };

/** The instant a work log of a day is given: noon, so the day holds in every nearby time zone. */
export function worklogStart(day: string): string {
  return new Date(`${day}T12:00:00`).toISOString();
}

/** What editing a work log changes; not ok when the duration cannot be read. */
export function worklogEdit(
  log: Pick<Worklog, 'started' | 'seconds' | 'body'>,
  form: WorklogForm,
): WorklogEdit {
  const seconds = parseDuration(form.duration);
  if (seconds === null || !form.day) return { ok: false };
  const patch: WorklogPatch = {};
  if (seconds !== log.seconds) patch.seconds = seconds;
  if (form.day !== isoDay(new Date(log.started))) patch.started = worklogStart(form.day);
  if (form.body.trim() !== (log.body ?? '').trim()) patch.body = form.body;
  return { ok: true, patch };
}
