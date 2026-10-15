import { describe, expect, it } from 'vitest';

import { readLocation, searchHref, toJql, writeLocation } from '../../src/core/filters-url.js';

describe('search address', () => {
  it('reads the defaults', () => {
    expect(readLocation('')).toEqual({
      loc: { jql: null, mode: null, filter: null, page: 1 },
      legacy: false,
    });
  });

  it('tells an empty query from a missing one', () => {
    expect(readLocation('?jql=').loc.jql).toBe('');
    expect(writeLocation({ jql: '', mode: null, filter: null, page: 1 })).toBe('?jql=');
    expect(writeLocation({ jql: null, mode: null, filter: null, page: 1 })).toBe('');
  });

  it('round-trips every part, with quotes and accents', () => {
    const loc = {
      jql: 'status in ("À faire", "En cours") AND text ~ "a & b = c?" ORDER BY key ASC',
      mode: 'jql' as const,
      filter: 12,
      page: 3,
    };
    const s = writeLocation(loc);
    expect(s).toMatch(/^\?filter=12&jql=status\+in\+/);
    expect(readLocation(s)).toEqual({ loc, legacy: false });
  });

  it('ignores invalid numbers and modes', () => {
    expect(readLocation('?page=0&filter=abc&mode=x').loc).toEqual({
      jql: null,
      mode: null,
      filter: null,
      page: 1,
    });
    expect(readLocation('?page=-2').loc.page).toBe(1);
  });

  it('reads former ?q= links as JQL', () => {
    expect(readLocation('?q=%40moi+est%3Aouvert')).toEqual({
      loc: {
        jql: 'assignee = currentUser() AND statusCategory != Done',
        mode: null,
        filter: null,
        page: 1,
      },
      legacy: true,
    });
    expect(readLocation('?q=project+%3D+DEMO').loc.jql).toBe('project = DEMO');
    // jql= wins over q=.
    expect(readLocation('?q=x&jql=a+%3D+1')).toMatchObject({
      loc: { jql: 'a = 1' },
      legacy: false,
    });
  });

  it('toJql keeps JQL and translates the former syntax', () => {
    expect(toJql(' status = Open ')).toBe('status = Open');
    expect(toJql('#urgent')).toBe('labels = urgent');
  });

  it('builds links to a search', () => {
    expect(searchHref('text ~ "panier"')).toBe('/issues?jql=text+%7E+%22panier%22');
    expect(readLocation(searchHref('text ~ "panier"').slice(7)).loc.jql).toBe('text ~ "panier"');
  });
});
