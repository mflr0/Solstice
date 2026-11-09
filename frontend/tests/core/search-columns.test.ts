import { describe, expect, it } from 'vitest';

import {
  DEFAULT_COLUMNS,
  exportColumns,
  NATIVE_COLUMNS,
  parseColumns,
  serverColumns,
  viewColumns,
} from '../../src/core/search-columns.js';

describe('the stored choice of columns', () => {
  it('gives the usual columns when nothing or nothing readable is stored', () => {
    for (const raw of [null, '', 'not json', '{"a":1}', '42']) {
      expect(parseColumns(raw)).toEqual(DEFAULT_COLUMNS);
    }
  });

  it('keeps the order and an empty choice', () => {
    expect(parseColumns('["updated","customfield_10","status"]')).toEqual([
      'updated',
      'customfield_10',
      'status',
    ]);
    expect(parseColumns('[]')).toEqual([]);
  });

  it('drops fixed, repeated and malformed entries', () => {
    expect(parseColumns('["issuekey","status",3,null,"summary","status","labels"]')).toEqual([
      'status',
      'labels',
    ]);
  });

  it('drops unknown or removed fields, not an error', () => {
    const known = ['issuekey', 'summary', 'status', 'labels', 'customfield_10'];
    expect(parseColumns('["customfield_99","status","sprint","customfield_10"]', known)).toEqual([
      'status',
      'customfield_10',
    ]);
  });
});

describe('the columns of a view', () => {
  const stored = '["status","assignee"]';

  it("shows a saved filter's columns over the stored choice", () => {
    expect(viewColumns(['labels', 'status'], stored)).toEqual(['labels', 'status']);
    expect(viewColumns([], stored)).toEqual([]);
  });

  it('falls back to the stored choice for a filter without columns', () => {
    expect(viewColumns(null, stored)).toEqual(['status', 'assignee']);
    expect(viewColumns(undefined, null)).toEqual(DEFAULT_COLUMNS);
  });

  it("cleans the filter's columns like a stored choice", () => {
    expect(viewColumns(['summary', 'customfield_99', 'status'], stored, ['status'])).toEqual([
      'status',
    ]);
  });
});

describe('what the choice asks of the server', () => {
  const chosen = ['customfield_10', 'status', 'issuetype', 'labels', 'reporter'];

  it('asks for values only for the columns the table cannot draw by itself', () => {
    expect(serverColumns(chosen)).toEqual(['customfield_10', 'labels']);
  });

  it('draws the resolution date itself, as a relative date', () => {
    expect(NATIVE_COLUMNS.has('resolutiondate')).toBe(true);
    expect(serverColumns(['resolutiondate', 'labels'])).toEqual(['labels']);
  });

  it('exports in the table order: type, key, summary, then the choice', () => {
    expect(exportColumns(chosen)).toEqual([
      'issuetype',
      'issuekey',
      'summary',
      'customfield_10',
      'status',
      'labels',
      'reporter',
    ]);
    expect(exportColumns([])).toEqual(['issuekey', 'summary']);
  });
});
