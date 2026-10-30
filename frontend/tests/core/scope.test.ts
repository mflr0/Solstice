import { describe, expect, it } from 'vitest';

import type { BoardSummary } from '../../src/core/board-types.js';
import type { SearchLocation } from '../../src/core/filters-url.js';
import {
  boardsLanding,
  scopeBoards,
  scopeGadgetQuery,
  scopeQuery,
  scopeSearch,
  scopeSearchSwitch,
  scopeSwitch,
} from '../../src/core/scope.js';

const board = (id: number, ...keys: string[]): BoardSummary => ({
  id,
  name: `Board ${id}`,
  kind: 'kanban',
  projects: keys.map((key, i) => ({ id: i + 1, key, name: key })),
  owner: null,
  activeSprint: null,
});
const boards = [board(1, 'DEMO'), board(2, 'WEB'), board(3, 'DEMO', 'WEB'), board(4, 'OPS')];
const bare: SearchLocation = { jql: null, mode: null, filter: null, page: 1 };

describe('every project', () => {
  it('leaves the pages as they are', () => {
    expect(scopeQuery('@moi est:ouvert', null)).toBe('@moi est:ouvert');
    expect(scopeGadgetQuery('tri:maj', 'recent', null)).toBe('tri:maj');
    expect(scopeBoards(boards, null)).toEqual(boards);
    expect(boardsLanding(boards, null)).toBeNull();
    expect(scopeSearch(bare, null)).toBeNull();
  });
});

describe('one project', () => {
  it('limits the work queries to it', () => {
    expect(scopeQuery('par:@moi est:ouvert', 'DEMO')).toBe('projet:DEMO par:@moi est:ouvert');
    expect(scopeGadgetQuery('tri:maj', 'recent', 'DEMO')).toBe('projet:DEMO tri:maj');
  });

  it('leaves saved filter gadgets to their own projects', () => {
    expect(scopeGadgetQuery('project = WEB', 'results', 'DEMO')).toBe('project = WEB');
    expect(scopeGadgetQuery('project = WEB', 'count', 'DEMO')).toBe('project = WEB');
    expect(scopeGadgetQuery(null, 'assigned', 'DEMO')).toBeNull();
  });

  it('keeps the boards it is one of the projects of', () => {
    expect(scopeBoards(boards, 'DEMO').map((b) => b.id)).toEqual([1, 3]);
    expect(scopeBoards(boards, 'NONE')).toEqual([]);
  });

  it('opens its only board, else lists its boards', () => {
    expect(boardsLanding(boards, 'OPS')).toBe('/boards/4');
    expect(boardsLanding(boards, 'DEMO')).toBe('/boards?project=DEMO');
    expect(boardsLanding(boards, 'NONE')).toBe('/boards?project=NONE');
  });

  it('starts a new search in it, never rewrites one', () => {
    expect(scopeSearch(bare, 'DEMO')).toBe('project = DEMO');
    expect(scopeSearch({ ...bare, mode: 'jql' }, 'DEMO')).toBe('project = DEMO');
    expect(scopeSearch({ ...bare, jql: 'type = Bug' }, 'DEMO')).toBeNull();
    expect(scopeSearch({ ...bare, jql: '' }, 'DEMO')).toBeNull();
    expect(scopeSearch({ ...bare, filter: 12 }, 'DEMO')).toBeNull();
  });
});

describe('switching the scope', () => {
  const on = (name: 'board' | 'backlog' | 'boardReports' | 'boardSettings', id: number) =>
    ({ name, id }) as const;

  it('sends the board list to the new project landing', () => {
    expect(scopeSwitch({ name: 'boards' }, boards, 'DEMO')).toBe('/boards?project=DEMO');
    expect(scopeSwitch({ name: 'boards' }, boards, 'OPS')).toBe('/boards/4');
    expect(scopeSwitch({ name: 'boards' }, boards, null)).toBe('/boards');
  });

  it('stays on a board of the new project', () => {
    expect(scopeSwitch(on('board', 3), boards, 'WEB')).toBeNull();
    expect(scopeSwitch(on('backlog', 1), boards, 'DEMO')).toBeNull();
  });

  it('leaves a board of another project for the new project landing', () => {
    expect(scopeSwitch(on('board', 1), boards, 'OPS')).toBe('/boards/4');
    expect(scopeSwitch(on('backlog', 4), boards, 'WEB')).toBe('/boards?project=WEB');
    expect(scopeSwitch(on('boardReports', 2), boards, 'DEMO')).toBe('/boards?project=DEMO');
    expect(scopeSwitch(on('boardSettings', 99), boards, 'OPS')).toBe('/boards/4');
  });

  it('stays on any board for every project, and off the boards', () => {
    expect(scopeSwitch(on('board', 4), boards, null)).toBeNull();
    expect(scopeSwitch({ name: 'search' }, boards, 'OPS')).toBeNull();
  });

  it('moves a search with no filter of its own to the new project', () => {
    const at = (jql: string) => ({ ...bare, jql });
    expect(scopeSearchSwitch(at('project = A'), 'A', 'B')).toBe('project = B');
    expect(scopeSearchSwitch(at('project = A'), 'A', null)).toBe('');
    expect(scopeSearchSwitch(at(''), null, 'A')).toBe('project = A');
    expect(scopeSearchSwitch(at(''), 'A', 'B')).toBe('project = B');
    expect(scopeSearchSwitch(at('project = A ORDER BY created DESC'), 'A', 'B')).toBe(
      'project = B ORDER BY created DESC',
    );
    expect(scopeSearchSwitch({ ...at('project = A'), mode: 'jql' }, 'A', 'B')).toBe('project = B');
  });

  it('leaves a search with a filter of its own, or a saved filter', () => {
    const at = (jql: string) => ({ ...bare, jql });
    expect(scopeSearchSwitch(at('project = A AND type = Bug'), 'A', 'B')).toBeNull();
    expect(scopeSearchSwitch(at('project = C'), 'A', 'B')).toBeNull();
    expect(scopeSearchSwitch(at('project = A'), null, 'B')).toBeNull();
    expect(scopeSearchSwitch(at('project in (A, B)'), 'A', 'C')).toBeNull();
    expect(scopeSearchSwitch(at('text ~ "x"'), 'A', 'B')).toBeNull();
    expect(scopeSearchSwitch(at('project = A OR type = Bug'), 'A', 'B')).toBeNull();
    expect(scopeSearchSwitch({ ...at('project = A'), filter: 12 }, 'A', 'B')).toBeNull();
    expect(scopeSearchSwitch({ ...bare, filter: 12 }, 'A', 'B')).toBeNull();
    expect(scopeSearchSwitch(bare, 'A', 'B')).toBeNull();
  });
});
