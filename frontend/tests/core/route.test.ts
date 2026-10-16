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
    ['/admin', { name: 'admin', section: 'overview', item: null }],
    ['/admin/users', { name: 'admin', section: 'users', item: null }],
    ['/admin/issue-types', { name: 'admin', section: 'issueTypes', item: null }],
    ['/admin/webhooks/12', { name: 'admin', section: 'webhooks', item: '12' }],
    ['/admin/webhooks/new', { name: 'admin', section: 'webhooks', item: 'new' }],
    ['/admin/mail/3', { name: 'notFound' }],
    ['/admin/nope', { name: 'notFound' }],
    ['/settings', { name: 'account', section: 'profile' }],
    ['/settings/notifications', { name: 'account', section: 'notifications' }],
    ['/settings/profile', { name: 'notFound' }],
    ['/themes', { name: 'themes' }],
    ['/browse', { name: 'notFound' }],
    ['/projects/DEMO/nope', { name: 'notFound' }],
    ['/projects/demo/settings/versions', { name: 'projectSettings', key: 'DEMO', tab: 'versions' }],
    ['/projects/DEMO/settings/general', { name: 'projectSettings', key: 'DEMO', tab: 'general' }],
    ['/projects/DEMO/settings/nope', { name: 'notFound' }],
    ['/projects/DEMO/settings/versions/3', { name: 'notFound' }],
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
    expect(paths.admin('overview')).toBe('/admin');
    expect(paths.admin('linkTypes')).toBe('/admin/link-types');
    expect(paths.admin('updates')).toBe('/admin/updates');
    expect(matchRoute(paths.admin('webhooks', 7))).toEqual({
      name: 'admin',
      section: 'webhooks',
      item: '7',
    });
    expect(paths.projectSettings('SW')).toBe('/projects/SW/settings');
    expect(paths.projectSettings('SW', 'general')).toBe('/projects/SW/settings');
    expect(paths.projectSettings('SW', 'people')).toBe('/projects/SW/settings/people');
    expect(matchRoute(paths.projectSettings('SW', 'rights'))).toEqual({
      name: 'projectSettings',
      key: 'SW',
      tab: 'rights',
    });
    expect(paths.account('profile')).toBe('/settings');
    expect(matchRoute(paths.account('tokens'))).toEqual({ name: 'account', section: 'tokens' });
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
