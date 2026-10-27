// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { stackWhenCrowded } from '../../src/lib/actions.js';

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
