/**
 * The interface language in use. Components import from here whatever the
 * language: `t` is a live binding that boot points at the visitor's
 * catalogue (src/i18n/index.ts), and the helpers below read it. English
 * (en.ts) is the source language and the catalogue until boot.
 */

import type { Ago, Day } from '../core/dates.js';
import { ago, parseDay, splitDuration } from '../core/dates.js';
import type { JqlCode, JqlError, JqlMessages } from '../core/jql.js';
import type { Grant, Holder, Permission, StatusCategory } from '../core/types.js';
import { dateTime, formatNumber, longDay } from './format.js';
import type { Messages } from './types.js';
import { en } from './en.js';

/** "12 mars 2026", "1er avril 2026", "March 12, 2026". */
export function renderDay(day: Day): string {
  return longDay(day);
}

export function renderAgo(a: Ago): string {
  switch (a.kind) {
    case 'now':
      return t.time.justNow;
    case 'minutes':
      return t.time.minutesAgo(a.n);
    case 'hours':
      return t.time.hoursAgo(a.n);
    case 'yesterday':
      return t.time.yesterday;
    case 'days':
      return t.time.daysAgo(a.n);
    case 'date':
      return longDay(a.day);
  }
}

/** Relative wording for an instant, e.g. "il y a 3 h". */
export function relative(iso: string, now?: Date): string {
  return renderAgo(ago(iso, now));
}

/** "12 mars 2026 à 14:05". */
export function formatDateTime(iso: string): string {
  return dateTime(iso);
}

/** A due date, "YYYY-MM-DD" → "12 mars 2026". */
export function formatDay(s: string): string {
  const d = parseDay(s);
  return d ? longDay(d) : s;
}

/** "2 h 30", "2 h", "45 min". */
export function formatDuration(seconds: number): string {
  const { h, m } = splitDuration(seconds);
  return t.time.duration(h, m);
}

/** "820 o", "12 Ko", "1,4 Mo". */
export function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} ${t.units.bytes}`;
  if (bytes < 1024 * 1024) return `${formatNumber(bytes / 1024, 1)} ${t.units.kilobytes}`;
  return `${formatNumber(bytes / (1024 * 1024), 1)} ${t.units.megabytes}`;
}

export function grantLabel(g: Grant): string {
  switch (g.holder) {
    case 'user':
    case 'group':
    case 'role':
      return g.name ? `${g.name}` : `${holderLabels[g.holder]} ${g.id ?? ''}`;
    default:
      return holderLabels[g.holder];
  }
}

/** A history entry's field (Jira's names, lower case), as Solstice shows it. */
export function fieldLabel(field: string): string {
  return (t.historyFields as Record<string, string>)[field.toLowerCase()] ?? field;
}

/** A JQL syntax error, in the language in use. */
export function renderJqlError<C extends JqlCode>(e: JqlError<C>): string {
  const messages: JqlMessages = t.jql;
  return messages[e.code](...e.args);
}

/** History fields whose values are durations in seconds. */
export const DURATION_FIELDS = new Set(['timeoriginalestimate', 'timeestimate', 'timespent']);

/** The catalogue in use: English until boot points it elsewhere (src/i18n/index.ts). */
export let t: Messages = en;

export let categoryLabels: Record<StatusCategory, string> = en.categories;
export let permissionLabels: Record<Permission, { label: string; help: string }> = en.permissions;
export let holderLabels: Record<Holder, string> = en.holders;

/** Makes a catalogue the one every component reads. Boot only: pages do not re-render. */
export function useMessages(m: Messages): void {
  t = m;
  categoryLabels = m.categories;
  permissionLabels = m.permissions;
  holderLabels = m.holders;
}
