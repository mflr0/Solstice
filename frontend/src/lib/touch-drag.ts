import { edgeScroll } from '../core/board.js';

/** What a touch drag reports to the page; x and y are viewport coordinates. */
export interface TouchDragHandlers {
  /** A card was held long enough; false refuses the drag. */
  start(card: HTMLElement): boolean;
  move(x: number, y: number): void;
  drop(x: number, y: number): void;
  cancel(): void;
}

// Long enough to tell a hold from a swipe, short enough to beat the
// browser's own long-press menu.
const HOLD_MS = 350;
const SLOP_PX = 10;

/**
 * Drags the node's `.board-card`s by touch: hold a card still, then move
 * it. A copy follows the finger and the node scrolls sideways (the page
 * up and down) when the finger nears an edge. A swipe still scrolls and a
 * tap still opens the card.
 */
export function touchDrag(node: HTMLElement, handlers: TouchDragHandlers) {
  let current = handlers;
  let timer = 0;
  let frame = 0;
  let touch: number | null = null;
  let active = false;
  let startX = 0;
  let startY = 0;
  let x = 0;
  let y = 0;
  let offsetX = 0;
  let offsetY = 0;
  let ghost: HTMLElement | null = null;

  const find = (e: TouchEvent) => [...e.changedTouches].find((t) => t.identifier === touch);

  function reset() {
    clearTimeout(timer);
    cancelAnimationFrame(frame);
    ghost?.remove();
    ghost = null;
    touch = null;
    active = false;
  }

  function lift(card: HTMLElement) {
    if (touch === null || !current.start(card)) return reset();
    active = true;
    const r = card.getBoundingClientRect();
    offsetX = startX - r.left;
    offsetY = startY - r.top;
    ghost = card.cloneNode(true) as HTMLElement;
    ghost.classList.add('touch-ghost');
    ghost.removeAttribute('data-key');
    ghost.setAttribute('aria-hidden', 'true');
    ghost.style.width = `${r.width}px`;
    document.body.append(ghost);
    place();
    navigator.vibrate?.(10);
    frame = requestAnimationFrame(scroll);
  }

  function place() {
    if (ghost) ghost.style.translate = `${x - offsetX}px ${y - offsetY}px`;
  }

  function scroll() {
    const r = node.getBoundingClientRect();
    const dx = edgeScroll(x, r.left, r.right);
    const dy = edgeScroll(y, 0, window.innerHeight);
    if (dx) node.scrollLeft += dx;
    if (dy) window.scrollBy(0, dy);
    if (dx || dy) current.move(x, y);
    frame = requestAnimationFrame(scroll);
  }

  function abort() {
    if (active) current.cancel();
    reset();
  }

  function onStart(e: TouchEvent) {
    if (touch !== null || e.touches.length > 1) return abort();
    const target = e.target as Element;
    const card = target.closest<HTMLElement>('.board-card');
    if (!card || !node.contains(card) || target.closest('button')) return;
    const t = e.changedTouches[0];
    if (!t) return;
    touch = t.identifier;
    startX = x = t.clientX;
    startY = y = t.clientY;
    timer = window.setTimeout(() => lift(card), HOLD_MS);
  }

  function onMove(e: TouchEvent) {
    const t = find(e);
    if (!t) return;
    x = t.clientX;
    y = t.clientY;
    if (!active) {
      if (Math.hypot(x - startX, y - startY) > SLOP_PX) reset();
      return;
    }
    e.preventDefault();
    place();
    current.move(x, y);
  }

  function onEnd(e: TouchEvent) {
    const t = find(e);
    if (!t) return;
    if (active) {
      // No click on the card's link after a drag.
      e.preventDefault();
      current.drop(t.clientX, t.clientY);
    }
    reset();
  }

  function onCancel(e: TouchEvent) {
    if (find(e)) abort();
  }

  // The browser's long-press menu, or its own drag, would take the hold over.
  function onHold(e: Event) {
    if (touch !== null) e.preventDefault();
  }

  node.addEventListener('touchstart', onStart, { passive: true });
  node.addEventListener('touchmove', onMove, { passive: false });
  node.addEventListener('touchend', onEnd);
  node.addEventListener('touchcancel', onCancel);
  node.addEventListener('contextmenu', onHold);
  node.addEventListener('dragstart', onHold, true);
  return {
    update(next: TouchDragHandlers) {
      current = next;
    },
    destroy() {
      abort();
      node.removeEventListener('touchstart', onStart);
      node.removeEventListener('touchmove', onMove);
      node.removeEventListener('touchend', onEnd);
      node.removeEventListener('touchcancel', onCancel);
      node.removeEventListener('contextmenu', onHold);
      node.removeEventListener('dragstart', onHold, true);
    },
  };
}
