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
import { fold } from './text.js';
import type {
  Effect,
  Grant,
  GroupRule,
  Holder,
  IdName,
  Permission,
  PermUser,
  TypeRef,
  UserRef,
} from './types.js';

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
  defaultAssignee: 'unassigned' | 'lead' | 'reporter';
  archived: boolean;
  /** The issue types allowed in the project, in order. */
  issueTypes: TypeRef[];
  issues: number;
}

/* Keys ---------------------------------------------------------------------- */

/** A project key: an upper-case letter, then 1 to 9 upper-case letters or digits. */
export const KEY_PATTERN = /^[A-Z][A-Z0-9]{1,9}$/;

/** What may be typed in the key field: upper case, letters and digits, 10 at most. */
export function normalizeKey(s: string): string {
  return fold(s)
    .toUpperCase()
    .replace(/[^A-Z0-9]/g, '')
    .slice(0, 10);
}

/**
 * A key suggested from a project name: the initials of several words
 * ("Site web" → "SW"), the start of a single one ("Marketing" → "MARK").
 */
export function suggestKey(name: string): string {
  const words = fold(name)
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

export type AccessKind = 'lead' | 'group' | 'everyone';

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
  defaultAssignee?: 'unassigned' | 'lead' | 'reporter';
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
  if (f.access === 'group' && !f.accessId) out.access = 'accessRequired';
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
    lead: f.lead?.id ?? null,
    ...(f.issueTypes ? { issueTypes: f.issueTypes } : {}),
    ...(f.defaultAssignee ? { defaultAssignee: f.defaultAssignee } : {}),
    category: f.category,
    access:
      f.access === 'everyone'
        ? ({ holder: 'everyone' } as const)
        : f.access === 'group' && f.accessId
          ? ({ holder: 'group', id: f.accessId } as const)
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
  'import',
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

/* CSV import ---------------------------------------------------------------- */

/**
 * The mapping after a column gets a field: a field goes to one column only,
 * so the column that had it loses it ("" leaves the column out).
 */
export function assignColumn<C extends { field: string }>(
  columns: C[],
  index: number,
  field: string,
): C[] {
  return columns.map((c, i) => {
    if (i === index) return { ...c, field };
    return field !== '' && c.field === field ? { ...c, field: '' } : c;
  });
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
  kind: 'group',
  id: number,
): boolean {
  return extra.some((e) => e.event === event && e.kind === kind && e.id === id);
}

/** Adds or removes a group among those an event is sent to. */
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
  defaultAssignee: 'unassigned' | 'lead' | 'reporter';
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
  if ((f.lead?.id ?? null) !== (before.lead?.id ?? null)) patch.lead = f.lead?.id ?? null;
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

/* Built-in groups ------------------------------------------------------------ */

/** The holders shown as built-in groups, in the order group lists offer them. */
export const BUILTIN_HOLDERS = ['everyone', 'lead', 'reporter', 'assignee'] as const;

export type BuiltinHolder = (typeof BUILTIN_HOLDERS)[number];

/** Whether a holder is shown as a built-in group. */
export function isBuiltinHolder(holder: Holder): holder is BuiltinHolder {
  return (BUILTIN_HOLDERS as readonly string[]).includes(holder);
}

/** A group a holder list offers: a built-in group, or a group of the instance. */
export interface HolderGroup {
  /** Stable across languages: `builtin:<holder>` or `group:<id>`. */
  key: string;
  label: string;
  builtin: boolean;
}

/** The key of the group a holder is; null for a user, or a group without its id. */
export function holderGroupKey(holder: Holder, id?: number | null): string | null {
  if (isBuiltinHolder(holder)) return `builtin:${holder}`;
  if (holder === 'group' && id != null) return `group:${id}`;
  return null;
}

/** The holder a group's key stands for, as the API stores it; null for an unknown key. */
export function groupKeyHolder(key: string): { holder: Holder; id?: number } | null {
  const [kind, rest] = key.split(':', 2);
  if (kind === 'builtin') {
    const holder = BUILTIN_HOLDERS.find((h) => h === rest);
    return holder ? { holder } : null;
  }
  if (kind === 'group' && rest && /^[0-9]+$/.test(rest))
    return { holder: 'group', id: Number(rest) };
  return null;
}

/** The groups a holder list offers: the built-in ones first, then the instance's. */
export function holderGroups(
  labels: Record<BuiltinHolder, string>,
  groups: readonly Pick<IdName, 'id' | 'name'>[],
): HolderGroup[] {
  return [
    ...BUILTIN_HOLDERS.map((h) => ({ key: `builtin:${h}`, label: labels[h], builtin: true })),
    ...groups.map((g) => ({ key: `group:${g.id}`, label: g.name, builtin: false })),
  ];
}

/* Permission matrix ---------------------------------------------------------- */

/** A cell of the permission matrix: the row's project rule, or none. */
export type CellState = 'inherit' | Effect;

/** The states in the order the switch shows them: block, inherit, grant. */
export const CELL_STATES: readonly CellState[] = ['deny', 'inherit', 'allow'];

/** The state Space moves a cell to: inherit, then grant, then block. */
export function nextCellState(s: CellState): CellState {
  return s === 'inherit' ? 'allow' : s === 'allow' ? 'deny' : 'inherit';
}

/** A row of the matrix: a built-in group, a group or a user. */
export interface MatrixRow {
  /** `builtin:<holder>`, `group:<id>` or `user:<id>`. */
  key: string;
  holder: Holder;
  id?: number;
  label: string;
  builtin: boolean;
}

/** The key of the row a grant belongs to; null for a group or user without its id. */
export function grantRowKey(g: Pick<Grant, 'holder' | 'id'>): string | null {
  if (g.holder === 'user') return g.id != null ? `user:${g.id}` : null;
  return holderGroupKey(g.holder, g.id);
}

/**
 * The rows of the matrix: the built-in groups, then the groups and the users
 * with a project rule or just added (`extra`), each by name.
 */
export function matrixRows(
  labels: Record<BuiltinHolder, string>,
  grants: readonly Grant[],
  groups: readonly Pick<IdName, 'id' | 'name'>[],
  users: readonly Pick<PermUser, 'id' | 'name'>[],
  extra: readonly string[] = [],
): MatrixRow[] {
  const keys = new Set(extra);
  for (const g of grants) {
    const k = grantRowKey(g);
    if (k) keys.add(k);
  }
  const groupRows: MatrixRow[] = [];
  const userRows: MatrixRow[] = [];
  for (const k of keys) {
    const [kind, rest] = k.split(':', 2);
    const id = Number(rest);
    if (kind === 'group') {
      const name =
        groups.find((g) => g.id === id)?.name ?? grants.find((g) => grantRowKey(g) === k)?.name;
      groupRows.push({ key: k, holder: 'group', id, label: name ?? rest ?? '', builtin: false });
    } else if (kind === 'user') {
      const name =
        users.find((u) => u.id === id)?.name ?? grants.find((g) => grantRowKey(g) === k)?.name;
      userRows.push({ key: k, holder: 'user', id, label: name ?? rest ?? '', builtin: false });
    }
  }
  const byName = (a: MatrixRow, b: MatrixRow) => a.label.localeCompare(b.label);
  return [
    ...BUILTIN_HOLDERS.map((h) => ({
      key: `builtin:${h}`,
      holder: h,
      label: labels[h],
      builtin: true,
    })),
    ...groupRows.sort(byName),
    ...userRows.sort(byName),
  ];
}

/** A cell's project rule: deny if the row has both, as the server decides. */
export function cellState(grants: readonly Grant[], row: string, p: Permission): CellState {
  let state: CellState = 'inherit';
  for (const g of grants) {
    if (g.permission !== p || grantRowKey(g) !== row) continue;
    if ((g.effect ?? 'allow') === 'deny') return 'deny';
    state = 'allow';
  }
  return state;
}

/** The grants with a cell set to a state. */
export function setCell(
  grants: readonly Grant[],
  row: MatrixRow,
  p: Permission,
  s: CellState,
): Grant[] {
  const out = grants.filter((g) => g.permission !== p || grantRowKey(g) !== row.key);
  if (s === 'inherit') return out;
  const g: Grant = { permission: p, holder: row.holder, effect: s };
  if (row.id != null) {
    g.id = row.id;
    g.name = row.label;
  }
  return [...out, g];
}

/** The grants with every cell of some rows and permissions set to a state: a whole row or column. */
export function setCells(
  grants: readonly Grant[],
  rows: readonly MatrixRow[],
  perms: readonly Permission[],
  s: CellState,
): Grant[] {
  let out = [...grants];
  for (const row of rows) for (const p of perms) out = setCell(out, row, p, s);
  return out;
}

/**
 * The state a key sets on a cell: 1, 2, 3, or the language's letter for
 * each state (`letters`, matched in any case); null for another key.
 */
export function stateForKey(key: string, letters: Record<CellState, string>): CellState | null {
  const digit = ['1', '2', '3'].indexOf(key);
  if (digit >= 0) return CELL_STATES[digit] ?? null;
  const k = key.toLowerCase();
  return CELL_STATES.find((s) => letters[s].toLowerCase() === k) ?? null;
}

/** The cell an arrow, Home or End key moves to in a grid; null for another key. */
export function moveCell(
  at: { row: number; col: number },
  key: string,
  rows: number,
  cols: number,
): { row: number; col: number } | null {
  const clamp = (n: number, max: number) => Math.max(0, Math.min(max - 1, n));
  switch (key) {
    case 'ArrowUp':
      return { row: clamp(at.row - 1, rows), col: at.col };
    case 'ArrowDown':
      return { row: clamp(at.row + 1, rows), col: at.col };
    case 'ArrowLeft':
      return { row: at.row, col: clamp(at.col - 1, cols) };
    case 'ArrowRight':
      return { row: at.row, col: clamp(at.col + 1, cols) };
    case 'Home':
      return { row: at.row, col: 0 };
    case 'End':
      return { row: at.row, col: cols - 1 };
  }
  return null;
}

/** The grants without any rule of a row. */
export function removeRow(grants: readonly Grant[], row: string): Grant[] {
  return grants.filter((g) => grantRowKey(g) !== row);
}

/** Levels of rules, from the most specific (backend/internal/perm). */
export type RuleLevel = 'projectUser' | 'globalUser' | 'projectGroup' | 'globalGroup' | 'none';

/** What a row gets for a permission, and the rule that decides. */
export interface CellAccess {
  granted: boolean;
  level: RuleLevel;
  /** The holder of the deciding rule, as a row key; null for level none. */
  source: string | null;
}

/** What decides the inherited values: the project's rules and what they inherit. */
export interface MatrixRules {
  grants: readonly Grant[];
  globals: readonly GroupRule[];
  users: readonly PermUser[];
}

/** Among rules of one level, the first deny, else the first allow. */
function decide<T extends { effect?: Effect }>(
  rules: T[],
  key: (r: T) => string | null,
  level: RuleLevel,
): CellAccess | null {
  const rl = rules.find((r) => r.effect === 'deny') ?? rules[0];
  return rl ? { granted: rl.effect !== 'deny', level, source: key(rl) } : null;
}

/**
 * What a row gets for a permission, in the server's order: a project user
 * rule, a global user rule, the project rules of the groups the user is in,
 * then their global rules; deny beats allow within a level. A group row
 * stands for its own rules only. With `own` false, the row's own project
 * rule is left out: what the cell inherits. Null for a user whose context
 * is not loaded yet.
 */
export function cellAccess(
  row: string,
  p: Permission,
  m: MatrixRules,
  own = true,
): CellAccess | null {
  const grants = m.grants.filter((g) => g.permission === p);
  const globals = m.globals.filter((g) => g.permission === p);
  const effect = (g: Grant) => ({ ...g, effect: g.effect ?? 'allow' });
  const none: CellAccess = { granted: false, level: 'none', source: null };
  if (!row.startsWith('user:')) {
    const mine = grants.filter((g) => grantRowKey(g) === row).map(effect);
    return (
      (own ? decide(mine, grantRowKey, 'projectGroup') : null) ??
      decide(
        globals.filter((g) => holderGroupKey(g.holder, g.id) === row),
        (g) => holderGroupKey(g.holder, g.id),
        'globalGroup',
      ) ??
      none
    );
  }
  if (own) {
    const mine = decide(
      grants.filter((g) => grantRowKey(g) === row).map(effect),
      grantRowKey,
      'projectUser',
    );
    if (mine) return mine;
  }
  const user = m.users.find((u) => `user:${u.id}` === row);
  if (!user) return null;
  const inGroup = (k: string | null) => k != null && user.groups.includes(k);
  return (
    decide(
      user.rules.filter((r) => r.permission === p),
      () => row,
      'globalUser',
    ) ??
    decide(
      grants.filter((g) => g.holder !== 'user' && inGroup(grantRowKey(g))).map(effect),
      grantRowKey,
      'projectGroup',
    ) ??
    decide(
      globals.filter((g) => inGroup(holderGroupKey(g.holder, g.id))),
      (g) => holderGroupKey(g.holder, g.id),
      'globalGroup',
    ) ??
    none
  );
}

/** The project rules as the server takes them. */
export function grantsPayload(grants: readonly Grant[]): Grant[] {
  return grants.map(({ permission, holder, id, effect }) => ({
    permission,
    holder,
    ...(id ? { id } : {}),
    ...(effect ? { effect } : {}),
  }));
}

/** Compares two sets of project rules, whatever their order. */
export function grantsSignature(grants: readonly Grant[]): string {
  return grants
    .map((g) => `${g.permission}:${g.holder}:${g.id ?? ''}:${g.effect ?? 'allow'}`)
    .sort()
    .join('|');
}
