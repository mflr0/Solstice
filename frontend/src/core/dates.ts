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

/** Today as "YYYY-MM-DD" in local time. */
export function todayISO(now: Date = new Date()): string {
  const d = dayOf(now);
  return `${d.y}-${String(d.m).padStart(2, '0')}-${String(d.d).padStart(2, '0')}`;
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

/**
 * Reads a duration typed by someone logging work: "2h30", "2 h 30",
 * "1,5 h", "45 min", "45m", "1j 2h", "1w 2d" or a bare number of hours
 * ("2"). A day is 8 hours and a week 5 days, as in Jira. Returns seconds, or
 * null when the text is not a positive duration.
 */
export function parseDuration(text: string): number | null {
  const s = text.toLowerCase().replace(/,/g, '.').trim();
  if (s === '') return null;
  if (/^\d+(\.\d+)?$/.test(s)) {
    const seconds = Math.round(Number(s) * 3600);
    return seconds > 0 ? seconds : null;
  }
  // Spaces may separate a number from its unit, never two numbers' digits.
  const token = /\s*(\d+(?:\.\d+)?)\s*(j|d|h|min|mn|m|w)?\s*/y;
  let seconds = 0;
  let previousUnit = '';
  let pos = 0;
  while (pos < s.length) {
    token.lastIndex = pos;
    const m = token.exec(s);
    if (!m) return null;
    const value = Number(m[1]);
    // "2h30": a bare number right after hours means minutes.
    const unit = m[2] ?? (previousUnit === 'h' ? 'min' : '');
    if (unit === '') return null;
    switch (unit) {
      case 'w':
        seconds += value * WORKWEEK_DAYS * WORKDAY_HOURS * 3600;
        break;
      case 'j':
      case 'd':
        seconds += value * WORKDAY_HOURS * 3600;
        break;
      case 'h':
        seconds += value * 3600;
        break;
      default:
        seconds += value * 60;
    }
    previousUnit = unit;
    pos = token.lastIndex;
  }
  const rounded = Math.round(seconds);
  return rounded > 0 ? rounded : null;
}

/**
 * Seconds in Jira's duration syntax, as the server reads it: "1w 2d 3h 30m"
 * (a day is 8 hours, a week 5 days). Seconds below a minute are dropped; a
 * duration under a minute is "0m". Not translated: it is what is typed and
 * sent, whatever the language.
 */
export function jiraDuration(seconds: number): string {
  let rest = Math.max(0, Math.round(seconds));
  const parts: string[] = [];
  for (const [size, unit] of [
    [WORKWEEK_DAYS * WORKDAY_HOURS * 3600, 'w'],
    [WORKDAY_HOURS * 3600, 'd'],
    [3600, 'h'],
    [60, 'm'],
  ] as const) {
    if (rest >= size) {
      parts.push(`${Math.floor(rest / size)}${unit}`);
      rest %= size;
    }
  }
  return parts.length ? parts.join(' ') : '0m';
}
