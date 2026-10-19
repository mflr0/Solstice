/**
 * Logic of the shared catalogues' administration (issue types, priorities,
 * resolutions, link types, statuses) and of custom fields: ordering,
 * checks before sending, what to send, where used items can go. Problems
 * are keys; the pages word them (src/i18n, blocks catalogAdmin and
 * fieldsAdmin).
 */

import type {
  CatalogItem,
  CatalogKind,
  FieldOption,
  FieldRow,
  FieldType,
  StatusRow,
} from './admin-types.js';
import { tooLong, type Problems } from './admin-forms.js';

/** The four catalogues of the "Tickets" group, as the navigation names them. */
export type CatalogSection = 'issueTypes' | 'priorities' | 'resolutions' | 'linkTypes';

/** The API's name of each catalogue (/api/admin/<kind>). */
export const CATALOG_KINDS: Record<CatalogSection, CatalogKind> = {
  issueTypes: 'issue-types',
  priorities: 'priorities',
  resolutions: 'resolutions',
  linkTypes: 'link-types',
};

/* Ordering ------------------------------------------------------------------ */

/** A copy of the list with the item at `from` moved to `to` (both clamped). */
export function moveItem<T>(list: readonly T[], from: number, to: number): T[] {
  const out = [...list];
  if (from < 0 || from >= out.length) return out;
  const target = Math.max(0, Math.min(out.length - 1, to));
  const [item] = out.splice(from, 1);
  out.splice(target, 0, item as T);
  return out;
}

/** Whether two lists hold the same ids in the same order. */
export function sameOrder(
  a: readonly { id: string | number }[],
  b: readonly { id: string | number }[],
) {
  return a.length === b.length && a.every((x, i) => String(x.id) === String(b[i]?.id));
}

/* Colours ------------------------------------------------------------------- */

/** "#rrggbb" (any case). */
export function isHexColor(s: string): boolean {
  return /^#[0-9a-fA-F]{6}$/.test(s.trim());
}

/**
 * A colour as typed, made "#rrggbb" lower case: accepts a missing "#" and
 * the short "#rgb" form. Returns null when it is not a colour.
 */
export function normalizeColor(s: string): string | null {
  let v = s.trim().toLowerCase();
  if (v === '') return '';
  if (!v.startsWith('#')) v = `#${v}`;
  if (/^#[0-9a-f]{3}$/.test(v)) v = `#${v[1]}${v[1]}${v[2]}${v[2]}${v[3]}${v[3]}`;
  return /^#[0-9a-f]{6}$/.test(v) ? v : null;
}

/* Catalogue items ----------------------------------------------------------- */

export interface CatalogForm {
  name: string;
  description: string;
  subtask: boolean;
  color: string;
  default: boolean;
  outward: string;
  inward: string;
}

export type CatalogProblem =
  'nameRequired' | 'nameTooLong' | 'nameTaken' | 'colorInvalid' | 'verbRequired';

export function catalogForm(item: CatalogItem | null): CatalogForm {
  return {
    name: item?.name ?? '',
    description: item?.description ?? '',
    subtask: item?.subtask ?? false,
    color: item?.color ?? '',
    default: item?.default ?? false,
    outward: item?.outward ?? '',
    inward: item?.inward ?? '',
  };
}

/** Whether another item of the list already has this name (case does not count). */
export function nameTaken(
  name: string,
  items: readonly { id: string | number; name: string }[],
  except: string | number | null,
): boolean {
  const n = name.trim().toLowerCase();
  return items.some((i) => String(i.id) !== String(except) && i.name.trim().toLowerCase() === n);
}

export function catalogProblems(
  section: CatalogSection,
  f: CatalogForm,
  items: readonly CatalogItem[],
  editing: string | number | null,
): Problems<'name' | 'color' | 'outward' | 'inward', CatalogProblem> {
  const out: Problems<'name' | 'color' | 'outward' | 'inward', CatalogProblem> = {};
  if (!f.name.trim()) out.name = 'nameRequired';
  else if (tooLong(f.name)) out.name = 'nameTooLong';
  else if (nameTaken(f.name, items, editing)) out.name = 'nameTaken';
  if (section === 'priorities' && normalizeColor(f.color) === null) out.color = 'colorInvalid';
  if (section === 'linkTypes') {
    if (!f.outward.trim()) out.outward = 'verbRequired';
    else if (tooLong(f.outward)) out.outward = 'nameTooLong';
    if (!f.inward.trim()) out.inward = 'verbRequired';
    else if (tooLong(f.inward)) out.inward = 'nameTooLong';
  }
  return out;
}

/**
 * What to send for an item: only the fields its catalogue takes (the
 * server refuses others) and, for a change, only those that changed.
 */
export function catalogPayload(
  section: CatalogSection,
  f: CatalogForm,
  before: CatalogItem | null,
): Partial<CatalogItem> {
  const all: Partial<CatalogItem> = { name: f.name.trim() };
  if (section !== 'linkTypes') all.description = f.description.trim();
  if (section === 'issueTypes') all.subtask = f.subtask;
  if (section === 'priorities') {
    all.color = normalizeColor(f.color) ?? '';
    // Only ever set: another priority becoming the default unsets this one.
    if (f.default) all.default = true;
  }
  // A resolution's default can be removed: done issues then stay unresolved.
  if (section === 'resolutions') all.default = f.default;
  if (section === 'linkTypes') {
    all.outward = f.outward.trim();
    all.inward = f.inward.trim();
  }
  if (!before) return all;
  const out: Partial<CatalogItem> = {};
  for (const [k, v] of Object.entries(all) as [keyof CatalogItem, unknown][]) {
    const was = before[k] ?? (k === 'description' || k === 'color' ? '' : undefined);
    if (k === 'default' ? (before.default ?? false) !== v : was !== v)
      (out as Record<string, unknown>)[k] = v;
  }
  return out;
}

/** How many issues (or links) use an item. */
export function usageOf(item: CatalogItem): number {
  return item.links ?? item.issues ?? 0;
}

/** The items a used item's issues (or links) may move to: the others, of the same kind for issue types. */
export function moveTargets(
  section: CatalogSection,
  items: readonly CatalogItem[],
  item: CatalogItem,
): CatalogItem[] {
  return items.filter(
    (i) =>
      String(i.id) !== String(item.id) &&
      (section !== 'issueTypes' || Boolean(i.subtask) === Boolean(item.subtask)),
  );
}

/** Whether an issue type's sub-task flag may change: only while no issue uses it. */
export function subtaskLocked(item: CatalogItem | null): boolean {
  return item !== null && (item.issues ?? 0) > 0;
}

/* Statuses ------------------------------------------------------------------ */

export type StatusProblem = 'nameRequired' | 'nameTooLong' | 'nameTaken';

export function statusProblems(
  name: string,
  statuses: readonly { id: string; name: string }[],
  editing: string | null,
): Problems<'name', StatusProblem> {
  if (!name.trim()) return { name: 'nameRequired' };
  if (tooLong(name)) return { name: 'nameTooLong' };
  if (nameTaken(name, statuses, editing)) return { name: 'nameTaken' };
  return {};
}

/** What to send to change a status: only what differs from `before`. */
export function statusPayload(
  before: Pick<StatusRow, 'name' | 'category'>,
  name: string,
  category: StatusRow['category'],
): { name?: string; category?: string } {
  const out: { name?: string; category?: string } = {};
  if (name.trim() !== before.name) out.name = name.trim();
  if (category !== before.category) out.category = category;
  return out;
}

/** Whether nothing uses a status any more (it may then be deleted). */
export function statusUnused(s: StatusRow): boolean {
  return s.usage.issues + s.usage.workflows + s.usage.boardColumns === 0;
}

/* Custom fields ------------------------------------------------------------- */

/** Field types as the creation dialog groups them. */
export const FIELD_TYPE_GROUPS: readonly { group: FieldGroup; types: readonly FieldType[] }[] = [
  { group: 'text', types: ['text', 'textarea', 'url'] },
  { group: 'choice', types: ['select', 'multiselect', 'radio', 'checkboxes'] },
  { group: 'values', types: ['number', 'date', 'datetime', 'labels'] },
  { group: 'people', types: ['user', 'multiuser'] },
];

export type FieldGroup = 'text' | 'choice' | 'values' | 'people';

const OPTION_TYPES: readonly FieldType[] = ['select', 'multiselect', 'radio', 'checkboxes'];

/** Whether a field type takes a list of options. */
export function hasOptions(type: FieldType | null): boolean {
  return type !== null && OPTION_TYPES.includes(type);
}

/** Options typed one per line: trimmed, blank lines left out. */
export function parseOptions(text: string): string[] {
  return text
    .split(/\r?\n/)
    .map((l) => l.trim())
    .filter((l) => l !== '');
}

/** The first option typed twice (case does not count), if any. */
export function duplicateOption(options: readonly string[]): string | null {
  const seen = new Set<string>();
  for (const o of options) {
    const k = o.toLowerCase();
    if (seen.has(k)) return o;
    seen.add(k);
  }
  return null;
}

export interface FieldForm {
  name: string;
  description: string;
  type: FieldType;
  /** Options, one per line (creation only). */
  options: string;
  allProjects: boolean;
  projects: string[];
  allTypes: boolean;
  issueTypes: string[];
}

export type FieldProblem =
  | 'nameRequired'
  | 'nameTooLong'
  | 'nameTaken'
  | 'optionsRequired'
  | 'optionTooLong'
  | 'optionDuplicate'
  | 'projectsRequired'
  | 'typesRequired';

export function newFieldForm(): FieldForm {
  return {
    name: '',
    description: '',
    type: 'text',
    options: '',
    allProjects: true,
    projects: [],
    allTypes: true,
    issueTypes: [],
  };
}

export function fieldProblems(
  f: FieldForm,
  fields: readonly { id: number; name: string }[],
  editing: number | null,
): Problems<'name' | 'options' | 'projects' | 'issueTypes', FieldProblem> {
  const out: Problems<'name' | 'options' | 'projects' | 'issueTypes', FieldProblem> = {};
  if (!f.name.trim()) out.name = 'nameRequired';
  else if (tooLong(f.name)) out.name = 'nameTooLong';
  else if (nameTaken(f.name, fields, editing)) out.name = 'nameTaken';
  if (editing === null && hasOptions(f.type)) {
    const options = parseOptions(f.options);
    if (options.length === 0) out.options = 'optionsRequired';
    else if (options.some((o) => tooLong(o))) out.options = 'optionTooLong';
    else if (duplicateOption(options)) out.options = 'optionDuplicate';
  }
  if (!f.allProjects && f.projects.length === 0) out.projects = 'projectsRequired';
  if (!f.allTypes && f.issueTypes.length === 0) out.issueTypes = 'typesRequired';
  return out;
}

/** The context to send: empty lists mean every project, every issue type. */
export function fieldContextPayload(f: FieldForm): { projects: string[]; issueTypes: string[] } {
  return {
    projects: f.allProjects ? [] : [...f.projects],
    issueTypes: f.allTypes ? [] : [...f.issueTypes],
  };
}

/* Custom field: change, contexts, options ------------------------------------ */

/** A copy of the list with `value` in it (once) or out of it. */
export function toggleIn<T>(list: readonly T[], value: T, on: boolean): T[] {
  const without = list.filter((x) => x !== value);
  return on ? [...without, value] : without;
}

/** The form of an existing field: name, description and context (the type and options do not change here). */
export function fieldFormOf(f: FieldRow): FieldForm {
  return {
    name: f.name,
    description: f.description ?? '',
    type: f.type ?? 'text',
    options: '',
    allProjects: f.context.projects.length === 0,
    projects: f.context.projects.map((p) => p.key),
    allTypes: f.context.issueTypes.length === 0,
    issueTypes: f.context.issueTypes.map((t) => t.id),
  };
}

const sameSet = (a: readonly string[], b: readonly string[]) =>
  a.length === b.length && a.every((x) => b.includes(x));

/**
 * What to send to change a field: only the name, description and context
 * that differ from the field as it is. An empty object: nothing to save.
 */
export function fieldChanges(
  before: FieldRow,
  f: FieldForm,
): {
  name?: string;
  description?: string | null;
  context?: { projects: string[]; issueTypes: string[] };
} {
  const out: ReturnType<typeof fieldChanges> = {};
  if (f.name.trim() !== before.name) out.name = f.name.trim();
  if (f.description.trim() !== (before.description ?? '')) {
    out.description = f.description.trim() === '' ? null : f.description.trim();
  }
  const now = fieldContextPayload(f);
  if (
    !sameSet(
      now.projects,
      before.context.projects.map((p) => p.key),
    ) ||
    !sameSet(
      now.issueTypes,
      before.context.issueTypes.map((t) => t.id),
    )
  ) {
    out.context = now;
  }
  return out;
}

/** The end of a Jira type key: "…customfieldtypes:cascadingselect" gives "cascadingselect". */
export function typeKind(typeKey: string): string {
  return typeKey.slice(typeKey.lastIndexOf(':') + 1);
}

/**
 * Whether a field's options can be managed here: the types Solstice
 * creates with options, and a cascading field imported from Jira (its top
 * level only).
 */
export function optionsEditable(f: Pick<FieldRow, 'type' | 'typeKey'>): boolean {
  return hasOptions(f.type) || typeKind(f.typeKey) === 'cascadingselect';
}

export type OptionProblem = 'optionRequired' | 'optionTooLong' | 'optionTaken';

/** Checks an option's text against the field's other options (case does not count). */
export function optionProblem(
  value: string,
  options: readonly { id: number; value: string }[],
  editing: number | null,
): OptionProblem | null {
  if (!value.trim()) return 'optionRequired';
  if (tooLong(value)) return 'optionTooLong';
  const v = value.trim().toLowerCase();
  if (options.some((o) => o.id !== editing && o.value.trim().toLowerCase() === v)) {
    return 'optionTaken';
  }
  return null;
}

/** The options an option's values may move to when it is deleted: the other enabled ones. */
export function optionTargets(options: readonly FieldOption[], option: FieldOption): FieldOption[] {
  return options.filter((o) => o.id !== option.id && !o.disabled);
}
