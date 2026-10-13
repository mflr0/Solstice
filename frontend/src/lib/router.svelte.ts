/**
 * A small history-API router: the current address as runes state, and a
 * click handler that keeps same-site links inside the application.
 */

import { isAppLink, matchRoute, type Route } from '../core/route.js';

class Router {
  path = $state(location.pathname);
  search = $state(location.search);
  route: Route = $derived(matchRoute(this.path));

  constructor() {
    window.addEventListener('popstate', () => this.sync());
  }

  private sync(): void {
    this.path = location.pathname;
    this.search = location.search;
  }

  /** The current path with its query string. */
  get here(): string {
    return this.path + this.search;
  }

  param(name: string): string | null {
    return new URLSearchParams(this.search).get(name);
  }

  navigate(to: string, opts: { replace?: boolean } = {}): void {
    const url = new URL(to, location.origin);
    const target = url.pathname + url.search + url.hash;
    if (target === this.here + location.hash) return;
    if (opts.replace) history.replaceState(null, '', target);
    else history.pushState(null, '', target);
    const pageChanged = url.pathname !== this.path;
    this.sync();
    if (pageChanged && !opts.replace) window.scrollTo(0, 0);
  }

  /** Changes query parameters of the current page (null removes one). */
  setParams(params: Record<string, string | null>, opts: { replace?: boolean } = {}): void {
    // eslint-disable-next-line svelte/prefer-svelte-reactivity -- a throwaway value, not state
    const sp = new URLSearchParams(this.search);
    for (const [k, v] of Object.entries(params)) {
      if (v === null || v === '') sp.delete(k);
      else sp.set(k, v);
    }
    const s = sp.toString();
    this.navigate(this.path + (s ? `?${s}` : ''), { replace: opts.replace ?? true });
  }
}

export const router = new Router();

/**
 * Document-level click handler: plain left clicks on same-site links are
 * routed in place; anything else (new tab, download, API file) is left to
 * the browser.
 */
export function interceptLinks(e: MouseEvent): void {
  if (e.defaultPrevented || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) {
    return;
  }
  const a = (e.target as Element | null)?.closest?.('a');
  if (!a || !a.href || a.target === '_blank' || a.hasAttribute('download')) return;
  // eslint-disable-next-line svelte/prefer-svelte-reactivity -- a throwaway value, not state
  const url = new URL(a.href);
  if (!isAppLink(url, location.origin)) return;
  e.preventDefault();
  router.navigate(url.pathname + url.search + url.hash);
}
