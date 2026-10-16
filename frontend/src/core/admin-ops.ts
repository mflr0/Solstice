/**
 * Logic of the operations pages: the audit log (every action the server
 * writes, retention), access tokens (policy, filtering), the system page
 * (uptime), the Jira import's confirmation and the filter subscriptions'
 * schedule. Wording lives in i18n; this module decides what to say.
 */

import type {
  AccessToken,
  AuditEntry,
  Schedule,
  Subscription,
  SubscriptionInput,
  TokenSettings,
} from './admin-types.js';
import { tokenState, type TokenState } from './admin-integrations.js';

/* Audit log ----------------------------------------------------------------- */

/** The shortest retention the server takes (0, keeping everything, aside). */
export const AUDIT_MIN_RETENTION = 30;
export const AUDIT_MAX_RETENTION = 36500;

const CATALOGUE_KINDS = ['issuetype', 'priority', 'resolution', 'linktype'] as const;

/**
 * Every action the server writes to the audit log (backend/internal/audit,
 * and the update and setup actions of the api package), in the order the
 * log's filter groups them. The test checks it against the Go sources.
 */
export const AUDIT_ACTIONS: readonly string[] = [
  'login.success',
  'login.failure',
  'password.change',
  'password.reset',
  'token.create',
  'token.revoke',
  'user.create',
  'user.update',
  'user.deactivate',
  'user.activate',
  'admin.grant',
  'admin.revoke',
  'group.create',
  'group.update',
  'group.delete',
  'group.members',
  'role.create',
  'role.update',
  'role.delete',
  'project.create',
  'project.update',
  'project.archive',
  'project.unarchive',
  'project.delete',
  'project.permissions',
  'project.roles',
  'category.create',
  'category.update',
  'category.delete',
  'component.create',
  'component.update',
  'component.delete',
  'version.create',
  'version.update',
  'version.order',
  'version.release',
  'version.merge',
  'version.delete',
  'workflow.create',
  'workflow.update',
  'workflow.delete',
  'status.create',
  'status.update',
  'status.delete',
  'status.order',
  'board.create',
  'board.update',
  'board.delete',
  'filter.share',
  ...CATALOGUE_KINDS.flatMap((k) =>
    ['create', 'update', 'delete', 'order'].map((v) => `${k}.${v}`),
  ),
  'field.create',
  'field.update',
  'field.delete',
  'field.options',
  'settings.general',
  'settings.mail',
  'settings.audit',
  'settings.tokens',
  'webhook.create',
  'webhook.update',
  'webhook.delete',
  'import.start',
  'import.done',
  'import.failed',
  'update.check',
  'update.settings',
  'update.request',
  'update.denied',
  'update.result',
  'setup.complete',
];

export type RetentionProblem = 'invalid' | 'tooShort';

/** What is wrong with a typed retention period (a whole number of days), if anything. */
export function retentionProblem(raw: string): RetentionProblem | null {
  const s = raw.trim();
  if (!/^\d{1,6}$/.test(s)) return 'invalid';
  const days = Number(s);
  if (days > AUDIT_MAX_RETENTION) return 'invalid';
  return days !== 0 && days < AUDIT_MIN_RETENTION ? 'tooShort' : null;
}

/** The actor of an entry as one line: the account's name, else the name recorded, else null. */
export function auditActor(e: Pick<AuditEntry, 'user' | 'username'>): {
  name: string;
  username: string;
} | null {
  if (e.user) return { name: e.user.name, username: e.user.username };
  return e.username ? { name: e.username, username: e.username } : null;
}

/* System -------------------------------------------------------------------- */

/** An uptime in days, hours and minutes (seconds are not shown). */
export function splitUptime(seconds: number): { days: number; hours: number; minutes: number } {
  const total = Math.max(0, Math.floor(seconds / 60));
  return {
    days: Math.floor(total / 1440),
    hours: Math.floor((total % 1440) / 60),
    minutes: total % 60,
  };
}

/* Access tokens ------------------------------------------------------------- */

export type TokenShow = 'all' | 'active' | 'expiring' | 'expired';
export const TOKEN_SHOWS: readonly TokenShow[] = ['all', 'active', 'expiring', 'expired'];

/**
 * The tokens an administrator's search keeps: text in the owner's name or
 * user name or in the token's name, and a state ("active" counts the ones
 * about to expire).
 */
export function filterTokens(
  list: readonly AccessToken[],
  q: string,
  show: TokenShow,
  now: Date,
): AccessToken[] {
  const needle = q.trim().toLowerCase();
  return list.filter((tok) => {
    if (needle) {
      const text = [tok.name, tok.user?.name, tok.user?.username].join('\n').toLowerCase();
      if (!text.includes(needle)) return false;
    }
    if (show === 'all') return true;
    const state: TokenState = tokenState(tok, now);
    return show === 'active' ? state !== 'expired' : state === show;
  });
}

/** What is wrong with the choice of who creates tokens, if anything. */
export function creatorsProblem(
  who: TokenSettings['who'],
  group: number | null,
): 'groupRequired' | null {
  return who === 'group' && !group ? 'groupRequired' : null;
}

/** Whether two policies are the same choice. */
export function sameCreators(
  saved: TokenSettings,
  who: TokenSettings['who'],
  group: number | null,
): boolean {
  return saved.who === who && (who !== 'group' || saved.group?.id === group);
}

/* Jira import --------------------------------------------------------------- */

/**
 * Whether a replacing import may start: the source tested, the backup
 * confirmed, and the database's name typed as the server asks.
 */
export function importReady(c: {
  tested: boolean;
  backup: boolean;
  typed: string;
  database: string;
}): boolean {
  return c.tested && c.backup && c.database !== '' && c.typed.trim() === c.database;
}

/* Subscriptions ------------------------------------------------------------- */

/** ISO weekdays, Monday first. */
export const WEEKDAYS: readonly number[] = [1, 2, 3, 4, 5, 6, 7];

/** A new subscription: every day at 8. */
export function newSubscription(): SubscriptionInput {
  return { schedule: 'daily', days: [], hour: 8, emptyToo: false };
}

/** The form's value of an existing subscription. */
export function subscriptionInputOf(
  s: Pick<Subscription, 'schedule' | 'days' | 'hour' | 'emptyToo'>,
): SubscriptionInput {
  return { schedule: s.schedule, days: [...s.days], hour: s.hour, emptyToo: s.emptyToo };
}

/** Switching to weekly with no day chosen picks Monday, as a start. */
export function withSchedule(s: SubscriptionInput, schedule: Schedule): SubscriptionInput {
  const days = schedule === 'weekly' && s.days.length === 0 ? [1] : s.days;
  return { ...s, schedule, days };
}

/** Ticks or unticks a weekday, keeping the days in week order. */
export function toggleDay(days: readonly number[], day: number, on: boolean): number[] {
  const set = new Set(days);
  if (on) set.add(day);
  else set.delete(day);
  return [...set].sort((a, b) => a - b);
}

/** "08:00". */
export function hourText(hour: number): string {
  return `${String(hour).padStart(2, '0')}:00`;
}
