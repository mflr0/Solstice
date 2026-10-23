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

export type Route =
  | { name: 'login' }
  | { name: 'forgot' }
  | { name: 'reset'; token: string }
  | { name: 'home' }
  | { name: 'projects' }
  | { name: 'project'; key: string }
  | { name: 'projectSettings'; key: string }
  | { name: 'search' }
  | { name: 'issue'; key: string }
  | { name: 'person'; username: string }
  | { name: 'admin'; section: AdminSection; item: string | null }
  | { name: 'updates' }
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

/**
 * Pages anyone may open without signing in: the sign-in form and the two
 * pages of the forgotten-password links.
 */
export function isPublicRoute(route: Route): boolean {
  return route.name === 'login' || route.name === 'forgot' || route.name === 'reset';
}

export function matchRoute(pathname: string): Route {
  const parts = pathname.split('/').filter((p) => p !== '');
  const [a, b, c] = parts.map(decode);
  if (parts.some((_, i) => [a, b, c][i] === null)) return { name: 'notFound' };
  switch (parts.length) {
    case 0:
      return { name: 'home' };
    case 1:
      if (a === 'login') return { name: 'login' };
      if (a === 'forgot') return { name: 'forgot' };
      if (a === 'projects') return { name: 'projects' };
      if (a === 'issues') return { name: 'search' };
      if (a === 'themes') return { name: 'themes' };
      if (a === 'boards') return { name: 'boards' };
      if (a === 'admin') return { name: 'admin', section: 'overview', item: null };
      if (a === 'settings') return { name: 'account', section: 'profile' };
      break;
    case 2:
      if (a === 'browse' && b) return { name: 'issue', key: b.toUpperCase() };
      if (a === 'reset' && b) return { name: 'reset', token: b };
      if (a === 'projects' && b) return { name: 'project', key: b.toUpperCase() };
      if (a === 'people' && b) return { name: 'person', username: b };
      if (a === 'admin' && b === 'updates') return { name: 'updates' };
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
  forgot: () => '/forgot',
  reset: (token: string) => `/reset/${encodeURIComponent(token)}`,
  projects: () => '/projects',
  project: (key: string) => `/projects/${encodeURIComponent(key)}`,
  projectSettings: (key: string) => `/projects/${encodeURIComponent(key)}/settings`,
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
