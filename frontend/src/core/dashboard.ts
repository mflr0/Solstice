/**
 * The personal dashboard on the home page (GET and PUT /api/dashboard): an
 * ordered list of gadgets, each on a saved filter (its results) or built in.
 */

import { columnDirection, columnSort, naturalDesc } from './filters.js';
import { toJql } from './filters-url.js';
import { format, parse, type SortTerm } from './jql.js';

export type GadgetKind = 'results' | 'assigned' | 'reported' | 'recent' | 'watching' | 'due';

export type BuiltinKind = Exclude<GadgetKind, 'results'>;

/** The ready-made views the New view menu offers, in its order. */
export const BUILTIN_KINDS: readonly BuiltinKind[] = [
  'assigned',
  'reported',
  'recent',
  'watching',
  'due',
];

export const DEFAULT_SIZE = 10;
export const MAX_SIZE = 50;
export const MAX_GADGETS = 20;

/** A gadget's settings; the columns are its filter's. */
export interface GadgetConfig {
  size?: number | undefined;
  /** Replaces the order of the gadget's query; absent, the query keeps its own. */
  sort?: SortTerm | undefined;
  /** Folded in the user's space. */
  collapsed?: boolean | undefined;
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

/** The queries of the built-in gadgets; backend/tests/dashboard_test.go runs the JQL ones. */
export const BUILTIN_QUERIES: Record<BuiltinKind, string> = {
  assigned: '@moi est:ouvert tri:priorité',
  reported: 'par:@moi est:ouvert',
  recent: 'tri:maj',
  watching: 'watcher = currentUser() ORDER BY updated DESC',
  due: 'statusCategory != Done AND due >= startOfDay() AND due <= 7d ORDER BY due ASC',
};

export function onFilter(kind: GadgetKind): kind is 'results' {
  return kind === 'results';
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

/** A gadget's query as a view: in JQL, in the order saved on the gadget. */
export function viewJql(query: string, sort: SortTerm | undefined): string {
  return toJql(sortedQuery(query, sort));
}

/** The new view that copies a built-in gadget; saving it puts the view in the gadget's place. */
export function copyOnEditPath(jql: string, gadget: number): string {
  return `/views/new?${new URLSearchParams({ jql, gadget: String(gadget) }).toString()}`;
}

/**
 * The gadgets with the one of that id turned into a results gadget on the
 * filter, folded and sized as it was; appended when it is gone. The order is
 * now the filter's own.
 */
export function swapToFilter(gadgets: Gadget[], id: number, filter: number): GadgetInput[] {
  const next = gadgets.map(toInput);
  const i = next.findIndex((g) => g.id === id);
  const old = next[i];
  const config: GadgetConfig = {};
  if (old?.config.size) config.size = old.config.size;
  if (old?.config.collapsed) config.collapsed = true;
  const swapped: GadgetInput = { id: old?.id ?? 0, kind: 'results', filter, config };
  if (i < 0) next.push(swapped);
  else next[i] = swapped;
  return next;
}

/** Whether the space already shows the built-in view, or a view on the filter. */
export function inSpace(gadgets: Gadget[], what: BuiltinKind | { filter: number }): boolean {
  return gadgets.some((g) =>
    typeof what === 'string' ? g.kind === what : onFilter(g.kind) && g.filter?.id === what.filter,
  );
}

/** The gadgets with these ones folded or unfolded. */
export function setCollapsed(
  gadgets: Gadget[],
  which: (g: Gadget) => boolean,
  on: boolean,
): Gadget[] {
  return gadgets.map((g) => (which(g) ? { ...g, config: { ...g.config, collapsed: on } } : g));
}
