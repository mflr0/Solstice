// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest';
import { flushSync, mount, tick, unmount } from 'svelte';

import type { BoardCard, BoardDetail } from '../../src/core/board-types.js';
import { router } from '../../src/lib/router.svelte.js';
import BoardPage from '../../src/ui/pages/BoardPage.svelte';

const status = { id: '1', name: 'To Do', category: 'todo' } as const;

const task = { id: '1', name: 'Task', subtask: false };
const bug = { id: '2', name: 'Bug', subtask: false };

function card(key: string, column: number, lane: number, type = task): BoardCard {
  return {
    key,
    summary: `Summary ${key}`,
    type,
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
    lane,
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
    swimlanes: 'custom',
    estimate: '',
    estimateField: null,
    doneCutoff: null,
    kanbanBacklog: false,
  },
  projects: [],
  columns: [
    { id: 10, name: 'To Do', min: null, max: null, statuses: [status] },
    { id: 20, name: 'Done', min: null, max: null, statuses: [status] },
  ],
  quickFilters: [],
  swimlanes: [
    { id: 1, name: 'Urgent', query: null },
    { id: 2, name: 'Later', query: null },
  ],
  sprints: [],
  estimateFields: [],
  shares: [],
  can: { manage: false, share: false },
};

vi.mock('../../src/lib/api.js', () => ({
  api: {
    board: () => Promise.resolve(detail),
    boardCards: () =>
      Promise.resolve({
        cards: [card('T-1', 10, 1), card('T-2', 20, 2), card('T-3', 10, 1, bug)],
        sprint: null,
        truncated: false,
      }),
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

let component: ReturnType<typeof mount> | undefined;

afterEach(() => {
  if (component) unmount(component);
  component = undefined;
  router.setParams({ type: null });
});

async function settle() {
  for (let i = 0; i < 5; i++) await tick();
  flushSync();
}

const shownKeys = () =>
  [...document.querySelectorAll<HTMLElement>('.board-card')].map((el) => el.dataset.key);
const columnCount = (id: number) =>
  document.querySelector(`#col-${id} + .board-column-count [aria-hidden]`)?.textContent;

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

  it('shows only the cards of a picked type, and counts them', async () => {
    component = mount(BoardPage, { target: document.body, props: { boardId: 1 } });
    await settle();
    expect(shownKeys()).toEqual(['T-1', 'T-3', 'T-2']);
    expect(columnCount(10)).toBe('2');

    const typeButton = [...document.querySelectorAll<HTMLButtonElement>('.filter-button')].find(
      (b) => b.textContent?.includes('Type'),
    );
    typeButton?.click();
    flushSync();
    const bugBox = [...document.querySelectorAll<HTMLLabelElement>('.card-option')]
      .find((l) => l.textContent?.includes('Bug'))
      ?.querySelector('input');
    bugBox?.click();
    await settle();

    expect(router.param('type')).toBe('2');
    expect(shownKeys()).toEqual(['T-3']);
    expect(columnCount(10)).toBe('1');
    expect(columnCount(20)).toBe('0');
    expect(typeButton?.textContent).toContain('Bug');
  });
});
