// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { touchDrag, type TouchDragHandlers } from '../../src/lib/touch-drag.js';

let board: HTMLElement;
let action: ReturnType<typeof touchDrag>;
let calls: string[];

beforeEach(() => {
  vi.useFakeTimers();
  document.body.innerHTML =
    '<div class="board"><article class="board-card" data-key="T-1"><a href="#">T-1</a><button>menu</button></article></div>';
  board = document.querySelector('.board') as HTMLElement;
  // Wide enough that no finger here nears an edge: no auto-scroll.
  board.getBoundingClientRect = () => new DOMRect(0, 0, 1000, 600);
  calls = [];
  const handlers: TouchDragHandlers = {
    start: (el) => (calls.push(`start ${el.dataset.key}`), true),
    move: (x, y) => calls.push(`move ${x},${y}`),
    drop: (x, y) => calls.push(`drop ${x},${y}`),
    cancel: () => calls.push('cancel'),
  };
  action = touchDrag(board, handlers);
});

afterEach(() => {
  action.destroy();
  vi.useRealTimers();
});

function touch(type: string, target: Element, x: number, y: number): Event {
  const e = new Event(type, { bubbles: true, cancelable: true });
  const point = { identifier: 1, clientX: x, clientY: y, target };
  Object.assign(e, { changedTouches: [point], touches: type.endsWith('end') ? [] : [point] });
  target.dispatchEvent(e);
  return e;
}

const link = () => document.querySelector('.board-card a') as Element;

describe('touchDrag', () => {
  it('drags a card held still', () => {
    touch('touchstart', link(), 100, 100);
    vi.advanceTimersByTime(400);
    expect(calls).toEqual(['start T-1']);
    expect(document.querySelector('.touch-ghost')).not.toBeNull();
    const move = touch('touchmove', link(), 300, 120);
    expect(move.defaultPrevented).toBe(true);
    const end = touch('touchend', link(), 300, 120);
    expect(end.defaultPrevented).toBe(true);
    expect(calls).toEqual(['start T-1', 'move 300,120', 'drop 300,120']);
    expect(document.querySelector('.touch-ghost')).toBeNull();
  });

  it('leaves a swipe to scrolling', () => {
    touch('touchstart', link(), 100, 100);
    const move = touch('touchmove', link(), 100, 160);
    vi.advanceTimersByTime(400);
    expect(move.defaultPrevented).toBe(false);
    expect(calls).toEqual([]);
  });

  it('leaves a tap to the link', () => {
    touch('touchstart', link(), 100, 100);
    const end = touch('touchend', link(), 100, 100);
    vi.advanceTimersByTime(400);
    expect(end.defaultPrevented).toBe(false);
    expect(calls).toEqual([]);
  });

  it('leaves the menu button alone', () => {
    touch('touchstart', document.querySelector('.board-card button') as Element, 100, 100);
    vi.advanceTimersByTime(400);
    expect(calls).toEqual([]);
  });

  it('gives up when the browser cancels the touch', () => {
    touch('touchstart', link(), 100, 100);
    vi.advanceTimersByTime(400);
    touch('touchcancel', link(), 100, 100);
    expect(calls).toEqual(['start T-1', 'cancel']);
    expect(document.querySelector('.touch-ghost')).toBeNull();
  });
});
