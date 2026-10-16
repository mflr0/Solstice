/**
 * Projects as administrators shape them: the creation form (key suggested
 * from the name), the project settings tabs, versions and components. Pure
 * logic; the pages word the problems (src/i18n, blocks projectsAdmin and
 * projectConfig).
 */

import { matches } from './admin-forms.js';
import type {
  AssigneeType,
  Category,
  ExtraRecipient,
  ProjectPatch,
  VersionInput,
  VersionRow,
} from './admin-types.js';
import { parseDay } from './dates.js';
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
  /** The issue types offered in the project; left out, the server offers them all. */
  issueTypes?: string[];
  /** Who new issues go to when nobody is chosen; left out, nobody. */
  defaultAssignee?: 'unassigned' | 'lead';
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
    ...(f.issueTypes ? { issueTypes: f.issueTypes } : {}),
    ...(f.defaultAssignee ? { defaultAssignee: f.defaultAssignee } : {}),
    category: f.category,
    access:
      f.access === 'everyone'
        ? ({ holder: 'everyone' } as const)
        : (f.access === 'group' || f.access === 'role') && f.accessId
          ? { holder: f.access, id: f.accessId }
          : null,
  };
}

/**
 * The server wants at least one issue type that is not a sub-task: a
 * project made only of sub-tasks could never hold a first issue.
 */
export function issueTypesProblem(
  selected: readonly string[],
  all: readonly { id: string; subtask: boolean }[],
): 'typesRequired' | null {
  return all.some((t) => !t.subtask && selected.includes(t.id)) ? null : 'typesRequired';
}

/* The administrators' list -------------------------------------------------- */

export type ProjectShow = 'all' | 'active' | 'archived';

export const PROJECT_SHOWS: readonly ProjectShow[] = ['all', 'active', 'archived'];

/** The category filter: every project, those without one, or one category. */
export type CategoryFilter = 'all' | 'none' | number;

/** The page's filters as the address keeps them (?show=, ?category=). */
export function parseShow(param: string | null): ProjectShow {
  return (PROJECT_SHOWS as readonly string[]).includes(param ?? '')
    ? (param as ProjectShow)
    : 'all';
}

export function parseCategoryFilter(param: string | null): CategoryFilter {
  if (param === 'none') return 'none';
  const n = Number(param);
  return param && Number.isInteger(n) && n > 0 ? n : 'all';
}

/** What the search box, the state and the category choose among the rows. */
export function filterProjects<
  R extends {
    key: string;
    name: string;
    archived: boolean;
    lead: { name: string; username: string } | null;
    category: { id: number; name: string } | null;
  },
>(rows: readonly R[], q: string, show: ProjectShow, category: CategoryFilter): R[] {
  return rows.filter((r) => {
    if (show === 'active' && r.archived) return false;
    if (show === 'archived' && !r.archived) return false;
    if (category === 'none' && r.category) return false;
    if (typeof category === 'number' && r.category?.id !== category) return false;
    return matches(q, r.key, r.name, r.lead?.name, r.lead?.username, r.category?.name);
  });
}

export type CategoryProblem = 'nameRequired' | 'nameTooLong' | 'nameTaken';

/** A category name: present, 255 characters at most, and not another's (any case). */
export function categoryProblem(
  name: string,
  categories: readonly Pick<Category, 'id' | 'name'>[],
  self: number | null,
): CategoryProblem | null {
  const n = name.trim();
  if (!n) return 'nameRequired';
  if ([...n].length > 255) return 'nameTooLong';
  const lower = n.toLowerCase();
  return categories.some((c) => c.id !== self && c.name.trim().toLowerCase() === lower)
    ? 'nameTaken'
    : null;
}

/* Project settings tabs ----------------------------------------------------- */

export const PROJECT_TABS = [
  'general',
  'components',
  'versions',
  'people',
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

/* General tab -------------------------------------------------------------- */

/** What the General tab edits (a project's key is not editable). */
export interface GeneralForm {
  name: string;
  description: string;
  lead: UserRef | null;
  category: number | null;
  defaultAssignee: 'unassigned' | 'lead';
  /** The issue types offered, in the catalogue's order. */
  issueTypes: string[];
}

export function generalForm(p: ProjectSettings): GeneralForm {
  return {
    name: p.name,
    description: p.description ?? '',
    lead: p.lead,
    category: p.category?.id ?? null,
    defaultAssignee: p.defaultAssignee,
    issueTypes: p.issueTypes.map((i) => i.id),
  };
}

export type GeneralProblem = 'nameRequired' | 'nameTooLong' | 'typesRequired' | 'leadRequired';

export function generalProblems(
  f: GeneralForm,
  all: readonly { id: string; subtask: boolean }[],
): Partial<Record<'name' | 'lead' | 'types', GeneralProblem>> {
  const out: Partial<Record<'name' | 'lead' | 'types', GeneralProblem>> = {};
  const name = f.name.trim();
  if (!name) out.name = 'nameRequired';
  else if ([...name].length > 255) out.name = 'nameTooLong';
  if (!f.lead) out.lead = 'leadRequired';
  if (issueTypesProblem(f.issueTypes, all)) out.types = 'typesRequired';
  return out;
}

const sameSet = (a: readonly string[], b: readonly string[]) =>
  a.length === b.length && a.every((x) => b.includes(x));

/** The fields of the form that differ from the saved project, as a PATCH body. */
export function generalPatch(f: GeneralForm, p: ProjectSettings): ProjectPatch {
  const patch: ProjectPatch = {};
  const name = f.name.trim();
  if (name !== p.name) patch.name = name;
  const description = f.description.trim();
  if (description !== (p.description ?? '')) patch.description = description || null;
  if (f.lead && f.lead.id !== p.lead?.id) patch.lead = f.lead.id;
  if (f.category !== (p.category?.id ?? null)) patch.category = f.category;
  if (f.defaultAssignee !== p.defaultAssignee) patch.defaultAssignee = f.defaultAssignee;
  if (
    !sameSet(
      f.issueTypes,
      p.issueTypes.map((i) => i.id),
    )
  )
    patch.issueTypes = f.issueTypes;
  return patch;
}

/** The types ticked, kept in the catalogue's order, with one more or less. */
export function toggleIssueType(
  all: readonly { id: string }[],
  selected: readonly string[],
  id: string,
  on: boolean,
): string[] {
  return all.map((x) => x.id).filter((x) => (x === id ? on : selected.includes(x)));
}

/* Names and days ------------------------------------------------------------ */

/** A name of a component or version: the rules of category names (any case). */
export const nameProblem = categoryProblem;

/** Whether text is a real calendar day, "YYYY-MM-DD" (the server refuses others). */
export function validDay(s: string): boolean {
  const d = parseDay(s);
  if (!d) return false;
  const t = new Date(Date.UTC(d.y, d.m - 1, d.d));
  return t.getUTCFullYear() === d.y && t.getUTCMonth() === d.m - 1 && t.getUTCDate() === d.d;
}

export type DayProblem = 'dayInvalid';

/** An optional day: empty is fine, anything else must be a real day. */
export function dayProblem(s: string): DayProblem | null {
  return !s.trim() || validDay(s.trim()) ? null : 'dayInvalid';
}

/* Components ---------------------------------------------------------------- */

export const ASSIGNEE_TYPES: readonly AssigneeType[] = [
  'project_default',
  'component_lead',
  'project_lead',
  'unassigned',
];

/** What the component dialog edits. */
export interface ComponentForm {
  name: string;
  description: string;
  lead: UserRef | null;
  assigneeType: AssigneeType;
}

/** A component's lead cannot decide the assignee when there is none. */
export function componentProblem(f: ComponentForm): 'leadNeeded' | null {
  return f.assigneeType === 'component_lead' && !f.lead ? 'leadNeeded' : null;
}

/** The body of POST/PATCH components: a cleared lead is sent as null. */
export function componentPayload(f: ComponentForm) {
  return {
    name: f.name.trim(),
    description: f.description.trim(),
    lead: f.lead?.id ?? null,
    assigneeType: f.assigneeType,
  };
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

/** Where a version stands: archived wins over released. */
export type VersionState = 'archived' | 'released' | 'unreleased';

export function versionState(v: Pick<VersionLike, 'released' | 'archived'>): VersionState {
  return v.archived ? 'archived' : v.released ? 'released' : 'unreleased';
}

/** An unreleased version whose release day is already past (days are "YYYY-MM-DD"). */
export function versionLate(
  v: Pick<VersionRow, 'released' | 'archived' | 'releaseDate'>,
  today: string,
): boolean {
  return !v.released && !v.archived && !!v.releaseDate && v.releaseDate < today;
}

/** What the version dialog edits. */
export interface VersionForm {
  name: string;
  description: string;
  startDate: string;
  releaseDate: string;
}

export type VersionProblems = Partial<
  Record<'start' | 'release' | 'dates', DayProblem | 'startAfterRelease'>
>;

export function versionProblems(f: VersionForm): VersionProblems {
  const out: VersionProblems = {};
  const s = dayProblem(f.startDate);
  const r = dayProblem(f.releaseDate);
  if (s) out.start = s;
  if (r) out.release = r;
  if (!s && !r) {
    const d = datesProblem(f.startDate.trim(), f.releaseDate.trim());
    if (d) out.dates = d;
  }
  return out;
}

/** The body of POST/PATCH versions: an empty day clears it. */
export function versionPayload(f: VersionForm): VersionInput {
  return {
    name: f.name.trim(),
    description: f.description.trim(),
    startDate: f.startDate.trim() || null,
    releaseDate: f.releaseDate.trim() || null,
  };
}

/**
 * The ids in the order they take when `id` moves one place up (-1) or down
 * (1); null at an end of the list or when `id` is not there.
 */
export function orderAfterStep(ids: readonly number[], id: number, by: -1 | 1): number[] | null {
  const from = ids.indexOf(id);
  const to = from + by;
  if (from < 0 || to < 0 || to >= ids.length) return null;
  return moveItem(ids, from, to);
}

/** A list with one item moved from one place to another (reordering). */
export function moveItem<T>(list: readonly T[], from: number, to: number): T[] {
  const out = [...list];
  if (from < 0 || from >= out.length || to < 0 || to >= out.length) return out;
  const [item] = out.splice(from, 1);
  out.splice(to, 0, item as T);
  return out;
}

/* Project roles ------------------------------------------------------------- */

export interface RoleMembers {
  users: number[];
  groups: number[];
}

/** Who holds a role, after one user or group is added or removed (no duplicates). */
export function withMember(
  role: { users: readonly { id: number }[]; groups: readonly { id: number }[] },
  kind: 'user' | 'group',
  id: number,
  on: boolean,
): RoleMembers {
  const users = role.users.map((u) => u.id);
  const groups = role.groups.map((g) => g.id);
  const list = kind === 'user' ? users : groups;
  const rest = list.filter((x) => x !== id);
  const next = on ? [...rest, id] : rest;
  return kind === 'user' ? { users: next, groups } : { users, groups: next };
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
