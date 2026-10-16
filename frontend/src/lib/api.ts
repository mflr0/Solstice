/**
 * The Solstice API, typed. Every request carries the X-Solstice header the server
 * requires on writes (it blocks cross-site forgery); errors come back as
 * ApiError with the server's message, in the interface's language, ready to show.
 */

import type {
  AdminUser,
  Comment,
  Grant,
  HiddenIssue,
  HistoryEntry,
  IssueDetail,
  IssuePage,
  Me,
  Meta,
  Person,
  Project,
  ProjectDetail,
  ProjectPermissions,
  ProjectWorkflows,
  Refs,
  StatusCategory,
  StatusRef,
  UserRef,
  Worklog,
  WorklogPatch,
  WorkflowInput,
  Counter,
  SavedFilter,
  SprintSuggestion,
  Facets,
  EpicRef,
  CustomFieldRef,
  ProjectValue,
} from '../core/types.js';
import type {
  Backlog,
  BoardCards,
  BoardDetail,
  BoardKind,
  BoardPatch,
  BoardSummary,
  CardMove,
  Sprint,
  SprintPatch,
} from '../core/board-types.js';
import { t } from '../i18n/t.js';
import { locale } from '../i18n/index.js';

export class ApiError extends Error {
  constructor(
    message: string,
    readonly status: number,
    /** The error's JSON body, for errors that say more than a message. */
    readonly data: unknown = undefined,
  ) {
    super(message);
  }
}

let onUnauthorized: () => void = () => {};

/** Called whenever the session has expired. */
export function setUnauthorizedHandler(fn: () => void): void {
  onUnauthorized = fn;
}

export async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  // The server answers its messages in the interface's language.
  const headers: Record<string, string> = { Accept: 'application/json', 'Accept-Language': locale };
  let payload: BodyInit | undefined;
  if (method !== 'GET') headers['X-Solstice'] = '1';
  if (body instanceof FormData) {
    payload = body;
  } else if (body !== undefined) {
    headers['Content-Type'] = 'application/json';
    payload = JSON.stringify(body);
  }
  let res: Response;
  try {
    res = await fetch(path, {
      method,
      headers,
      credentials: 'same-origin',
      ...(payload !== undefined ? { body: payload } : {}),
    });
  } catch {
    throw new ApiError(t.errors.network, 0);
  }
  if (res.status === 401 && path !== '/api/login') onUnauthorized();
  if (!res.ok) {
    let message: string = t.errors.generic;
    let data: unknown;
    try {
      data = await res.json();
      if (data && typeof data === 'object' && 'error' in data && typeof data.error === 'string') {
        message = data.error;
      }
    } catch {
      // Not JSON: keep the generic message.
    }
    // Not installed yet: the setup wizard comes first.
    if (
      res.status === 503 &&
      data &&
      typeof data === 'object' &&
      'setup' in data &&
      data.setup === true
    ) {
      location.assign('/setup');
    }
    throw new ApiError(message, res.status, data);
  }
  if (res.status === 204) return undefined as T;
  const text = await res.text();
  return (text ? JSON.parse(text) : undefined) as T;
}

const enc = encodeURIComponent;

export function query(params: Record<string, string | number | undefined>): string {
  const sp = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) {
    if (v !== undefined && v !== '') sp.set(k, String(v));
  }
  const s = sp.toString();
  return s ? `?${s}` : '';
}

/** Fields accepted when creating or updating an issue. */
export interface IssueFields {
  project?: string;
  type?: string;
  summary?: string;
  description?: string | null;
  environment?: string | null;
  priority?: string | null;
  assignee?: number | null;
  labels?: string[];
  components?: number[];
  affectsVersions?: number[];
  fixVersions?: number[];
  due?: string | null;
  parent?: string;
  status?: string;
  resolution?: string | null;
  securityLevel?: number | null;
  /** Jira durations ("1w 2d 3h 30m"); null clears. */
  originalEstimate?: string | null;
  remainingEstimate?: string | null;
}

export const api = {
  login: (username: string, password: string) =>
    request<Me>('POST', '/api/login', { username, password }),
  logout: () => request<void>('POST', '/api/logout'),
  me: () => request<Me>('GET', '/api/me'),
  /** The account's own settings: the interface language, or null for none. */
  updateMe: (patch: { locale: string | null }) => request<Me>('PATCH', '/api/me', patch),
  meta: () => request<Meta>('GET', '/api/meta'),

  projects: () => request<Project[]>('GET', '/api/projects'),
  project: (key: string) => request<ProjectDetail>('GET', `/api/projects/${enc(key)}`),
  assignable: (key: string, q: string) =>
    request<UserRef[]>('GET', `/api/projects/${enc(key)}/assignable${query({ q })}`),
  permissions: (key: string) =>
    request<ProjectPermissions>('GET', `/api/projects/${enc(key)}/permissions`),
  savePermissions: (key: string, grants: Grant[]) =>
    request<ProjectPermissions>('PUT', `/api/projects/${enc(key)}/permissions`, { grants }),

  issues: (params: {
    q?: string | undefined;
    /** JQL, read as such whatever it looks like. */
    jql?: string | undefined;
    project?: string | undefined;
    page?: number | undefined;
    size?: number | undefined;
  }) => request<IssuePage>('GET', `/api/issues${query(params)}`),
  issue: (key: string) => request<IssueDetail>('GET', `/api/issues/${enc(key)}`),
  createIssue: (fields: IssueFields) => request<{ key: string }>('POST', '/api/issues', fields),
  updateIssue: (key: string, fields: IssueFields) =>
    request<IssueDetail | HiddenIssue>('PATCH', `/api/issues/${enc(key)}`, fields),
  deleteIssue: (key: string) => request<void>('DELETE', `/api/issues/${enc(key)}`),

  comments: (key: string) => request<Comment[]>('GET', `/api/issues/${enc(key)}/comments`),
  addComment: (key: string, body: string) =>
    request<Comment>('POST', `/api/issues/${enc(key)}/comments`, { body }),
  editComment: (id: number, body: string) =>
    request<Comment>('PATCH', `/api/comments/${id}`, { body }),
  deleteComment: (id: number) => request<void>('DELETE', `/api/comments/${id}`),
  history: (key: string) => request<HistoryEntry[]>('GET', `/api/issues/${enc(key)}/history`),
  worklogs: (key: string) => request<Worklog[]>('GET', `/api/issues/${enc(key)}/worklogs`),
  addWorklog: (key: string, entry: { started?: string; seconds: number; body: string }) =>
    request<void>('POST', `/api/issues/${enc(key)}/worklogs`, entry),
  editWorklog: (id: number, patch: WorklogPatch) =>
    request<Worklog>('PUT', `/api/worklogs/${id}`, patch),
  deleteWorklog: (id: number) => request<void>('DELETE', `/api/worklogs/${id}`),

  watch: (key: string, on: boolean) =>
    request<Counter>(on ? 'PUT' : 'DELETE', `/api/issues/${enc(key)}/watch`),
  vote: (key: string, on: boolean) =>
    request<Counter>(on ? 'PUT' : 'DELETE', `/api/issues/${enc(key)}/vote`),
  addLink: (key: string, type: number, direction: 'outward' | 'inward', target: string) =>
    request<IssueDetail | HiddenIssue>('POST', `/api/issues/${enc(key)}/links`, {
      type,
      direction,
      target,
    }),
  deleteLink: (id: number) => request<void>('DELETE', `/api/links/${id}`),
  upload: (key: string, files: File[]) => {
    const form = new FormData();
    for (const f of files) form.append('file', f, f.name);
    return request<IssueDetail | HiddenIssue>('POST', `/api/issues/${enc(key)}/attachments`, form);
  },
  deleteAttachment: (id: number) => request<void>('DELETE', `/api/attachments/${id}`),

  users: (q: string) => request<UserRef[]>('GET', `/api/users${query({ q })}`),
  person: (username: string) => request<Person>('GET', `/api/people/${enc(username)}`),
  refs: (users: string[], issueKeys: string[]) =>
    request<Refs>('POST', '/api/refs', { users, issueKeys }),

  // Boards and sprints.
  boards: (project?: string) => request<BoardSummary[]>('GET', `/api/boards${query({ project })}`),
  createBoard: (b: { name?: string; kind: BoardKind; project: string }) =>
    request<{ id: number }>('POST', '/api/boards', b),
  board: (id: number) => request<BoardDetail>('GET', `/api/boards/${id}`),
  saveBoard: (id: number, patch: BoardPatch) =>
    request<BoardDetail>('PUT', `/api/boards/${id}`, patch),
  deleteBoard: (id: number) => request<void>('DELETE', `/api/boards/${id}`),
  boardCards: (id: number, params: { quick?: string | undefined; sprint?: number | undefined }) =>
    request<BoardCards>('GET', `/api/boards/${id}/cards${query(params)}`),
  backlog: (id: number, quick?: string) =>
    request<Backlog>('GET', `/api/boards/${id}/backlog${query({ quick })}`),
  moveCard: (id: number, move: CardMove) => request<void>('POST', `/api/boards/${id}/move`, move),
  createSprint: (board: number, s: { name?: string; goal?: string } = {}) =>
    request<{ id: number }>('POST', `/api/boards/${board}/sprints`, s),
  startSprint: (id: number, s: { name?: string; goal?: string; end?: string }) =>
    request<void>('POST', `/api/sprints/${id}/start`, s),
  updateSprint: (id: number, patch: SprintPatch) =>
    request<Sprint>('PATCH', `/api/sprints/${id}`, patch),
  deleteSprint: (id: number) => request<void>('DELETE', `/api/sprints/${id}`),
  completeSprint: (id: number, moveTo?: number) =>
    request<{ moved: number }>('POST', `/api/sprints/${id}/complete`, moveTo ? { moveTo } : {}),
  /** The statuses a project's workflows use, for mapping board columns. */
  boardProjectStatuses: (key: string) =>
    request<{ workflows: { statuses: StatusRef[] }[] }>(
      'GET',
      `/api/projects/${enc(key)}/workflows`,
    ),

  // Workflows: each project owns its own.
  workflows: (key: string) =>
    request<ProjectWorkflows>('GET', `/api/projects/${enc(key)}/workflows`),
  createWorkflow: (key: string, w: { name: string; copyFrom?: number }) =>
    request<{ id: number }>('POST', `/api/projects/${enc(key)}/workflows`, w),
  saveWorkflow: (id: number, w: WorkflowInput) =>
    request<ProjectWorkflows>('PUT', `/api/workflows/${id}`, w),
  deleteWorkflow: (id: number, remapDefault?: Record<string, string>) =>
    request<void>('DELETE', `/api/workflows/${id}`, remapDefault ? { remapDefault } : undefined),
  createStatus: (s: { name: string; category: StatusCategory; project: string }) =>
    request<StatusRef>('POST', '/api/statuses', s),

  adminUsers: (q: string) => request<AdminUser[]>('GET', `/api/admin/users${query({ q })}`),
  adminCreateUser: (u: {
    username: string;
    name: string;
    email: string;
    password: string;
    admin: boolean;
  }) => request<{ id: number }>('POST', '/api/admin/users', u),
  adminUpdateUser: (
    id: number,
    patch: { name?: string; email?: string; active?: boolean; admin?: boolean; password?: string },
  ) => request<void>('PATCH', `/api/admin/users/${id}`, patch),

  // Search: saved filters and values to pick from.
  filters: () => request<SavedFilter[]>('GET', '/api/filters'),
  createFilter: (f: { name: string; query: string; description?: string }) =>
    request<SavedFilter>('POST', '/api/filters', f),
  updateFilter: (id: number, patch: { name?: string; query?: string; description?: string }) =>
    request<SavedFilter | undefined>('PATCH', `/api/filters/${id}`, patch),
  deleteFilter: (id: number) => request<void>('DELETE', `/api/filters/${id}`),
  favouriteFilter: (id: number, on: boolean) =>
    request<void>(on ? 'PUT' : 'DELETE', `/api/filters/${id}/favourite`),
  labels: (q: string, project?: string) =>
    request<string[]>('GET', `/api/labels${query({ q, project })}`),
  sprintSuggestions: (q: string) =>
    request<SprintSuggestion[]>('GET', `/api/sprints${query({ q })}`),
  /** Counts per value of the given fields (comma-separated) for a JQL search. */
  facets: (params: { jql: string; project?: string | undefined; fields: string }) =>
    request<Facets>('GET', `/api/issues/facets${query(params)}`),
  epics: (q: string, project?: string) =>
    request<EpicRef[]>('GET', `/api/epics${query({ q, project })}`),
  customFields: () => request<CustomFieldRef[]>('GET', '/api/fields'),
  versions: (q: string, project?: string) =>
    request<ProjectValue[]>('GET', `/api/versions${query({ q, project })}`),
  components: (q: string, project?: string) =>
    request<ProjectValue[]>('GET', `/api/components${query({ q, project })}`),
};

/** Narrows an update response: false when the issue became invisible. */
export function isVisible(d: IssueDetail | HiddenIssue): d is IssueDetail {
  return !('hidden' in d);
}

export function errorMessage(e: unknown): string {
  return e instanceof Error ? e.message : t.errors.generic;
}
