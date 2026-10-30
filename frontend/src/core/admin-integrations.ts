/**
 * Logic of the integration pages of the administration: webhook events in
 * groups, how a delivery went, access tokens' state, and the audit log's
 * details made readable. Wording lives in i18n; this module decides what
 * to say.
 */

import type { AccessToken, WebhookDelivery } from './admin-types.js';

/* Webhook events ------------------------------------------------------------ */

export type EventGroup = 'issues' | 'comments' | 'worklogs' | 'sprints' | 'boards' | 'other';

const GROUP_OF_PREFIX: Record<string, EventGroup> = {
  issue: 'issues',
  comment: 'comments',
  worklog: 'worklogs',
  sprint: 'sprints',
  board: 'boards',
};

const GROUP_ORDER: EventGroup[] = ['issues', 'comments', 'worklogs', 'sprints', 'boards', 'other'];

/** The group of a webhook event: "jira:issue_created" → issues. */
export function eventGroup(event: string): EventGroup {
  const name = event.replace(/^jira:/, '');
  const prefix = name.split('_')[0] ?? '';
  return GROUP_OF_PREFIX[prefix] ?? 'other';
}

/** The events the server offers, in groups, each keeping the server's order. */
export function groupEvents(events: readonly string[]): { group: EventGroup; events: string[] }[] {
  const out: { group: EventGroup; events: string[] }[] = [];
  for (const g of GROUP_ORDER) {
    const list = events.filter((e) => eventGroup(e) === g);
    if (list.length) out.push({ group: g, events: list });
  }
  return out;
}

/** Whether every, some or none of a group's events are chosen. */
export function groupState(
  chosen: readonly string[],
  group: readonly string[],
): 'all' | 'some' | 'none' {
  const n = group.filter((e) => chosen.includes(e)).length;
  return n === 0 ? 'none' : n === group.length ? 'all' : 'some';
}

/**
 * Ticks or unticks a whole group, keeping the chosen events in the
 * server's order (`all`).
 */
export function toggleGroup(
  chosen: readonly string[],
  group: readonly string[],
  on: boolean,
  all: readonly string[],
): string[] {
  const set = new Set(chosen);
  for (const e of group) {
    if (on) set.add(e);
    else set.delete(e);
  }
  return all.filter((e) => set.has(e)).concat([...set].filter((e) => !all.includes(e)));
}

/** Ticks or unticks one event, keeping the server's order. */
export function toggleEvent(
  chosen: readonly string[],
  event: string,
  on: boolean,
  all: readonly string[],
): string[] {
  return toggleGroup(chosen, [event], on, all);
}

/* Deliveries ---------------------------------------------------------------- */

export type DeliveryTone = 'ok' | 'waiting' | 'failed';

/**
 * How a delivery went: sent with a 2xx answer, still waiting (queued,
 * sending, or to be retried), or failed for good.
 */
export function deliveryTone(d: Pick<WebhookDelivery, 'status' | 'statusCode'>): DeliveryTone {
  if (d.status === 'failed') return 'failed';
  if (d.status === 'sent') {
    return d.statusCode === null || (d.statusCode >= 200 && d.statusCode < 300) ? 'ok' : 'failed';
  }
  return 'waiting';
}

/** The stamp class of a tone (src/themes/ANATOMY.md: .stamp.done, .doing, .is-failed). */
export function toneClass(tone: DeliveryTone): string {
  return tone === 'ok' ? 'done' : tone === 'waiting' ? 'doing' : 'is-failed';
}

/* Access tokens ------------------------------------------------------------- */

export type TokenState = 'active' | 'expiring' | 'expired';

const WEEK = 7 * 24 * 60 * 60 * 1000;

/** Whether a token works, expires within a week, or no longer works. */
export function tokenState(tok: Pick<AccessToken, 'expires' | 'expired'>, now: Date): TokenState {
  if (tok.expired) return 'expired';
  if (!tok.expires) return 'active';
  const left = new Date(tok.expires).getTime() - now.getTime();
  if (left <= 0) return 'expired';
  return left <= WEEK ? 'expiring' : 'active';
}

/* Audit log ----------------------------------------------------------------- */

/** The action families the log's filter offers, in the order shown. */
export const AUDIT_FAMILIES = [
  'login',
  'password',
  'token',
  'oauth',
  'user',
  'admin',
  'group',
  'role',
  'project',
  'category',
  'component',
  'version',
  'workflow',
  'status',
  'board',
  'sprint',
  'filter',
  'issue',
  'attachment',
  'issuetype',
  'priority',
  'resolution',
  'linktype',
  'field',
  'settings',
  'import',
  'update',
  'setup',
  'webhook',
  'securitylevel',
] as const;

export type AuditFamily = (typeof AUDIT_FAMILIES)[number];

/** A readable line of an entry's details. */
export type DetailLine =
  | { kind: 'change'; key: string; from: unknown; to: unknown }
  | { kind: 'value'; key: string; value: unknown };

function isRecord(v: unknown): v is Record<string, unknown> {
  return typeof v === 'object' && v !== null && !Array.isArray(v);
}

function isChange(v: unknown): v is { from: unknown; to: unknown } {
  return isRecord(v) && 'from' in v && 'to' in v && Object.keys(v).length === 2;
}

function same(a: unknown, b: unknown): boolean {
  return JSON.stringify(a) === JSON.stringify(b);
}

/**
 * An entry's details as lines: changes ("name: A → B") and values
 * ("method: form"). Nested objects are flattened one level ("source"
 * under "baseURL" becomes "baseURL.source"); a whole-entry {from, to} of
 * objects (a status renamed) becomes one change per field that moved.
 */
export function detailLines(details: Record<string, unknown> | null | undefined): DetailLine[] {
  if (!details) return [];
  if (isChange(details)) {
    const { from, to } = details;
    if (isRecord(from) && isRecord(to)) {
      const keys = [...new Set([...Object.keys(from), ...Object.keys(to)])];
      return keys
        .filter((k) => !same(from[k], to[k]))
        .map((k) => ({ kind: 'change', key: k, from: from[k], to: to[k] }));
    }
    return [{ kind: 'change', key: '', from, to }];
  }
  const out: DetailLine[] = [];
  for (const [key, v] of Object.entries(details)) {
    if (isChange(v)) {
      out.push({ kind: 'change', key, from: v.from, to: v.to });
    } else if (isRecord(v)) {
      for (const [sub, w] of Object.entries(v)) {
        const k = `${key}.${sub}`;
        if (isChange(w)) out.push({ kind: 'change', key: k, from: w.from, to: w.to });
        else out.push({ kind: 'value', key: k, value: w });
      }
    } else {
      out.push({ kind: 'value', key, value: v });
    }
  }
  return out;
}

/**
 * A detail value as text: lists joined, empty as "—", anything else
 * compact. Booleans are left to the page (yes / no in its language).
 */
export function plainValue(v: unknown, yes = 'true', no = 'false', list = ', '): string {
  if (v === null || v === undefined || v === '') return '—';
  if (typeof v === 'boolean') return v ? yes : no;
  if (typeof v === 'string' || typeof v === 'number') return String(v);
  if (Array.isArray(v))
    return v.length ? v.map((x) => plainValue(x, yes, no, list)).join(list) : '—';
  return JSON.stringify(v);
}

/**
 * Whether a new retention period purges entries at once: it does when it
 * becomes shorter, or when entries were kept forever (0) until now.
 */
export function retentionShortens(before: number, after: number): boolean {
  return after > 0 && (before === 0 || after < before);
}
