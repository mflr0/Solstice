/**
 * The interface's addresses. Issue pages keep Jira's /browse/KEY form so old
 * links, bookmarks and e-mails keep working after the move.
 */

import {
  accountPath,
  accountSectionOf,
  ADMIN_ITEM_SECTIONS,
  adminPath,
  adminSectionOf,
  type AccountSection,
  type AdminSection,
} from './areas.js';
import { PROJECT_TABS, type ProjectTab } from './project-admin.js';

export type Route =
  | { name: 'login' }
  | { name: 'home' }
  | { name: 'projects' }
  | { name: 'project'; key: string }
  | { name: 'projectSettings'; key: string; tab?: ProjectTab }
  | { name: 'search' }
  | { name: 'issue'; key: string }
  | { name: 'person'; username: string }
  | { name: 'admin'; section: AdminSection; item: string | null }
  | { name: 'account'; section: AccountSection }
  | { name: 'themes' }
  | { name: 'boards' }
  | { name: 'board'; id: number }
  | { name: 'backlog'; id: number }
  | { name: 'boardSettings'; id: number }
  | { name: 'legacyBoard' }
  | { name: 'notFound' };

function decode(segment: string): string | null {
  try {
    return decodeURIComponent(segment);
  } catch {
    return null;
  }
}

/** Whether an address segment is a board id: a positive whole number. */
function boardId(segment: string | null | undefined): boolean {
  return !!segment && /^[1-9]\d{0,15}$/.test(segment);
}

export function matchRoute(pathname: string): Route {
  const parts = pathname.split('/').filter((p) => p !== '');
  const [a, b, c, d] = parts.map(decode);
  if (parts.some((_, i) => [a, b, c, d][i] === null)) return { name: 'notFound' };
  switch (parts.length) {
    case 0:
      return { name: 'home' };
    case 1:
      if (a === 'login') return { name: 'login' };
      if (a === 'projects') return { name: 'projects' };
      if (a === 'issues') return { name: 'search' };
      if (a === 'themes') return { name: 'themes' };
      if (a === 'boards') return { name: 'boards' };
      if (a === 'admin') return { name: 'admin', section: 'overview', item: null };
      if (a === 'settings') return { name: 'account', section: 'profile' };
      break;
    case 2:
      if (a === 'browse' && b) return { name: 'issue', key: b.toUpperCase() };
      if (a === 'projects' && b) return { name: 'project', key: b.toUpperCase() };
      if (a === 'people' && b) return { name: 'person', username: b };
      if (a === 'admin' && b) {
        const section = adminSectionOf(b);
        if (section) return { name: 'admin', section, item: null };
      }
      if (a === 'settings' && b) {
        const section = accountSectionOf(b);
        if (section) return { name: 'account', section };
      }
      if (a === 'boards' && boardId(b)) return { name: 'board', id: Number(b) };
      if (a === 'secure' && (b === 'RapidBoard.jspa' || b === 'RapidView.jspa')) {
        return { name: 'legacyBoard' };
      }
      break;
    case 4: {
      const tab = PROJECT_TABS.find((x) => x === d);
      if (a === 'projects' && b && c === 'settings' && tab) {
        return { name: 'projectSettings', key: b.toUpperCase(), tab };
      }
      break;
    }
    case 3:
      if (a === 'projects' && b && c === 'settings') {
        return { name: 'projectSettings', key: b.toUpperCase() };
      }
      if (a === 'boards' && boardId(b) && c === 'backlog') {
        return { name: 'backlog', id: Number(b) };
      }
      if (a === 'boards' && boardId(b) && c === 'settings') {
        return { name: 'boardSettings', id: Number(b) };
      }
      if (a === 'admin' && b && c) {
        const section = adminSectionOf(b);
        if (section && ADMIN_ITEM_SECTIONS.includes(section)) {
          return { name: 'admin', section, item: c };
        }
      }
      break;
  }
  return { name: 'notFound' };
}

export const paths = {
  home: () => '/',
  login: (next?: string) =>
    next && next !== '/' ? `/login?next=${encodeURIComponent(next)}` : '/login',
  projects: () => '/projects',
  project: (key: string) => `/projects/${encodeURIComponent(key)}`,
  /** A project's settings; the General tab is the bare address. */
  projectSettings: (key: string, tab?: ProjectTab) =>
    `/projects/${encodeURIComponent(key)}/settings${tab && tab !== 'general' ? `/${tab}` : ''}`,
  search: (q?: string) => (q ? `/issues?q=${encodeURIComponent(q)}` : '/issues'),
  issue: (key: string) => `/browse/${encodeURIComponent(key)}`,
  person: (username: string) => `/people/${encodeURIComponent(username)}`,
  admin: adminPath,
  account: accountPath,
  themes: () => '/themes',
  boards: (project?: string) =>
    project ? `/boards?project=${encodeURIComponent(project)}` : '/boards',
  board: (id: number) => `/boards/${id}`,
  backlog: (id: number) => `/boards/${id}/backlog`,
  boardSettings: (id: number) => `/boards/${id}/settings`,
};

/**
 * Whether a link should be handled by the interface rather than the
 * browser: same origin, and not an API address (downloads, events).
 */
export function isAppLink(url: URL, origin: string): boolean {
  if (url.origin !== origin) return false;
  return !url.pathname.startsWith('/api/') && !url.pathname.startsWith('/rest/');
}

/** A safe "next" address after signing in: only same-site paths. */
export function safeNext(next: string | null): string {
  if (!next || !next.startsWith('/') || next.startsWith('//') || next.startsWith('/\\')) return '/';
  return next;
}
