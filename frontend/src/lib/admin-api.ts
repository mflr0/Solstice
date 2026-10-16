/**
 * The administration and personal-settings API, typed: e-mail, webhooks,
 * access tokens, audit log, notification preferences and filter
 * subscriptions. Same client as the rest of the interface (src/lib/api.ts).
 */

import type {
  AccessToken,
  AuditPage,
  CatalogItem,
  CatalogKind,
  Category,
  ComponentInput,
  ComponentRow,
  FieldInput,
  FieldRow,
  FieldTypeInfo,
  GeneralSettings,
  GeneralSettingsInput,
  Group,
  GroupDetail,
  ImportCheck,
  ImportSource,
  ImportView,
  KeyCheck,
  ProjectInput,
  ProjectPatch,
  ProjectRole,
  RoleRow,
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
  Subscription,
  SubscriptionInput,
  Webhook,
  WebhookDelivery,
  WebhookInput,
} from '../core/admin-types.js';
import type { AdminProjectRow, ProjectSettings } from '../core/project-admin.js';
import type { Me } from '../core/types.js';
import { query, request } from './api.js';

const enc = encodeURIComponent;

export const adminApi = {
  // General settings.
  settings: () => request<GeneralSettings>('GET', '/api/admin/settings'),
  saveSettings: (input: GeneralSettingsInput) =>
    request<GeneralSettings>('PUT', '/api/admin/settings', input),

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

  // Roles (shared by every project).
  roles: () => request<RoleRow[]>('GET', '/api/admin/roles'),
  createRole: (r: { name: string; description?: string }) =>
    request<RoleRow>('POST', '/api/admin/roles', r),
  updateRole: (id: number, r: { name?: string; description?: string }) =>
    request<RoleRow>('PATCH', `/api/admin/roles/${id}`, r),
  deleteRole: (id: number) => request<void>('DELETE', `/api/admin/roles/${id}`),

  // Projects.
  /** Every project, archived ones included, seen or not. */
  projects: () => request<AdminProjectRow[]>('GET', '/api/admin/projects'),
  checkKey: (key: string) => request<KeyCheck>('GET', `/api/admin/project-key${query({ key })}`),
  createProject: (p: ProjectInput) =>
    request<{ id: number; key: string }>('POST', '/api/projects', p),
  archiveProject: (key: string, on: boolean) =>
    request<ProjectSettings>('POST', `/api/projects/${enc(key)}/${on ? 'archive' : 'unarchive'}`),
  deleteProject: (key: string, confirm?: string) =>
    request<void>('DELETE', `/api/projects/${enc(key)}`, confirm ? { confirm } : {}),
  categories: () => request<Category[]>('GET', '/api/project-categories'),
  createCategory: (c: { name: string; description?: string }) =>
    request<Category>('POST', '/api/admin/project-categories', c),
  updateCategory: (id: number, c: { name?: string; description?: string }) =>
    request<Category>('PATCH', `/api/admin/project-categories/${id}`, c),
  deleteCategory: (id: number) => request<void>('DELETE', `/api/admin/project-categories/${id}`),

  // Catalogues.
  catalog: (kind: CatalogKind) => request<CatalogItem[]>('GET', `/api/admin/${kind}`),
  createCatalogItem: (kind: CatalogKind, item: Partial<CatalogItem>) =>
    request<CatalogItem>('POST', `/api/admin/${kind}`, item),
  updateCatalogItem: (kind: CatalogKind, id: string | number, item: Partial<CatalogItem>) =>
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
  mail: () => request<MailSettings>('GET', '/api/admin/mail'),
  saveMail: (input: MailSettingsInput) => request<MailSettings>('PUT', '/api/admin/mail', input),
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

  // Access tokens, everyone's.
  tokens: (user: string) => request<AccessToken[]>('GET', `/api/admin/tokens${query({ user })}`),
  revokeToken: (id: number) => request<void>('DELETE', `/api/admin/tokens/${id}`),

  // Audit log.
  audit: (q: AuditQuery) =>
    request<AuditPage>(
      'GET',
      `/api/admin/audit${query({ user: q.user, action: q.action, from: q.from, to: q.to, q: q.q, page: q.page > 1 ? q.page : undefined })}`,
    ),
  auditSettings: () => request<{ retentionDays: number }>('GET', '/api/admin/audit/settings'),
  saveAuditSettings: (retentionDays: number) =>
    request<{ retentionDays: number }>('PUT', '/api/admin/audit/settings', { retentionDays }),
};

/** What a project administrator changes in a project (project settings). */
export const projectAdminApi = {
  settings: (key: string) => request<ProjectSettings>('GET', `/api/projects/${enc(key)}/settings`),
  update: (key: string, patch: ProjectPatch) =>
    request<ProjectSettings>('PATCH', `/api/projects/${enc(key)}`, patch),
  roles: (key: string) => request<ProjectRole[]>('GET', `/api/projects/${enc(key)}/roles`),
  setRole: (key: string, role: number, members: { users: number[]; groups: number[] }) =>
    request<ProjectRole>('PUT', `/api/projects/${enc(key)}/roles/${role}`, members),

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
};

export const accountApi = {
  /** One's profile: name, e-mail, language, password (with the current one). */
  updateProfile: (patch: {
    name?: string;
    email?: string;
    locale?: string | null;
    currentPassword?: string;
    newPassword?: string;
  }) => request<Me>('PATCH', '/api/me', patch),

  // Access tokens, one's own.
  tokens: () => request<AccessToken[]>('GET', '/api/me/tokens'),
  createToken: (name: string, expires: string | null) =>
    request<AccessToken>('POST', '/api/me/tokens', { name, expires }),
  revokeToken: (id: number) => request<void>('DELETE', `/api/me/tokens/${id}`),

  // E-mail notifications.
  notifications: () => request<NotificationPrefs>('GET', '/api/me/notifications'),
  saveNotifications: (prefs: Partial<NotificationPrefs>) =>
    request<NotificationPrefs>('PUT', '/api/me/notifications', prefs),
  projectNotifications: (key: string) =>
    request<ProjectNotifications>('GET', `/api/projects/${enc(key)}/notifications`),
  saveProjectNotifications: (key: string, s: Pick<ProjectNotifications, 'enabled' | 'extra'>) =>
    request<ProjectNotifications>('PUT', `/api/projects/${enc(key)}/notifications`, s),

  // Filter subscriptions.
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
