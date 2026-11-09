// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { listKeys, outside, refocusLater, stackWhenCrowded, tabs } from '../../src/lib/actions.js';

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

describe('refocusLater', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    document.body.innerHTML = '<button id="b"></button><button id="c"></button>';
  });

  afterEach(() => {
    vi.useRealTimers();
    document.body.innerHTML = '';
  });

  it('waits for the next task, then focuses the first id on the page', () => {
    refocusLater(['gone', 'b', 'c']);
    expect(document.activeElement).toBe(document.body);
    vi.runAllTimers();
    expect(document.activeElement?.id).toBe('b');
  });

  it('leaves focus alone when it did not fall to the page', () => {
    document.getElementById('c')?.focus();
    refocusLater(['b']);
    vi.runAllTimers();
    expect(document.activeElement?.id).toBe('c');
  });

  it('does nothing when no id is on the page', () => {
    refocusLater(['gone']);
    vi.runAllTimers();
    expect(document.activeElement).toBe(document.body);
  });
});

describe('tabs', () => {
  let list: HTMLElement;
  let action: ReturnType<typeof tabs>;
  let selected: string[];

  function tab(name: string) {
    return list.querySelector<HTMLElement>(`#${name}`)!;
  }

  function select(name: string) {
    for (const t of list.children) t.setAttribute('aria-selected', String(t.id === name));
  }

  function press(key: string, init: KeyboardEventInit = {}) {
    const event = new KeyboardEvent('keydown', { key, bubbles: true, cancelable: true, ...init });
    (document.activeElement as HTMLElement).dispatchEvent(event);
    return event;
  }

  beforeEach(() => {
    list = document.createElement('div');
    list.innerHTML = ['a', 'b', 'c']
      .map((id) => `<button role="tab" id="${id}" aria-selected="${id === 'a'}"></button>`)
      .join('');
    document.body.append(list);
    selected = [];
    action = tabs(list, (t) => {
      selected.push(t.id);
      select(t.id);
    });
    tab('a').focus();
  });

  afterEach(() => {
    action.destroy();
    list.remove();
  });

  it('leaves one Tab stop, the selected tab, and follows the selection', async () => {
    expect([...list.children].map((t) => (t as HTMLElement).tabIndex)).toEqual([0, -1, -1]);
    select('c');
    await Promise.resolve();
    expect([...list.children].map((t) => (t as HTMLElement).tabIndex)).toEqual([-1, -1, 0]);
  });

  it('moves and selects with Left and Right, with wrap', () => {
    expect(press('ArrowRight').defaultPrevented).toBe(true);
    expect(document.activeElement).toBe(tab('b'));
    press('ArrowLeft');
    press('ArrowLeft');
    expect(document.activeElement).toBe(tab('c'));
    press('ArrowRight');
    expect(document.activeElement).toBe(tab('a'));
    expect(selected).toEqual(['b', 'a', 'c', 'a']);
  });

  it('goes to the ends with Home and End', () => {
    press('End');
    expect(document.activeElement).toBe(tab('c'));
    press('Home');
    expect(document.activeElement).toBe(tab('a'));
    expect(selected).toEqual(['c', 'a']);
  });

  it('swaps Left and Right, right to left', () => {
    list.style.direction = 'rtl';
    press('ArrowLeft');
    expect(document.activeElement).toBe(tab('b'));
    press('ArrowRight');
    expect(document.activeElement).toBe(tab('a'));
  });

  it('leaves other keys and browser shortcuts alone', () => {
    expect(press('ArrowRight', { altKey: true }).defaultPrevented).toBe(false);
    expect(press('Tab').defaultPrevented).toBe(false);
    expect(press('a').defaultPrevented).toBe(false);
    expect(selected).toEqual([]);
  });

  it('calls the latest handler after an update', () => {
    const next = vi.fn();
    action.update(next);
    press('ArrowRight');
    expect(next).toHaveBeenCalledWith(tab('b'));
    expect(selected).toEqual([]);
  });
});

describe('listKeys', () => {
  let list: HTMLElement;
  let field: HTMLInputElement;
  let action: ReturnType<typeof listKeys>;
  let opened: string[];
  let picked: string[];

  function mount(withSelect: boolean) {
    list = document.createElement('ul');
    list.innerHTML = ['A-1', 'A-2', 'A-3']
      .map((k) => `<li data-key="${k}"><a href="#${k}">${k}</a></li>`)
      .join('');
    field = document.createElement('input');
    document.body.append(field, list);
    opened = [];
    picked = [];
    action = listKeys(list, {
      rows: 'li',
      open: (row) => opened.push(row.dataset.key!),
      select: withSelect ? (row) => picked.push(row.dataset.key!) : undefined,
    });
  }

  const row = (i: number) => list.querySelectorAll('li')[i]!;

  function press(key: string, from: Element = document.activeElement ?? document.body) {
    const e = new KeyboardEvent('keydown', { key, bubbles: true, cancelable: true });
    from.dispatchEvent(e);
    return e;
  }

  afterEach(() => {
    action.destroy();
    list.remove();
    field.remove();
    (document.activeElement as HTMLElement | null)?.blur();
  });

  it('makes the rows focusable by the keys only', () => {
    mount(false);
    expect(row(0).getAttribute('tabindex')).toBe('-1');
  });

  it('focuses the first row on j from nowhere', () => {
    mount(false);
    press('j');
    expect(document.activeElement).toBe(row(0));
    press('j');
    expect(document.activeElement).toBe(row(1));
  });

  it('stays on the first row on k', () => {
    mount(false);
    row(0).focus();
    press('k');
    expect(document.activeElement).toBe(row(0));
  });

  it('opens the focused row on o and on Enter', () => {
    mount(false);
    row(1).focus();
    press('o');
    press('Enter');
    expect(opened).toEqual(['A-2', 'A-2']);
  });

  it('leaves Enter on a link in the row to the link', () => {
    mount(false);
    row(1).querySelector('a')!.focus();
    expect(press('Enter').defaultPrevented).toBe(false);
    expect(opened).toEqual([]);
  });

  it('selects on x only with select', () => {
    mount(false);
    row(2).focus();
    expect(press('x').defaultPrevented).toBe(false);
    action.destroy();
    list.remove();
    field.remove();
    mount(true);
    row(2).focus();
    press('x');
    expect(picked).toEqual(['A-3']);
  });

  it('does nothing while typing in a field', () => {
    mount(false);
    field.focus();
    press('j', field);
    press('o', field);
    expect(document.activeElement).toBe(field);
    expect(opened).toEqual([]);
  });

  it('leaves the keys to the list around it', () => {
    mount(false);
    const inner = listKeys(list.querySelector('li')!, { rows: 'a', open: () => {} });
    press('j');
    expect(document.activeElement).toBe(row(0));
    inner.destroy();
  });
});
