/**
 * Projects as administrators shape them: the creation form (key suggested
 * from the name), the project settings tabs, versions and components. Pure
 * logic; the pages word the problems (src/i18n, blocks projectsAdmin and
 * projectConfig).
 */

import type { ExtraRecipient } from './admin-types.js';
import type { IdName, TypeRef, UserRef } from './types.js';

/* Shapes (backend/internal/api/admin_projects.go) --------------------------- */

/** A line of the administrators' project list (GET /api/admin/projects). */
export interface AdminProjectRow {
  id: number;
  key: string;
  name: string;
  kind: string;
  lead: UserRef | null;
  category: IdName | null;
  archived: boolean;
  /** Issues in the project. */
  issues: number;
}

/** A project's settings (GET /api/projects/{key}/settings, PATCH, archive). */
export interface ProjectSettings {
  id: number;
  key: string;
  name: string;
  description: string | null;
  kind: string;
  lead: UserRef | null;
  category: IdName | null;
  defaultAssignee: 'unassigned' | 'lead';
  archived: boolean;
  /** The issue types allowed in the project, in order. */
  issueTypes: TypeRef[];
  issues: number;
}

/* Keys ---------------------------------------------------------------------- */

/** A project key: an upper-case letter, then 1 to 9 upper-case letters or digits. */
export const KEY_PATTERN = /^[A-Z][A-Z0-9]{1,9}$/;

function plain(s: string): string {
  return s.normalize('NFD').replace(/[̀-ͯ]/g, '');
}

/** What may be typed in the key field: upper case, letters and digits, 10 at most. */
export function normalizeKey(s: string): string {
  return plain(s)
    .toUpperCase()
    .replace(/[^A-Z0-9]/g, '')
    .slice(0, 10);
}

/**
 * A key suggested from a project name: the initials of several words
 * ("Site web" → "SW"), the start of a single one ("Marketing" → "MARK").
 */
export function suggestKey(name: string): string {
  const words = plain(name)
    .toUpperCase()
    .split(/[^A-Z0-9]+/)
    .filter(Boolean);
  if (words.length === 0) return '';
  let key = words.length > 1 ? words.map((w) => w[0]).join('') : (words[0] ?? '').slice(0, 4);
  if (key.length < 2) key = (key + (words[0] ?? '').slice(1)).slice(0, 4);
  key = key.replace(/^[0-9]+/, '');
  if (key.length < 2) key = (key + 'PROJ').slice(0, Math.max(2, key.length));
  return key.slice(0, 10);
}

export type KeyProblem = 'keyRequired' | 'keyPattern';

export function keyProblem(key: string): KeyProblem | null {
  if (!key.trim()) return 'keyRequired';
  return KEY_PATTERN.test(key) ? null : 'keyPattern';
}

/* Creation form ------------------------------------------------------------- */

export type AccessKind = 'lead' | 'group' | 'role' | 'everyone';

export interface ProjectForm {
  name: string;
  key: string;
  template: 'software' | 'simple';
  lead: UserRef | null;
  category: number | null;
  access: AccessKind;
  accessId: number | null;
  description: string;
}

export type ProjectProblem = 'nameRequired' | 'nameTooLong' | KeyProblem | 'accessRequired';

export function projectProblems(
  f: ProjectForm,
): Partial<Record<'name' | 'key' | 'access', ProjectProblem>> {
  const out: Partial<Record<'name' | 'key' | 'access', ProjectProblem>> = {};
  const name = f.name.trim();
  if (!name) out.name = 'nameRequired';
  else if ([...name].length > 255) out.name = 'nameTooLong';
  const k = keyProblem(f.key);
  if (k) out.key = k;
  if ((f.access === 'group' || f.access === 'role') && !f.accessId) out.access = 'accessRequired';
  return out;
}

/** The body of POST /api/projects. */
export function projectPayload(f: ProjectForm) {
  const description = f.description.trim();
  return {
    key: f.key,
    name: f.name.trim(),
    template: f.template,
    ...(description ? { description } : {}),
    ...(f.lead ? { lead: f.lead.id } : {}),
    category: f.category,
    access:
      f.access === 'everyone'
        ? ({ holder: 'everyone' } as const)
        : (f.access === 'group' || f.access === 'role') && f.accessId
          ? { holder: f.access, id: f.accessId }
          : null,
  };
}

/* Project settings tabs ----------------------------------------------------- */

export const PROJECT_TABS = [
  'general',
  'components',
  'versions',
  'rights',
  'workflows',
  'notifications',
] as const;

export type ProjectTab = (typeof PROJECT_TABS)[number];

/**
 * The tab an address asks for (?tab=). Project administrators open on
 * « Général »; others only see the workflows, read-only.
 */
export function parseTab(param: string | null, configure: boolean): ProjectTab {
  if (!configure) return 'workflows';
  return (PROJECT_TABS as readonly string[]).includes(param ?? '')
    ? (param as ProjectTab)
    : 'general';
}

/** The next tab with the arrow keys, wrapping around. */
export function nextTab(
  tabs: readonly ProjectTab[],
  current: ProjectTab,
  step: 1 | -1,
): ProjectTab {
  const i = tabs.indexOf(current);
  return tabs[(i + step + tabs.length) % tabs.length] ?? current;
}

/* Versions and components --------------------------------------------------- */

export interface VersionLike {
  id: number;
  name: string;
  released: boolean;
  archived: boolean;
}

/** Start after release: the server refuses it. Days are "YYYY-MM-DD". */
export function datesProblem(start: string, release: string): 'startAfterRelease' | null {
  return start && release && start > release ? 'startAfterRelease' : null;
}

/** Where a released version's unresolved issues may go: another unreleased, unarchived version. */
export function releaseTargets<V extends VersionLike>(versions: V[], self: number): V[] {
  return versions.filter((v) => v.id !== self && !v.released && !v.archived);
}

/** Versions a version may be merged into, or replaced by when deleted: any other one. */
export function otherVersions<V extends VersionLike>(versions: V[], self: number): V[] {
  return versions.filter((v) => v.id !== self);
}

/** Components a deleted one's issues may move to: other, not archived. */
export function otherComponents<C extends { id: number; archived: boolean }>(
  components: C[],
  self: number,
): C[] {
  return components.filter((c) => c.id !== self && !c.archived);
}

/** A list with one item moved from one place to another (reordering). */
export function moveItem<T>(list: readonly T[], from: number, to: number): T[] {
  const out = [...list];
  if (from < 0 || from >= out.length || to < 0 || to >= out.length) return out;
  const [item] = out.splice(from, 1);
  out.splice(to, 0, item as T);
  return out;
}

/* Project notifications ----------------------------------------------------- */

export function hasExtra(
  extra: readonly ExtraRecipient[],
  event: string,
  kind: 'group' | 'role',
  id: number,
): boolean {
  return extra.some((e) => e.event === event && e.kind === kind && e.id === id);
}

/** Adds or removes a group or role among those an event is sent to. */
export function toggleExtra(
  extra: readonly ExtraRecipient[],
  r: ExtraRecipient,
  on: boolean,
): ExtraRecipient[] {
  const rest = extra.filter((e) => !(e.event === r.event && e.kind === r.kind && e.id === r.id));
  return on ? [...rest, r] : rest;
}

/** A stable signature of recipients, to tell unsaved changes. */
export function extraSignature(extra: readonly ExtraRecipient[]): string {
  return extra
    .map((e) => `${e.event}:${e.kind}:${e.id}`)
    .sort()
    .join('|');
}
