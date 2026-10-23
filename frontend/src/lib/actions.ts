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
