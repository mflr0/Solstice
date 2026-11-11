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
  | { name: 'view'; id: number | null }
  | { name: 'legacySearch' }
  | { name: 'issue'; key: string }
  | { name: 'person'; username: string }
  | { name: 'admin'; section: AdminSection; item: string | null }
  | { name: 'account'; section: AccountSection }
  | { name: 'themes' }
  | { name: 'boards' }
  | { name: 'board'; id: number }
  | { name: 'backlog'; id: number }
  | { name: 'boardReports'; id: number }
  | { name: 'boardSettings'; id: number }
  | { name: 'legacyBoard' }
  | { name: 'oauthAuthorize' }
  | { name: 'notFound' };

function decode(segment: string): string | null {
  try {
    return decodeURIComponent(segment);
  } catch {
    return null;
  }
}

/** Whether an address segment is a record id (board, view): a positive whole number. */
function recordId(segment: string | null | undefined): boolean {
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
      if (a === 'issues') return { name: 'legacySearch' };
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
      if (a === 'admin' && b) {
        const section = adminSectionOf(b);
        if (section) return { name: 'admin', section, item: null };
      }
      if (a === 'settings' && b) {
        const section = accountSectionOf(b);
        if (section) return { name: 'account', section };
      }
      if (a === 'boards' && recordId(b)) return { name: 'board', id: Number(b) };
      if (a === 'views' && b === 'new') return { name: 'view', id: null };
      if (a === 'views' && recordId(b)) return { name: 'view', id: Number(b) };
      if (a === 'oauth' && b === 'authorize') return { name: 'oauthAuthorize' };
      if (a === 'secure' && (b === 'RapidBoard.jspa' || b === 'RapidView.jspa')) {
        return { name: 'legacyBoard' };
      }
      break;
    case 3:
      if (a === 'projects' && b && c === 'settings') {
        return { name: 'projectSettings', key: b.toUpperCase() };
      }
      if (a === 'boards' && recordId(b) && c === 'backlog') {
        return { name: 'backlog', id: Number(b) };
      }
      if (a === 'boards' && recordId(b) && c === 'reports') {
        return { name: 'boardReports', id: Number(b) };
      }
      if (a === 'boards' && recordId(b) && c === 'settings') {
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
  search: (q?: string) => (q ? `/views/new?q=${encodeURIComponent(q)}` : '/views/new'),
  view: (id: number) => `/views/${id}`,
  issue: (key: string) => `/browse/${encodeURIComponent(key)}`,
  person: (username: string) => `/people/${encodeURIComponent(username)}`,
  admin: adminPath,
  account: accountPath,
  themes: () => '/themes',
  boards: (project?: string) =>
    project ? `/boards?project=${encodeURIComponent(project)}` : '/boards',
  /** The board list with the create dialog open, the project chosen in it. */
  newBoard: (project?: string) =>
    project ? `/boards?project=${encodeURIComponent(project)}&create=1` : '/boards?create=1',
  board: (id: number) => `/boards/${id}`,
  backlog: (id: number) => `/boards/${id}/backlog`,
  boardReports: (id: number, sprint?: number) =>
    `/boards/${id}/reports${sprint ? `?sprint=${sprint}` : ''}`,
  boardVelocity: (id: number) => `/boards/${id}/reports?report=velocity`,
  boardFlow: (id: number) => `/boards/${id}/reports?report=flow`,
  boardSettings: (id: number) => `/boards/${id}/settings`,
};

/**
 * Where a former search address (/issues?…) leads: its saved filter's view,
 * or an unsaved one, with the other parameters kept.
 */
export function legacySearchPath(search: string): string {
  const sp = new URLSearchParams(search);
  const filter = sp.get('filter');
  sp.delete('filter');
  const rest = sp.toString();
  const to = filter && recordId(filter) ? paths.view(Number(filter)) : '/views/new';
  return rest ? `${to}?${rest}` : to;
}

/**
 * Whether a link should be handled by the interface rather than the
 * browser: same origin, and not an API address (downloads, events).
 */
export function isAppLink(url: URL, origin: string): boolean {
  if (url.origin !== origin) return false;
  return !url.pathname.startsWith('/api/') && !url.pathname.startsWith('/rest/');
}

/** The issue shown in the modal over the page (`?issue=KEY`), or null. */
export function modalIssue(search: string): string | null {
  const key = new URLSearchParams(search).get('issue')?.trim();
  return key ? key.toUpperCase() : null;
}

/** A query string with the modal's issue set, or removed for null; the other parameters stay. */
export function withModalIssue(search: string, key: string | null): string {
  const sp = new URLSearchParams(search);
  if (key) sp.set('issue', key);
  else sp.delete('issue');
  const s = sp.toString();
  return s ? `?${s}` : '';
}

/**
 * The issue an in-app link leads to (/browse/KEY), or null. A link with a
 * query or a hash (a comment) wants the full page.
 */
export function issueLinkKey(url: URL, origin: string): string | null {
  if (!isAppLink(url, origin) || url.search || url.hash) return null;
  const route = matchRoute(url.pathname);
  return route.name === 'issue' ? route.key : null;
}

/**
 * Whether moving from one path to an address lands on a new page, whose heading
 * then takes the focus: not a query-only change, a replace or a hash target.
 */
export function focusesPageHeading(
  from: string,
  to: { pathname: string; hash: string },
  replace: boolean,
): boolean {
  return !replace && to.pathname !== from && to.hash === '';
}

/** A safe "next" address after signing in: only same-site paths. */
export function safeNext(next: string | null): string {
  if (!next || !next.startsWith('/') || next.startsWith('//') || next.startsWith('/\\')) return '/';
  return next;
}
