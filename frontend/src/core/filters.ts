/**
 * The search page's filters, and how they read and write JQL. The filters
 * are a plain conjunction: each field holds alternatives (OR), fields all
 * apply (AND). A query the filters cannot hold exactly is left to the JQL
 * editor rather than approximated.
 *
 * Values are kept as JQL values (quoted when needed, functions with their
 * parentheses, EMPTY), so writing a clause is a join and nothing is lost.
 */

import {
  conjunction,
  fieldInfo,
  formatOrder,
  formatValue,
  functionInfo,
  JqlError,
  parse,
  quote,
  quoteField,
  quoteString,
  type Clause,
  type SortTerm,
  type Value,
} from './jql.js';
import { STATUS_CATEGORIES, type StatusCategory } from './types.js';

/** Fields chosen from a list of values. */
export const LIST_FIELDS = [
  'project',
  'people',
  'type',
  'status',
  'assignee',
  'priority',
  'labels',
  'reporter',
  'resolution',
  'sprint',
  'epic',
  'fixVersion',
  'component',
] as const;
export type ListField = (typeof LIST_FIELDS)[number];

export const DATE_FIELDS = ['created', 'updated', 'due', 'resolved'] as const;
export type DateField = (typeof DATE_FIELDS)[number];

/**
 * The roles the « Personnes » filter can hold, each its own JQL field. The
 * assignee and the reporter have their own filters, so they are not here.
 */
export const PEOPLE_ROLES = ['participant', 'creator', 'watcher', 'commenter'] as const;
export type PeopleRole = (typeof PEOPLE_ROLES)[number];

/** The JQL name each list field is written with (people: see peopleRole). */
export const JQL_FIELD: Record<ListField, string> = {
  project: 'project',
  people: 'participant',
  type: 'type',
  status: 'status',
  assignee: 'assignee',
  priority: 'priority',
  labels: 'labels',
  reporter: 'reporter',
  resolution: 'resolution',
  sprint: 'sprint',
  epic: 'Epic Link',
  fixVersion: 'fixVersion',
  component: 'component',
};

/** Special values. */
export const ME = 'currentUser()';
export const EMPTY = 'EMPTY';

/** Fields that always have a value: "is EMPTY" means nothing there. */
const NEVER_EMPTY: ListField[] = ['project', 'type', 'status'];

export interface DateBound {
  op: '>' | '>=' | '<' | '<=';
  /** A JQL value: 2026-10-01, -7d, startOfWeek()… */
  value: string;
}

export interface DateRange {
  from: DateBound | null;
  to: DateBound | null;
  /** "is EMPTY": no date at all. */
  empty: boolean;
}

export interface FilterState {
  /** Words searched in the summary and description. */
  text: string;
  /** Chosen values per field, as JQL values. */
  values: Partial<Record<ListField, string[]>>;
  /** Status categories; none or all of them means no constraint. */
  categories: StatusCategory[];
  dates: Partial<Record<DateField, DateRange>>;
  /** What the people in values.people are to an issue (participant: any role). */
  peopleRole: PeopleRole;
  /** Empty: most recently updated first. */
  order: SortTerm[];
}

export function emptyFilters(): FilterState {
  return {
    text: '',
    values: {},
    categories: [],
    dates: {},
    peopleRole: 'participant',
    order: [],
  };
}

export function emptyRange(): DateRange {
  return { from: null, to: null, empty: false };
}

/** Whether a range constrains anything. */
export function rangeSet(r: DateRange | undefined): r is DateRange {
  return !!r && (r.empty || r.from !== null || r.to !== null);
}

/** Whether no filter is set (the sort aside). */
export function isBlank(s: FilterState): boolean {
  return (
    !s.text.trim() &&
    !LIST_FIELDS.some((f) => s.values[f]?.length) &&
    categoryClause(s.categories) === '' &&
    !DATE_FIELDS.some((f) => rangeSet(s.dates[f]))
  );
}

/* Writing ------------------------------------------------------------------- */

/** "field = v", "field in (a, b)", "field is EMPTY", "sprint in openSprints()". */
export function listClause(field: string, values: string[]): string {
  const f = quoteField(field);
  if (values.length > 0 && values.every((v) => v === EMPTY)) return `${f} is EMPTY`;
  if (values.length === 1) {
    const v = values[0] as string;
    const fn = /^([A-Za-z]+)\(/.exec(v)?.[1];
    return fn && functionInfo(fn)?.list ? `${f} in ${v}` : `${f} = ${v}`;
  }
  return `${f} in (${values.join(', ')})`;
}

/** The names JQL gives the status categories. */
export const CATEGORY_JQL: Record<StatusCategory, string> = {
  todo: 'To Do',
  doing: 'In Progress',
  done: 'Done',
};

/** The statusCategory clause, or '' when every or no category is chosen. */
export function categoryClause(categories: StatusCategory[]): string {
  const set = STATUS_CATEGORIES.filter((c) => categories.includes(c));
  if (set.length === 0 || set.length === STATUS_CATEGORIES.length) return '';
  if (set.length === 2 && !set.includes('done'))
    return `statusCategory != ${quote(CATEGORY_JQL.done)}`;
  const names = set.map((c) => quote(CATEGORY_JQL[c]));
  if (names.length === 1) return `statusCategory = ${names[0]}`;
  return `statusCategory in (${names.join(', ')})`;
}

function dateClauses(field: DateField, r: DateRange | undefined): string[] {
  if (!r) return [];
  if (r.empty) return [`${field} is EMPTY`];
  const out: string[] = [];
  if (r.from) out.push(`${field} ${r.from.op} ${r.from.value}`);
  if (r.to) out.push(`${field} ${r.to.op} ${r.to.value}`);
  return out;
}

/** The filters as JQL, sort included. */
export function filtersToJql(s: FilterState): string {
  const parts: string[] = [];
  for (const f of LIST_FIELDS) {
    const vals = s.values[f];
    if (vals?.length) parts.push(listClause(f === 'people' ? s.peopleRole : JQL_FIELD[f], vals));
    if (f === 'status') {
      const c = categoryClause(s.categories);
      if (c) parts.push(c);
    }
  }
  for (const f of DATE_FIELDS) parts.push(...dateClauses(f, s.dates[f]));
  const text = s.text.trim();
  if (text) parts.push(`text ~ ${quoteString(text)}`);
  return [parts.join(' AND '), formatOrder(s.order)].filter((p) => p !== '').join(' ');
}

/** Fields the server counts per value (GET /api/issues/facets). */
export const FACET_FIELDS: ListField[] = [
  'project',
  'type',
  'status',
  'assignee',
  'priority',
  'labels',
  'reporter',
  'resolution',
];

/**
 * The query whose counts a field's list shows: the search without that
 * field's own filter (so its other values keep their counts), unsorted.
 */
export function facetJql(s: FilterState, field: ListField): string {
  const values = { ...s.values };
  delete values[field];
  return filtersToJql({
    ...s,
    values,
    categories: field === 'status' ? [] : s.categories,
    order: [],
  });
}

/* Reading ------------------------------------------------------------------- */

export type ReadResult =
  | { ok: true; state: FilterState }
  | { ok: false; reason: 'syntax'; error: JqlError }
  | { ok: false; reason: 'complex' };

const LIST_NAMES = new Map<string, ListField>([
  ['project', 'project'],
  ['type', 'type'],
  ['issuetype', 'type'],
  ['status', 'status'],
  ['assignee', 'assignee'],
  ['priority', 'priority'],
  ['labels', 'labels'],
  ['label', 'labels'],
  ['reporter', 'reporter'],
  ['resolution', 'resolution'],
  ['sprint', 'sprint'],
  ['epic link', 'epic'],
  ['epiclink', 'epic'],
  ['fixversion', 'fixVersion'],
  ['component', 'component'],
]);

const DATE_NAMES = new Map<string, DateField>([
  ['created', 'created'],
  ['createddate', 'created'],
  ['updated', 'updated'],
  ['updateddate', 'updated'],
  ['due', 'due'],
  ['duedate', 'due'],
  ['resolved', 'resolved'],
  ['resolutiondate', 'resolved'],
]);

/** Category names the server accepts (jql.categories). */
const CATEGORY_NAMES: Record<string, StatusCategory> = {
  'to do': 'todo',
  todo: 'todo',
  new: 'todo',
  'à faire': 'todo',
  'a faire': 'todo',
  'in progress': 'doing',
  indeterminate: 'doing',
  doing: 'doing',
  'en cours': 'doing',
  done: 'done',
  complete: 'done',
  terminé: 'done',
  termine: 'done',
};

/** A list clause's values, or null when the filters cannot hold it. */
function readList(field: ListField, c: Clause, role?: PeopleRole): string[] | null {
  const single = c.op === '=' && c.values.length === 1;
  const empty = c.op === 'is' && c.values.length === 1 && c.values[0]?.kind === 'empty';
  if (!single && !empty && c.op !== 'in') return null;
  const out: string[] = [];
  for (const v of c.values) {
    let s: string;
    if (v.kind === 'empty') {
      if (NEVER_EMPTY.includes(field)) return null;
      // Nobody takes part in everything: only watchers and commenters may be none.
      if (field === 'people' && role !== 'watcher' && role !== 'commenter') return null;
      s = EMPTY;
    } else if (v.kind === 'func') {
      const fn = functionInfo(v.name);
      if (!fn || fn.kind !== fieldInfo(JQL_FIELD[field])?.kind) return null;
      s = formatValue(v);
    } else if (field === 'resolution' && v.text.toLowerCase() === 'unresolved') {
      s = EMPTY;
    } else {
      s = quote(v.text);
    }
    if (!out.includes(s)) out.push(s);
  }
  return out;
}

function readCategories(c: Clause): StatusCategory[] | null {
  if (!['=', '!=', 'in', 'not in'].includes(c.op)) return null;
  const set = new Set<StatusCategory>();
  for (const v of c.values) {
    if (v.kind !== 'text') return null;
    const cat = CATEGORY_NAMES[v.text.toLowerCase()];
    if (!cat) return null;
    set.add(cat);
  }
  const negate = c.op === '!=' || c.op === 'not in';
  return STATUS_CATEGORIES.filter((cat) => set.has(cat) !== negate);
}

function readDateValue(v: Value | undefined): string | null {
  if (!v || v.kind === 'empty') return null;
  if (v.kind === 'func') return functionInfo(v.name)?.kind === 'date' ? formatValue(v) : null;
  return quote(v.text);
}

/** Adds a date clause to a range; false when the filters cannot hold it. */
function readDate(r: DateRange, c: Clause): boolean {
  if (r.empty) return false;
  if (c.op === 'is') {
    if (r.from || r.to || c.values[0]?.kind !== 'empty') return false;
    r.empty = true;
    return true;
  }
  const value = c.values.length === 1 ? readDateValue(c.values[0]) : null;
  if (value === null) return false;
  if (c.op === '>' || c.op === '>=') {
    if (r.from) return false;
    r.from = { op: c.op, value };
    return true;
  }
  if (c.op === '<' || c.op === '<=') {
    if (r.to) return false;
    r.to = { op: c.op, value };
    return true;
  }
  return false;
}

/** JQL names of the people roles. */
const PEOPLE_NAMES = new Map<string, PeopleRole>([
  ['participant', 'participant'],
  ['participants', 'participant'],
  ['involves', 'participant'],
  ['creator', 'creator'],
  ['watcher', 'watcher'],
  ['commenter', 'commenter'],
]);

/** Reads JQL into filters, when it is a plain conjunction they can show. */
export function jqlToFilters(jql: string): ReadResult {
  let query;
  try {
    query = parse(jql);
  } catch (e) {
    if (e instanceof JqlError) return { ok: false, reason: 'syntax', error: e };
    throw e;
  }
  const clauses = conjunction(query.where);
  if (!clauses) return { ok: false, reason: 'complex' };
  const s = emptyFilters();
  let categories = false;
  for (const c of clauses) {
    if (c.preds.length) return { ok: false, reason: 'complex' };
    const name = c.field.toLowerCase();
    const list = LIST_NAMES.get(name);
    const date = DATE_NAMES.get(name);
    const role = PEOPLE_NAMES.get(name);
    if (role) {
      // One people filter: a second role (an AND) is left to the editor.
      const vals = s.values.people ? null : readList('people', c, role);
      if (!vals) return { ok: false, reason: 'complex' };
      s.values.people = vals;
      s.peopleRole = role;
    } else if (list) {
      const vals = s.values[list] ? null : readList(list, c);
      if (!vals) return { ok: false, reason: 'complex' };
      s.values[list] = vals;
    } else if (date) {
      const r = s.dates[date] ?? emptyRange();
      if (!readDate(r, c)) return { ok: false, reason: 'complex' };
      s.dates[date] = r;
    } else if (name === 'statuscategory') {
      const cats = categories ? null : readCategories(c);
      if (!cats) return { ok: false, reason: 'complex' };
      s.categories = cats;
      categories = true;
    } else if (
      name === 'text' &&
      c.op === '~' &&
      c.values.length === 1 &&
      c.values[0]?.kind === 'text' &&
      !s.text
    ) {
      s.text = c.values[0].text;
    } else {
      return { ok: false, reason: 'complex' };
    }
  }
  // The status list ticks either categories or statuses, as alternatives;
  // both at once (an AND) would read wrongly there.
  if (categories && s.values.status) return { ok: false, reason: 'complex' };
  s.order = query.order;
  return { ok: true, state: s };
}

/* Sorting ------------------------------------------------------------------- */

/** The sorts offered, each with its natural direction. */
export const SORTS = [
  { field: 'updated', desc: true },
  { field: 'created', desc: true },
  { field: 'priority', desc: true },
  { field: 'key', desc: false },
  { field: 'due', desc: false },
  { field: 'status', desc: false },
  { field: 'resolved', desc: true },
] as const;
export type SortField = (typeof SORTS)[number]['field'];

/** The offered sort a query uses, or null for one written by hand. */
export function currentSort(order: SortTerm[]): { field: SortField; desc: boolean } | null {
  const first = order[0];
  if (!first) return { field: 'updated', desc: true };
  const name = fieldInfo(first.field)?.name ?? first.field;
  const known = SORTS.find((s) => s.field === name);
  return known ? { field: known.field, desc: first.desc } : null;
}

/** A single-field sort; the default (latest updates first) is written as nothing. */
export function sortBy(field: SortField, desc: boolean): SortTerm[] {
  return field === 'updated' && desc ? [] : [{ field, desc }];
}

/** The natural direction of a sort. */
export function naturalDesc(field: SortField): boolean {
  return SORTS.find((s) => s.field === field)?.desc ?? false;
}

/* Dates --------------------------------------------------------------------- */

export interface DatePreset {
  id: string;
  range: DateRange;
}

const r = (from: DateBound | null, to: DateBound | null = null, empty = false): DateRange => ({
  from,
  to,
  empty,
});

/** Shortcuts for past dates (created, updated, resolved). */
const PAST_PRESETS: DatePreset[] = [
  { id: 'today', range: r({ op: '>=', value: 'startOfDay()' }) },
  { id: 'week', range: r({ op: '>=', value: 'startOfWeek()' }) },
  { id: 'last7', range: r({ op: '>=', value: '-7d' }) },
  { id: 'last30', range: r({ op: '>=', value: '-30d' }) },
  { id: 'month', range: r({ op: '>=', value: 'startOfMonth()' }) },
  { id: 'older30', range: r(null, { op: '<', value: '-30d' }) },
];

/** Shortcuts for due dates. */
const DUE_PRESETS: DatePreset[] = [
  { id: 'overdue', range: r(null, { op: '<', value: 'startOfDay()' }) },
  {
    id: 'today',
    range: r({ op: '>=', value: 'startOfDay()' }, { op: '<=', value: 'endOfDay()' }),
  },
  { id: 'next7', range: r({ op: '>=', value: 'startOfDay()' }, { op: '<=', value: '7d' }) },
  {
    id: 'month',
    range: r({ op: '>=', value: 'startOfMonth()' }, { op: '<=', value: 'endOfMonth()' }),
  },
  { id: 'none', range: r(null, null, true) },
];

export function datePresets(field: DateField): DatePreset[] {
  return field === 'due' ? DUE_PRESETS : PAST_PRESETS;
}

const sameBound = (a: DateBound | null, b: DateBound | null) =>
  a === b || (!!a && !!b && a.op === b.op && a.value.toLowerCase() === b.value.toLowerCase());

/** The shortcut a range is, if any. */
export function matchPreset(field: DateField, range: DateRange): string | null {
  const p = datePresets(field).find(
    (p) =>
      p.range.empty === range.empty &&
      sameBound(p.range.from, range.from) &&
      sameBound(p.range.to, range.to),
  );
  return p?.id ?? null;
}

const DAY = /^\d{4}-\d{2}-\d{2}$/;

/** "2026-10-01" plus n days. */
export function addDays(day: string, n: number): string {
  const [y, m, d] = day.split('-').map(Number);
  const t = new Date(Date.UTC(y ?? 1970, (m ?? 1) - 1, (d ?? 1) + n));
  return t.toISOString().slice(0, 10);
}

/** A range between two calendar days, both included (either may be missing). */
export function dayRange(from: string, to: string): DateRange {
  return {
    from: DAY.test(from) ? { op: '>=', value: from } : null,
    to: DAY.test(to) ? { op: '<', value: addDays(to, 1) } : null,
    empty: false,
  };
}

/** The calendar days a range spans, for date inputs; '' where it is open or not a day. */
export function rangeDays(range: DateRange): { from: string; to: string } {
  const from = range.from && DAY.test(range.from.value) ? range.from.value : '';
  let to = '';
  if (range.to && DAY.test(range.to.value)) {
    to = range.to.op === '<' ? addDays(range.to.value, -1) : range.to.value;
  }
  return { from, to };
}

/* Values -------------------------------------------------------------------- */

/** The text of a literal value ("En cours" for "\"En cours\""); null for EMPTY and functions. */
export function literal(value: string): string | null {
  if (value === EMPTY || /^[A-Za-z]+\(.*\)$/.test(value)) return null;
  if (!value.startsWith('"')) return value;
  try {
    const v = parse(`x = ${value}`).where;
    return v?.type === 'clause' && v.values[0]?.kind === 'text' ? v.values[0].text : value;
  } catch {
    return value;
  }
}

/** The JQL value for a name picked in a list. */
export function valueOf(name: string): string {
  return quote(name);
}

/** Whether two JQL values name the same thing (case aside). */
export function sameValue(a: string, b: string): boolean {
  return a.toLowerCase() === b.toLowerCase();
}

/** Toggles a value in a list of values. */
export function toggle(values: string[], value: string): string[] {
  return values.some((v) => sameValue(v, value))
    ? values.filter((v) => !sameValue(v, value))
    : [...values, value];
}
