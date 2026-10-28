import { describe, expect, it } from 'vitest';

import {
  gadgetQuery,
  jqlValue,
  narrowedJql,
  toInput,
  type Gadget,
} from '../../src/core/dashboard.js';

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
