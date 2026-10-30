import { describe, expect, it } from 'vitest';

import {
  BUILTIN_QUERIES,
  clickGadgetSort,
  gadgetQuery,
  jqlValue,
  narrowedJql,
  queryOrder,
  sortedQuery,
  toInput,
  type Gadget,
} from '../../src/core/dashboard.js';
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
