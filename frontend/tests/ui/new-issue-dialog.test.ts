// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { flushSync, mount, tick, unmount } from 'svelte';

import type { OfferedField, ProjectDetail, UserRef } from '../../src/core/types.js';
import { t } from '../../src/i18n/t.js';
import { session } from '../../src/lib/session.svelte.js';
import { dialogs, notes } from '../../src/lib/ui.svelte.js';
import NewIssueDialog from '../../src/ui/NewIssueDialog.svelte';

const alice: UserRef = { id: 1, username: 'alice', name: 'Alice Martin', active: true };

const detail = {
  project: { id: 1, key: 'ALPHA', name: 'Alpha' },
  issueTypes: [
    { id: '1', name: 'Task', subtask: false },
    { id: '2', name: 'Bug', subtask: false },
  ],
  components: [],
  versions: [],
  securityLevels: [],
  defaultAssignee: 'unassigned',
  can: { create: true, attach: true },
} as unknown as ProjectDetail;

// Alice can create issues in Beta, not attach files.
const beta = {
  ...detail,
  project: { id: 2, key: 'BETA', name: 'Beta' },
  can: { create: true },
} as unknown as ProjectDetail;

let assignable: UserRef[] = [];

// Offered for tasks only.
const team: OfferedField = { id: 7, name: 'Team', type: 'text', typeKey: '', options: [] };

const created = vi.fn();
const uploaded = vi.fn();

vi.mock('../../src/lib/api.js', () => ({
  ApiError: class extends Error {
    constructor(
      message: string,
      readonly status: number,
      readonly data: unknown,
    ) {
      super(message);
    }
  },
  api: {
    projects: () => Promise.resolve([detail.project, beta.project]),
    project: (key: string) => Promise.resolve(key === 'BETA' ? beta : detail),
    assignable: () => Promise.resolve(assignable),
    projectFields: (_key: string, type: string) => Promise.resolve(type === '1' ? [team] : []),
    createIssue: (fields: unknown) => created(fields),
    upload: (key: string, files: File[]) => uploaded(key, files),
  },
  isVisible: () => true,
  errorMessage: (e: Error) => e.message,
  pictureUrl: (id: number) => `/api/users/${id}/picture`,
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
  // jsdom has no modal dialogs.
  HTMLDialogElement.prototype.showModal = function (this: HTMLDialogElement) {
    this.open = true;
  };
  HTMLDialogElement.prototype.close = function (this: HTMLDialogElement) {
    this.open = false;
  };
  // jsdom has no top layer.
  HTMLElement.prototype.showPopover = function () {};
  session.me = { id: 1, username: 'alice', name: 'Alice Martin', email: '', admin: false };
});

afterEach(() => {
  created.mockReset();
  uploaded.mockReset();
  notes.length = 0;
  if (component) unmount(component);
  component = undefined;
  session.me = null;
  dialogs.create = false;
});

async function open() {
  dialogs.create = true;
  component = mount(NewIssueDialog, { target: document.body });
  for (let i = 0; i < 6; i++) await tick();
  flushSync();
}

async function settle() {
  for (let i = 0; i < 6; i++) await tick();
  flushSync();
}

function fill(input: HTMLInputElement, value: string) {
  input.value = value;
  input.dispatchEvent(new Event('input', { bubbles: true }));
  flushSync();
}

const teamInput = () =>
  document.getElementById(teamLabel()?.htmlFor ?? '') as HTMLInputElement | null;
const teamLabel = () =>
  [...document.querySelectorAll('label')].find((l) => l.textContent === team.name);

async function chooseType(name: string) {
  const combo = [...document.querySelectorAll<HTMLElement>('[role="combobox"]')].find(
    (c) => c.getAttribute('aria-label') === t.create.type,
  )!;
  combo.click();
  flushSync();
  [...document.querySelectorAll<HTMLElement>('[role="option"]')]
    .find((o) => o.textContent?.includes(name))!
    .click();
  await settle();
}

async function submit() {
  (document.querySelector('form#new-issue') as HTMLFormElement).requestSubmit();
  await settle();
}

function link(): HTMLButtonElement | undefined {
  return [...document.querySelectorAll<HTMLButtonElement>('button')].find(
    (b) => b.textContent === t.common.assignToMe,
  );
}

async function chooseProject(name: string) {
  const combo = [...document.querySelectorAll<HTMLElement>('[role="combobox"]')].find(
    (c) => c.getAttribute('aria-label') === t.create.project,
  )!;
  combo.click();
  flushSync();
  [...document.querySelectorAll<HTMLElement>('[role="option"]')]
    .find((o) => o.textContent?.includes(name))!
    .click();
  await settle();
}

function pick(...files: File[]) {
  const input = document.querySelector<HTMLInputElement>('.files input[type="file"]')!;
  Object.defineProperty(input, 'files', { value: files, configurable: true });
  input.dispatchEvent(new Event('change', { bubbles: true }));
  flushSync();
}

const queued = () => [...document.querySelectorAll('.queue .name')].map((n) => n.textContent);

describe('NewIssueDialog', () => {
  it('queues picked files and uploads them once the issue exists', async () => {
    created.mockResolvedValue({ key: 'ALPHA-1' });
    uploaded.mockImplementation((_key: string, files: File[]) =>
      Promise.resolve({ attachments: files.map((f) => ({ filename: f.name })) }),
    );
    await open();
    fill(document.querySelector('input.summary') as HTMLInputElement, 'With files');
    pick(new File(['%PDF'], 'spec.pdf'), new File(['png'], 'shot.png'));
    pick(new File(['newer'], 'spec.pdf'));
    expect(queued()).toEqual(['shot.png', 'spec.pdf']);
    document
      .querySelector<HTMLButtonElement>(`[aria-label="${t.common.remove} shot.png"]`)!
      .click();
    flushSync();
    expect(queued()).toEqual(['spec.pdf']);
    await submit();
    expect(uploaded).toHaveBeenCalledTimes(1);
    expect(uploaded.mock.calls[0]![0]).toBe('ALPHA-1');
    expect((uploaded.mock.calls[0]![1] as File[]).map((f) => f.size)).toEqual([5]);
    expect(notes.filter((n) => n.kind === 'error')).toEqual([]);
    expect(dialogs.create).toBe(false);
  });

  it('keeps the issue and names the files when the upload fails', async () => {
    const { ApiError } = await import('../../src/lib/api.js');
    created.mockResolvedValue({ key: 'ALPHA-2' });
    uploaded.mockRejectedValue(new ApiError('Storage is full.', 413, null));
    await open();
    fill(document.querySelector('input.summary') as HTMLInputElement, 'Too big');
    pick(new File(['a'], 'a.pdf'), new File(['b'], 'b.png'));
    await submit();
    expect(dialogs.create).toBe(false);
    expect(notes.find((n) => n.kind === 'error')?.text).toBe(
      `Storage is full. ${t.create.notAttached('a.pdf, b.png')}`,
    );
  });

  it('hides the files and drops the queue in a project without attach', async () => {
    await open();
    pick(new File(['a'], 'a.pdf'));
    expect(queued()).toEqual(['a.pdf']);
    await chooseProject('Beta');
    expect(document.querySelector('.files')).toBeNull();
    await chooseProject('Alpha');
    expect(document.querySelector('.files')).not.toBeNull();
    expect(queued()).toEqual([]);
  });

  it('drops the queue when the dialog closes', async () => {
    await open();
    pick(new File(['a'], 'a.pdf'));
    dialogs.create = false;
    await settle();
    dialogs.create = true;
    await settle();
    expect(queued()).toEqual([]);
  });

  it('assigns the issue to the viewer from the Assignee label', async () => {
    assignable = [alice];
    await open();
    link()!.click();
    flushSync();
    expect(document.querySelector('.assignee .picker')?.textContent).toContain(alice.name);
    expect(link()).toBeUndefined();
  });

  it('shows a field offered for the project and type, and sends its value', async () => {
    created.mockResolvedValue({ key: 'ALPHA-1' });
    await open();
    fill(document.querySelector('input.summary') as HTMLInputElement, 'A task');
    fill(teamInput()!, 'Platform');
    await submit();
    expect(created).toHaveBeenCalledTimes(1);
    expect(created.mock.calls[0]![0]).toMatchObject({
      type: '1',
      customFields: { '7': 'Platform' },
    });
  });

  it('drops the field and its value on a type outside its context', async () => {
    created.mockResolvedValue({ key: 'ALPHA-1' });
    await open();
    fill(document.querySelector('input.summary') as HTMLInputElement, 'A bug');
    fill(teamInput()!, 'Platform');
    await chooseType('Bug');
    expect(teamLabel()).toBeUndefined();
    await chooseType('Task');
    expect(teamInput()!.value).toBe('');
    await chooseType('Bug');
    await submit();
    expect(created.mock.calls[0]![0]).toMatchObject({ type: '2' });
    expect(created.mock.calls[0]![0]).not.toHaveProperty('customFields');
  });

  it('shows a server error about a field next to it', async () => {
    const { ApiError } = await import('../../src/lib/api.js');
    created.mockRejectedValue(new ApiError('Not a team.', 400, { field: 'customfield_7' }));
    await open();
    fill(document.querySelector('input.summary') as HTMLInputElement, 'A task');
    fill(teamInput()!, 'Nobody');
    await submit();
    const error = document.getElementById(teamInput()!.getAttribute('aria-describedby') ?? '');
    expect(error?.textContent).toBe('Not a team.');
    expect(teamInput()!.getAttribute('aria-invalid')).toBe('true');
  });

  it('offers no link to a viewer who cannot be assigned in the project', async () => {
    assignable = [];
    await open();
    expect(document.querySelector('.assignee .picker')).not.toBeNull();
    expect(link()).toBeUndefined();
  });
});
