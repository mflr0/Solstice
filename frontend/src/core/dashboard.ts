/**
 * The personal dashboard on the home page (GET and PUT /api/dashboard): an
 * ordered list of gadgets, each on a saved filter (its results, or its
 * issues counted by a field) or built in.
 */

import { EMPTY, JQL_FIELD, listClause, type ListField } from './filters.js';
import { toJql } from './filters-url.js';
import { format, formatOrder, parse, quote } from './jql.js';

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
