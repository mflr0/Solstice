/**
 * A small history-API router: the current address as runes state, and a
 * click handler that keeps same-site links inside the application.
 */

import { tick } from 'svelte';

import {
  focusesPageHeading,
  isAppLink,
  matchRoute,
  modalIssue,
  withModalIssue,
  type Route,
} from '../core/route.js';

/** How long a page may take to draw its heading before the focus settles on main. */
const HEADING_WAIT_MS = 5000;
/** How long a page may still replace its heading (a reload) and have the focus follow. */
const HEADING_WATCH_MS = 30000;

let stopHeadingFocus: (() => void) | null = null;

/** Focuses an element that is not normally focusable, for as long as it keeps the focus. */
export function focusTransient(el: HTMLElement, opts?: FocusOptions): void {
  if (!el.hasAttribute('tabindex')) {
    el.setAttribute('tabindex', '-1');
    el.addEventListener('blur', () => el.removeAttribute('tabindex'), { once: true });
  }
  el.focus(opts);
}

/** The page's own h1: skin previews carry theirs inside an inert, hidden mock. */
function pageHeading(main: HTMLElement): HTMLElement | undefined {
  return [...main.querySelectorAll('h1')].find((h) => !h.closest('[inert], [aria-hidden="true"]'));
}

/**
 * Takes the focus to the new page's heading once it is drawn, however late.
 * Any key or press from the user ends the wait: the focus is theirs from then on.
 */
function focusPageHeading(): void {
  stopHeadingFocus?.();
  const main = document.getElementById('main-content');
  if (!main) return;

  let placed = false;
  const place = (): void => {
    if (document.querySelector('dialog[open]')) return;
    const heading = pageHeading(main);
    if (!heading || heading === document.activeElement) return;
    const active = document.activeElement;
    // Once placed, only follow a focus that fell off a replaced heading or waits on main.
    if (placed && active && active !== document.body && active !== main) return;
    placed = true;
    focusTransient(heading, { preventScroll: true });
  };
  const settle = setTimeout(() => {
    if (placed || document.querySelector('dialog[open]')) return;
    placed = true;
    focusTransient(main, { preventScroll: true });
  }, HEADING_WAIT_MS);
  const observer = new MutationObserver(place);
  const stop = (): void => {
    clearTimeout(settle);
    clearTimeout(end);
    observer.disconnect();
    window.removeEventListener('keydown', stop, true);
    window.removeEventListener('pointerdown', stop, true);
    if (stopHeadingFocus === stop) stopHeadingFocus = null;
  };
  const end = setTimeout(stop, HEADING_WATCH_MS);
  stopHeadingFocus = stop;
  observer.observe(main, { childList: true, subtree: true });
  window.addEventListener('keydown', stop, true);
  window.addEventListener('pointerdown', stop, true);
  void tick().then(() => {
    if (stopHeadingFocus === stop) place();
  });
}

class Router {
  path = $state(location.pathname);
  search = $state(location.search);
  /** Whether the page was reached by a link or back/forward, not by loading its address. */
  arrived = $state(false);
  route: Route = $derived(matchRoute(this.path));

  constructor() {
    window.addEventListener('popstate', () => this.sync(false));
  }

  private sync(replace: boolean): void {
    const toHeading = focusesPageHeading(this.path, location, replace);
    if (location.pathname !== this.path) this.arrived = toHeading;
    this.path = location.pathname;
    this.search = location.search;
    if (toHeading) focusPageHeading();
  }

  /** The current path with its query string. */
  get here(): string {
    return this.path + this.search;
  }

  param(name: string): string | null {
    return new URLSearchParams(this.search).get(name);
  }

  /** Goes to an address; a replace keeps the entry's history state, a push sets `state`. */
  navigate(to: string, opts: { replace?: boolean; state?: unknown } = {}): void {
    const url = new URL(to, location.origin);
    const target = url.pathname + url.search + url.hash;
    if (target === this.here + location.hash) return;
    if (opts.replace) history.replaceState(history.state, '', target);
    else history.pushState(opts.state ?? null, '', target);
    const pageChanged = url.pathname !== this.path;
    this.sync(opts.replace ?? false);
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

/** How many entries back the page is without its modal issue; 0 when unknown. */
function issueDepth(): number {
  const d = (history.state as { issueDepth?: unknown } | null)?.issueDepth;
  return typeof d === 'number' ? d : 0;
}

/** Shows an issue in the modal over the current page, in a new history entry unless replacing. */
export function showIssue(key: string, opts: { replace?: boolean } = {}): void {
  const to = router.path + withModalIssue(router.search, key);
  if (opts.replace) return router.navigate(to, { replace: true });
  const depth = modalIssue(router.search) ? issueDepth() : 0;
  const known = depth > 0 || !modalIssue(router.search);
  router.navigate(to, { state: known ? { issueDepth: depth + 1 } : null });
}

let closing = false;

/** Closes the modal: back to the entry it opened over, or the same page without it. */
export function closeIssue(): void {
  // history.go lands later: a second close meanwhile would go back further.
  if (closing) return;
  const depth = issueDepth();
  if (modalIssue(router.search) && depth > 0) {
    closing = true;
    window.addEventListener('popstate', () => (closing = false), { once: true });
    history.go(-depth);
  } else router.navigate(router.path + withModalIssue(router.search, null), { replace: true });
}

/** A plain left click: no modifier key, not the middle button. */
export function isPlainClick(e: MouseEvent): boolean {
  return e.button === 0 && !e.metaKey && !e.ctrlKey && !e.shiftKey && !e.altKey;
}

/**
 * Document-level click handler: plain left clicks on same-site links are
 * routed in place; anything else (new tab, download, API file) is left to
 * the browser.
 */
export function interceptLinks(e: MouseEvent): void {
  if (e.defaultPrevented || !isPlainClick(e)) return;
  const a = (e.target as Element | null)?.closest?.('a');
  if (!a || !a.href || a.target === '_blank' || a.hasAttribute('download')) return;
  // eslint-disable-next-line svelte/prefer-svelte-reactivity -- a throwaway value, not state
  const url = new URL(a.href);
  if (!isAppLink(url, location.origin)) return;
  e.preventDefault();
  router.navigate(url.pathname + url.search + url.hash);
}
