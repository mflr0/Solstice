// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { outside, stackWhenCrowded } from '../../src/lib/actions.js';

describe('outside', () => {
  let box: HTMLElement;
  let elsewhere: HTMLButtonElement;
  let action: ReturnType<typeof outside>;
  let closed = 0;

  /** A popover's root: its opener, and a panel with a field in it. */
  function mountBox(opener: string) {
    box = document.createElement('div');
    box.innerHTML = `${opener}<div class="panel"><input class="inner" /></div>`;
    elsewhere = document.createElement('button');
    document.body.append(box, elsewhere);
    closed = 0;
    action = outside(box, () => closed++);
  }

  function escape(from: Element) {
    from.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
  }

  afterEach(() => {
    action.destroy();
    box.remove();
    elsewhere.remove();
  });

  it('gives focus back to the open button on Escape', () => {
    mountBox('<button class="opener" aria-expanded="true"></button>');
    const inner = box.querySelector<HTMLElement>('.inner')!;
    inner.focus();
    escape(inner);
    expect(closed).toBe(1);
    expect(document.activeElement).toBe(box.querySelector('.opener'));
  });

  it('moves no focus on a press outside', () => {
    mountBox('<button class="opener" aria-expanded="true"></button>');
    elsewhere.focus();
    elsewhere.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true }));
    expect(closed).toBe(1);
    expect(document.activeElement).toBe(elsewhere);
  });

  it('leaves focus alone on Escape when nothing is open', () => {
    mountBox('<button class="opener" aria-expanded="false"></button>');
    const inner = box.querySelector<HTMLElement>('.inner')!;
    inner.focus();
    escape(inner);
    expect(document.activeElement).toBe(inner);
  });

  it('leaves focus in a search field on Escape', () => {
    mountBox('<input class="opener" role="combobox" aria-expanded="true" />');
    const field = box.querySelector<HTMLElement>('.opener')!;
    field.focus();
    escape(field);
    expect(closed).toBe(1);
    expect(document.activeElement).toBe(field);
  });
});

describe('stackWhenCrowded', () => {
  // jsdom lays nothing out: the nav needs 300 px, and on one row the tools
  // take 200 px of the bar's width.
  let barWidth = 0;
  let onResize: () => void = () => {};
  let frames: FrameRequestCallback[] = [];
  let bar: HTMLElement;

  beforeEach(() => {
    vi.stubGlobal(
      'ResizeObserver',
      class {
        constructor(cb: () => void) {
          onResize = cb;
        }
        observe() {}
        disconnect() {}
      },
    );
    vi.stubGlobal('requestAnimationFrame', (cb: FrameRequestCallback) => frames.push(cb));
    vi.stubGlobal('cancelAnimationFrame', () => {});
    bar = document.createElement('header');
    bar.innerHTML = '<nav class="topbar-nav"><a>Mon travail</a><a>Recherche</a></nav>';
    const nav = bar.querySelector('nav')!;
    Object.defineProperty(nav, 'scrollWidth', { get: () => 300 });
    Object.defineProperty(nav, 'clientWidth', {
      get: () => Math.min(300, bar.classList.contains('is-stacked') ? barWidth : barWidth - 200),
    });
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    frames = [];
  });

  function resize(width: number) {
    barWidth = width;
    onResize();
    const run = frames;
    frames = [];
    for (const cb of run) cb(0);
  }

  it('stacks the nav when it does not fit beside the tools', () => {
    barWidth = 390;
    stackWhenCrowded(bar);
    expect(bar.classList.contains('is-stacked')).toBe(true);
  });

  it('keeps one row when the nav fits', () => {
    barWidth = 900;
    stackWhenCrowded(bar);
    expect(bar.classList.contains('is-stacked')).toBe(false);
  });

  it('measures on one row, not on the row the nav has to itself', () => {
    barWidth = 390;
    stackWhenCrowded(bar);
    resize(450);
    expect(bar.classList.contains('is-stacked')).toBe(true);
    resize(500);
    expect(bar.classList.contains('is-stacked')).toBe(false);
    resize(320);
    expect(bar.classList.contains('is-stacked')).toBe(true);
  });

  it('leaves one row when destroyed', () => {
    barWidth = 390;
    const action = stackWhenCrowded(bar);
    action.destroy?.();
    expect(bar.classList.contains('is-stacked')).toBe(false);
  });
});
