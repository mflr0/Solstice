/**
 * The project a user scopes Work, Boards and Search to (Me.project): what each
 * page shows with it. A null key covers every project, as without a scope.
 */

import type { BoardSummary } from './board-types.js';
import { onFilter, type GadgetKind } from './dashboard.js';
import type { SearchLocation } from './filters-url.js';
import { paths } from './route.js';

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
 * The query a search that names nothing (no query, no saved filter) starts
 * with for the project. null: the address stays as it is.
 */
export function scopeSearch(loc: SearchLocation, key: string | null): string | null {
  if (!key || loc.jql !== null || loc.filter !== null) return null;
  return `project = ${key}`;
}
