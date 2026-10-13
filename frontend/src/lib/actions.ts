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
