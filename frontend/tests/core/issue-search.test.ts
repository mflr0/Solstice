import { describe, expect, it } from 'vitest';

import { findIssues, type IssueQuery } from '../../src/core/issue-search.js';
import type { IssueSummary } from '../../src/core/types.js';

const issue = (key: string) => ({ key, summary: key }) as IssueSummary;

/** A search where the key query finds byKey and the text query finds byText. */
function stub(byKey: string[], byText: string[]): { query: IssueQuery; asked: string[] } {
  const asked: string[] = [];
  const query: IssueQuery = ({ jql }) => {
    asked.push(jql);
    const keys = jql.startsWith('key = ') ? byKey : byText;
    return Promise.resolve({
      total: keys.length,
      page: 0,
      size: keys.length,
      issues: keys.map(issue),
    });
  };
  return { query, asked };
}

const keys = (found: IssueSummary[]) => found.map((i) => i.key);

describe('findIssues', () => {
  it('puts the exact key first and drops it from the text matches', async () => {
    const { query } = stub(['DEMO-12'], ['DEMO-3', 'DEMO-12', 'DEMO-4']);
    expect(keys(await findIssues('demo-12', query))).toEqual(['DEMO-12', 'DEMO-3', 'DEMO-4']);
  });

  it('drops excluded keys, whatever their case', async () => {
    const { query, asked } = stub(['DEMO-1'], ['DEMO-1', 'DEMO-2', 'DEMO-3']);
    expect(keys(await findIssues('DEMO-1', query, { exclude: ['demo-1', 'DEMO-3'] }))).toEqual([
      'DEMO-2',
    ]);
    expect(asked.some((q) => q.startsWith('key = '))).toBe(false);
  });

  it('keeps to the size', async () => {
    const { query } = stub([], ['A-1', 'A-2', 'A-3']);
    expect(keys(await findIssues('word', query, { size: 2 }))).toEqual(['A-1', 'A-2']);
  });

  it('asks nothing for blank words, and only the text for words that are not a key', async () => {
    const { query, asked } = stub(['X-1'], ['A-1']);
    expect(await findIssues('  ', query)).toEqual([]);
    expect(asked).toEqual([]);
    expect(keys(await findIssues('login page', query))).toEqual(['A-1']);
    expect(asked).toEqual(['text ~ "login page" ORDER BY updated DESC']);
  });

  it('counts a failed query as no match', async () => {
    const query: IssueQuery = ({ jql }) =>
      jql.startsWith('key = ')
        ? Promise.reject(new Error('down'))
        : Promise.resolve({ total: 1, page: 0, size: 1, issues: [issue('A-1')] });
    expect(keys(await findIssues('A-9', query))).toEqual(['A-1']);
  });
});
