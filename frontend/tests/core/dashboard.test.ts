import { describe, expect, it } from 'vitest';

import {
  BUILTIN_QUERIES,
  clickGadgetSort,
  copyOnEditPath,
  gadgetQuery,
  inSpace,
  jqlValue,
  narrowedJql,
  queryOrder,
  setCollapsed,
  sortedQuery,
  swapToFilter,
  toInput,
  viewJql,
  type Gadget,
} from '../../src/core/dashboard.js';
import { filtersToJql, jqlToFilters } from '../../src/core/filters.js';
import { toJql } from '../../src/core/filters-url.js';

const gadget = (over: Partial<Gadget>): Gadget => ({
  id: 7,
  kind: 'results',
  filter: { id: 3, name: 'Bugs', query: 'project = DEMO' },
  unavailable: false,
  config: {},
  ...over,
});

describe('narrowedJql', () => {
  it('adds the value to the query, keeping its order', () => {
    expect(
      narrowedJql('project = DEMO OR type = Bug ORDER BY created DESC', 'status', '"Done"'),
    ).toBe('(project = DEMO OR type = Bug) AND status = "Done" ORDER BY created DESC');
  });

  it('narrows a query in the simple syntax', () => {
    expect(narrowedJql('', 'assignee', 'EMPTY')).toBe('assignee is EMPTY');
    expect(narrowedJql('#ui', 'priority', '"High"')).toMatch(
      /^\(.*labels.*\) AND priority = "High"/,
    );
  });

  it('gives up on a query that does not parse', () => {
    expect(narrowedJql('project = (', 'status', '"Done"')).toBeNull();
  });
});

describe('jqlValue', () => {
  it('quotes a value, and reads none as EMPTY', () => {
    expect(jqlValue('In progress')).toBe('"In progress"');
    expect(jqlValue('')).toBe('EMPTY');
  });
});

describe('gadgetQuery', () => {
  it('reads the filter, the built-in query, or nothing when unavailable', () => {
    expect(gadgetQuery(gadget({}))).toBe('project = DEMO');
    expect(gadgetQuery(gadget({ kind: 'assigned', filter: null }))).toContain('@moi');
    expect(gadgetQuery(gadget({ filter: null, unavailable: true }))).toBeNull();
  });
});

describe('toInput', () => {
  it('names the filter only when it knows it', () => {
    expect(toInput(gadget({})).filter).toBe(3);
    expect(toInput(gadget({ filter: null, unavailable: true })).filter).toBeNull();
    expect(toInput(gadget({ kind: 'recent' })).filter).toBeNull();
  });
});

describe('sortedQuery', () => {
  it("replaces a filter's order and keeps its conditions", () => {
    expect(
      sortedQuery('project = DEMO OR type = Bug ORDER BY created DESC', {
        field: 'updated',
        desc: false,
      }),
    ).toBe('project = DEMO OR type = Bug ORDER BY updated ASC');
  });

  it("replaces a built-in query's order", () => {
    const q = sortedQuery(BUILTIN_QUERIES.assigned, { field: 'updated', desc: true });
    const base = toJql(BUILTIN_QUERIES.assigned);
    expect(q).toBe(`${base.slice(0, base.indexOf(' ORDER BY'))} ORDER BY updated DESC`);
    expect(queryOrder(q)).toEqual([{ field: 'updated', desc: true }]);
    expect(sortedQuery('', { field: 'key', desc: false })).toBe('ORDER BY key ASC');
  });

  it('keeps the query without a sort, or when it does not parse', () => {
    expect(sortedQuery(BUILTIN_QUERIES.recent, undefined)).toBe(BUILTIN_QUERIES.recent);
    expect(sortedQuery('project = (', { field: 'key', desc: false })).toBe('project = (');
  });
});

describe('clickGadgetSort', () => {
  const byPriority = queryOrder(BUILTIN_QUERIES.assigned);

  it('sorts by the column, then reverses, then clears', () => {
    const first = clickGadgetSort('updated', undefined, byPriority);
    expect(first).toEqual({ field: 'updated', desc: true });
    const second = clickGadgetSort('updated', first, [first!]);
    expect(second).toEqual({ field: 'updated', desc: false });
    expect(clickGadgetSort('updated', second, [second!])).toBeUndefined();
  });

  it('reverses the order the query already has on the column', () => {
    expect(clickGadgetSort('updated', undefined, [])).toEqual({ field: 'updated', desc: false });
  });

  it('keeps the sort for a column ORDER BY cannot take', () => {
    const sort = { field: 'key', desc: false };
    expect(clickGadgetSort('description', sort, [sort])).toBe(sort);
  });
});

describe('swapToFilter', () => {
  const builtins = [
    gadget({
      id: 1,
      kind: 'assigned',
      filter: null,
      config: { size: 5, sort: { field: 'key', desc: false } },
    }),
    gadget({ id: 2, kind: 'reported', filter: null, config: { collapsed: true } }),
    gadget({ id: 3, kind: 'recent', filter: null }),
  ];

  it('turns that gadget into one on the filter, in the same place', () => {
    const next = swapToFilter(builtins, 2, 40);
    expect(next.map((g) => [g.id, g.kind, g.filter])).toEqual([
      [1, 'assigned', null],
      [2, 'results', 40],
      [3, 'recent', null],
    ]);
    expect(next[1]?.config).toEqual({ collapsed: true });
  });

  it('keeps its size and drops its sort, now the filter’s order', () => {
    expect(swapToFilter(builtins, 1, 40)[0]?.config).toEqual({ size: 5 });
  });

  it('adds the view when the gadget is gone', () => {
    const next = swapToFilter(builtins, 9, 40);
    expect(next).toHaveLength(4);
    expect(next[3]).toEqual({ id: 0, kind: 'results', filter: 40, config: {} });
  });
});

describe('collapsed', () => {
  it('survives a save', () => {
    const folded = setCollapsed([gadget({}), gadget({ id: 8 })], (g) => g.id === 8, true);
    expect(folded.map((g) => toInput(g).config.collapsed)).toEqual([undefined, true]);
  });
});

describe('copyOnEditPath', () => {
  it('opens a new view on the gadget’s query, in its order', () => {
    const jql = viewJql(BUILTIN_QUERIES.assigned, { field: 'key', desc: false });
    expect(jql).toMatch(/ORDER BY key ASC$/);
    const url = new URL(copyOnEditPath(jql, 4), 'http://x');
    expect(url.pathname).toBe('/views/new');
    expect(url.searchParams.get('jql')).toBe(jql);
    expect(url.searchParams.get('gadget')).toBe('4');
  });
});

describe('the ready-made views', () => {
  it('open as filters, unchanged', () => {
    for (const kind of ['watching', 'due'] as const) {
      const read = jqlToFilters(BUILTIN_QUERIES[kind]);
      expect(read.ok && filtersToJql(read.state)).toBe(BUILTIN_QUERIES[kind]);
    }
  });

  it('are in the space when a gadget shows them', () => {
    const gadgets = [gadget({ kind: 'watching', filter: null }), gadget({ kind: 'count' })];
    expect(inSpace(gadgets, 'watching')).toBe(true);
    expect(inSpace(gadgets, 'due')).toBe(false);
    expect(inSpace(gadgets, { filter: 3 })).toBe(true);
    expect(inSpace(gadgets, { filter: 4 })).toBe(false);
    expect(inSpace([gadget({ unavailable: true, filter: null })], { filter: 3 })).toBe(false);
  });
});
