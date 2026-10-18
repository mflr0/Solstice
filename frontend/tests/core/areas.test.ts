import { describe, expect, it } from 'vitest';

import {
  ACCOUNT_SECTIONS,
  ADMIN_NAV,
  ADMIN_SECTIONS,
  accountPath,
  accountSectionOf,
  adminPath,
  adminSectionOf,
} from '../../src/core/areas.js';
import type { AccountSection, AdminNavItem } from '../../src/core/areas.js';

describe('adminPath', () => {
  it.each<[AdminNavItem, string]>([
    ['overview', '/admin'],
    ['mail', '/admin/mail'],
    ['issueTypes', '/admin/issue-types'],
    ['linkTypes', '/admin/link-types'],
    ['updates', '/admin/updates'],
  ])('%s is %s', (item, want) => {
    expect(adminPath(item)).toBe(want);
  });

  it('addresses an item below the section', () => {
    expect(adminPath('webhooks', 12)).toBe('/admin/webhooks/12');
  });

  it('encodes the item of the address', () => {
    expect(adminPath('webhooks', 'a b/c')).toBe('/admin/webhooks/a%20b%2Fc');
  });
});

describe('adminSectionOf', () => {
  it.each<[string, string]>([
    ['mail', 'mail'],
    ['issue-types', 'issueTypes'],
  ])('%s names %s', (slug, section) => {
    expect(adminSectionOf(slug)).toBe(section);
  });

  it.each(['', 'overview', 'issueTypes', 'nope'])('does not name %s', (slug) => {
    expect(adminSectionOf(slug)).toBeNull();
  });

  it('gives back the section of every address', () => {
    for (const section of ADMIN_SECTIONS) {
      if (section === 'overview') continue;
      const path = adminPath(section);
      expect(adminSectionOf(path.slice(path.lastIndexOf('/') + 1))).toBe(section);
    }
  });
});

describe('ADMIN_NAV', () => {
  const items = ADMIN_NAV.flatMap((group) => group.items);

  it.each(ADMIN_SECTIONS)('links %s exactly once', (section) => {
    expect(items.filter((item) => item === section).length).toBe(1);
  });

  it('links updates exactly once', () => {
    expect(items.filter((item) => item === 'updates').length).toBe(1);
  });
});

describe('accountPath', () => {
  it.each<[AccountSection, string]>([
    ['profile', '/settings'],
    ['notifications', '/settings/notifications'],
  ])('%s is %s', (section, want) => {
    expect(accountPath(section)).toBe(want);
  });
});

describe('accountSectionOf', () => {
  it('names notifications', () => {
    expect(accountSectionOf('notifications')).toBe('notifications');
  });

  it.each(['profile', '', 'nope'])('does not name %s', (slug) => {
    expect(accountSectionOf(slug)).toBeNull();
  });

  it('gives back the section of every address', () => {
    for (const section of ACCOUNT_SECTIONS) {
      if (section === 'profile') continue;
      const path = accountPath(section);
      expect(accountSectionOf(path.slice(path.lastIndexOf('/') + 1))).toBe(section);
    }
  });
});
