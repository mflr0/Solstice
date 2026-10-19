/**
 * Form logic of the administration and personal settings: what to check
 * before sending (so mistakes show beside the field, as one types) and what
 * to send. Problems are keys; the pages word them (src/i18n).
 */

import type {
  AuditQuery,
  MailSecurity,
  MailSettings,
  MailSettingsInput,
  NotificationPrefs,
  SubscriptionInput,
  WebhookInput,
} from './admin-types.js';

/** Problems found in a form, by field. */
export type Problems<F extends string, P extends string = string> = Partial<Record<F, P>>;

export function hasProblems(p: Problems<string>): boolean {
  return Object.values(p).some(Boolean);
}

/** Whether what was typed to confirm a deletion is the phrase asked for (spaces around ignored). */
export function typedMatches(typed: string, phrase: string): boolean {
  return phrase !== '' && typed.trim() === phrase;
}

/**
 * An e-mail address as the server takes it: one address, no display name,
 * something on both sides of a single "@" (the server has the last word).
 */
export function isEmail(s: string): boolean {
  return /^[^\s@<>(),;:"]+@[^\s@<>(),;:"]+$/.test(s);
}

/** The usual port of each SMTP security mode. */
export function defaultPort(security: MailSecurity): number {
  return security === 'tls' ? 465 : security === 'starttls' ? 587 : 25;
}

/* E-mail -------------------------------------------------------------------- */

/** What happens to the stored SMTP password on save. */
export type SecretAction = 'keep' | 'replace' | 'remove';

export interface MailForm {
  enabled: boolean;
  host: string;
  port: string;
  security: MailSecurity;
  username: string;
  password: string;
  passwordAction: SecretAction;
  from: string;
  fromName: string;
}

export type MailProblem =
  'hostRequired' | 'portInvalid' | 'fromRequired' | 'fromInvalid' | 'passwordRequired';

/**
 * Whether saving would send the stored password to another server, port,
 * security mode or user name than the saved ones: the server then wants the
 * password entered again.
 */
export function mailDestinationChanged(f: MailForm, saved: MailSettings): boolean {
  const port = f.port.trim() ? Number(f.port) : defaultPort(f.security);
  const savedPort = saved.port || defaultPort(saved.security);
  return (
    f.host.trim().toLowerCase() !== saved.host.trim().toLowerCase() ||
    port !== savedPort ||
    f.security !== saved.security ||
    f.username.trim() !== saved.username
  );
}

export function mailForm(s: MailSettings): MailForm {
  return {
    enabled: s.enabled,
    host: s.host,
    port: String(s.port || defaultPort(s.security)),
    security: s.security,
    username: s.username,
    password: '',
    passwordAction: 'keep',
    from: s.from,
    fromName: s.fromName,
  };
}

/**
 * What is wrong with the form. With the saved settings, a stored password
 * that would follow a new destination must be typed again (or removed).
 */
export function mailProblems(
  f: MailForm,
  saved?: MailSettings,
): Problems<'host' | 'port' | 'from' | 'password', MailProblem> {
  const out: Problems<'host' | 'port' | 'from' | 'password', MailProblem> = {};
  if (f.enabled && !f.host.trim()) out.host = 'hostRequired';
  const port = Number(f.port);
  if (f.port.trim() && (!Number.isInteger(port) || port < 1 || port > 65535)) {
    out.port = 'portInvalid';
  }
  const from = f.from.trim();
  if (from && !isEmail(from)) out.from = 'fromInvalid';
  else if (f.enabled && !from) out.from = 'fromRequired';
  if (
    saved?.passwordSet &&
    mailDestinationChanged(f, saved) &&
    (f.passwordAction === 'keep' || (f.passwordAction === 'replace' && !f.password))
  ) {
    out.password = 'passwordRequired';
  }
  return out;
}

export function mailPayload(f: MailForm): MailSettingsInput {
  const out: MailSettingsInput = {
    enabled: f.enabled,
    host: f.host.trim(),
    port: f.port.trim() ? Number(f.port) : defaultPort(f.security),
    security: f.security,
    username: f.username.trim(),
    from: f.from.trim(),
    fromName: f.fromName.trim(),
  };
  if (f.passwordAction === 'replace') out.password = f.password;
  else if (f.passwordAction === 'remove') out.password = null;
  return out;
}

/* Webhooks ------------------------------------------------------------------ */

export interface WebhookForm {
  name: string;
  url: string;
  events: string[];
  jql: string;
  enabled: boolean;
  excludeBody: boolean;
  secret: string;
  secretAction: SecretAction;
}

export type WebhookProblem = 'nameRequired' | 'nameTooLong' | 'urlInvalid' | 'eventsRequired';

export function newWebhookForm(): WebhookForm {
  return {
    name: '',
    url: '',
    events: ['jira:issue_created', 'jira:issue_updated'],
    jql: '',
    enabled: true,
    excludeBody: false,
    secret: '',
    secretAction: 'replace',
  };
}

/** Whether an address is one a webhook can call: http(s) with a host. */
export function isHookUrl(s: string): boolean {
  try {
    const u = new URL(s.trim());
    return (u.protocol === 'http:' || u.protocol === 'https:') && u.hostname !== '';
  } catch {
    return false;
  }
}

export function webhookProblems(
  f: WebhookForm,
): Problems<'name' | 'url' | 'events', WebhookProblem> {
  const out: Problems<'name' | 'url' | 'events', WebhookProblem> = {};
  const name = f.name.trim();
  if (!name) out.name = 'nameRequired';
  else if (tooLong(name)) out.name = 'nameTooLong';
  if (!isHookUrl(f.url)) out.url = 'urlInvalid';
  if (f.events.length === 0) out.events = 'eventsRequired';
  return out;
}

export function webhookPayload(f: WebhookForm): WebhookInput {
  const out: WebhookInput = {
    name: f.name.trim(),
    url: f.url.trim(),
    events: f.events,
    jql: f.jql.trim(),
    enabled: f.enabled,
    excludeBody: f.excludeBody,
  };
  if (f.secretAction === 'replace' && f.secret !== '') out.secret = f.secret;
  else if (f.secretAction === 'remove') out.secret = '';
  return out;
}

/* Access tokens ------------------------------------------------------------- */

export type ExpiryChoice = '30' | '90' | '365' | 'date' | 'never';

export type TokenProblem = 'nameRequired' | 'nameTooLong' | 'dateRequired' | 'dateInPast';

/** "YYYY-MM-DD" of a local date. */
export function isoDay(d: Date): string {
  const p = (n: number) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}`;
}

/** The last day a token works, or null for never. */
export function tokenExpiry(choice: ExpiryChoice, date: string, today: Date): string | null {
  if (choice === 'never') return null;
  if (choice === 'date') return date;
  const d = new Date(today.getFullYear(), today.getMonth(), today.getDate() + Number(choice));
  return isoDay(d);
}

export function tokenProblems(
  name: string,
  choice: ExpiryChoice,
  date: string,
  today: Date,
): Problems<'name' | 'expires', TokenProblem> {
  const out: Problems<'name' | 'expires', TokenProblem> = {};
  const n = name.trim();
  if (!n) out.name = 'nameRequired';
  else if (tooLong(n, TOKEN_NAME_MAX)) out.name = 'nameTooLong';
  if (choice === 'date') {
    if (!/^\d{4}-\d{2}-\d{2}$/.test(date)) out.expires = 'dateRequired';
    else if (date < isoDay(today)) out.expires = 'dateInPast';
  }
  return out;
}

/* Audit log ----------------------------------------------------------------- */

const AUDIT_KEYS = ['user', 'action', 'from', 'to', 'q'] as const;

/** The audit log's filters, read from the address. */
export function auditQuery(search: string): AuditQuery {
  const sp = new URLSearchParams(search);
  const page = Number(sp.get('page'));
  return {
    user: sp.get('user') ?? '',
    action: sp.get('action') ?? '',
    from: sp.get('from') ?? '',
    to: sp.get('to') ?? '',
    q: sp.get('q') ?? '',
    page: Number.isInteger(page) && page > 1 ? page : 1,
  };
}

/** The address parameters of the audit log's filters (null removes one). */
export function auditParams(q: AuditQuery): Record<string, string | null> {
  const out: Record<string, string | null> = {};
  for (const k of AUDIT_KEYS) out[k] = q[k].trim() || null;
  out.page = q.page > 1 ? String(q.page) : null;
  return out;
}

export function auditFiltered(q: AuditQuery): boolean {
  return AUDIT_KEYS.some((k) => q[k].trim() !== '');
}

/** The family of an audit action: "user" for "user.create". */
export function actionFamily(action: string): string {
  const i = action.indexOf('.');
  return i < 0 ? action : action.slice(0, i);
}

/* Notifications ------------------------------------------------------------- */

/** The preferences to send back: the server ignores the read-only fields anyway. */
export function prefsPayload(p: NotificationPrefs): Partial<NotificationPrefs> {
  return {
    delivery: p.delivery,
    digestHour: p.digestHour,
    ownChanges: p.ownChanges,
    events: { ...p.events },
    dueReminder: p.dueReminder,
    reminderHour: p.reminderHour,
  };
}

/* Filter subscriptions ------------------------------------------------------ */

export type SubscriptionProblem = 'daysRequired' | 'hourInvalid';

export function subscriptionProblems(
  s: SubscriptionInput,
): Problems<'days' | 'hour', SubscriptionProblem> {
  const out: Problems<'days' | 'hour', SubscriptionProblem> = {};
  if (s.schedule === 'weekly' && s.days.length === 0) out.days = 'daysRequired';
  if (!Number.isInteger(s.hour) || s.hour < 0 || s.hour > 23) out.hour = 'hourInvalid';
  return out;
}

export function subscriptionPayload(s: SubscriptionInput): SubscriptionInput {
  const out: SubscriptionInput = {
    schedule: s.schedule,
    days: s.schedule === 'weekly' ? [...new Set(s.days)].sort((a, b) => a - b) : [],
    hour: s.hour,
    emptyToo: s.emptyToo,
  };
  if (s.group) out.group = s.group;
  return out;
}

/* Users ---------------------------------------------------------------------- */

/** User names as the server takes them: lower case, digits, . _ @ - (100 at most). */
export const USERNAME_PATTERN = /^[a-z0-9][a-z0-9._@-]{0,99}$/;

export interface UserForm {
  username: string;
  name: string;
  email: string;
  password: string;
  admin: boolean;
  active: boolean;
  groups: number[];
  /** Create the account without a password and e-mail its person a link to choose one. */
  invite: boolean;
}

export type UserProblem =
  'usernameRequired' | 'usernameInvalid' | 'emailInvalid' | 'emailRequired' | 'passwordShort';

export function newUserForm(): UserForm {
  return {
    username: '',
    name: '',
    email: '',
    password: '',
    admin: false,
    active: true,
    groups: [],
    invite: false,
  };
}

/**
 * Problems of a user form. A new account needs a user name and a password
 * (or, for an invitation, an e-mail address to send it to); an existing one
 * only checks what may change (e-mail).
 */
export function userProblems(
  f: UserForm,
  isNew: boolean,
  minPassword: number,
): Problems<'username' | 'email' | 'password', UserProblem> {
  const out: Problems<'username' | 'email' | 'password', UserProblem> = {};
  if (isNew) {
    const u = f.username.trim().toLowerCase();
    if (!u) out.username = 'usernameRequired';
    else if (!USERNAME_PATTERN.test(u)) out.username = 'usernameInvalid';
    if (f.invite) {
      if (!f.email.trim()) out.email = 'emailRequired';
    } else if ([...f.password].length < minPassword) {
      out.password = 'passwordShort';
    }
  }
  if (f.email.trim() && !isEmail(f.email.trim())) out.email = 'emailInvalid';
  return out;
}

/** A user name suggested from a display name: "Marie Durand" → "marie.durand". */
export function suggestUsername(name: string): string {
  return name
    .normalize('NFD')
    .replace(/[̀-ͯ]/g, '')
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9]+/g, '.')
    .replace(/^[^a-z0-9]+|[^a-z0-9]+$/g, '')
    .slice(0, 100);
}

/** Whether a list matches a typed search: every word in one of the texts. */
export function matches(search: string, ...texts: (string | null | undefined)[]): boolean {
  const words = search.toLowerCase().split(/\s+/).filter(Boolean);
  if (words.length === 0) return true;
  const hay = texts.filter(Boolean).join(' ').toLowerCase().normalize('NFD').replace(/[̀-ͯ]/g, '');
  return words.every((w) => hay.includes(w.normalize('NFD').replace(/[̀-ͯ]/g, '')));
}

/** Longest name the server takes, in characters: a 255-character column. */
export const NAME_MAX = 255;

/** Longest API token name the server takes, in characters. */
export const TOKEN_NAME_MAX = 100;

/** Longest group description: the server stores it in a 255-character column. */
export const GROUP_DESCRIPTION_MAX = 255;

/** Whether s, trimmed, has more than max characters (code points, as the server counts them). */
export function tooLong(s: string, max = NAME_MAX): boolean {
  return [...s.trim()].length > max;
}

export interface NamedForm {
  name: string;
  description: string;
}

export type NamedProblem = 'nameRequired' | 'nameTooLong' | 'descriptionTooLong';

function nameProblem(name: string): 'nameRequired' | 'nameTooLong' | undefined {
  const n = name.trim();
  if (!n) return 'nameRequired';
  return tooLong(n) ? 'nameTooLong' : undefined;
}

export function groupProblems(f: NamedForm): Problems<'name' | 'description', NamedProblem> {
  const out: Problems<'name' | 'description', NamedProblem> = {};
  const name = nameProblem(f.name);
  if (name) out.name = name;
  if (tooLong(f.description, GROUP_DESCRIPTION_MAX)) out.description = 'descriptionTooLong';
  return out;
}

export function roleProblems(f: NamedForm): Problems<'name', 'nameRequired' | 'nameTooLong'> {
  const name = nameProblem(f.name);
  return name ? { name } : {};
}

/** What to send for a group or a role: trimmed; an empty description clears it. */
export function namedPayload(f: NamedForm): { name: string; description: string } {
  return { name: f.name.trim(), description: f.description.trim() };
}

/**
 * The server stores a group name in lower case (`lowerName`), so a name that
 * differs by case alone is not a change; a description may be null.
 */
export function namedPatch(
  f: NamedForm,
  current: { name: string; description: string | null },
  lowerName = false,
): Partial<NamedForm> {
  const out: Partial<NamedForm> = {};
  const name = f.name.trim();
  if ((lowerName ? name.toLowerCase() : name) !== current.name) out.name = name;
  const description = f.description.trim();
  if (description !== (current.description ?? '')) out.description = description;
  return out;
}

/**
 * The comments and work logs that keep a group or a role from being deleted,
 * read from the server's 409 answer; null when the answer does not say.
 */
export function restrictionUsage(data: unknown): { comments: number; worklogs: number } | null {
  if (!data || typeof data !== 'object' || !('usage' in data)) return null;
  const u = data.usage;
  if (!u || typeof u !== 'object') return null;
  const count = (k: string) => {
    const n = (u as Record<string, unknown>)[k];
    return typeof n === 'number' ? n : 0;
  };
  return { comments: count('comments'), worklogs: count('worklogs') };
}

/* Security levels ------------------------------------------------------------ */

export type SecurityLevelProblem = 'nameRequired' | 'nameTooLong' | 'nameTaken';

/** A level's name: present, 255 characters at most, and not another level's of the project (any case). */
export function securityLevelProblem(
  name: string,
  levels: readonly { id: number; name: string }[],
  self: number | null,
): SecurityLevelProblem | null {
  const n = name.trim();
  if (!n) return 'nameRequired';
  if (tooLong(n)) return 'nameTooLong';
  const lower = n.toLowerCase();
  return levels.some((l) => l.id !== self && l.name.trim().toLowerCase() === lower)
    ? 'nameTaken'
    : null;
}

/** The levels the issues of a deleted level can move to: the project's other levels. */
export function levelTargets<T extends { id: number }>(levels: readonly T[], removing: T): T[] {
  return levels.filter((l) => l.id !== removing.id);
}

/** The kinds of holder a level takes (those of the permission checks), in the order offered. */
export const LEVEL_HOLDERS = [
  'group',
  'role',
  'user',
  'everyone',
  'lead',
  'reporter',
  'assignee',
] as const;

export type LevelHolder = (typeof LEVEL_HOLDERS)[number];

/** Whether a holder kind names someone: a user, a group or a role. */
export function holderNeedsId(holder: LevelHolder): boolean {
  return holder === 'user' || holder === 'group' || holder === 'role';
}

/** What to send to add a holder; null while the one it names is not chosen. */
export function memberPayload(
  holder: LevelHolder,
  id: number | null,
): { holder: LevelHolder; id?: number } | null {
  if (!holderNeedsId(holder)) return { holder };
  return id === null ? null : { holder, id };
}
