/**
 * The project a user scopes Work, Boards and Search to (Me.project): what each
 * page shows with it. A null key covers every project, as without a scope.
 */

import type { BoardSummary } from './board-types.js';
import { onFilter, type GadgetKind } from './dashboard.js';
import { emptyFilters, filtersToJql, isBlank, jqlToFilters } from './filters.js';
import type { SearchLocation } from './filters-url.js';
import { format, formatOrder, looksLikeJql, parse, quote } from './jql.js';
import { projectOfIssueKey } from './marks.js';
import { paths, type Route } from './route.js';

/** A query (simple syntax or JQL) limited to the project; as it is when it does not parse. */
export function scopeQuery(q: string, key: string | null): string {
  if (!key) return q;
  if (!looksLikeJql(q)) return `projet:${key} ${q}`;
  try {
    const { where, order } = parse(q);
    const project = `project = ${quote(key)}`;
    const scoped = where ? `${project} AND (${format({ where, order: [] })})` : project;
    return [scoped, formatOrder(order)].filter((s) => s !== '').join(' ');
  } catch {
    return q;
  }
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
 * list filtered to it. null: it stays, as it does when one came to create a board.
 */
export function boardsLanding(
  boards: BoardSummary[],
  key: string | null,
  creating = false,
): string | null {
  if (!key || creating) return null;
  const mine = scopeBoards(boards, key);
  return mine.length === 1 ? paths.board(mine[0]!.id) : paths.boards(key);
}

/**
 * Where a page goes when the scope changes to the project: the project pages
 * go to the new project's page (the list for every project); the board list,
 * and a board of another project, go to the project's landing. null: it stays.
 */
export function scopeSwitch(
  route: Route,
  boards: BoardSummary[],
  key: string | null,
): string | null {
  if (onProjects(route)) return key ? paths.project(key) : paths.projects();
  if (route.name === 'boards') return boardsLanding(boards, key) ?? paths.boards();
  if (!onBoard(route) || !key) return null;
  const board = boards.find((b) => b.id === route.id);
  if (board?.projects.some((p) => p.key === key)) return null;
  return boardsLanding(boards, key);
}

/** Whether the route is the project list, a project's page or its settings. */
export function onProjects(route: Route): boolean {
  return ['projects', 'project', 'projectSettings'].includes(route.name);
}

/**
 * The project a board page stands for: the scoped one when the board covers
 * it, else the board's first. null: the board has no project.
 */
export function boardProject(projects: { key: string }[], key: string | null): string | null {
  if (key && projects.some((p) => p.key === key)) return key;
  return projects[0]?.key ?? null;
}

/** A project and its boards, in a board page's rail. */
export interface RailProject {
  key: string;
  name: string;
  boards: BoardSummary[];
}

/**
 * What a board page's rail lists: the boards of the project, then every other
 * project by key, with its boards. A board in several projects is under each.
 */
export function boardRail(
  boards: BoardSummary[],
  key: string | null,
): { current: BoardSummary[]; others: RailProject[] } {
  const others = new Map<string, RailProject>();
  for (const b of boards) {
    for (const p of b.projects) {
      if (p.key === key) continue;
      let group = others.get(p.key);
      if (!group) others.set(p.key, (group = { key: p.key, name: p.name, boards: [] }));
      group.boards.push(b);
    }
  }
  return {
    current: key ? scopeBoards(boards, key) : [],
    others: [...others.values()].sort((a, b) => (a.key < b.key ? -1 : a.key > b.key ? 1 : 0)),
  };
}

/** Where a board's way back to the list goes: the list of the project it stands for. */
export function boardListPath(projects: { key: string }[], key: string | null): string {
  return paths.boards(boardProject(projects, key) ?? undefined);
}

/**
 * The project New issue offers first: the page's (a project, an issue's, the
 * board shown), else the scoped one. board: the projects of the board the
 * route shows, null while it loads.
 */
export function createProject(
  route: Route,
  key: string | null,
  board: { key: string }[] | null,
): string | undefined {
  switch (route.name) {
    case 'project':
    case 'projectSettings':
      return route.key;
    case 'issue':
      return projectOfIssueKey(route.key) ?? key ?? undefined;
  }
  if (onBoard(route) && board) return boardProject(board, key) ?? key ?? undefined;
  return key ?? undefined;
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
