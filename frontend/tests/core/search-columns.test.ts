import { describe, expect, it } from 'vitest';

import {
  DEFAULT_COLUMNS,
  exportColumns,
  parseColumns,
  serverColumns,
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

describe('what the choice asks of the server', () => {
  const chosen = ['customfield_10', 'status', 'issuetype', 'labels', 'reporter'];

  it('asks for values only for the columns the table cannot draw by itself', () => {
    expect(serverColumns(chosen)).toEqual(['customfield_10', 'labels']);
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
