/**
 * Shapes of the administration and personal-settings API (mail, webhooks,
 * access tokens, audit log, notification preferences, filter subscriptions).
 * Dates are RFC 3339 strings in UTC.
 */

import type { IdName, UserRef } from './types.js';

/* E-mail -------------------------------------------------------------------- */

export type MailSecurity = 'none' | 'starttls' | 'tls';

export interface MailSettings {
  enabled: boolean;
  host: string;
  port: number;
  security: MailSecurity;
  username: string;
  from: string;
  fromName: string;
  /** Whether a password is stored; the password itself never comes back. */
  passwordSet: boolean;
  /** The public address links in e-mails use; null when not set. */
  baseUrl: string | null;
}

/** What saving the SMTP settings sends; password left out keeps the stored one, null removes it. */
export interface MailSettingsInput {
  enabled: boolean;
  host: string;
  port: number;
  security: MailSecurity;
  username: string;
  from: string;
  fromName: string;
  password?: string | null;
}

export interface MailEntry {
  id: number;
  kind: string;
  to: string;
  subject: string;
  status: 'pending' | 'sent' | 'failed' | string;
  attempts: number;
  lastError: string | null;
  created: string;
  nextAttempt: string | null;
  sent: string | null;
}

export interface MailQueue {
  pending: number;
  failed: number;
  sentLastDay: number;
  oldestPending: string | null;
  recent: MailEntry[];
  errors: MailEntry[];
  /** Changes waiting to be grouped into one e-mail. */
  batching: number;
  /** Daily digests waiting for their hour. */
  digests: number;
}

/* Webhooks ------------------------------------------------------------------ */

export interface WebhookDelivery {
  id: number;
  event: string;
  status: 'pending' | 'sending' | 'sent' | 'failed';
  statusCode: number | null;
  durationMs: number | null;
  attempts: number;
  error: string | null;
  created: string;
  attempted: string | null;
  nextAttempt: string | null;
}

export interface Webhook {
  id: number;
  name: string;
  url: string;
  events: string[];
  jql: string;
  hasSecret: boolean;
  enabled: boolean;
  excludeBody: boolean;
  created: string;
  updated: string;
  lastDelivery: WebhookDelivery | null;
}

/** What saving a webhook sends; secret left out keeps it, "" removes it. */
export interface WebhookInput {
  name: string;
  url: string;
  events: string[];
  jql: string;
  enabled: boolean;
  excludeBody: boolean;
  secret?: string;
}

/* Access tokens ------------------------------------------------------------- */

export interface AccessToken {
  id: number;
  name: string;
  created: string;
  expires: string | null;
  lastUsed: string | null;
  expired: boolean;
  /** The owner, in the administrators' list. */
  user?: UserRef;
  /** The secret, only in the answer that created it. */
  token?: string;
}

/* Audit log ----------------------------------------------------------------- */

export interface AuditEntry {
  id: number;
  created: string;
  /** The account, if it still exists. */
  user: UserRef | null;
  /** The user name as recorded (a failed sign-in may name no account). */
  username: string | null;
  ip: string | null;
  action: string;
  target: { type: string; id: string | null; label: string | null } | null;
  details: Record<string, unknown> | null;
}

export interface AuditPage {
  total: number;
  page: number;
  size: number;
  entries: AuditEntry[];
}

export interface AuditQuery {
  user: string;
  action: string;
  from: string;
  to: string;
  q: string;
  page: number;
}

/* Notifications ------------------------------------------------------------- */

export type Delivery = 'immediate' | 'digest' | 'none';

export interface NotificationPrefs {
  delivery: Delivery;
  digestHour: number;
  ownChanges: boolean;
  /** Event group → whether it sends an e-mail. */
  events: Record<string, boolean>;
  dueReminder: boolean;
  reminderHour: number;
  /** The time zone the hours are in (read-only). */
  timeZone: string;
  /** Whether this Solstice sends e-mail at all (read-only). */
  mailEnabled: boolean;
}

export interface ExtraRecipient {
  event: string;
  kind: 'group' | 'role';
  id: number;
  name: string;
}

export interface ProjectNotifications {
  enabled: boolean;
  extra: ExtraRecipient[];
  /** The event groups a project may send to extra people (read-only). */
  events: string[];
}

export type Schedule = 'daily' | 'weekly';

export interface Subscription {
  id: number;
  filterId: number;
  owner: UserRef | null;
  user: UserRef | null;
  group: IdName | null;
  schedule: Schedule;
  /** ISO weekdays, 1 = Monday. */
  days: number[];
  hour: number;
  emptyToo: boolean;
  lastRun: string | null;
  nextRun: string;
  canEdit: boolean;
}

export interface SubscriptionInput {
  schedule: Schedule;
  days: number[];
  hour: number;
  emptyToo: boolean;
  group?: number;
}

/* General settings ---------------------------------------------------------- */

/** Where a setting's value in effect comes from. */
export type SettingSource = 'database' | 'env' | 'file' | 'flag' | 'default';

export interface BannerSettings {
  enabled: boolean;
  text: string;
  level: 'info' | 'warning';
}

export interface GeneralSettings {
  instanceName: string;
  /** Public address, "" when unknown. */
  baseURL: string;
  /** IANA name, "" for the server's local zone. */
  timeZone: string;
  defaultLanguage: string;
  banner: BannerSettings;
  sessionDays: number;
  passwordMinLength: number;
  sources: Partial<Record<GeneralField, SettingSource>>;
  languages: string[];
}

export type GeneralField =
  | 'instanceName'
  | 'baseURL'
  | 'timeZone'
  | 'defaultLanguage'
  | 'banner'
  | 'sessionDays'
  | 'passwordMinLength';

/** A change to the settings: null puts a field back to its inherited value. */
export type GeneralSettingsInput = {
  [K in GeneralField]?: GeneralSettings[K] | null;
};

/* Users, groups, roles ------------------------------------------------------ */

export interface UserRow {
  id: number;
  username: string;
  name: string;
  email: string;
  active: boolean;
  admin: boolean;
  hasPassword: boolean;
  created: string;
  /** Absent on older servers. */
  lastLogin?: string | null;
  groups?: IdName[];
  /** In the detail only: personal access tokens. */
  tokens?: number;
}

export interface UserInput {
  username: string;
  name: string;
  email: string;
  password: string;
  admin: boolean;
  groups?: number[];
}

export interface Group {
  id: number;
  name: string;
  description: string;
  /** Number of members. */
  members: number;
}

export interface GroupUsage {
  grants: number;
  roles: number;
  securityLevels: number;
  comments: number;
  worklogs: number;
}

export interface GroupDetail {
  id: number;
  name: string;
  description: string;
  members: UserRef[];
  usage: GroupUsage;
}

export interface RoleRow {
  id: number;
  name: string;
  description: string;
  /** Projects with members in the role. */
  projects: number;
  /** Grants naming the role. */
  grants: number;
}

export interface ProjectRole {
  id: number;
  name: string;
  description: string | null;
  users: UserRef[];
  groups: IdName[];
}

/* Projects ------------------------------------------------------------------ */

export type ProjectTemplate = 'software' | 'simple';

export type ProjectAccess =
  { holder: 'group'; id: number } | { holder: 'role'; id: number } | { holder: 'everyone' } | null;

export interface ProjectInput {
  key: string;
  name: string;
  description?: string;
  lead?: number;
  category?: number | null;
  template: ProjectTemplate;
  access?: ProjectAccess;
  issueTypes?: string[];
  defaultAssignee?: 'unassigned' | 'lead';
}

export interface ProjectPatch {
  name?: string;
  description?: string | null;
  lead?: number;
  category?: number | null;
  defaultAssignee?: 'unassigned' | 'lead';
  issueTypes?: string[];
}

export interface KeyCheck {
  key: string;
  available: boolean;
  reason?: string;
}

export interface Category {
  id: number;
  name: string;
  description: string | null;
  /** Number of projects in it. */
  projects: number;
}

export type AssigneeType = 'project_default' | 'component_lead' | 'project_lead' | 'unassigned';

export interface ComponentRow {
  id: number;
  name: string;
  description: string | null;
  lead: UserRef | null;
  assigneeType: AssigneeType;
  archived: boolean;
  /** Issues with the component. */
  issues: number;
}

export interface ComponentInput {
  name?: string;
  description?: string;
  lead?: number | null;
  assigneeType?: AssigneeType;
  archived?: boolean;
}

export interface VersionRow {
  id: number;
  name: string;
  description: string | null;
  startDate: string | null;
  releaseDate: string | null;
  released: boolean;
  archived: boolean;
  position: number;
  issues: { total: number; unresolved: number };
}

export interface VersionInput {
  name?: string;
  description?: string;
  startDate?: string | null;
  releaseDate?: string | null;
  released?: boolean;
  archived?: boolean;
}

/* Catalogues ---------------------------------------------------------------- */

export type CatalogKind = 'issue-types' | 'priorities' | 'resolutions' | 'link-types';

export interface CatalogItem {
  id: string | number;
  name: string;
  description?: string;
  position: number;
  /** Issue types. */
  subtask?: boolean;
  /** Priorities. */
  color?: string;
  /** Priorities and resolutions. */
  default?: boolean;
  /** Link types. */
  inward?: string;
  outward?: string;
  /** Usage, as the catalogue counts it (issues, projects, links). */
  issues?: number;
  projects?: number;
  links?: number;
}

export interface StatusRow {
  id: string;
  name: string;
  category: 'todo' | 'doing' | 'done';
  position: number;
  usage: { issues: number; workflows: number; boardColumns: number };
}

/* Custom fields ------------------------------------------------------------- */

export type FieldType =
  | 'text'
  | 'textarea'
  | 'number'
  | 'date'
  | 'datetime'
  | 'select'
  | 'multiselect'
  | 'checkboxes'
  | 'radio'
  | 'user'
  | 'multiuser'
  | 'labels'
  | 'url';

export interface FieldContext {
  projects: { id: number; key: string; name: string }[];
  issueTypes: { id: string; name: string; subtask: boolean }[];
}

export interface FieldOption {
  id: number;
  value: string;
  disabled: boolean;
  position: number;
}

export interface FieldRow {
  id: number;
  name: string;
  description: string | null;
  /** null: a Jira type Solstice only displays. */
  type: FieldType | null;
  typeKey: string;
  /** A count in the list, the options in the detail. */
  options: number | FieldOption[];
  /** Issues with a value. */
  issues: number;
  context: FieldContext;
}

export interface FieldInput {
  name?: string;
  description?: string | null;
  type?: FieldType;
  options?: string[];
  context?: { projects: string[]; issueTypes: string[] };
}

/* System and import --------------------------------------------------------- */

export interface SystemInfo {
  version: string;
  goVersion: string;
  startedAt: string;
  uptimeSeconds: number;
  database: { server: string; name: string; sizeBytes: number };
  dataDir: { path: string; sizeBytes: number; error: string | null };
  counts: {
    usersActive: number;
    usersInactive: number;
    projects: number;
    projectsArchived: number;
    issues: number;
    comments: number;
    attachments: number;
    attachmentBytes: number;
  };
  mail: {
    enabled: boolean;
    pending: number;
    failed: number;
    sentLastDay: number;
    oldestPending: string | null;
  };
  webhooks: {
    total: number;
    failing: { id: number; name: string; failed: number; lastError: string; lastAttempt: string }[];
  };
  migrations: { count: number; latest: string; list: { version: string; appliedAt: string }[] };
  config: {
    path: string;
    found: boolean;
    sources: Record<string, SettingSource>;
    addr: string;
  };
  import: ImportView | null;
}

export interface ImportView {
  state: 'running' | 'done' | 'failed';
  phase: string;
  done: number;
  total: number;
  rows: number;
  steps: { name: string; imported: number; skipped: number }[];
  warnings: string[];
  error?: string;
  seconds: number;
}

export interface ImportSource {
  url: string;
  jiraHome: string;
  tz: string;
}

export interface ImportCheck {
  engine: string;
  projects: number;
  issues: number;
  users: number;
  attachments: number;
  warning?: string;
}
