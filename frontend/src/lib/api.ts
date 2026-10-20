/**
 * The Solstice API, typed. Every request carries the X-Solstice header the server
 * requires on writes (it blocks cross-site forgery); errors come back as
 * ApiError with the server's message, in the interface's language, ready to show.
 */

import { avatarUrl } from '../core/avatar.js';
import type {
  Comment,
  Grant,
  HiddenIssue,
  HistoryEntry,
  IdName,
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
  TypeRef,
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

/** What an issue has with no same-named match in the project it moves to, and what that offers. */
export interface MoveParts {
  unmatched: IdName[];
  options: IdName[];
}

/** The comments and work logs restricted to a role of the current project, and what they can be restricted to instead. */
export interface MoveRestricted {
  comments: number;
  worklogs: number;
  roles: IdName[];
  groups: IdName[];
}

/** The dry run of a move: what is still to choose, and what the move drops or changes. */
export interface MoveCheck {
  project: { id: number; key: string; name: string };
  ready: boolean;
  missing: string[];
  types?: TypeRef[];
  subtaskTypes?: TypeRef[];
  statuses?: StatusRef[];
  components?: MoveParts;
  affectsVersions?: MoveParts;
  fixVersions?: MoveParts;
  securityLevel?: MoveParts;
  restricted?: MoveRestricted;
  droppedFields: string[];
  sprints: string[];
  assignee?: { from: UserRef | null; to: UserRef | null };
  subtasks: { count: number; reset: number; lost: number; reassigned: number };
}

/** The choices a move can be given; a list replaces the whole list the issue ends with. */
export interface MoveChoices {
  type?: string;
  subtaskType?: string;
  status?: string;
  components?: number[];
  affectsVersions?: number[];
  fixVersions?: number[];
  /** A level of the new project, or null for none. */
  securityLevel?: number | null;
  /** A role of the new project, or a group, for what was restricted to a role of the current one; or giving the move up. */
  restricted?: { to: 'role'; role: number } | { to: 'group'; group: number } | { to: 'refuse' };
}

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
  // Links sent by e-mail. Asking for one always gets the same answer.
  forgotPassword: (identifier: string) =>
    request<{ ok: boolean }>('POST', '/api/auth/forgot', { identifier }),
  /** What a link is for; fails when it is unknown, used, expired or replaced. */
  checkLink: (token: string) =>
    request<{ purpose: string; name: string; username: string }>(
      'GET',
      `/api/auth/reset/${enc(token)}`,
    ),
  /** Chooses the password with a link, and signs in. */
  useLink: (token: string, password: string) =>
    request<Me>('POST', '/api/auth/reset', { token, password }),
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
  /** Without a summary the server makes one; warnings are files it could not copy. */
  cloneIssue: (
    key: string,
    options: {
      summary?: string | undefined;
      includeSubtasks: boolean;
      includeLinks: boolean;
      includeAttachments: boolean;
    },
  ) =>
    request<{ key: string; warnings: string[] }>('POST', `/api/issues/${enc(key)}/clone`, options),
  /** What the target project lacks for the issue (a dry run), given the choices made so far. */
  checkMove: (key: string, project: string, choices: MoveChoices) =>
    request<MoveCheck>('POST', `/api/issues/${enc(key)}/move`, {
      project,
      check: true,
      ...choices,
    }),
  /** Moves the issue (and its sub-tasks) to another project; answers its new key. */
  moveIssue: (key: string, project: string, choices: MoveChoices) =>
    request<{ key: string }>('POST', `/api/issues/${enc(key)}/move`, { project, ...choices }),

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

  setMyAvatar: (picture: Blob) => {
    const form = new FormData();
    form.append('file', picture, 'avatar.png');
    return request<void>('PUT', '/api/me/avatar', form);
  },
  removeMyAvatar: () => request<void>('DELETE', '/api/me/avatar'),

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

// Profile pictures: the server draws initials for people without one, so a
// request says which. Asked once per person; changes only forget what is known.
const pictures = new Map<number, Promise<boolean>>();
const pictureVersions = new Map<number, number>();

/** Where a person's picture is served. */
export function pictureUrl(userId: number): string {
  return avatarUrl(userId, pictureVersions.get(userId));
}

/** Whether a person has uploaded a picture. */
export function hasPicture(userId: number): Promise<boolean> {
  let known = pictures.get(userId);
  if (!known) {
    known = fetch(pictureUrl(userId), { credentials: 'same-origin' })
      .then(async (res) => {
        await res.blob();
        return res.headers.get('Content-Type') === 'image/png';
      })
      .catch(() => false);
    pictures.set(userId, known);
  }
  return known;
}

/** Forgets what is known of a person's picture, after it changed. */
export function pictureChanged(userId: number): void {
  pictures.delete(userId);
  pictureVersions.set(userId, Date.now());
}

/** Narrows an update response: false when the issue became invisible. */
export function isVisible(d: IssueDetail | HiddenIssue): d is IssueDetail {
  return !('hidden' in d);
}

export function errorMessage(e: unknown): string {
  return e instanceof Error ? e.message : t.errors.generic;
}
