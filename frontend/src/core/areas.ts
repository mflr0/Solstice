/**
 * The administration area (/admin) and the personal settings (/settings):
 * their sections, the address of each, and how the side navigation groups
 * them. The updates page (/admin/updates) has its own route; the navigation
 * only links to it.
 */

export const ADMIN_SECTIONS = [
  'overview',
  'settings',
  'banner',
  'users',
  'groups',
  'roles',
  'projects',
  'securityLevels',
  'issueTypes',
  'priorities',
  'resolutions',
  'statuses',
  'linkTypes',
  'fields',
  'mail',
  'webhooks',
  'tokens',
  'audit',
  'import',
  'system',
] as const;

export type AdminSection = (typeof ADMIN_SECTIONS)[number];

/** An entry of the administration's navigation: a section, or the updates page. */
export type AdminNavItem = AdminSection | 'updates';

const ADMIN_SLUGS: Record<AdminSection, string> = {
  overview: '',
  settings: 'settings',
  banner: 'banner',
  users: 'users',
  groups: 'groups',
  roles: 'roles',
  projects: 'projects',
  securityLevels: 'security-levels',
  issueTypes: 'issue-types',
  priorities: 'priorities',
  resolutions: 'resolutions',
  statuses: 'statuses',
  linkTypes: 'link-types',
  fields: 'fields',
  mail: 'mail',
  webhooks: 'webhooks',
  tokens: 'tokens',
  audit: 'audit',
  import: 'import',
  system: 'system',
};

export type AdminGroup = 'general' | 'people' | 'projects' | 'issues' | 'integrations' | 'system';

/** The side navigation of the administration, in order. */
export const ADMIN_NAV: readonly { group: AdminGroup; items: readonly AdminNavItem[] }[] = [
  { group: 'general', items: ['overview', 'settings', 'banner'] },
  { group: 'people', items: ['users', 'groups', 'roles'] },
  { group: 'projects', items: ['projects', 'securityLevels'] },
  {
    group: 'issues',
    items: ['issueTypes', 'priorities', 'resolutions', 'statuses', 'linkTypes', 'fields'],
  },
  { group: 'integrations', items: ['mail', 'webhooks', 'tokens'] },
  { group: 'system', items: ['audit', 'import', 'system', 'updates'] },
];

/** The section an address segment names (/admin/<slug>), if any. */
export function adminSectionOf(slug: string): AdminSection | null {
  for (const s of ADMIN_SECTIONS) {
    if (s !== 'overview' && ADMIN_SLUGS[s] === slug) return s;
  }
  return null;
}

/** /admin, /admin/mail, /admin/webhooks/12… */
export function adminPath(item: AdminNavItem, sub?: string | number): string {
  if (item === 'updates') return '/admin/updates';
  const slug = ADMIN_SLUGS[item];
  const base = slug ? `/admin/${slug}` : '/admin';
  return sub === undefined ? base : `${base}/${encodeURIComponent(String(sub))}`;
}

/** Sections that have pages below them (/admin/<slug>/<item>). */
export const ADMIN_ITEM_SECTIONS: readonly AdminSection[] = ['webhooks'];

export const ACCOUNT_SECTIONS = [
  'profile',
  'appearance',
  'notifications',
  'tokens',
  'subscriptions',
] as const;

export type AccountSection = (typeof ACCOUNT_SECTIONS)[number];

/** The section an address segment names (/settings/<slug>), if any. */
export function accountSectionOf(slug: string): AccountSection | null {
  return slug !== 'profile' && (ACCOUNT_SECTIONS as readonly string[]).includes(slug)
    ? (slug as AccountSection)
    : null;
}

/** /settings, /settings/notifications… (e-mails link to the latter). */
export function accountPath(section: AccountSection): string {
  return section === 'profile' ? '/settings' : `/settings/${section}`;
}
