import { quote, quoteString } from './jql.js';
import { isIssueKey } from './marks.js';
import type { IssuePage, IssueSummary } from './types.js';

/** The issue search the suggestions run on: api.issues, or a stand-in in tests. */
export type IssueQuery = (params: { jql: string; size: number }) => Promise<IssuePage>;

/**
 * The issues that match typed words: the issue with that exact key first,
 * then those whose text matches, most recently updated first. Keys in
 * exclude are left out; a failed query counts as no match.
 */
export async function findIssues(
  words: string,
  query: IssueQuery,
  { size = 6, exclude = [] }: { size?: number; exclude?: readonly string[] } = {},
): Promise<IssueSummary[]> {
  const text = words.trim();
  if (!text) return [];
  const key = isIssueKey(text) ? text.toUpperCase() : null;
  const skip = new Set(exclude.map((k) => k.toUpperCase()));
  const [byKey, byText] = await Promise.all([
    key && !skip.has(key) ? query({ jql: `key = ${quote(key)}`, size: 1 }).catch(() => null) : null,
    query({
      jql: `text ~ ${quoteString(text)} ORDER BY updated DESC`,
      size: size + skip.size,
    }).catch(() => null),
  ]);
  const out: IssueSummary[] = [];
  for (const i of [...(byKey?.issues ?? []), ...(byText?.issues ?? [])]) {
    if (skip.has(i.key) || out.some((j) => j.key === i.key)) continue;
    out.push(i);
    if (out.length === size) break;
  }
  return out;
}
