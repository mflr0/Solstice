/**
 * The project a user scopes Work, Boards and Search to (Me.project): what each
 * page shows with it. A null key covers every project, as without a scope.
 */

import type { BoardSummary } from './board-types.js';
import { onFilter, type GadgetKind } from './dashboard.js';
import { emptyFilters, filtersToJql, isBlank, jqlToFilters } from './filters.js';
import type { SearchLocation } from './filters-url.js';
import { quote } from './jql.js';
import { paths, type Route } from './route.js';

/** A simple-syntax query (@moi est:ouvert…) limited to the project. */
export function scopeQuery(q: string, key: string | null): string {
  return key ? `projet:${key} ${q}` : q;
}

/**
 * The query of a dashboard gadget: the built-in lists of one's work are
 * limited to the project, a saved filter keeps the projects it names.
 */
export function scopeGadgetQuery(
  q: string | null,
  kind: GadgetKind,
  key: string | null,
): string | null {
  return q === null || onFilter(kind) ? q : scopeQuery(q, key);
}

/** The boards of the project: those whose projects include it. */
export function scopeBoards(boards: BoardSummary[], key: string | null): BoardSummary[] {
  return key ? boards.filter((b) => b.projects.some((p) => p.key === key)) : boards;
}

/**
 * Where the bare board list goes for the project: its only board, or the
 * list filtered to it. null: it stays.
 */
export function boardsLanding(boards: BoardSummary[], key: string | null): string | null {
  if (!key) return null;
  const mine = scopeBoards(boards, key);
  return mine.length === 1 ? paths.board(mine[0]!.id) : paths.boards(key);
}

/**
 * Where a page goes when the scope changes to the project: the board list,
 * and a board of another project, go to the project's landing. null: it stays.
 */
export function scopeSwitch(
  route: Route,
  boards: BoardSummary[],
  key: string | null,
): string | null {
  if (route.name === 'boards') return boardsLanding(boards, key) ?? paths.boards();
  if (!onBoard(route) || !key) return null;
  const board = boards.find((b) => b.id === route.id);
  if (board?.projects.some((p) => p.key === key)) return null;
  return boardsLanding(boards, key);
}

type BoardRoute = Extract<Route, { name: 'board' | 'backlog' | 'boardReports' | 'boardSettings' }>;

/** Whether the route is one board's page: the board, its backlog, reports or settings. */
export function onBoard(route: Route): route is BoardRoute {
  return ['board', 'backlog', 'boardReports', 'boardSettings'].includes(route.name);
}

/**
 * The query a search that names nothing (no query, no saved filter) starts
 * with for the project. null: the address stays as it is.
 */
export function scopeSearch(loc: SearchLocation, key: string | null): string | null {
  if (!key || loc.jql !== null || loc.filter !== null) return null;
  return `project = ${key}`;
}

/**
 * The query a search follows the scope with when it changes from one project
 * to another: one with no filter but the old project (or none) moves to the
 * new project, its sort kept. null: the address stays as it is.
 */
export function scopeSearchSwitch(
  loc: SearchLocation,
  from: string | null,
  to: string | null,
): string | null {
  if (loc.jql === null || loc.filter !== null) return null;
  const read = jqlToFilters(loc.jql);
  if (!read.ok) return null;
  const { project, ...values } = read.state.values;
  if (project && (from === null || project.length !== 1 || project[0] !== quote(from))) return null;
  if (!isBlank({ ...read.state, values })) return null;
  const next = filtersToJql({
    ...emptyFilters(),
    values: to ? { project: [quote(to)] } : {},
    order: read.state.order,
  });
  return next === loc.jql ? null : next;
}
