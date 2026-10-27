// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest';
import { flushSync, mount, tick, unmount } from 'svelte';

import type { BoardCard, BoardDetail } from '../../src/core/board-types.js';
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
    query: '',
    subQuery: null,
    swimlanes: 'custom',
    estimate: '',
    estimateField: null,
    doneCutoff: null,
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
  can: { manage: false },
};

vi.mock('../../src/lib/api.js', () => ({
  api: {
    board: () => Promise.resolve(detail),
    boardCards: () =>
      Promise.resolve({
        cards: [card('T-1', 10, 1), card('T-2', 20, 2)],
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
});

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
});
