// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { flushSync, mount, tick, unmount } from 'svelte';

import type { BoardDetail } from '../../src/core/board-types.js';
import type { Meta } from '../../src/core/types.js';
import { en } from '../../src/i18n/en.js';
import { session } from '../../src/lib/session.svelte.js';
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
  boards: () => Promise.resolve([]),
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
const announced = () => document.querySelector('[aria-live]')?.textContent?.trim();

/** Presses Alt+key on the focused grip, as many times as asked. */
async function altKey(key: string, times = 1) {
  for (let n = 0; n < times; n++) {
    document.activeElement!.dispatchEvent(
      new KeyboardEvent('keydown', { key, altKey: true, bubbles: true }),
    );
    await settle();
  }
}
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

  it('moves the last quick filter first with the keyboard', async () => {
    api.saveBoard.mockResolvedValue(structuredClone(detail));
    component = mount(BoardSettingsPage, { target: document.body, props: { boardId: 7 } });
    await settle();
    expect(document.getElementById('quick-grip-1')?.getAttribute('aria-label')).toBe(
      en.boardSettings.reorder('Bugs'),
    );
    document.getElementById('quick-grip-1')!.focus();
    await altKey('ArrowUp');
    expect(document.activeElement?.id).toBe('quick-grip-0');
    expect(announced()).toBe(en.boardSettings.moved('Bugs', 1, 2));
    // Columns move sideways; a quick filter doesn't.
    await altKey('ArrowLeft');
    expect(document.activeElement?.id).toBe('quick-grip-0');

    button(en.boardSettings.save).click();
    await settle();
    expect(api.saveBoard).toHaveBeenCalledWith(7, {
      quickFilters: [
        { id: 2, name: 'Bugs', query: 'type = Bug' },
        { id: 1, name: 'Mine', query: 'assignee = currentUser()' },
      ],
    });
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

describe('BoardSettingsPage columns', () => {
  const todo = { id: '1', name: 'To Do', category: 'todo' } as const;
  const progress = { id: '2', name: 'In Progress', category: 'progress' } as const;
  const done = { id: '3', name: 'Done', category: 'done' } as const;
  const review = { id: '4', name: 'In Review', category: 'progress' } as const;
  const blocked = { id: '5', name: 'Blocked', category: 'progress' } as const;

  beforeEach(() => {
    HTMLElement.prototype.showPopover = function () {};
    const board = structuredClone(detail);
    board.projects = [{ key: 'T' } as BoardDetail['projects'][number]];
    board.columns = [
      { id: 10, name: 'To Do', statuses: [todo] },
      { id: 11, name: 'Doing', statuses: [] },
      { id: 12, name: 'Finished', statuses: [done] },
    ];
    api.board.mockResolvedValue(board);
    // The workflow's order, not the admin one the page sorts by.
    api.boardProjectStatuses.mockResolvedValue({
      workflows: [{ statuses: [blocked, review, done, progress, todo] }],
    });
    session.meta = { statuses: [todo, progress, done, review, blocked] } as unknown as Meta;
  });

  afterEach(() => {
    delete (HTMLElement.prototype as Partial<HTMLElement>).showPopover;
    session.meta = null;
  });

  const byLabel = (label: string) =>
    document.querySelector<HTMLButtonElement>(`[aria-label="${label}"]`)!;
  const item = (name: string) =>
    [...document.querySelectorAll<HTMLElement>('[role=menuitemcheckbox], [role=option]')].find(
      (b) => b.textContent?.trim().startsWith(name),
    )!;

  function pick(column: string, ...names: string[]) {
    byLabel(en.boardSettings.statusesOf(column)).click();
    flushSync();
    for (const n of names) item(n).click();
    flushSync();
    byLabel(en.boardSettings.statusesOf(column)).click();
    flushSync();
  }

  async function mountPage() {
    component = mount(BoardSettingsPage, { target: document.body, props: { boardId: 7 } });
    await settle();
  }

  it('saves the drop status first, the others in status order, and moves a held status', async () => {
    api.saveBoard.mockResolvedValue(structuredClone(detail));
    await mountPage();
    expect(document.querySelector('.unmapped')?.textContent).toContain(
      'In Progress, In Review, Blocked',
    );

    byLabel(en.boardSettings.statusesOf('Doing')).click();
    flushSync();
    expect(item('Done').textContent).toContain(en.boardSettings.inColumn('Finished'));
    byLabel(en.boardSettings.statusesOf('Doing')).click();
    flushSync();

    pick('Doing', 'In Review', 'Done', 'In Progress');
    byLabel(en.boardSettings.dropStatusOf('Doing')).click();
    flushSync();
    item('In Review').click();
    flushSync();
    expect(document.querySelector('.unmapped')?.textContent).toContain('Blocked');
    expect(document.querySelector('.unmapped')?.textContent).not.toContain('In Review');

    button(en.boardSettings.save).click();
    await settle();
    expect(api.saveBoard).toHaveBeenCalledWith(7, {
      columns: [
        { id: 10, name: 'To Do', statuses: ['1'] },
        { id: 11, name: 'Doing', statuses: ['4', '2', '3'] },
        { id: 12, name: 'Finished', statuses: [] },
      ],
    });
  });

  it('moves the last column first with the keyboard', async () => {
    api.saveBoard.mockResolvedValue(structuredClone(detail));
    await mountPage();
    document.getElementById('columns-grip-2')!.focus();
    await altKey('ArrowUp');
    expect(document.activeElement?.id).toBe('columns-grip-2');
    await altKey('ArrowLeft', 2);
    expect(document.activeElement?.id).toBe('columns-grip-0');
    expect(announced()).toBe(en.boardSettings.moved('Finished', 1, 3));

    button(en.boardSettings.save).click();
    await settle();
    expect(api.saveBoard).toHaveBeenCalledWith(7, {
      columns: [
        { id: 12, name: 'Finished', statuses: ['3'] },
        { id: 10, name: 'To Do', statuses: ['1'] },
        { id: 11, name: 'Doing', statuses: [] },
      ],
    });
  });

  it('keeps the drop status first when more statuses join', async () => {
    await mountPage();
    pick('Doing', 'In Review');
    pick('Doing', 'Blocked', 'In Progress');
    expect(byLabel(en.boardSettings.dropStatusOf('Doing')).textContent?.trim()).toBe('In Review');
    pick('Doing', 'In Review');
    expect(byLabel(en.boardSettings.dropStatusOf('Doing')).textContent?.trim()).toBe('In Progress');
  });
});
