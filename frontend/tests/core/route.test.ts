import { describe, expect, it } from 'vitest';

import { isAppLink, matchRoute, paths, safeNext } from '../../src/core/route.js';

describe('matchRoute', () => {
  it.each([
    ['/', { name: 'home' }],
    ['/login', { name: 'login' }],
    ['/projects', { name: 'projects' }],
    ['/projects/demo', { name: 'project', key: 'DEMO' }],
    ['/projects/DEMO/settings', { name: 'projectSettings', key: 'DEMO' }],
    ['/issues', { name: 'search' }],
    ['/browse/demo-12', { name: 'issue', key: 'DEMO-12' }],
    ['/people/marie.durand', { name: 'person', username: 'marie.durand' }],
    ['/people/ren%C3%A9', { name: 'person', username: 'rené' }],
    ['/admin/users', { name: 'adminUsers' }],
    ['/themes', { name: 'themes' }],
    ['/browse', { name: 'notFound' }],
    ['/projects/DEMO/nope', { name: 'notFound' }],
    ['/people/%E0%A4%A', { name: 'notFound' }],
  ])('%s', (path, route) => {
    expect(matchRoute(path)).toEqual(route);
  });

  it('round-trips the addresses it builds', () => {
    expect(matchRoute(paths.issue('WEB-3'))).toEqual({ name: 'issue', key: 'WEB-3' });
    expect(matchRoute(paths.person('jeanne dupont'))).toEqual({
      name: 'person',
      username: 'jeanne dupont',
    });
    expect(paths.search('#urgent @moi')).toBe('/issues?q=%23urgent%20%40moi');
  });
});

describe('links', () => {
  const origin = 'https://solstice.example';

  it('keeps same-site pages in the application', () => {
    expect(isAppLink(new URL('/browse/DEMO-1', origin), origin)).toBe(true);
  });

  it('leaves files, the API and other sites to the browser', () => {
    expect(isAppLink(new URL('/api/attachments/3', origin), origin)).toBe(false);
    expect(isAppLink(new URL('https://example.org/x'), origin)).toBe(false);
  });

  it('only goes back to same-site paths after sign-in', () => {
    expect(safeNext('/browse/DEMO-1?x=1')).toBe('/browse/DEMO-1?x=1');
    expect(safeNext(null)).toBe('/');
    expect(safeNext('https://evil.example')).toBe('/');
    expect(safeNext('//evil.example')).toBe('/');
    expect(safeNext('/\\evil.example')).toBe('/');
  });
});
