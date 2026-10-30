/**
 * Numbers, plurals, lists, dates and relative times in the language in use,
 * built on Intl so a catalogue only supplies words. Boot points these at the
 * visitor's locale (src/i18n/index.ts); until then they speak French, which
 * the tests are written against.
 */

import type { Day } from '../core/dates.js';

/** The BCP 47 tag Intl formats with: "fr-FR", "en-GB", "hi"… */
let tag = 'fr-FR';
let language = 'fr';
const formats = new Map<string, unknown>();

/** Points every formatter at a locale. Called once at boot (and by tests). */
export function setFormatLocale(next: string): void {
  tag = next;
  language = next.split('-')[0]?.toLowerCase() ?? next;
  formats.clear();
}

/** The tag in use, for code that calls Intl itself. */
export function formatLocale(): string {
  return tag;
}

/** One Intl object per locale and options: they are costly to build. */
function cached<T>(key: string, make: () => T): T {
  const id = `${tag}|${key}`;
  let f = formats.get(id) as T | undefined;
  if (f === undefined) {
    f = make();
    formats.set(id, f);
  }
  return f;
}

/** "1 204" in French, "1,204" in English; at most `fraction` decimals. */
export function formatNumber(n: number, fraction = 0): string {
  return cached(
    `n${fraction}`,
    () => new Intl.NumberFormat(tag, { maximumFractionDigits: fraction }),
  ).format(n);
}

/**
 * A count and its noun, in the form the language's plural rules pick:
 * "1 ticket", "0 ticket", "1 204 tickets" in French; "0 issues" in English.
 */
export function plural(n: number, one: string, other: string): string {
  const rule = cached('plural', () => new Intl.PluralRules(tag)).select(n);
  return `${formatNumber(n)} ${rule === 'one' ? one : other}`;
}

/** "A, B et C", "A, B, and C", "A, B und C". */
export function formatList(items: readonly string[]): string {
  return cached('list', () => new Intl.ListFormat(tag, { type: 'conjunction' })).format(items);
}

/** "3 minutes ago", "vor 3 Stunden", "3 दिन पहले": Intl's own wording. */
export function timeAgo(n: number, unit: 'minute' | 'hour' | 'day'): string {
  return cached('ago', () => new Intl.RelativeTimeFormat(tag, { numeric: 'always' })).format(
    -n,
    unit,
  );
}

/** "in 3 minutes", "in 3 Stunden", "3 दिन में": Intl's own wording. */
export function timeIn(n: number, unit: 'minute' | 'hour' | 'day'): string {
  return cached('ago', () => new Intl.RelativeTimeFormat(tag, { numeric: 'always' })).format(
    n,
    unit,
  );
}

/**
 * Intl's parts, with the one rule it lacks: French writes the first of the
 * month as an ordinal, « 1er mars ».
 */
function join(parts: Intl.DateTimeFormatPart[]): string {
  return parts
    .map((p) => (language === 'fr' && p.type === 'day' && p.value === '1' ? '1er' : p.value))
    .join('');
}

/** A calendar day: "12 mars 2026", "1er avril 2026", "March 12, 2026". */
export function longDay(day: Day): string {
  const f = cached(
    'longDay',
    () =>
      new Intl.DateTimeFormat(tag, {
        day: 'numeric',
        month: 'long',
        year: 'numeric',
        timeZone: 'UTC',
      }),
  );
  return join(f.formatToParts(Date.UTC(day.y, day.m - 1, day.d)));
}

/** A calendar day, abbreviated: "1 oct. 2026", "Oct 1, 2026". */
export function shortDay(day: Day): string {
  return cached(
    'shortDay',
    () =>
      new Intl.DateTimeFormat(tag, {
        day: 'numeric',
        month: 'short',
        year: 'numeric',
        timeZone: 'UTC',
      }),
  ).format(Date.UTC(day.y, day.m - 1, day.d));
}

/** The local day and month of an instant: "1 mars", "Mar 1". */
export function dayMonth(iso: string): string {
  return cached(
    'dayMonth',
    () => new Intl.DateTimeFormat(tag, { day: 'numeric', month: 'short' }),
  ).format(new Date(iso));
}

/** The local date and time of an instant: "12 mars 2026 à 14:05", "March 12, 2026 at 2:05 PM". */
export function dateTime(iso: string): string {
  const f = cached(
    'dateTime',
    () => new Intl.DateTimeFormat(tag, { dateStyle: 'long', timeStyle: 'short' }),
  );
  return join(f.formatToParts(new Date(iso)));
}
