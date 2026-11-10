import { describe, expect, it } from 'vitest';

import {
  applyMove,
  boardFilterQuery,
  boardStep,
  dayAfter,
  daysLeft,
  dropIndex,
  edgeScroll,
  groupSections,
  legacyBoardPath,
  neighbours,
  parseQuick,
  readBoardSearch,
  sprintEnd,
  staysInPlace,
  stepMove,
  toggleQuick,
  totalEstimate,
} from '../../src/core/board.js';
import type { BoardCard } from '../../src/core/board-types.js';
import type { IssueBrief, UserRef } from '../../src/core/types.js';
import { matchRoute, paths } from '../../src/core/route.js';

const user = (id: number, name: string): UserRef => ({
  id,
  username: `u${id}`,
  name,
  active: true,
});

const brief = (key: string): IssueBrief => ({
  key,
  summary: `Résumé ${key}`,
  type: { id: '1', name: 'Story', subtask: false },
  status: { id: '1', name: 'À faire', category: 'todo' },
  assignee: null,
});

function card(key: string, extra: Partial<BoardCard> = {}): BoardCard {
  return {
    key,
    summary: key,
    type: { id: '3', name: 'Task', subtask: false },
    status: { id: '1', name: 'À faire', category: 'todo' },
    priority: null,
    resolution: null,
    assignee: null,
    reporter: null,
    created: '2026-10-01T10:00:00Z',
    updated: '2026-10-01T10:00:00Z',
    resolved: null,
    due: null,
    confidential: false,
    epic: null,
    parent: null,
    estimate: null,
    column: 1,
    can: { create: true, edit: true },
    watching: false,
    ...extra,
  };
}

const keys = (cards: BoardCard[]) => cards.map((c) => c.key);

describe('groupSections', () => {
  it('keeps everything in one band without sections', () => {
    const sections = groupSections([card('A-1'), card('A-2')], 'none', []);
    expect(sections).toHaveLength(1);
    expect(sections[0]?.kind).toBe('all');
    expect(keys(sections[0]?.cards ?? [])).toEqual(['A-1', 'A-2']);
  });

  it('follows custom sections in the board order, the rest last', () => {
    const sections = groupSections(
      [
        card('A-1', { section: 20 }),
        card('A-2', { section: 10 }),
        card('A-3'),
        card('A-4', { section: 20 }),
      ],
      'custom',
      [
        { id: 10, name: 'Urgent', query: 'priority = Highest' },
        { id: 20, name: 'Défaut', query: null, default: true },
        { id: 30, name: 'Vide', query: 'labels = x' },
      ],
    );
    expect(sections.map((l) => l.id)).toEqual(['section-10', 'section-20', 'section-30', 'other']);
    expect(sections.map((l) => keys(l.cards))).toEqual([['A-2'], ['A-1', 'A-4'], [], ['A-3']]);
  });

  it('sorts people by name and puts the unassigned last', () => {
    const zoe = user(1, 'Zoé Martin');
    const anne = user(2, 'Anne Durand');
    const sections = groupSections(
      [
        card('A-1', { assignee: zoe }),
        card('A-2'),
        card('A-3', { assignee: anne }),
        card('A-4', { assignee: zoe }),
      ],
      'assignee',
      [],
    );
    expect(sections.map((l) => l.id)).toEqual(['user-2', 'user-1', 'user-none']);
    expect(keys(sections[1]?.cards ?? [])).toEqual(['A-1', 'A-4']);
  });

  it('orders epics by their first card, issues without one last', () => {
    const sections = groupSections(
      [
        card('A-1'),
        card('A-2', { epic: brief('A-9') }),
        card('A-3', { epic: brief('A-8') }),
        card('A-4', { epic: brief('A-9') }),
      ],
      'epic',
      [],
    );
    expect(sections.map((l) => l.id)).toEqual(['epic-A-9', 'epic-A-8', 'epic-none']);
    expect(keys(sections[0]?.cards ?? [])).toEqual(['A-2', 'A-4']);
    expect(keys(sections[2]?.cards ?? [])).toEqual(['A-1']);
  });

  it('gives each parent a section it heads, other issues last', () => {
    const sections = groupSections(
      [
        card('A-1'),
        card('A-2', { parent: brief('A-1') }),
        card('A-3'),
        card('A-4', { parent: brief('A-7') }),
      ],
      'parent',
      [],
    );
    expect(sections.map((l) => l.id)).toEqual(['parent-A-1', 'parent-A-7', 'other']);
    // A-1 heads its section rather than appearing as a card.
    expect(keys(sections[2]?.cards ?? [])).toEqual(['A-3']);
  });

  it('omits the catch-all band when nothing falls in it', () => {
    expect(
      groupSections([card('A-1', { epic: brief('A-9') })], 'epic', []).map((l) => l.id),
    ).toEqual(['epic-A-9']);
  });
});

describe('dropping', () => {
  it('counts the cards above the pointer', () => {
    expect(dropIndex([10, 30, 50], 5)).toBe(0);
    expect(dropIndex([10, 30, 50], 31)).toBe(2);
    expect(dropIndex([10, 30, 50], 90)).toBe(3);
    expect(dropIndex([], 90)).toBe(0);
  });

  it('scrolls faster the closer a held card is to an edge', () => {
    expect(edgeScroll(500, 0, 1000)).toBe(0);
    expect(edgeScroll(48, 0, 1000)).toBe(0);
    expect(edgeScroll(24, 0, 1000)).toBe(-7);
    expect(edgeScroll(0, 0, 1000)).toBe(-14);
    expect(edgeScroll(-30, 0, 1000)).toBe(-14);
    expect(edgeScroll(976, 0, 1000)).toBe(7);
    expect(edgeScroll(1200, 0, 1000)).toBe(14);
    // Narrower than both zones: the nearer edge wins.
    expect(edgeScroll(10, 0, 60)).toBeLessThan(0);
    expect(edgeScroll(50, 0, 60)).toBeGreaterThan(0);
  });

  it('names the new neighbours', () => {
    const list = ['A', 'B', 'C'];
    expect(neighbours(list, 'X', 0)).toEqual({ before: 'A' });
    expect(neighbours(list, 'X', 1)).toEqual({ after: 'A', before: 'B' });
    expect(neighbours(list, 'X', 3)).toEqual({ after: 'C' });
    expect(neighbours(list, 'X', 9)).toEqual({ after: 'C' });
    expect(neighbours([], 'X', 0)).toEqual({});
    // The card itself does not count, wherever it was.
    expect(neighbours(list, 'A', 2)).toEqual({ after: 'C' });
    expect(neighbours(list, 'C', 0)).toEqual({ before: 'A' });
  });

  it('notices a drop back where the card was', () => {
    expect(staysInPlace(['A', 'B', 'C'], 'B', 1)).toBe(true);
    expect(staysInPlace(['A', 'B', 'C'], 'B', 0)).toBe(false);
    expect(staysInPlace(['A', 'B', 'C'], 'C', 5)).toBe(true);
    expect(staysInPlace(['A', 'B'], 'X', 0)).toBe(false);
  });

  it('steps a card up or down', () => {
    expect(stepMove(['A', 'B', 'C'], 'C', -1)).toEqual({ issue: 'C', after: 'A', before: 'B' });
    expect(stepMove(['A', 'B', 'C'], 'A', 1)).toEqual({ issue: 'A', after: 'B', before: 'C' });
    expect(stepMove(['A', 'B', 'C'], 'B', -1)).toEqual({ issue: 'B', before: 'A' });
    expect(stepMove(['A', 'B', 'C'], 'A', -1)).toBeNull();
    expect(stepMove(['A', 'B', 'C'], 'C', 1)).toBeNull();
  });
});

describe('applyMove', () => {
  const cards = [card('A-1'), card('A-2'), card('A-3', { column: 2 }), card('A-4', { column: 2 })];

  it('moves a card after its new upper neighbour, into its new column', () => {
    const next = applyMove(cards, { issue: 'A-1', column: 2, after: 'A-3' });
    expect(keys(next)).toEqual(['A-2', 'A-3', 'A-1', 'A-4']);
    expect(next.find((c) => c.key === 'A-1')?.column).toBe(2);
    // The original list is untouched (it is what a failed move goes back to).
    expect(cards[0]?.column).toBe(1);
  });

  it('moves a card before its new lower neighbour', () => {
    expect(keys(applyMove(cards, { issue: 'A-4', before: 'A-2' }))).toEqual([
      'A-1',
      'A-4',
      'A-2',
      'A-3',
    ]);
  });

  it('keeps the order when only the column changes', () => {
    const next = applyMove(cards, { issue: 'A-2', column: 3 });
    expect(keys(next)).toEqual(keys(cards));
    expect(next[1]?.column).toBe(3);
  });

  it('changes sprints, 0 being the backlog', () => {
    const planned = applyMove(cards, { issue: 'A-1', sprint: 7 });
    expect(planned[0]?.sprint).toBe(7);
    expect(applyMove(planned, { issue: 'A-1', sprint: 0 })[0]).not.toHaveProperty('sprint');
  });

  it('ignores unknown cards', () => {
    expect(applyMove(cards, { issue: 'Z-1', column: 2 })).toBe(cards);
  });
});

describe('sums', () => {
  it('adds up estimates, null when there is none', () => {
    expect(totalEstimate([card('A-1'), card('A-2')])).toBeNull();
    expect(
      totalEstimate([card('A-1', { estimate: 3 }), card('A-2'), card('A-3', { estimate: 0.5 })]),
    ).toBe(3.5);
    expect(totalEstimate([card('A-1', { estimate: 0.1 }), card('A-2', { estimate: 0.2 })])).toBe(
      0.3,
    );
  });
});

describe('sprint dates', () => {
  const now = new Date(2026, 9, 13, 14, 30);

  it('counts the days left on the calendar', () => {
    expect(daysLeft(new Date(2026, 9, 20, 14, 30).toISOString(), now)).toBe(7);
    expect(daysLeft(new Date(2026, 9, 13, 18, 0).toISOString(), now)).toBe(0);
    expect(daysLeft(new Date(2026, 9, 14, 0, 5).toISOString(), now)).toBe(1);
    expect(daysLeft(new Date(2026, 9, 11, 14, 30).toISOString(), now)).toBe(-2);
    // Across the change to winter time (25 October): still two weeks.
    expect(daysLeft(new Date(2026, 9, 27, 14, 30).toISOString(), now)).toBe(14);
  });

  it('ends a sprint on the chosen day, at the current time', () => {
    expect(sprintEnd('2026-10-27', now)).toBe(new Date(2026, 9, 27, 14, 30).toISOString());
    expect(sprintEnd('27/10/2026', now)).toBeNull();
  });

  it('counts days forward across months', () => {
    expect(dayAfter(14, now)).toBe('2026-10-27');
    expect(dayAfter(20, now)).toBe('2026-11-02');
  });
});

describe('the filter row in the address', () => {
  it('sends what the row can show', () => {
    const r = readBoardSearch(' assignee = currentUser() AND text ~ "panier" ');
    expect(r.jql).toBe('assignee = currentUser() AND text ~ "panier"');
    expect(r.filters.text).toBe('panier');
    expect(r.refused).toBe(false);
  });

  it('sends nothing for no query', () => {
    for (const p of [null, '', '  '])
      expect(readBoardSearch(p)).toMatchObject({ jql: undefined, refused: false });
  });

  it('refuses, and sends nothing for, a query the row cannot show', () => {
    for (const p of ['status = Done OR assignee is EMPTY', 'summary ~'])
      expect(readBoardSearch(p)).toMatchObject({ jql: undefined, refused: true });
  });
});

describe('quick filters in the address', () => {
  it('reads ids, ignoring junk and repeats', () => {
    expect(parseQuick(null)).toEqual([]);
    expect(parseQuick('3,1,x,3,-2, 4')).toEqual([3, 1, 4]);
  });

  it('toggles one', () => {
    expect(toggleQuick([1, 3], 3)).toBe('1');
    expect(toggleQuick([1], 5)).toBe('1,5');
    expect(toggleQuick([1], 1)).toBe('');
  });
});

describe('the board filters between views', () => {
  it('carries the quick filters and the row', () => {
    expect(boardFilterQuery('?quick=1,2&jql=type+%3D+Bug&sprint=4&type=3')).toBe(
      '?quick=1%2C2&jql=type+%3D+Bug',
    );
    expect(boardFilterQuery('?sprint=4')).toBe('');
  });
});

describe('board addresses', () => {
  it.each([
    ['/boards', { name: 'boards' }],
    ['/boards/12', { name: 'board', id: 12 }],
    ['/boards/12/backlog', { name: 'backlog', id: 12 }],
    ['/boards/12/reports', { name: 'boardReports', id: 12 }],
    ['/boards/12/settings', { name: 'boardSettings', id: 12 }],
    ['/boards/0', { name: 'notFound' }],
    ['/boards/abc', { name: 'notFound' }],
    ['/boards/12/nope', { name: 'notFound' }],
    ['/secure/RapidBoard.jspa', { name: 'legacyBoard' }],
    ['/secure/RapidView.jspa', { name: 'legacyBoard' }],
  ])('%s', (path, route) => {
    expect(matchRoute(path)).toEqual(route);
  });

  it('round-trips the addresses it builds', () => {
    expect(matchRoute(paths.board(4))).toEqual({ name: 'board', id: 4 });
    expect(matchRoute(paths.backlog(4))).toEqual({ name: 'backlog', id: 4 });
    expect(matchRoute(paths.boardReports(4))).toEqual({ name: 'boardReports', id: 4 });
    expect(paths.boardReports(4, 9)).toBe('/boards/4/reports?sprint=9');
    expect(paths.boardVelocity(4)).toBe('/boards/4/reports?report=velocity');
    expect(paths.boardFlow(4)).toBe('/boards/4/reports?report=flow');
    expect(matchRoute(paths.boardSettings(4))).toEqual({ name: 'boardSettings', id: 4 });
    expect(paths.boards('WEB')).toBe('/boards?project=WEB');
  });

  it.each([
    ['/secure/RapidBoard.jspa', '?rapidView=7', '/boards/7'],
    ['/secure/RapidBoard.jspa', '?rapidView=7&projectKey=WEB&view=detail', '/boards/7'],
    ['/secure/RapidBoard.jspa', '?rapidView=7&view=planning', '/boards/7/backlog'],
    [
      '/secure/RapidBoard.jspa',
      '?rapidView=7&view=planning.nodetail&selectedIssue=WEB-3',
      '/boards/7/backlog',
    ],
    ['/secure/RapidBoard.jspa', '', '/boards'],
    ['/secure/RapidBoard.jspa', '?rapidView=x', '/boards'],
    ['/secure/RapidView.jspa', '?rapidView=7&tab=filter', '/boards/7/settings'],
    ['/browse/WEB-3', '', null],
  ])('%s%s → %s', (path, search, to) => {
    expect(legacyBoardPath(path, search)).toBe(to);
  });
});

describe('board keys', () => {
  // Two sections of three columns; the first section's middle column is empty.
  const counts = [
    [2, 0, 1],
    [1, 3, 0],
  ];

  it('goes from nowhere to the first card', () => {
    expect(boardStep(counts, null, 'next')).toEqual({ section: 0, column: 0, index: 0 });
    expect(boardStep([[0, 2]], null, 'left')).toEqual({ section: 0, column: 1, index: 0 });
    expect(boardStep([[0, 0]], null, 'next')).toBeNull();
  });

  it('goes down and up the column, across sections', () => {
    expect(boardStep(counts, { section: 0, column: 0, index: 0 }, 'next')).toEqual({
      section: 0,
      column: 0,
      index: 1,
    });
    expect(boardStep(counts, { section: 0, column: 0, index: 1 }, 'next')).toEqual({
      section: 1,
      column: 0,
      index: 0,
    });
    expect(boardStep(counts, { section: 1, column: 0, index: 0 }, 'previous')).toEqual({
      section: 0,
      column: 0,
      index: 1,
    });
  });

  it('skips sections with no card in the column', () => {
    expect(boardStep([[1], [0], [2]], { section: 0, column: 0, index: 0 }, 'next')).toEqual({
      section: 2,
      column: 0,
      index: 0,
    });
    expect(boardStep([[1], [0], [2]], { section: 2, column: 0, index: 0 }, 'previous')).toEqual({
      section: 0,
      column: 0,
      index: 0,
    });
  });

  it('skips an empty column', () => {
    expect(boardStep(counts, { section: 0, column: 0, index: 0 }, 'right')).toEqual({
      section: 0,
      column: 2,
      index: 0,
    });
    expect(boardStep(counts, { section: 0, column: 2, index: 0 }, 'left')).toEqual({
      section: 0,
      column: 0,
      index: 0,
    });
  });

  it('lands on the last card of a shorter column', () => {
    expect(boardStep(counts, { section: 1, column: 1, index: 2 }, 'left')).toEqual({
      section: 1,
      column: 0,
      index: 0,
    });
    expect(boardStep(counts, { section: 0, column: 0, index: 1 }, 'right')).toEqual({
      section: 0,
      column: 2,
      index: 0,
    });
  });

  it('stays put at the edges', () => {
    for (const [at, dir] of [
      [{ section: 0, column: 0, index: 0 }, 'previous'],
      [{ section: 0, column: 0, index: 0 }, 'left'],
      [{ section: 1, column: 1, index: 2 }, 'next'],
      [{ section: 1, column: 1, index: 0 }, 'right'],
      [{ section: 0, column: 2, index: 0 }, 'right'],
    ] as const)
      expect(boardStep(counts, at, dir)).toEqual(at);
  });
});
