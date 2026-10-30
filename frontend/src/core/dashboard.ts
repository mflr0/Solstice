/**
 * The personal dashboard on the home page (GET and PUT /api/dashboard): an
 * ordered list of gadgets, each on a saved filter (its results, or its
 * issues counted by a field) or built in.
 */

import {
  columnDirection,
  columnSort,
  EMPTY,
  JQL_FIELD,
  listClause,
  naturalDesc,
  type ListField,
} from './filters.js';
import { toJql } from './filters-url.js';
import { format, formatOrder, parse, quote, type SortTerm } from './jql.js';

export type GadgetKind = 'results' | 'count' | 'assigned' | 'reported' | 'recent';

export const GADGET_KINDS: readonly GadgetKind[] = [
  'results',
  'count',
  'assigned',
  'reported',
  'recent',
];

/** The fields a count gadget groups by: those the server counts (GET /api/issues/facets). */
export const COUNT_FIELDS = [
  'status',
  'assignee',
  'priority',
  'type',
  'project',
  'resolution',
  'reporter',
  'labels',
] as const satisfies readonly ListField[];
export type CountField = (typeof COUNT_FIELDS)[number];

export const DEFAULT_SIZE = 10;
export const MAX_SIZE = 50;
export const MAX_GADGETS = 20;

export interface GadgetConfig {
  columns?: string[] | undefined;
  size?: number | undefined;
  /** Replaces the order of the gadget's query; absent, the query keeps its own. */
  sort?: SortTerm | undefined;
  field?: CountField | undefined;
}

export interface Gadget {
  /** 0 for a default gadget the user has not saved yet. */
  id: number;
  kind: GadgetKind;
  filter: { id: number; name: string; query: string } | null;
  /** The filter is deleted or no longer shared with the user. */
  unavailable: boolean;
  config: GadgetConfig;
}

export interface Dashboard {
  customised: boolean;
  gadgets: Gadget[];
}

/** A gadget as PUT /api/dashboard takes it. */
export interface GadgetInput {
  id: number;
  kind: GadgetKind;
  filter: number | null;
  config: GadgetConfig;
}

/** The queries of the built-in gadgets. */
export const BUILTIN_QUERIES: Record<'assigned' | 'reported' | 'recent', string> = {
  assigned: '@moi est:ouvert tri:priorité',
  reported: 'par:@moi est:ouvert',
  recent: 'tri:maj',
};

export function onFilter(kind: GadgetKind): kind is 'results' | 'count' {
  return kind === 'results' || kind === 'count';
}

/** The query a gadget shows, or null when it has none to show. */
export function gadgetQuery(g: Gadget): string | null {
  if (onFilter(g.kind)) return g.unavailable ? null : (g.filter?.query ?? null);
  return BUILTIN_QUERIES[g.kind];
}

/** The query in JQL, ordered by sort instead of its own order; as it is when it does not parse. */
export function sortedQuery(query: string, sort: SortTerm | undefined): string {
  if (!sort) return query;
  try {
    return format({ where: parse(toJql(query)).where, order: [sort] });
  } catch {
    return query;
  }
}

/** The order of a query in either syntax; none when it does not parse. */
export function queryOrder(query: string): SortTerm[] {
  try {
    return parse(toJql(query)).order;
  } catch {
    return [];
  }
}

/**
 * The gadget's sort after a click on a column header, given the order it
 * shows: the column in its natural direction (the other one when the gadget
 * already shows it sorted), then reversed, then none (the query's own order).
 */
export function clickGadgetSort(
  column: string,
  sort: SortTerm | undefined,
  shown: readonly SortTerm[],
): SortTerm | undefined {
  const field = columnSort(column);
  if (field === null) return sort;
  const natural = naturalDesc(field);
  if (sort && columnDirection(column, [sort]) !== null) {
    return sort.desc === natural ? { field, desc: !natural } : undefined;
  }
  const now = columnDirection(column, shown);
  return { field, desc: now === null ? natural : !now };
}

/**
 * The input that saves a gadget as it is. Without a filter, a saved filter
 * gadget keeps the one it has: an unavailable one does not know it.
 */
export function toInput(g: Gadget): GadgetInput {
  return {
    id: g.id,
    kind: g.kind,
    filter: onFilter(g.kind) ? (g.filter?.id ?? null) : null,
    config: g.config,
  };
}

/**
 * A search narrowed to one counted value (a JQL value, or EMPTY for none),
 * as JQL, keeping the query's order. Null when the query does not parse.
 */
export function narrowedJql(query: string, field: CountField, value: string): string | null {
  let q;
  try {
    q = parse(toJql(query));
  } catch {
    return null;
  }
  const clause = listClause(JQL_FIELD[field], [value]);
  const where = q.where ? `(${format({ where: q.where, order: [] })}) AND ${clause}` : clause;
  const order = formatOrder(q.order);
  return order ? `${where} ${order}` : where;
}

/** A counted value as a JQL value: "" (none) becomes EMPTY. */
export function jqlValue(text: string): string {
  return text === '' ? EMPTY : quote(text);
}
