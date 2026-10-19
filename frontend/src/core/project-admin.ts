/**
 * Projects as administrators shape them: the creation form (key suggested
 * from the name), the project settings tabs, versions and components. Pure
 * logic; the pages word the problems (src/i18n, blocks projectsAdmin and
 * projectConfig).
 */

import { matches, tooLong } from './admin-forms.js';
import type {
  AssigneeType,
  Category,
  ComponentInput,
  ComponentRow,
  ExtraRecipient,
  ProjectNotifications,
  ProjectPatch,
  VersionInput,
  VersionRow,
} from './admin-types.js';
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
  else if (tooLong(name)) out.name = 'nameTooLong';
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
  if (tooLong(n)) return 'nameTooLong';
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

/* General tab ----------------------------------------------------------------- */

/** What the General tab edits. */
export interface GeneralForm {
  name: string;
  description: string;
  lead: UserRef | null;
  category: number | null;
  defaultAssignee: 'unassigned' | 'lead';
  /** The issue types offered, in the project's order. */
  issueTypes: string[];
}

export function generalForm(s: ProjectSettings): GeneralForm {
  return {
    name: s.name,
    description: s.description ?? '',
    lead: s.lead,
    category: s.category?.id ?? null,
    defaultAssignee: s.defaultAssignee,
    issueTypes: s.issueTypes.map((i) => i.id),
  };
}

export type GeneralProblem = 'nameRequired' | 'nameTooLong' | 'typesRequired';

export function generalProblems(
  f: GeneralForm,
  allTypes: readonly { id: string; subtask: boolean }[],
): Partial<Record<'name' | 'types', GeneralProblem>> {
  const out: Partial<Record<'name' | 'types', GeneralProblem>> = {};
  const name = f.name.trim();
  if (!name) out.name = 'nameRequired';
  else if (tooLong(name)) out.name = 'nameTooLong';
  const types = issueTypesProblem(f.issueTypes, allTypes);
  if (types) out.types = types;
  return out;
}

/** An issue type added at the end of the list or taken out of it. */
export function toggleIssueType(list: readonly string[], id: string, on: boolean): string[] {
  const rest = list.filter((x) => x !== id);
  return on ? [...rest, id] : rest;
}

/** The body of PATCH /api/projects/{key}: only what the form changed. */
export function generalPatch(before: ProjectSettings, f: GeneralForm): ProjectPatch {
  const patch: ProjectPatch = {};
  const name = f.name.trim();
  if (name !== before.name) patch.name = name;
  const description = f.description.trim();
  if (description !== (before.description ?? '')) patch.description = description || null;
  if (f.lead && f.lead.id !== before.lead?.id) patch.lead = f.lead.id;
  if (f.category !== (before.category?.id ?? null)) patch.category = f.category;
  if (f.defaultAssignee !== before.defaultAssignee) patch.defaultAssignee = f.defaultAssignee;
  const had = before.issueTypes.map((i) => i.id);
  if (f.issueTypes.length !== had.length || f.issueTypes.some((id) => !had.includes(id))) {
    patch.issueTypes = f.issueTypes;
  }
  return patch;
}

/** Whether the form differs from the saved settings. */
export function generalChanged(before: ProjectSettings, f: GeneralForm): boolean {
  return Object.keys(generalPatch(before, f)).length > 0;
}

/**
 * The issue types still used by issues, from the 409 body of a refused
 * change ({inUse: [{type, issues}]}); empty when the body says otherwise.
 */
export function typesInUse(data: unknown): { name: string; issues: number }[] {
  const list = (data as { inUse?: unknown } | null | undefined)?.inUse;
  if (!Array.isArray(list)) return [];
  return list.flatMap((x: { type?: { name?: unknown }; issues?: unknown }) =>
    typeof x?.type?.name === 'string' && typeof x.issues === 'number'
      ? [{ name: x.type.name, issues: x.issues }]
      : [],
  );
}

/* Components tab -------------------------------------------------------------- */

export const ASSIGNEE_TYPES: readonly AssigneeType[] = [
  'project_default',
  'component_lead',
  'project_lead',
  'unassigned',
];

export type NameProblem = 'nameRequired' | 'nameTooLong' | 'nameTaken';

/** A component or version name: present, 255 characters at most, and not another's (any case). */
export function uniqueNameProblem(
  name: string,
  others: readonly { id: number; name: string }[],
  self: number | null,
): NameProblem | null {
  const n = name.trim();
  if (!n) return 'nameRequired';
  if (tooLong(n)) return 'nameTooLong';
  const lower = n.toLowerCase();
  return others.some((o) => o.id !== self && o.name.trim().toLowerCase() === lower)
    ? 'nameTaken'
    : null;
}

/** What the component dialog edits. */
export interface ComponentForm {
  name: string;
  description: string;
  lead: UserRef | null;
  assigneeType: AssigneeType;
}

export function componentForm(c: ComponentRow | null): ComponentForm {
  return {
    name: c?.name ?? '',
    description: c?.description ?? '',
    lead: c?.lead ?? null,
    assigneeType: c?.assigneeType ?? 'project_default',
  };
}

/**
 * The body of POST (no `before`) or PATCH on a component. A change sends only
 * the fields that differ; an emptied lead goes as null, which clears it.
 */
export function componentPayload(f: ComponentForm, before: ComponentRow | null): ComponentInput {
  const name = f.name.trim();
  const description = f.description.trim();
  if (!before) {
    return {
      name,
      ...(description ? { description } : {}),
      ...(f.lead ? { lead: f.lead.id } : {}),
      assigneeType: f.assigneeType,
    };
  }
  const out: ComponentInput = {};
  if (name !== before.name) out.name = name;
  if (description !== (before.description ?? '')) out.description = description;
  if ((f.lead?.id ?? null) !== (before.lead?.id ?? null)) out.lead = f.lead?.id ?? null;
  if (f.assigneeType !== before.assigneeType) out.assigneeType = f.assigneeType;
  return out;
}

/* Versions tab ---------------------------------------------------------------- */

/** What the version dialog edits. Days are "YYYY-MM-DD"; empty is no date. */
export interface VersionForm {
  name: string;
  description: string;
  startDate: string;
  releaseDate: string;
}

export function versionForm(v: VersionRow | null): VersionForm {
  return {
    name: v?.name ?? '',
    description: v?.description ?? '',
    startDate: v?.startDate ?? '',
    releaseDate: v?.releaseDate ?? '',
  };
}

export function versionProblems(
  f: VersionForm,
  versions: readonly { id: number; name: string }[],
  self: number | null,
): { name?: NameProblem; dates?: 'startAfterRelease' } {
  const out: { name?: NameProblem; dates?: 'startAfterRelease' } = {};
  const name = uniqueNameProblem(f.name, versions, self);
  if (name) out.name = name;
  const dates = datesProblem(f.startDate, f.releaseDate);
  if (dates) out.dates = dates;
  return out;
}

/**
 * The body of POST (no `before`) or PATCH on a version. A change sends only
 * the fields that differ; an emptied day goes as null, which clears it.
 */
export function versionPayload(f: VersionForm, before: VersionRow | null): VersionInput {
  const name = f.name.trim();
  const description = f.description.trim();
  if (!before) {
    return {
      name,
      ...(description ? { description } : {}),
      ...(f.startDate ? { startDate: f.startDate } : {}),
      ...(f.releaseDate ? { releaseDate: f.releaseDate } : {}),
    };
  }
  const out: VersionInput = {};
  if (name !== before.name) out.name = name;
  if (description !== (before.description ?? '')) out.description = description;
  if (f.startDate !== (before.startDate ?? '')) out.startDate = f.startDate || null;
  if (f.releaseDate !== (before.releaseDate ?? '')) out.releaseDate = f.releaseDate || null;
  return out;
}

/** Whether the e-mail switch or the extra recipients differ from what was saved. */
export function notificationsChanged(
  saved: Pick<ProjectNotifications, 'enabled' | 'extra'>,
  enabled: boolean,
  extra: readonly ExtraRecipient[],
): boolean {
  return saved.enabled !== enabled || extraSignature(saved.extra) !== extraSignature(extra);
}
