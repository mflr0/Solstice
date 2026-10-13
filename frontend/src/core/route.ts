/**
 * The interface's addresses. Issue pages keep Jira's /browse/KEY form so old
 * links, bookmarks and e-mails keep working after the move.
 */

export type Route =
  | { name: 'login' }
  | { name: 'home' }
  | { name: 'projects' }
  | { name: 'project'; key: string }
  | { name: 'projectSettings'; key: string }
  | { name: 'search' }
  | { name: 'issue'; key: string }
  | { name: 'person'; username: string }
  | { name: 'adminUsers' }
  | { name: 'notFound' };

function decode(segment: string): string | null {
  try {
    return decodeURIComponent(segment);
  } catch {
    return null;
  }
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
      if (a === 'projects') return { name: 'projects' };
      if (a === 'issues') return { name: 'search' };
      break;
    case 2:
      if (a === 'browse' && b) return { name: 'issue', key: b.toUpperCase() };
      if (a === 'projects' && b) return { name: 'project', key: b.toUpperCase() };
      if (a === 'people' && b) return { name: 'person', username: b };
      if (a === 'admin' && b === 'users') return { name: 'adminUsers' };
      break;
    case 3:
      if (a === 'projects' && b && c === 'settings') {
        return { name: 'projectSettings', key: b.toUpperCase() };
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
  projectSettings: (key: string) => `/projects/${encodeURIComponent(key)}/settings`,
  search: (q?: string) => (q ? `/issues?q=${encodeURIComponent(q)}` : '/issues'),
  issue: (key: string) => `/browse/${encodeURIComponent(key)}`,
  person: (username: string) => `/people/${encodeURIComponent(username)}`,
  adminUsers: () => '/admin/users',
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
