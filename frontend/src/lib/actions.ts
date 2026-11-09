/** Small DOM actions shared by components. */

import { shortcutFor } from '../core/shortcuts.js';
import { appearance, ignoreShortcut, mac } from './ui.svelte.js';

/**
 * Calls handler on a pointer press outside the node, or on Escape. Escape also
 * gives focus back to the node's open button, if it has one.
 */
export function outside(node: HTMLElement, handler: () => void) {
  let current = handler;
  const onPointer = (e: PointerEvent) => {
    if (!node.contains(e.target as Node)) current();
  };
  const onKey = (e: KeyboardEvent) => {
    if (e.key === 'Escape') {
      e.stopPropagation();
      const opener = node.querySelector<HTMLElement>(':scope > button[aria-expanded="true"]');
      current();
      opener?.focus();
    }
  };
  document.addEventListener('pointerdown', onPointer, true);
  node.addEventListener('keydown', onKey);
  return {
    update(next: () => void) {
      current = next;
    },
    destroy() {
      document.removeEventListener('pointerdown', onPointer, true);
      node.removeEventListener('keydown', onKey);
    },
  };
}

/**
 * Keyboard for a menu opened by the [aria-haspopup] button beside it: focus
 * starts on the checked item, else the first; arrows, Home and End move
 * between items; Escape goes back to the button (outside closes the menu);
 * Tab leaves the menu and closes it.
 */
export function menu(node: HTMLElement, close: () => void) {
  let current = close;
  const trigger = node.parentElement?.querySelector<HTMLElement>('[aria-haspopup]');
  const items = () => [...node.querySelectorAll<HTMLElement>('[role^="menuitem"]')];
  // Tab goes past the menu, not from item to item.
  for (const item of items()) item.tabIndex = -1;
  queueMicrotask(() =>
    (node.querySelector<HTMLElement>('[aria-checked="true"]') ?? items()[0])?.focus(),
  );

  const onKey = (e: KeyboardEvent) => {
    const list = items();
    const at = list.indexOf(document.activeElement as HTMLElement);
    let next = -1;
    if (e.key === 'ArrowDown') next = (at + 1) % list.length;
    else if (e.key === 'ArrowUp') next = (at - 1 + list.length) % list.length;
    else if (e.key === 'Home') next = 0;
    else if (e.key === 'End') next = list.length - 1;
    else if (e.key === 'Escape') trigger?.focus();
    if (next >= 0) {
      e.preventDefault();
      list[next]?.focus();
    }
  };
  // Not on the button: pressing it moves focus there first, and then toggles.
  const onFocusOut = (e: FocusEvent) => {
    const to = e.relatedTarget as Node | null;
    if (to && !node.contains(to) && to !== trigger) current();
  };
  node.addEventListener('keydown', onKey);
  node.addEventListener('focusout', onFocusOut);
  return {
    update(next: () => void) {
      current = next;
    },
    destroy() {
      node.removeEventListener('keydown', onKey);
      node.removeEventListener('focusout', onFocusOut);
    },
  };
}

/**
 * Keyboard for a tablist that selects as focus moves. The selected tab
 * (aria-selected, set by the page) is the only Tab stop; Left and Right, swapped
 * right to left, move with wrap; Home and End go to the ends. Calls select with
 * the new tab.
 */
export function tabs(node: HTMLElement, select: (tab: HTMLElement) => void) {
  let current = select;
  const list = () => [...node.querySelectorAll<HTMLElement>('[role="tab"]')];
  const rove = () => {
    for (const tab of list()) tab.tabIndex = tab.getAttribute('aria-selected') === 'true' ? 0 : -1;
  };
  rove();
  const watch = new MutationObserver(rove);
  watch.observe(node, { subtree: true, attributeFilter: ['aria-selected'] });

  const onKey = (e: KeyboardEvent) => {
    // Alt+Left is the browser's back.
    if (e.altKey || e.ctrlKey || e.metaKey || e.shiftKey) return;
    const all = list();
    const at = all.indexOf(e.target as HTMLElement);
    if (at < 0) return;
    const step = getComputedStyle(node).direction === 'rtl' ? -1 : 1;
    let next = -1;
    if (e.key === 'ArrowRight') next = (at + step + all.length) % all.length;
    else if (e.key === 'ArrowLeft') next = (at - step + all.length) % all.length;
    else if (e.key === 'Home') next = 0;
    else if (e.key === 'End') next = all.length - 1;
    const tab = all[next];
    if (!tab) return;
    e.preventDefault();
    tab.focus();
    current(tab);
  };
  node.addEventListener('keydown', onKey);
  return {
    update(next: (tab: HTMLElement) => void) {
      current = next;
    },
    destroy() {
      watch.disconnect();
      node.removeEventListener('keydown', onKey);
    },
  };
}

/** Options of listKeys. */
export interface ListKeysOptions {
  /** Selects the rows inside the node, in reading order. */
  rows: string;
  /** Called on o or Enter with the row that holds the focus. */
  open: (row: HTMLElement) => void;
  /** Called on x with the row that holds the focus; without it, x does nothing. */
  select?: ((row: HTMLElement) => void) | undefined;
}

const LIST = 'data-list-keys';

/**
 * The search results' keys on a list of rows: j and k move the focus from row
 * to row (j from nowhere goes to the first), o or Enter opens, x selects.
 * Rows inside [hidden] are skipped. A list inside another leaves the keys to
 * it; of several lists, the one holding the focus takes them, else the first.
 */
export function listKeys(node: HTMLElement, options: ListKeysOptions) {
  let current = options;
  node.setAttribute(LIST, '');
  const rows = () =>
    [...node.querySelectorAll<HTMLElement>(current.rows)].filter((r) => !r.closest('[hidden]'));
  // Reachable by the keys, not by Tab.
  const mark = () => {
    for (const row of node.querySelectorAll<HTMLElement>(current.rows))
      if (!row.hasAttribute('tabindex')) row.tabIndex = -1;
  };
  mark();
  const watch = new MutationObserver(mark);
  watch.observe(node, { childList: true, subtree: true });

  const owner = () => {
    const lists = [...document.querySelectorAll(`[${LIST}]`)].filter(
      (l) => !l.parentElement?.closest(`[${LIST}]`),
    );
    return lists.find((l) => l.contains(document.activeElement)) ?? lists[0];
  };

  const onKey = (e: KeyboardEvent) => {
    if (ignoreShortcut(e) || owner() !== node) return;
    const action = shortcutFor(e, 'results', appearance.shortcuts, appearance.bindings, mac);
    // A list has no columns to move between: the arrows keep scrolling it.
    if (!action || action === 'left' || action === 'right') return;
    const all = rows();
    const at = all.findIndex((r) => r.contains(document.activeElement));
    if (action === 'next' || action === 'previous') {
      if (!all.length) return;
      e.preventDefault();
      const step = action === 'next' ? 1 : -1;
      const to = at < 0 ? 0 : Math.min(Math.max(at + step, 0), all.length - 1);
      all[to]?.focus();
      return;
    }
    const row = all[at];
    // Enter on a link or a checkbox in the row keeps its own meaning.
    if (!row || (e.key === 'Enter' && document.activeElement !== row)) return;
    if (action === 'open') {
      e.preventDefault();
      current.open(row);
    } else if (action === 'select' && current.select) {
      e.preventDefault();
      current.select(row);
    }
  };
  addEventListener('keydown', onKey);
  return {
    update(next: ListKeysOptions) {
      current = next;
      mark();
    },
    destroy() {
      watch.disconnect();
      removeEventListener('keydown', onKey);
      node.removeAttribute(LIST);
    },
  };
}

/** Options of layer. */
export interface LayerOptions {
  /** What the node opens against, usually the button that opens it. */
  anchor: HTMLElement | undefined;
  /** Makes the node at least as wide as the anchor. */
  matchWidth?: boolean | undefined;
  /** Called on a press outside the node and the anchor. */
  onoutside?: (() => void) | undefined;
}

/**
 * Shows a popover="manual" node in the top layer, so neither a dialog's
 * scrolling nor the window's edge clips it: fixed under its anchor, or above
 * it when there is more room there. It follows the anchor on scroll and
 * resize, and is placed again when its content grows. Escape pressed inside
 * it does not close a dialog around it.
 */
export function layer(node: HTMLElement, options: LayerOptions) {
  let current = options;
  let up = false;
  let frame = 0;
  node.showPopover?.();
  node.style.position = 'fixed';
  node.style.inset = 'auto';
  node.style.margin = '0';
  const place = () => {
    frame = 0;
    const { anchor, matchWidth } = current;
    if (!anchor) return;
    const r = anchor.getBoundingClientRect();
    const width = document.documentElement.clientWidth;
    const height = document.documentElement.clientHeight;
    if (matchWidth) node.style.minWidth = `${r.width}px`;
    node.style.maxHeight = '';
    // The stylesheet's own limit, if any, still holds.
    const cap = parseFloat(getComputedStyle(node).maxHeight) || Infinity;
    node.style.left = `${Math.max(8, Math.min(r.left, width - 8 - node.offsetWidth))}px`;
    const below = height - r.bottom - 12;
    const above = r.top - 12;
    // It only changes side when it no longer fits where it is, so a list
    // that shrinks while filtering stays put.
    const [here, there] = up ? [above, below] : [below, above];
    if (node.offsetHeight > here && there > here) up = !up;
    if (up) {
      node.style.top = 'auto';
      node.style.bottom = `${height - r.top + 4}px`;
      node.style.maxHeight = `${Math.min(cap, above)}px`;
    } else {
      node.style.bottom = 'auto';
      node.style.top = `${r.bottom + 4}px`;
      node.style.maxHeight = `${Math.min(cap, below)}px`;
    }
  };
  place();
  const onPointer = (e: PointerEvent) => {
    const target = e.target as Node;
    if (!node.contains(target) && !current.anchor?.contains(target)) current.onoutside?.();
  };
  // The dialog closes on Escape's default action, whatever stops its propagation.
  const onKey = (e: KeyboardEvent) => {
    if (e.key === 'Escape') e.preventDefault();
  };
  const onScroll = (e: Event) => {
    if (!node.contains(e.target as Node)) place();
  };
  // In the next frame: placing changes the node's size inside the observer's callback.
  const watch =
    typeof ResizeObserver === 'function'
      ? new ResizeObserver(() => {
          frame ||= requestAnimationFrame(place);
        })
      : undefined;
  watch?.observe(node);
  node.addEventListener('keydown', onKey);
  document.addEventListener('pointerdown', onPointer, true);
  addEventListener('scroll', onScroll, { capture: true, passive: true });
  addEventListener('resize', place);
  return {
    update(next: LayerOptions) {
      current = next;
    },
    destroy() {
      watch?.disconnect();
      node.removeEventListener('keydown', onKey);
      if (frame) cancelAnimationFrame(frame);
      document.removeEventListener('pointerdown', onPointer, true);
      removeEventListener('scroll', onScroll, { capture: true });
      removeEventListener('resize', place);
    },
  };
}

/**
 * Focuses the target if focus fell to the page when its holder left; a user
 * who already moved on keeps their place.
 */
export function refocus(target: HTMLElement | null | undefined): void {
  if (document.activeElement === document.body) target?.focus();
}

/**
 * Refocuses the first of these ids on the page, in the next task: by then, a
 * dialog whose action just ended has closed (a closed dialog returns focus to
 * its opener, and a modal one keeps the page inert) and the list has rendered.
 */
export function refocusLater(ids: readonly string[]): void {
  setTimeout(() => {
    const target = ids.map((id) => document.getElementById(id)).find(Boolean);
    refocus(target);
  });
}

/** Focuses the node once mounted. */
export function autofocus(node: HTMLElement) {
  queueMicrotask(() => node.focus());
}

type SidesHandler = (left: boolean, right: boolean) => void;

/**
 * Calls handler with whether a sideways scroller hides content on its left
 * and on its right: once mounted, then on each scroll and resize.
 */
export function hiddenSides(node: HTMLElement, handler: SidesHandler) {
  let current = handler;
  const measure = () => {
    const rest = node.scrollWidth - node.clientWidth - node.scrollLeft;
    current(node.scrollLeft >= 1, rest >= 1);
  };
  const watch = new ResizeObserver(measure);
  watch.observe(node);
  node.addEventListener('scroll', measure, { passive: true });
  return {
    update(next: SidesHandler) {
      current = next;
    },
    destroy() {
      watch.disconnect();
      node.removeEventListener('scroll', measure);
    },
  };
}

/**
 * Keeps a CSS variable on the document at the height of a bar pinned to an edge
 * of the screen, 0 when the skin doesn't pin it.
 */
export function pinnedHeight(node: HTMLElement, name: string) {
  const root = document.documentElement;
  const measure = () => {
    const pinned = ['sticky', 'fixed'].includes(getComputedStyle(node).position);
    root.style.setProperty(name, pinned ? `${node.offsetHeight}px` : '0px');
  };
  measure();
  const watch = new ResizeObserver(measure);
  watch.observe(node);
  return {
    destroy() {
      watch.disconnect();
      root.style.removeProperty(name);
    },
  };
}

/**
 * Keeps is-stacked on a top bar while its .topbar-nav does not fit on one row,
 * so the stylesheet can give the nav a row of its own.
 */
export function stackWhenCrowded(node: HTMLElement) {
  const nav = node.querySelector<HTMLElement>('.topbar-nav');
  if (!nav) return {};
  let frame = 0;
  // Always measured on one row: a stacked nav has room by definition.
  const measure = () => {
    frame = 0;
    node.classList.remove('is-stacked');
    node.classList.toggle('is-stacked', nav.scrollWidth - nav.clientWidth >= 1);
  };
  // In the next frame: toggling the class inside the observer's callback would
  // resize what it observes and make the browser report a loop.
  const schedule = () => {
    frame ||= requestAnimationFrame(measure);
  };
  measure();
  const watch = new ResizeObserver(schedule);
  watch.observe(node);
  watch.observe(nav);
  for (const link of nav.children) watch.observe(link);
  return {
    destroy() {
      watch.disconnect();
      cancelAnimationFrame(frame);
      node.classList.remove('is-stacked');
    },
  };
}
