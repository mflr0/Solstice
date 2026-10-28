/**
 * Time estimates and work logs typed in a form: reading the text, and
 * working out what to send. Pure functions, tested in
 * tests/core/estimates.test.ts.
 */

import {
  type DurationUnits,
  isZeroDuration,
  isoDay,
  jiraDuration,
  parseDuration,
  typedDuration,
} from './dates.js';
import type { TimeTracking, Worklog, WorklogPatch } from './types.js';

/** A typed estimate: nothing (it is cleared), a duration in seconds, or not understood. */
export type EstimateRead = { ok: true; seconds: number | null } | { ok: false };

/**
 * Reads an estimate, in Jira's units or units': empty clears it, "0m" is
 * nothing left to do, else a duration.
 */
export function readEstimate(text: string, units?: DurationUnits): EstimateRead {
  if (text.trim() === '') return { ok: true, seconds: null };
  if (isZeroDuration(text, units)) return { ok: true, seconds: 0 };
  const seconds = parseDuration(text, units);
  return seconds === null ? { ok: false } : { ok: true, seconds };
}

/** An estimate as it is written in a field, in units: "1j 2h", or nothing. */
export function estimateText(seconds: number | null, units?: DurationUnits): string {
  return seconds === null ? '' : typedDuration(seconds, units);
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
  units?: DurationUnits,
): EstimateChanges {
  const out: EstimateChanges = { fields: {}, invalid: null };
  for (const [name, typed, now, key] of [
    ['original', form.original, current.original, 'originalEstimate'],
    ['remaining', form.remaining, current.remaining, 'remainingEstimate'],
  ] as const) {
    if (typed.trim() === estimateText(now, units)) continue;
    const read = readEstimate(typed, units);
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
  units?: DurationUnits,
): WorklogEdit {
  const seconds = parseDuration(form.duration, units);
  if (seconds === null || !form.day) return { ok: false };
  const patch: WorklogPatch = {};
  if (seconds !== log.seconds) patch.seconds = seconds;
  if (form.day !== isoDay(new Date(log.started))) patch.started = worklogStart(form.day);
  if (form.body.trim() !== (log.body ?? '').trim()) patch.body = form.body;
  return { ok: true, patch };
}
