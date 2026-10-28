/** Small DOM actions shared by components. */

/** Calls handler on a pointer press outside the node, or on Escape. */
export function outside(node: HTMLElement, handler: () => void) {
  let current = handler;
  const onPointer = (e: PointerEvent) => {
    if (!node.contains(e.target as Node)) current();
  };
  const onKey = (e: KeyboardEvent) => {
    if (e.key === 'Escape') {
      e.stopPropagation();
      current();
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
