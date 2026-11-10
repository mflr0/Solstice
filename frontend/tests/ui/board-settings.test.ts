// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { flushSync, mount, tick, unmount } from 'svelte';

import type { BoardDetail } from '../../src/core/board-types.js';
import { en } from '../../src/i18n/en.js';
import BoardSettingsPage from '../../src/ui/pages/BoardSettingsPage.svelte';

const status = { id: '1', name: 'To Do', category: 'todo' } as const;

const detail: BoardDetail = {
  board: {
    id: 7,
    name: 'Team',
    kind: 'scrum',
    filterId: 1,
    filterName: 'Filter',
    query: 'project = T',
    subQuery: null,
    sectionBy: 'none',
    estimate: '',
    estimateField: null,
    doneCutoff: null,
    kanbanBacklog: false,
  },
  projects: [],
  columns: [{ id: 10, name: 'To Do', statuses: [status] }],
  quickFilters: [
    { id: 1, name: 'Mine', query: 'assignee = currentUser()', description: '' },
    { id: 2, name: 'Bugs', query: 'type = Bug', description: '' },
  ],
  sections: [],
  sprints: [],
  estimateFields: [],
  shares: [],
  can: { manage: true, share: true },
};

const api = vi.hoisted(() => ({
  board: vi.fn(),
  saveBoard: vi.fn(),
  boardProjectStatuses: vi.fn(),
}));

vi.mock('../../src/lib/api.js', () => ({
  api,
  errorMessage: (e: unknown) => (e instanceof Error ? e.message : String(e)),
}));

vi.stubGlobal(
  'ResizeObserver',
  class {
    observe() {}
    disconnect() {}
  },
);

let component: ReturnType<typeof mount> | undefined;

beforeEach(() => {
  api.board.mockResolvedValue(structuredClone(detail));
});

afterEach(() => {
  if (component) unmount(component);
  component = undefined;
  document.body.innerHTML = '';
  vi.clearAllMocks();
});

async function settle() {
  for (let i = 0; i < 10; i++) await tick();
  flushSync();
}

function type(input: HTMLInputElement, value: string) {
  input.value = value;
  input.dispatchEvent(new Event('input', { bubbles: true }));
}

const nameInput = () =>
  document.querySelector<HTMLInputElement>('section[aria-labelledby=part-general] input')!;
const quickRows = () => [...document.querySelectorAll<HTMLLIElement>('.row-edit')];
const quickQuery = (i: number) => quickRows()[i]!.querySelector<HTMLInputElement>('input.mono')!;
const bar = () => document.querySelector<HTMLElement>('.savebar');
const button = (label: string) =>
  [...bar()!.querySelectorAll('button')].find((b) => b.textContent?.trim() === label)!;

async function open() {
  component = mount(BoardSettingsPage, { target: document.body, props: { boardId: 7 } });
  await settle();
  type(nameInput(), 'Team 2');
  type(quickQuery(1), 'type = Story');
  flushSync();
}

describe('BoardSettingsPage', () => {
  it('shows the save bar only once something changed', async () => {
    component = mount(BoardSettingsPage, { target: document.body, props: { boardId: 7 } });
    await settle();
    expect(bar()).toBeNull();
    expect(document.querySelectorAll('button[type=submit]')).toHaveLength(0);
    type(nameInput(), 'Team 2');
    flushSync();
    expect(bar()?.textContent).toContain(en.boardSettings.unsaved);
  });

  it('saves the name and a quick filter in one patch', async () => {
    api.saveBoard.mockImplementation((_id: number) => {
      const next = structuredClone(detail);
      next.board.name = 'Team 2';
      next.quickFilters[1]!.query = 'type = Story';
      return Promise.resolve(next);
    });
    await open();
    button(en.boardSettings.save).click();
    await settle();

    expect(api.saveBoard).toHaveBeenCalledTimes(1);
    expect(api.saveBoard).toHaveBeenCalledWith(7, {
      name: 'Team 2',
      quickFilters: [
        { id: 1, name: 'Mine', query: 'assignee = currentUser()' },
        { id: 2, name: 'Bugs', query: 'type = Story' },
      ],
    });
    expect(bar()).toBeNull();
  });

  it('discards both changes', async () => {
    await open();
    button(en.boardSettings.discard).click();
    flushSync();
    expect(nameInput().value).toBe('Team');
    expect(quickQuery(1).value).toBe('type = Bug');
    expect(bar()).toBeNull();
    expect(api.saveBoard).not.toHaveBeenCalled();
  });

  it('shows a server error in the bar and marks the quick filter it names', async () => {
    api.saveBoard.mockRejectedValue(new Error('Quick filter “Bugs”: unknown field “typo”.'));
    await open();
    button(en.boardSettings.save).click();
    await settle();

    expect(bar()?.querySelector('[role=alert]')?.textContent).toContain('Quick filter “Bugs”');
    expect(quickRows()[1]!.classList.contains('invalid')).toBe(true);
    expect(quickQuery(1).getAttribute('aria-invalid')).toBe('true');
    expect(quickRows()[0]!.classList.contains('invalid')).toBe(false);
    expect(nameInput().value).toBe('Team 2');
  });
});
