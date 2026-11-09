import { describe, expect, it } from 'vitest';

import {
  applyMove,
  boardFilterQuery,
  cardFacets,
  cardFilterParams,
  dayAfter,
  daysLeft,
  dropIndex,
  edgeScroll,
  filterCards,
  groupLanes,
  knownCardFilter,
  legacyBoardPath,
  neighbours,
  parseCardFilter,
  parseQuick,
  sprintEnd,
  staysInPlace,
  stepMove,
  toggleIn,
  toggleQuick,
  totalEstimate,
  wipState,
} from '../../src/core/board.js';
import type { BoardCard } from '../../src/core/board-types.js';
import type { IssueBrief, PriorityRef, TypeRef, UserRef } from '../../src/core/types.js';
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
    ...extra,
  };
}

const keys = (cards: BoardCard[]) => cards.map((c) => c.key);

describe('groupLanes', () => {
  it('keeps everything in one band without swimlanes', () => {
    const lanes = groupLanes([card('A-1'), card('A-2')], 'none', []);
    expect(lanes).toHaveLength(1);
    expect(lanes[0]?.kind).toBe('all');
    expect(keys(lanes[0]?.cards ?? [])).toEqual(['A-1', 'A-2']);
  });

  it('follows custom lanes in the board order, the rest last', () => {
    const lanes = groupLanes(
      [
        card('A-1', { lane: 20 }),
        card('A-2', { lane: 10 }),
        card('A-3'),
        card('A-4', { lane: 20 }),
      ],
      'custom',
      [
        { id: 10, name: 'Urgent', query: 'priority = Highest' },
        { id: 20, name: 'Défaut', query: null, default: true },
        { id: 30, name: 'Vide', query: 'labels = x' },
      ],
    );
    expect(lanes.map((l) => l.id)).toEqual(['lane-10', 'lane-20', 'lane-30', 'other']);
    expect(lanes.map((l) => keys(l.cards))).toEqual([['A-2'], ['A-1', 'A-4'], [], ['A-3']]);
  });

  it('sorts people by name and puts the unassigned last', () => {
    const zoe = user(1, 'Zoé Martin');
    const anne = user(2, 'Anne Durand');
    const lanes = groupLanes(
      [
        card('A-1', { assignee: zoe }),
        card('A-2'),
        card('A-3', { assignee: anne }),
        card('A-4', { assignee: zoe }),
      ],
      'assignee',
      [],
    );
    expect(lanes.map((l) => l.id)).toEqual(['user-2', 'user-1', 'user-none']);
    expect(keys(lanes[1]?.cards ?? [])).toEqual(['A-1', 'A-4']);
  });

  it('orders epics by their first card, issues without one last', () => {
    const lanes = groupLanes(
      [
        card('A-1'),
        card('A-2', { epic: brief('A-9') }),
        card('A-3', { epic: brief('A-8') }),
        card('A-4', { epic: brief('A-9') }),
      ],
      'epic',
      [],
    );
    expect(lanes.map((l) => l.id)).toEqual(['epic-A-9', 'epic-A-8', 'epic-none']);
    expect(keys(lanes[0]?.cards ?? [])).toEqual(['A-2', 'A-4']);
    expect(keys(lanes[2]?.cards ?? [])).toEqual(['A-1']);
  });

  it('gives each parent a lane it heads, other issues last', () => {
    const lanes = groupLanes(
      [
        card('A-1'),
        card('A-2', { parent: brief('A-1') }),
        card('A-3'),
        card('A-4', { parent: brief('A-7') }),
      ],
      'parent',
      [],
    );
    expect(lanes.map((l) => l.id)).toEqual(['parent-A-1', 'parent-A-7', 'other']);
    // A-1 heads its lane rather than appearing as a card.
    expect(keys(lanes[2]?.cards ?? [])).toEqual(['A-3']);
  });

  it('omits the catch-all band when nothing falls in it', () => {
    expect(groupLanes([card('A-1', { epic: brief('A-9') })], 'epic', []).map((l) => l.id)).toEqual([
      'epic-A-9',
    ]);
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

describe('limits and sums', () => {
  it('tells a column over or under its limits', () => {
    expect(wipState(3, null, null)).toBe('ok');
    expect(wipState(3, 1, 3)).toBe('ok');
    expect(wipState(4, 1, 3)).toBe('over');
    expect(wipState(0, 1, 3)).toBe('under');
    expect(wipState(0, null, 0)).toBe('ok');
  });

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

describe('card filters', () => {
  const bug: TypeRef = { id: '1', name: 'Bug', subtask: false };
  const task: TypeRef = { id: '3', name: 'Task', subtask: false };
  const high: PriorityRef = { id: '2', name: 'High' };
  const low: PriorityRef = { id: '4', name: 'Low' };
  const ann = user(5, 'Ann');
  const bob = user(6, 'Bob');
  const cards = [
    card('A-1', { type: bug, assignee: ann, priority: high }),
    card('A-2', { type: task, assignee: bob, priority: low }),
    card('A-3', { type: bug, assignee: null, priority: null }),
    card('A-4', { type: task, assignee: ann, priority: null }),
  ];
  const params = (q: string) => {
    const sp = new URLSearchParams(q);
    return parseCardFilter((n) => sp.get(n));
  };
  const known = (q: string, me: number | null = null) => {
    const f = params(q);
    return knownCardFilter(f, cardFacets(cards, f, {}, me));
  };

  it('reads the address and writes it back, dropping unknown ids', () => {
    const f = known('type=1,99,1&assignee=me,none,5,77,x,-2&priority=2,none,zz');
    expect(f).toEqual({ types: ['1'], assignees: ['me', 'none', 5], priorities: ['2', 'none'] });
    expect(cardFilterParams(f)).toEqual({ type: '1', assignee: 'me,none,5', priority: '2,none' });
    expect(cardFilterParams(params(''))).toEqual({ type: null, assignee: null, priority: null });
  });

  it('keeps a chosen type or priority the catalogue knows when no card has it', () => {
    const story: TypeRef = { id: '7', name: 'Story', subtask: false };
    const f = params('type=7&priority=9');
    const facets = cardFacets(cards, f, { types: [story], priorities: [high, low] }, null);
    expect(knownCardFilter(f, facets)).toEqual({ types: ['7'], assignees: [], priorities: [] });
    expect(facets.types.map((x) => x.name)).toEqual(['Bug', 'Story', 'Task']);
  });

  it('offers the values on the cards, the signed-in user as "me"', () => {
    const facets = cardFacets(cards, params(''), { priorities: [high, low] }, 5);
    expect(facets.types.map((x) => x.id)).toEqual(['1', '3']);
    expect(facets.users.map((u) => u.name)).toEqual(['Bob']);
    expect(facets.priorities.map((p) => p.id)).toEqual(['2', '4']);
    expect(facets.noPriority).toBe(true);
    expect(cardFacets(cards.slice(0, 2), params(''), {}, null).noPriority).toBe(false);
  });

  it('shows any of the values of a facet', () => {
    expect(keys(filterCards(cards, known('assignee=5,6'), null))).toEqual(['A-1', 'A-2', 'A-4']);
    expect(keys(filterCards(cards, known(''), null))).toEqual(['A-1', 'A-2', 'A-3', 'A-4']);
  });

  it('shows the cards matching every facet', () => {
    expect(keys(filterCards(cards, known('type=1&assignee=5'), null))).toEqual(['A-1']);
    expect(keys(filterCards(cards, known('type=3&priority=none'), null))).toEqual(['A-4']);
  });

  it('takes "me" as the signed-in user, nobody when signed out', () => {
    expect(keys(filterCards(cards, known('assignee=me', 6), 6))).toEqual(['A-2']);
    expect(keys(filterCards(cards, known('assignee=me'), null))).toEqual([]);
  });

  it('takes "none" as unassigned or without priority', () => {
    expect(keys(filterCards(cards, known('assignee=none'), null))).toEqual(['A-3']);
    expect(keys(filterCards(cards, known('priority=none'), null))).toEqual(['A-3', 'A-4']);
  });

  it('toggles a value', () => {
    expect(toggleIn(['1', '3'], '3')).toEqual(['1']);
    expect(toggleIn<number | 'me'>([5], 'me')).toEqual([5, 'me']);
  });

  it('carries the board filters to another view', () => {
    expect(boardFilterQuery('?quick=1,2&type=3&sprint=4&assignee=me&priority=none')).toBe(
      '?quick=1%2C2&type=3&assignee=me&priority=none',
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
