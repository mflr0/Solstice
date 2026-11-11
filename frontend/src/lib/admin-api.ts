/**
 * The administration and personal-settings API, typed: e-mail, webhooks,
 * access tokens, audit log, notification preferences and filter
 * subscriptions. Same client as the rest of the interface (src/lib/api.ts).
 */

import type {
  AccessToken,
  AdminOAuthGrant,
  AuditPage,
  BuiltinHolder,
  CatalogInput,
  CatalogItem,
  CatalogKind,
  Category,
  ComponentInput,
  ComponentRow,
  FieldInput,
  FieldRow,
  FieldTypeInfo,
  GlobalWorkflow,
  GeneralSettings,
  GeneralSettingsInput,
  Group,
  GroupDetail,
  IssueLayout,
  ImportCheck,
  ImportSource,
  ImportView,
  KeyCheck,
  LayoutField,
  LayoutForm,
  Onboarding,
  ProjectInput,
  ProjectPatch,
  ProjectGroup,
  StatusRow,
  StatusUsage,
  SystemInfo,
  UserInput,
  UserRow,
  VersionInput,
  VersionRow,
  AuditQuery,
  MailQueue,
  MailSettings,
  MailSettingsInput,
  NotificationPrefs,
  ProjectNotifications,
  MySubscription,
  Subscription,
  SubscriptionInput,
  OAuthGrant,
  OAuthRequest,
  PermissionRule,
  TokenScope,
  TokenSettings,
  UserAccess,
  Webhook,
  WebhookDelivery,
  WebhookInput,
} from '../core/admin-types.js';
import type { BoardKind } from '../core/board-types.js';
import type { AdminTheme, AdminThemes } from '../core/admin-themes.js';
import type { AdminProjectRow, ProjectSettings } from '../core/project-admin.js';
import type { IdName, Me, Preferences, UserRef, WorkflowInput } from '../core/types.js';
import { query, request } from './api.js';

const enc = encodeURIComponent;

/** The SMTP settings, and what keeps them from sending: saved with a username, no encryption, a remote server. */
export type MailSettingsReply = MailSettings & { problem?: 'credentialsNeedTLS' };

/** Who may see the issues at a security level: one holder, with the user or group it names. */
export interface SecurityMember {
  id: number;
  holder: 'everyone' | 'user' | 'group' | 'lead' | 'reporter' | 'assignee';
  user: UserRef | null;
  group: IdName | null;
}

/** An issue security level of a project. */
export interface SecurityLevel {
  id: number;
  name: string;
  description: string | null;
  /** Issues at the level. */
  issues: number;
  memberCount: number;
  members: SecurityMember[];
}

export interface SecurityLevelInput {
  name?: string;
  description?: string | null;
}

export const adminApi = {
  // General settings.
  settings: () => request<GeneralSettings>('GET', '/api/admin/settings'),
  saveSettings: (input: GeneralSettingsInput) =>
    request<GeneralSettings>('PUT', '/api/admin/settings', input),
  onboarding: () => request<Onboarding>('GET', '/api/admin/onboarding'),
  saveOnboarding: (dismissed: boolean) =>
    request<Onboarding>('PUT', '/api/admin/onboarding', { dismissed }),

  // Users.
  users: (q: string) => request<UserRow[]>('GET', `/api/admin/users${query({ q })}`),
  user: (id: number) => request<UserRow>('GET', `/api/admin/users/${id}`),
  createUser: (u: UserInput) => request<{ id: number }>('POST', '/api/admin/users', u),
  updateUser: (
    id: number,
    patch: { name?: string; email?: string; active?: boolean; admin?: boolean; password?: string },
  ) => request<void>('PATCH', `/api/admin/users/${id}`, patch),
  setUserGroups: (id: number, groups: number[]) =>
    request<UserRow>('PUT', `/api/admin/users/${id}/groups`, { groups }),
  resetPassword: (id: number) =>
    request<{ password: string }>('POST', `/api/admin/users/${id}/reset-password`),
  /** Mails the account a link to choose its own password; nothing changes until it is used. */
  sendResetLink: (id: number) =>
    request<{ sent: boolean }>('POST', `/api/admin/users/${id}/reset-password`, { method: 'link' }),
  /** Sends the invitation of an account that never signed in again. */
  inviteUser: (id: number) => request<{ sent: boolean }>('POST', `/api/admin/users/${id}/invite`),

  // Groups.
  groups: (q = '') => request<Group[]>('GET', `/api/admin/groups${query({ q })}`),
  group: (id: number) => request<GroupDetail>('GET', `/api/admin/groups/${id}`),
  createGroup: (g: { name: string; description?: string }) =>
    request<Group>('POST', '/api/admin/groups', g),
  updateGroup: (id: number, g: { name?: string; description?: string }) =>
    request<Group>('PATCH', `/api/admin/groups/${id}`, g),
  deleteGroup: (id: number) => request<void>('DELETE', `/api/admin/groups/${id}`),
  addMembers: (id: number, users: number[]) =>
    request<GroupDetail>('POST', `/api/admin/groups/${id}/members`, { users }),
  removeMember: (id: number, user: number) =>
    request<void>('DELETE', `/api/admin/groups/${id}/members/${user}`),
  groupPermissions: (id: number) =>
    request<PermissionRule[]>('GET', `/api/admin/groups/${id}/permissions`),
  setGroupPermissions: (id: number, rules: PermissionRule[]) =>
    request<PermissionRule[]>('PUT', `/api/admin/groups/${id}/permissions`, rules),
  builtinPermissions: (holder: BuiltinHolder) =>
    request<PermissionRule[]>('GET', `/api/admin/builtin-holders/${holder}/permissions`),
  setBuiltinPermissions: (holder: BuiltinHolder, rules: PermissionRule[]) =>
    request<PermissionRule[]>('PUT', `/api/admin/builtin-holders/${holder}/permissions`, rules),
  userPermissions: (id: number) =>
    request<PermissionRule[]>('GET', `/api/admin/users/${id}/permissions`),
  setUserPermissions: (id: number, rules: PermissionRule[]) =>
    request<PermissionRule[]>('PUT', `/api/admin/users/${id}/permissions`, rules),
  userAccess: (id: number, project: string) =>
    request<UserAccess[]>('GET', `/api/admin/users/${id}/access?project=${enc(project)}`),

  // Projects.
  /** Every project, archived ones included, seen or not. */
  projects: () => request<AdminProjectRow[]>('GET', '/api/admin/projects'),
  /** `project` is the project whose key is being changed: its former keys are free to it. */
  checkKey: (key: string, project?: string) =>
    request<KeyCheck>('GET', `/api/admin/project-key${query({ key, project })}`),
  /** board: a first board, made with the project; left out, none. */
  createProject: (p: ProjectInput & { board?: BoardKind }) =>
    request<{ id: number; key: string }>('POST', '/api/projects', p),
  archiveProject: (key: string, on: boolean) =>
    request<ProjectSettings>('POST', `/api/projects/${enc(key)}/${on ? 'archive' : 'unarchive'}`),
  deleteProject: (key: string, confirm?: string) =>
    request<void>('DELETE', `/api/projects/${enc(key)}`, confirm ? { confirm } : {}),
  renameProject: (key: string, newKey: string, confirm: string) =>
    request<ProjectSettings>('POST', `/api/projects/${enc(key)}/rename`, { newKey, confirm }),
  categories: () => request<Category[]>('GET', '/api/project-categories'),
  createCategory: (c: { name: string; description?: string }) =>
    request<Category>('POST', '/api/admin/project-categories', c),
  updateCategory: (id: number, c: { name?: string; description?: string }) =>
    request<Category>('PATCH', `/api/admin/project-categories/${id}`, c),
  deleteCategory: (id: number) => request<void>('DELETE', `/api/admin/project-categories/${id}`),

  // Catalogues.
  catalog: (kind: CatalogKind) => request<CatalogItem[]>('GET', `/api/admin/${kind}`),
  createCatalogItem: (kind: CatalogKind, item: CatalogInput) =>
    request<CatalogItem>('POST', `/api/admin/${kind}`, item),
  updateCatalogItem: (kind: CatalogKind, id: string | number, item: CatalogInput) =>
    request<CatalogItem>('PATCH', `/api/admin/${kind}/${enc(String(id))}`, item),
  orderCatalog: (kind: CatalogKind | 'statuses', ids: (string | number)[]) =>
    request<CatalogItem[]>('POST', `/api/admin/${kind}/order`, { ids }),
  deleteCatalogItem: (kind: CatalogKind, id: string | number, moveTo?: string | number) =>
    request<void>(
      'DELETE',
      `/api/admin/${kind}/${enc(String(id))}`,
      moveTo !== undefined ? { moveTo } : {},
    ),
  statuses: () => request<StatusRow[]>('GET', '/api/admin/statuses'),
  createStatus: (s: { name: string; category: string }) =>
    request<{ id: string; name: string; category: string }>('POST', '/api/statuses', s),
  statusUsage: (id: string) => request<StatusUsage>('GET', `/api/admin/statuses/${enc(id)}/usage`),
  updateStatus: (id: string, s: { name?: string; category?: string }) =>
    request<StatusRow>('PATCH', `/api/statuses/${enc(id)}`, s),
  deleteStatus: (id: string) => request<void>('DELETE', `/api/statuses/${enc(id)}`),

  // Global workflows: the ones projects copy.
  workflows: () => request<GlobalWorkflow[]>('GET', '/api/admin/workflows'),
  createWorkflow: (w: { name: string; copyFrom?: number }) =>
    request<{ id: number }>('POST', '/api/admin/workflows', w),
  saveWorkflow: (id: number, w: WorkflowInput) =>
    request<GlobalWorkflow[]>('PUT', `/api/admin/workflows/${id}`, w),
  deleteWorkflow: (id: number) => request<void>('DELETE', `/api/admin/workflows/${id}`),

  // Issue layouts: an issue type's templates.
  typeLayout: (type: string | number, form: LayoutForm) =>
    request<IssueLayout>('GET', `/api/admin/issue-types/${type}/layouts/${form}`),
  saveTypeLayout: (type: string | number, form: LayoutForm, fields: LayoutField[]) =>
    request<IssueLayout>('PUT', `/api/admin/issue-types/${type}/layouts/${form}`, { fields }),
  resetTypeLayout: (type: string | number, form: LayoutForm) =>
    request<void>('DELETE', `/api/admin/issue-types/${type}/layouts/${form}`),

  // Custom fields.
  fields: () => request<FieldRow[]>('GET', '/api/admin/fields'),
  fieldTypes: () => request<FieldTypeInfo[]>('GET', '/api/admin/fields/types'),
  field: (id: number) => request<FieldRow>('GET', `/api/admin/fields/${id}`),
  createField: (f: FieldInput) => request<FieldRow>('POST', '/api/admin/fields', f),
  updateField: (id: number, f: FieldInput) =>
    request<FieldRow>('PATCH', `/api/admin/fields/${id}`, f),
  deleteField: (id: number, confirm?: string) =>
    request<void>('DELETE', `/api/admin/fields/${id}`, confirm ? { confirm } : {}),
  addOption: (id: number, value: string) =>
    request<FieldRow>('POST', `/api/admin/fields/${id}/options`, { value }),
  updateOption: (id: number, option: number, o: { value?: string; disabled?: boolean }) =>
    request<FieldRow>('PATCH', `/api/admin/fields/${id}/options/${option}`, o),
  /** Removes an option; an option in use needs where its values go, or `clear`. */
  deleteOption: (id: number, option: number, how: { moveTo?: number; clear?: boolean } = {}) =>
    request<FieldRow>('DELETE', `/api/admin/fields/${id}/options/${option}`, how),
  orderOptions: (id: number, ids: number[]) =>
    request<FieldRow>('POST', `/api/admin/fields/${id}/options/order`, { ids }),

  // System and import.
  system: () => request<SystemInfo>('GET', '/api/admin/system'),
  importState: () =>
    request<{ confirm: string; job: ImportView | null }>('GET', '/api/admin/import'),
  testImport: (src: ImportSource) => request<ImportCheck>('POST', '/api/admin/import/test', src),
  startImport: (src: ImportSource & { confirm: string }) =>
    request<ImportView>('POST', '/api/admin/import', src),

  // E-mail.
  mail: () => request<MailSettingsReply>('GET', '/api/admin/mail'),
  saveMail: (input: MailSettingsInput) =>
    request<MailSettingsReply>('PUT', '/api/admin/mail', input),
  testMail: (to: string) =>
    request<{ sent: boolean; to: string }>('POST', '/api/admin/mail/test', { to }),
  mailQueue: () => request<MailQueue>('GET', '/api/admin/mail/queue'),
  retryMail: () => request<{ retried: number }>('POST', '/api/admin/mail/queue/retry'),

  // Webhooks.
  webhooks: () => request<Webhook[]>('GET', '/api/admin/webhooks'),
  webhookEvents: () => request<string[]>('GET', '/api/admin/webhooks/events'),
  webhook: (id: number) => request<Webhook>('GET', `/api/admin/webhooks/${id}`),
  createWebhook: (input: WebhookInput) => request<Webhook>('POST', '/api/admin/webhooks', input),
  saveWebhook: (id: number, input: Partial<WebhookInput>) =>
    request<Webhook>('PUT', `/api/admin/webhooks/${id}`, input),
  deleteWebhook: (id: number) => request<void>('DELETE', `/api/admin/webhooks/${id}`),
  deliveries: (id: number) =>
    request<WebhookDelivery[]>('GET', `/api/admin/webhooks/${id}/deliveries`),
  testWebhook: (id: number) => request<WebhookDelivery>('POST', `/api/admin/webhooks/${id}/test`),
  /** Tests an address not saved yet; nothing is stored. */
  testUnsavedWebhook: (input: {
    name: string;
    url: string;
    secret: string;
    excludeBody: boolean;
  }) => request<WebhookDelivery>('POST', '/api/admin/webhooks/test', input),

  // Access tokens, everyone's.
  tokens: (user: string) => request<AccessToken[]>('GET', `/api/admin/tokens${query({ user })}`),
  revokeToken: (id: number) => request<void>('DELETE', `/api/admin/tokens/${id}`),
  /** Who may create tokens. */
  tokenSettings: () => request<TokenSettings>('GET', '/api/admin/tokens/settings'),
  saveTokenSettings: (
    who: TokenSettings['who'],
    group: number | undefined,
    refreshDays: number,
    registration: boolean,
  ) =>
    request<TokenSettings>('PUT', '/api/admin/tokens/settings', {
      who,
      group,
      refreshDays,
      registration,
    }),
  // Applications authorized through OAuth, everyone's.
  grants: () => request<AdminOAuthGrant[]>('GET', '/api/admin/oauth-grants'),
  revokeGrant: (id: number) => request<void>('DELETE', `/api/admin/oauth-grants/${id}`),

  // Audit log.
  audit: (q: AuditQuery) =>
    request<AuditPage>(
      'GET',
      `/api/admin/audit${query({ user: q.user, action: q.action, from: q.from, to: q.to, q: q.q, page: q.page > 1 ? q.page : undefined })}`,
    ),
  auditSettings: () => request<{ retentionDays: number }>('GET', '/api/admin/audit/settings'),
  saveAuditSettings: (retentionDays: number) =>
    request<{ retentionDays: number }>('PUT', '/api/admin/audit/settings', { retentionDays }),

  // Themes.
  themes: () => request<AdminThemes>('GET', '/api/admin/themes'),
  /** A refused package answers 400 with its findings in the error's data. */
  importTheme: (file: File) => {
    const form = new FormData();
    form.append('file', file, file.name);
    return request<AdminTheme>('POST', '/api/admin/themes', form);
  },
  replaceTheme: (id: string, file: File) => {
    const form = new FormData();
    form.append('file', file, file.name);
    return request<AdminTheme>('PUT', `/api/admin/themes/${enc(id)}`, form);
  },
  switchTheme: (id: string, on: boolean) =>
    request<AdminTheme>('POST', `/api/admin/themes/${enc(id)}/${on ? 'enable' : 'disable'}`),
  deleteTheme: (id: string) => request<void>('DELETE', `/api/admin/themes/${enc(id)}`),
  themeExportUrl: (id: string) => `/api/themes/${enc(id)}/export`,
};

/** What a project administrator changes in a project (project settings). */
export const projectAdminApi = {
  settings: (key: string) => request<ProjectSettings>('GET', `/api/projects/${enc(key)}/settings`),
  update: (key: string, patch: ProjectPatch) =>
    request<ProjectSettings>('PATCH', `/api/projects/${enc(key)}`, patch),
  /** Every group, with its project members in this project. */
  groups: (key: string) => request<ProjectGroup[]>('GET', `/api/projects/${enc(key)}/groups`),
  setGroupMembers: (key: string, group: number, members: { users: number[]; groups: number[] }) =>
    request<ProjectGroup>('PUT', `/api/projects/${enc(key)}/groups/${group}`, members),

  components: (key: string) =>
    request<ComponentRow[]>('GET', `/api/projects/${enc(key)}/components`),
  createComponent: (key: string, c: ComponentInput) =>
    request<ComponentRow>('POST', `/api/projects/${enc(key)}/components`, c),
  updateComponent: (key: string, id: number, c: ComponentInput) =>
    request<ComponentRow>('PATCH', `/api/projects/${enc(key)}/components/${id}`, c),
  deleteComponent: (key: string, id: number, moveTo?: number) =>
    request<void>('DELETE', `/api/projects/${enc(key)}/components/${id}`, moveTo ? { moveTo } : {}),

  versions: (key: string) => request<VersionRow[]>('GET', `/api/projects/${enc(key)}/versions`),
  createVersion: (key: string, v: VersionInput) =>
    request<VersionRow>('POST', `/api/projects/${enc(key)}/versions`, v),
  updateVersion: (key: string, id: number, v: VersionInput) =>
    request<VersionRow>('PATCH', `/api/projects/${enc(key)}/versions/${id}`, v),
  orderVersions: (key: string, ids: number[]) =>
    request<VersionRow[]>('POST', `/api/projects/${enc(key)}/versions/order`, { ids }),
  releaseVersion: (
    key: string,
    id: number,
    r: { releaseDate?: string; moveUnresolvedTo?: number },
  ) => request<VersionRow>('POST', `/api/projects/${enc(key)}/versions/${id}/release`, r),
  mergeVersion: (key: string, id: number, into: number) =>
    request<void>('POST', `/api/projects/${enc(key)}/versions/${id}/merge`, { into }),
  deleteVersion: (key: string, id: number, to: { fixTo?: number; affectsTo?: number } = {}) =>
    request<void>('DELETE', `/api/projects/${enc(key)}/versions/${id}`, to),

  /** The project's override of a type's template, or what it inherits (source). */
  layout: (key: string, type: string, form: LayoutForm) =>
    request<IssueLayout>(
      'GET',
      `/api/projects/${enc(key)}/issue-types/${enc(type)}/layouts/${form}`,
    ),
  saveLayout: (key: string, type: string, form: LayoutForm, fields: LayoutField[]) =>
    request<IssueLayout>(
      'PUT',
      `/api/projects/${enc(key)}/issue-types/${enc(type)}/layouts/${form}`,
      { fields },
    ),
  resetLayout: (key: string, type: string, form: LayoutForm) =>
    request<void>('DELETE', `/api/projects/${enc(key)}/issue-types/${enc(type)}/layouts/${form}`),
};

/** The issue security levels of a project, and who sees the issues at each. */
export const securityApi = {
  levels: (key: string) =>
    request<SecurityLevel[]>('GET', `/api/projects/${enc(key)}/security-levels`),
  create: (key: string, l: SecurityLevelInput) =>
    request<SecurityLevel>('POST', `/api/projects/${enc(key)}/security-levels`, l),
  update: (key: string, id: number, l: SecurityLevelInput) =>
    request<SecurityLevel>('PATCH', `/api/projects/${enc(key)}/security-levels/${id}`, l),
  /** Removes a level; one with issues needs where they go, or `clear` (they then have no level). */
  remove: (key: string, id: number, how: { moveTo?: number; clear?: boolean } = {}) =>
    request<void>('DELETE', `/api/projects/${enc(key)}/security-levels/${id}`, how),
  addMember: (key: string, id: number, member: { holder: SecurityMember['holder']; id?: number }) =>
    request<SecurityLevel>(
      'POST',
      `/api/projects/${enc(key)}/security-levels/${id}/members`,
      member,
    ),
  removeMember: (key: string, id: number, member: number) =>
    request<void>('DELETE', `/api/projects/${enc(key)}/security-levels/${id}/members/${member}`),
};

export const accountApi = {
  /** One's profile: name, e-mail, language, project scope, password (with the current one). */
  updateProfile: (patch: {
    name?: string;
    email?: string;
    locale?: string | null;
    project?: string | null;
    currentPassword?: string;
    newPassword?: string;
  }) => request<Me>('PATCH', '/api/me', patch),
  /** Stores one interface preference on the account. */
  setPreference: <K extends keyof Preferences>(key: K, value: NonNullable<Preferences[K]>) =>
    request<Me>('PUT', `/api/me/preferences/${enc(key)}`, value),
  /** Puts one interface preference back to its default. */
  deletePreference: (key: keyof Preferences) =>
    request<Me>('DELETE', `/api/me/preferences/${enc(key)}`),

  // Access tokens, one's own.
  tokens: () => request<AccessToken[]>('GET', '/api/me/tokens'),
  createToken: (name: string, scopes: TokenScope[], expires: string | null) =>
    request<AccessToken>('POST', '/api/me/tokens', { name, scopes, expires }),
  revokeToken: (id: number) => request<void>('DELETE', `/api/me/tokens/${id}`),
  /** Whether the instance lets this user create a token now. */
  tokenPolicy: () => request<{ canCreate: boolean }>('GET', '/api/me/tokens/policy'),

  // Applications authorized through OAuth.
  grants: () => request<OAuthGrant[]>('GET', '/api/me/oauth-grants'),
  revokeGrant: (id: number) => request<void>('DELETE', `/api/me/oauth-grants/${id}`),
  /** Checks an authorization request: query is the search of /oauth/authorize. */
  checkAuthorization: (query: string) =>
    request<OAuthRequest>('GET', `/api/oauth/authorize${query}`),
  /** Allows or refuses it; the answer is where to send the browser. */
  consent: (query: string, allow: boolean, scopes: TokenScope[]) =>
    request<{ redirect: string }>('POST', '/api/oauth/consent', { request: query, allow, scopes }),

  // E-mail notifications.
  notifications: () => request<NotificationPrefs>('GET', '/api/me/notifications'),
  saveNotifications: (prefs: Partial<NotificationPrefs>) =>
    request<NotificationPrefs>('PUT', '/api/me/notifications', prefs),
  projectNotifications: (key: string) =>
    request<ProjectNotifications>('GET', `/api/projects/${enc(key)}/notifications`),
  saveProjectNotifications: (key: string, s: Pick<ProjectNotifications, 'enabled' | 'extra'>) =>
    request<ProjectNotifications>('PUT', `/api/projects/${enc(key)}/notifications`, s),

  // Filter subscriptions.
  /** Every subscription of the user, on every filter. */
  mySubscriptions: () => request<MySubscription[]>('GET', '/api/me/subscriptions'),
  subscriptions: (filter: number) =>
    request<Subscription[]>('GET', `/api/filters/${filter}/subscriptions`),
  subscribe: (filter: number, s: SubscriptionInput) =>
    request<Subscription>('POST', `/api/filters/${filter}/subscriptions`, s),
  saveSubscription: (filter: number, id: number, s: SubscriptionInput) =>
    request<Subscription>('PUT', `/api/filters/${filter}/subscriptions/${id}`, s),
  unsubscribe: (filter: number, id: number) =>
    request<void>('DELETE', `/api/filters/${filter}/subscriptions/${id}`),
  runSubscription: (filter: number, id: number) =>
    request<{ emails: number }>('POST', `/api/filters/${filter}/subscriptions/${id}/run`),
};
