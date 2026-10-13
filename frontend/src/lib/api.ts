/**
 * The Solstice API, typed. Every request carries the X-Solstice header the server
 * requires on writes (it blocks cross-site forgery); errors come back as
 * ApiError with the server's French message, ready to show.
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
  Refs,
  UserRef,
  Worklog,
  Counter,
} from '../core/types.js';
import { t } from '../i18n/fr.js';

export class ApiError extends Error {
  constructor(
    message: string,
    readonly status: number,
  ) {
    super(message);
  }
}

let onUnauthorized: () => void = () => {};

/** Called whenever the session has expired. */
export function setUnauthorizedHandler(fn: () => void): void {
  onUnauthorized = fn;
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const headers: Record<string, string> = { Accept: 'application/json' };
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
    try {
      const data: unknown = await res.json();
      if (data && typeof data === 'object' && 'error' in data && typeof data.error === 'string') {
        message = data.error;
      }
    } catch {
      // Not JSON: keep the generic message.
    }
    throw new ApiError(message, res.status);
  }
  if (res.status === 204) return undefined as T;
  const text = await res.text();
  return (text ? JSON.parse(text) : undefined) as T;
}

const enc = encodeURIComponent;

function query(params: Record<string, string | number | undefined>): string {
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
}

export const api = {
  login: (username: string, password: string) =>
    request<Me>('POST', '/api/login', { username, password }),
  logout: () => request<void>('POST', '/api/logout'),
  me: () => request<Me>('GET', '/api/me'),
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
};

/** Narrows an update response: false when the issue became invisible. */
export function isVisible(d: IssueDetail | HiddenIssue): d is IssueDetail {
  return !('hidden' in d);
}

export function errorMessage(e: unknown): string {
  return e instanceof Error ? e.message : t.errors.generic;
}
