/**
 * The search page's address: the query as JQL (so a link carries the whole
 * search), how it is shown, the saved filter it came from, the page.
 *
 *   /issues?jql=status+%3D+Open&mode=jql&filter=12&page=2
 *
 * A missing jql means "the default search"; an empty one means "everything".
 * Older links carry ?q= in the former syntax (or JQL): they still work.
 */

import { looksLikeJql } from './jql.js';
import { simpleToJql } from './jql-simple.js';

export interface SearchLocation {
  /** null: not given, the page's default applies. */
  jql: string | null;
  /** 'jql' when the query editor was chosen over the filters. */
  mode: 'jql' | null;
  /** The saved filter the search started from. */
  filter: number | null;
  page: number;
}

/** A search typed in either syntax, as JQL. */
export function toJql(query: string): string {
  const q = query.trim();
  return looksLikeJql(q) ? q : simpleToJql(q);
}

function positive(s: string | null): number | null {
  if (!s || !/^[1-9]\d{0,9}$/.test(s)) return null;
  return Number(s);
}

/** Reads the address; legacy is true when it used ?q= and should be rewritten. */
export function readLocation(search: string): { loc: SearchLocation; legacy: boolean } {
  const sp = new URLSearchParams(search);
  let jql = sp.get('jql');
  let legacy = false;
  const q = sp.get('q');
  if (jql === null && q !== null) {
    jql = toJql(q);
    legacy = true;
  }
  return {
    loc: {
      jql,
      mode: sp.get('mode') === 'jql' ? 'jql' : null,
      filter: positive(sp.get('filter')),
      page: positive(sp.get('page')) ?? 1,
    },
    legacy,
  };
}

/** The query string for a search ('' when everything is default). */
export function writeLocation(loc: SearchLocation): string {
  const sp = new URLSearchParams();
  if (loc.filter !== null) sp.set('filter', String(loc.filter));
  if (loc.jql !== null) sp.set('jql', loc.jql);
  if (loc.mode === 'jql') sp.set('mode', 'jql');
  if (loc.page > 1) sp.set('page', String(loc.page));
  const s = sp.toString();
  return s ? `?${s}` : '';
}

/** The search page showing a query. */
export function searchHref(jql: string): string {
  return `/issues${writeLocation({ jql, mode: null, filter: null, page: 1 })}`;
}
