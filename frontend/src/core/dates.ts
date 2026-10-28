/**
 * Dates and durations as descriptors. Wording lives in i18n; this module only
 * decides what to say ("3 hours ago", "yesterday", a calendar day…).
 */

/** A calendar day, month 1–12. */
export interface Day {
  y: number;
  m: number;
  d: number;
}

export type Ago =
  | { kind: 'now' }
  | { kind: 'minutes'; n: number }
  | { kind: 'hours'; n: number }
  | { kind: 'yesterday' }
  | { kind: 'days'; n: number }
  | { kind: 'date'; day: Day };

const MINUTE = 60_000;
const HOUR = 60 * MINUTE;

function dayOf(date: Date): Day {
  return { y: date.getFullYear(), m: date.getMonth() + 1, d: date.getDate() };
}

/** Whole calendar days between two local dates (b − a). */
function calendarDiff(a: Date, b: Date): number {
  const ua = Date.UTC(a.getFullYear(), a.getMonth(), a.getDate());
  const ub = Date.UTC(b.getFullYear(), b.getMonth(), b.getDate());
  return Math.round((ub - ua) / (24 * HOUR));
}

/**
 * How long ago an instant was, the way people say it: "just now", minutes,
 * hours while it is still today, "yesterday", days within a week, then the
 * date itself. Future instants (clock skew) count as now.
 */
export function ago(iso: string, now: Date = new Date()): Ago {
  const then = new Date(iso);
  const diff = now.getTime() - then.getTime();
  if (diff < MINUTE) return { kind: 'now' };
  if (diff < HOUR) return { kind: 'minutes', n: Math.floor(diff / MINUTE) };
  const days = calendarDiff(then, now);
  if (days === 0) return { kind: 'hours', n: Math.floor(diff / HOUR) };
  if (days === 1) return { kind: 'yesterday' };
  if (days < 7) return { kind: 'days', n: days };
  return { kind: 'date', day: dayOf(then) };
}

/** The local calendar day and time of an instant. */
export function localParts(iso: string): Day & { hh: number; mm: number } {
  const t = new Date(iso);
  return { ...dayOf(t), hh: t.getHours(), mm: t.getMinutes() };
}

/** Reads "YYYY-MM-DD" without any time zone shift. */
export function parseDay(s: string): Day | null {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(s);
  if (!m) return null;
  return { y: Number(m[1]), m: Number(m[2]), d: Number(m[3]) };
}

/** The local calendar day of a date as "YYYY-MM-DD". */
export function isoDay(date: Date): string {
  const d = dayOf(date);
  return `${d.y}-${String(d.m).padStart(2, '0')}-${String(d.d).padStart(2, '0')}`;
}

/** Today as "YYYY-MM-DD" in local time. */
export function todayISO(now: Date = new Date()): string {
  return isoDay(now);
}

/** Whether a due day is already past. */
export function isOverdue(due: string, now: Date = new Date()): boolean {
  return due < todayISO(now);
}

/** Splits a duration into hours and minutes (minutes rounded). */
export function splitDuration(seconds: number): { h: number; m: number } {
  const total = Math.max(0, Math.round(seconds / 60));
  return { h: Math.floor(total / 60), m: total % 60 };
}

/** A working day, as Jira counts it when people type "1j". */
const WORKDAY_HOURS = 8;

/** A working week, in days. */
const WORKWEEK_DAYS = 5;

/** How a language writes the units of a duration ("sem", "j", "h", "min"). */
export interface DurationUnits {
  week: string;
  day: string;
  hour: string;
  minute: string;
}

type Unit = keyof DurationUnits;

/** The units every language reads: Jira's, and the French ones it always took. */
const BASE_UNITS: readonly (readonly [string, Unit])[] = [
  ['w', 'week'],
  ['d', 'day'],
  ['j', 'day'],
  ['h', 'hour'],
  ['m', 'minute'],
  ['mn', 'minute'],
  ['min', 'minute'],
];

const UNIT_SECONDS: Record<Unit, number> = {
  week: WORKWEEK_DAYS * WORKDAY_HOURS * 3600,
  day: WORKDAY_HOURS * 3600,
  hour: 3600,
  minute: 60,
};

const escapeRe = (s: string) => s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');

/** The units read, lower case, by what they write: the base ones and units'. */
function unitTable(units?: DurationUnits): Map<string, Unit> {
  const table = new Map(BASE_UNITS);
  if (units)
    for (const unit of Object.keys(UNIT_SECONDS) as Unit[])
      table.set(units[unit].toLowerCase(), unit);
  return table;
}

/** A regular expression source matching any unit of table, the longest first. */
function unitSource(table: Map<string, Unit>): string {
  return [...table.keys()]
    .sort((a, b) => b.length - a.length)
    .map(escapeRe)
    .join('|');
}

/** Whether text is a duration of no time at all: "0", "0h", "0 min"… */
export function isZeroDuration(text: string, units?: DurationUnits): boolean {
  return new RegExp(`^\\s*0+(?:[.,]0*)?\\s*(?:${unitSource(unitTable(units))})?\\s*$`, 'i').test(
    text,
  );
}

/**
 * Reads a duration typed by someone logging work: "2h30", "2 h 30",
 * "1,5 h", "45 min", "45m", "1j 2h", "1w 2d" or a bare number of hours
 * ("2"); units also takes a language's own ("1sem 2j", "3Std"). A day is 8
 * hours and a week 5 days, as in Jira. Returns seconds, or null when the
 * text is not a positive duration.
 */
export function parseDuration(text: string, units?: DurationUnits): number | null {
  const s = text.toLowerCase().replace(/,/g, '.').trim();
  if (s === '') return null;
  if (/^\d+(\.\d+)?$/.test(s)) {
    const seconds = Math.round(Number(s) * 3600);
    return seconds > 0 ? seconds : null;
  }
  const table = unitTable(units);
  // Spaces may separate a number from its unit, never two numbers' digits.
  const token = new RegExp(`\\s*(\\d+(?:\\.\\d+)?)\\s*(${unitSource(table)})?\\s*`, 'y');
  let seconds = 0;
  let previous: Unit | '' = '';
  let pos = 0;
  while (pos < s.length) {
    token.lastIndex = pos;
    const m = token.exec(s);
    if (!m) return null;
    // "2h30": a bare number right after hours means minutes.
    const unit: Unit | undefined =
      m[2] !== undefined ? table.get(m[2]) : previous === 'hour' ? 'minute' : undefined;
    if (unit === undefined) return null;
    seconds += Number(m[1]) * UNIT_SECONDS[unit];
    previous = unit;
    pos = token.lastIndex;
  }
  const rounded = Math.round(seconds);
  return rounded > 0 ? rounded : null;
}

/**
 * Seconds as a duration is typed, in a language's units: "1sem 2j 3h 30min"
 * (Jira's "1w 2d 3h 30m" without units). Seconds below a minute are
 * dropped; a duration under a minute is "0" minutes.
 */
export function typedDuration(seconds: number, units?: DurationUnits): string {
  const names: DurationUnits = units ?? { week: 'w', day: 'd', hour: 'h', minute: 'm' };
  let rest = Math.max(0, Math.round(seconds));
  const parts: string[] = [];
  for (const unit of ['week', 'day', 'hour', 'minute'] as const) {
    const size = UNIT_SECONDS[unit];
    if (rest >= size) {
      parts.push(`${Math.floor(rest / size)}${names[unit]}`);
      rest %= size;
    }
  }
  return parts.length ? parts.join(' ') : `0${names.minute}`;
}

/**
 * Seconds in Jira's duration syntax, as the server reads it: "1w 2d 3h 30m"
 * (a day is 8 hours, a week 5 days). Not translated: it is what is sent,
 * whatever the language.
 */
export function jiraDuration(seconds: number): string {
  return typedDuration(seconds);
}
