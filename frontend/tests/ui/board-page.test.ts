// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest';
import { flushSync, mount, tick, unmount } from 'svelte';

import type { BoardCard, BoardDetail } from '../../src/core/board-types.js';
import { router } from '../../src/lib/router.svelte.js';
import BoardPage from '../../src/ui/pages/BoardPage.svelte';

const status = { id: '1', name: 'To Do', category: 'todo' } as const;

function card(key: string, column: number, lane: number): BoardCard {
  return {
    key,
    summary: `Summary ${key}`,
    type: { id: '1', name: 'Task', subtask: false },
    status,
    priority: null,
    resolution: null,
    assignee: null,
    reporter: null,
    created: '2026-01-01T00:00:00Z',
    updated: '2026-01-01T00:00:00Z',
    resolved: null,
    due: null,
    confidential: false,
    epic: null,
    parent: null,
    estimate: null,
    column,
    section: lane,
    can: { create: true, edit: true },
    watching: false,
  };
}

const detail: BoardDetail = {
  board: {
    id: 1,
    name: 'Team',
    kind: 'kanban',
    filterId: 1,
    filterName: 'Filter',
    query: '',
    subQuery: null,
    sectionBy: 'custom',
    estimate: '',
    estimateField: null,
    doneCutoff: null,
    kanbanBacklog: false,
  },
  projects: [],
  columns: [
    { id: 10, name: 'To Do', statuses: [status] },
    { id: 20, name: 'Done', statuses: [status] },
  ],
  quickFilters: [],
  sections: [
    { id: 1, name: 'Urgent', query: null },
    { id: 2, name: 'Later', query: null },
  ],
  sprints: [],
  estimateFields: [],
  shares: [],
  can: { manage: false, share: false },
};

const boardCards = vi.hoisted(() => vi.fn());
const watch = vi.hoisted(() => vi.fn());

vi.mock('../../src/lib/api.js', () => ({
  api: {
    board: () => Promise.resolve(detail),
    boardCards,
    watch,
  },
  errorMessage: String,
}));

vi.stubGlobal(
  'ResizeObserver',
  class {
    observe() {}
    disconnect() {}
  },
);

boardCards.mockImplementation(() =>
  Promise.resolve({
    cards: [card('T-1', 10, 1), { ...card('T-2', 20, 2), can: { create: false, edit: true } }],
    sprint: null,
    truncated: false,
  }),
);

let component: ReturnType<typeof mount> | undefined;

afterEach(() => {
  if (component) unmount(component);
  component = undefined;
  router.setParams({ jql: null, quick: null });
  boardCards.mockClear();
});

HTMLElement.prototype.hidePopover = function () {};

/** Opens a card's menu the way the browser would, and lists it by group. */
function openMenu(key: string): { label: string; items: HTMLButtonElement[] }[] {
  const menu = document.querySelector(`.board-card[data-key="${key}"] .card-menu`);
  menu?.dispatchEvent(Object.assign(new Event('beforetoggle'), { newState: 'open' }));
  flushSync();
  return [...(menu?.querySelectorAll('[role="group"]') ?? [])].map((g) => ({
    label: g.getAttribute('aria-label') ?? '',
    items: [...g.querySelectorAll<HTMLButtonElement>('[role="menuitem"]')],
  }));
}

const labels = (items: HTMLButtonElement[]) => items.map((b) => b.textContent);

async function settle() {
  for (let i = 0; i < 5; i++) await tick();
  flushSync();
}

describe('BoardPage', () => {
  it('puts the swimlane toggles before the cards in the tab order', async () => {
    component = mount(BoardPage, { target: document.body, props: { boardId: 1 } });
    for (let i = 0; i < 5; i++) await tick();
    flushSync();
    const order = [...document.querySelectorAll('.board button, .board a')].map((el) =>
      el.classList.contains('board-lane-toggle')
        ? `lane ${el.querySelector('.board-lane-title')?.textContent}`
        : el.closest<HTMLElement>('.board-card')?.dataset.key,
    );
    expect(order.filter((o) => o?.startsWith('lane'))).toEqual(['lane Urgent', 'lane Later']);
    expect(order.indexOf('lane Later')).toBeLessThan(order.indexOf('T-1'));
  });

  it('starts the card menu with the issue entries', async () => {
    component = mount(BoardPage, { target: document.body, props: { boardId: 1 } });
    await settle();
    const groups = openMenu('T-1');
    expect(groups.map((g) => g.label).slice(0, 2)).toEqual(['Issue', 'Move to']);
    expect(labels(groups[0]?.items ?? [])).toEqual(['Duplicate…', 'Watch', 'Copy link']);
  });

  it('offers no duplicate without the right to create', async () => {
    component = mount(BoardPage, { target: document.body, props: { boardId: 1 } });
    await settle();
    expect(labels(openMenu('T-2')[0]?.items ?? [])).toEqual(['Watch', 'Copy link']);
  });

  it('turns Watch into Stop watching once clicked', async () => {
    watch.mockResolvedValue({ count: 1, me: true });
    component = mount(BoardPage, { target: document.body, props: { boardId: 1 } });
    await settle();
    openMenu('T-1')[0]
      ?.items.find((b) => b.textContent === 'Watch')
      ?.click();
    await settle();
    expect(watch).toHaveBeenCalledWith('T-1', true);
    expect(labels(openMenu('T-1')[0]?.items ?? [])).toContain('Stop watching');
  });

  it('sends the query of the filter row with the quick filters', async () => {
    router.setParams({ quick: '2', jql: 'assignee is EMPTY AND text ~ "zèbre"' });
    component = mount(BoardPage, { target: document.body, props: { boardId: 1 } });
    await settle();
    expect(boardCards).toHaveBeenLastCalledWith(1, {
      quick: '2',
      jql: 'assignee is EMPTY AND text ~ "zèbre"',
    });
    expect(document.querySelector<HTMLInputElement>('.filter-bar input')?.value).toBe('zèbre');
  });

  it('sends no query the row cannot show, and says so', async () => {
    router.setParams({ jql: 'status = Done OR assignee is EMPTY' });
    component = mount(BoardPage, { target: document.body, props: { boardId: 1 } });
    await settle();
    expect(boardCards).toHaveBeenLastCalledWith(1, { quick: undefined, jql: undefined });
    expect(document.querySelector('.notice')?.textContent).toBe(
      'Query too complex for the filters',
    );
  });
});
