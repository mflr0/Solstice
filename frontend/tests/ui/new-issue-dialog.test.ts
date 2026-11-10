// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { flushSync, mount, tick, unmount } from 'svelte';

import type { IssueLayout, LayoutField } from '../../src/core/admin-types.js';
import { defaultCreateLayout } from '../../src/core/new-issue.js';
import type { IssueDetail, OfferedField, ProjectDetail, UserRef } from '../../src/core/types.js';
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

// The request that fails, if any.
let failing: 'projects' | 'project' | 'fields' | null = null;
const down = () => Promise.reject(new Error('The server can’t be reached right now.'));

// Offered for tasks only.
const team: OfferedField = { id: 7, name: 'Team', type: 'text', typeKey: '', options: [] };

// The create layout by type id; the default for the others.
let layouts: Record<string, LayoutField[]> = {};
const layoutFor = vi.fn((_key: string, type: string): Promise<IssueLayout> =>
  Promise.resolve({
    form: 'create',
    source: type in layouts ? 'type' : 'default',
    fields: layouts[type] ?? [...defaultCreateLayout],
  }),
);

const created = vi.fn();
const updated = vi.fn();
let issue: IssueDetail;
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
    projects: () =>
      failing === 'projects' ? down() : Promise.resolve([detail.project, beta.project]),
    project: (key: string) =>
      failing === 'project' ? down() : Promise.resolve(key === 'BETA' ? beta : detail),
    assignable: () => Promise.resolve(assignable),
    projectFields: (_key: string, type: string) =>
      failing === 'fields' ? down() : Promise.resolve(type === '1' ? [team] : []),
    layout: (key: string, type: string) => layoutFor(key, type),
    createIssue: (fields: unknown) => created(fields),
    issue: () => Promise.resolve(issue),
    updateIssue: (key: string, fields: unknown) => updated(key, fields),
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
  updated.mockReset();
  layoutFor.mockClear();
  layouts = {};
  failing = null;
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

// The names of the fields, in the form's order.
const fieldNames = () =>
  [...document.querySelectorAll('#new-issue .field:not(.files)')].map((f) =>
    f.firstElementChild?.textContent?.replace('*', '').trim(),
  );

const required = (name: string) =>
  [...document.querySelectorAll('#new-issue .field')]
    .find((f) => f.firstElementChild?.textContent?.replace('*', '').trim() === name)
    ?.querySelector('.req') != null;

// ALPHA-5, a task with a description and a Team.
function existing(): IssueDetail {
  return {
    key: 'ALPHA-5',
    summary: 'Old summary',
    type: { id: '1', name: 'Task', subtask: false },
    project: { id: 1, key: 'ALPHA', name: 'Alpha' },
    description: 'The body',
    environment: null,
    priority: null,
    assignee: alice,
    labels: ['ui'],
    components: [],
    affectsVersions: [],
    fixVersions: [],
    due: null,
    time: { original: 3600, remaining: 3600, spent: null },
    securityLevel: null,
    customFields: [{ id: 7, name: 'Team', kind: 'text', values: ['Platform'] }],
    can: { edit: true },
  } as unknown as IssueDetail;
}

async function openEdit(onsaved?: () => void) {
  dialogs.createDefaults = { edit: 'ALPHA-5', onsaved };
  await open();
  await settle();
}

const combo = (label: string) =>
  [...document.querySelectorAll<HTMLButtonElement>('[role="combobox"]')].find(
    (c) => c.getAttribute('aria-label') === label,
  );

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

  it('follows the layout of the chosen type, and reloads it with the type', async () => {
    layouts = {
      '2': [
        { field: 'environment', hidden: false },
        { field: 'duedate', hidden: false, required: true },
        { field: 'description', hidden: false },
        { field: 'priority', hidden: false },
        { field: 'labels', hidden: true },
        { field: 'assignee', hidden: true },
      ],
    };
    await open();
    expect(layoutFor).toHaveBeenLastCalledWith('ALPHA', '1');
    expect(fieldNames()).toContain(t.create.labels);
    await chooseType('Bug');
    expect(layoutFor).toHaveBeenLastCalledWith('ALPHA', '2');
    expect(fieldNames()).toEqual([
      t.create.project,
      t.create.type,
      t.create.summary,
      t.issue.environment,
      t.create.due,
      t.create.description,
      t.create.priority,
    ]);
    expect(fieldNames()).not.toContain(t.create.labels);
    expect(required(t.create.due)).toBe(true);
    expect(required(t.issue.environment)).toBe(false);
    await chooseType('Task');
    expect(fieldNames()).toContain(t.create.labels);
    expect(required(t.create.due)).toBe(false);
  });

  it('blocks creating while a required field is empty', async () => {
    created.mockResolvedValue({ key: 'ALPHA-3' });
    layouts = { '1': [{ field: 'duedate', hidden: false, required: true }] };
    await open();
    fill(document.querySelector('input.summary') as HTMLInputElement, 'Due soon');
    await submit();
    expect(created).not.toHaveBeenCalled();
    const due = document.querySelector<HTMLInputElement>('#new-issue input[type="date"]')!;
    expect(due.getAttribute('aria-invalid')).toBe('true');
    fill(due, '2026-12-01');
    await submit();
    expect(created).toHaveBeenCalledTimes(1);
    expect(created.mock.calls[0]![0]).toMatchObject({ due: '2026-12-01' });
    expect(created.mock.calls[0]![0]).not.toHaveProperty('labels');
  });

  it('offers no link to a viewer who cannot be assigned in the project', async () => {
    assignable = [];
    await open();
    expect(document.querySelector('.assignee .picker')).not.toBeNull();
    expect(link()).toBeUndefined();
  });

  describe('edit mode', () => {
    afterEach(() => {
      dialogs.createDefaults = {};
    });

    it('prefills the fields and keeps project and type read-only', async () => {
      issue = existing();
      await openEdit();
      expect(document.querySelector('dialog')?.textContent).toContain(
        t.create.editTitle('ALPHA-5'),
      );
      expect((document.querySelector('input.summary') as HTMLInputElement).value).toBe(
        'Old summary',
      );
      expect(teamInput()!.value).toBe('Platform');
      expect(
        document.querySelector('#new-issue .labels, #new-issue .boxed')?.textContent,
      ).toContain('ui');
      expect(combo(t.create.project)?.disabled).toBe(true);
      expect(combo(t.create.project)?.textContent).toContain('ALPHA');
      expect(combo(t.create.type)?.disabled).toBe(true);
      expect(combo(t.create.type)?.textContent).toContain('Task');
      expect(document.querySelector('button[type="submit"]')?.textContent).toBe(t.common.save);
    });

    it('saves only the changed fields with a PATCH', async () => {
      issue = existing();
      updated.mockResolvedValue({ key: 'ALPHA-5' });
      const onsaved = vi.fn();
      await openEdit(onsaved);
      fill(document.querySelector('input.summary') as HTMLInputElement, 'New summary');
      fill(teamInput()!, 'Core');
      await submit();
      expect(created).not.toHaveBeenCalled();
      expect(updated).toHaveBeenCalledTimes(1);
      expect(updated.mock.calls[0]).toEqual([
        'ALPHA-5',
        { summary: 'New summary', customFields: { '7': 'Core' } },
      ]);
      expect(dialogs.create).toBe(false);
      expect(onsaved).toHaveBeenCalledTimes(1);
      expect(notes.at(-1)?.text).toBe(t.create.saved('ALPHA-5'));
    });

    it('saves with a required field the issue never had still empty', async () => {
      issue = existing();
      updated.mockResolvedValue({ key: 'ALPHA-5' });
      layouts = { '1': [{ field: 'duedate', hidden: false, required: true }] };
      await openEdit();
      fill(document.querySelector('input.summary') as HTMLInputElement, 'New summary');
      await submit();
      expect(updated.mock.calls[0]).toEqual(['ALPHA-5', { summary: 'New summary' }]);
    });

    it('blocks clearing a required field', async () => {
      issue = { ...existing(), due: '2026-12-01' };
      layouts = { '1': [{ field: 'duedate', hidden: false, required: true }] };
      await openEdit();
      const due = document.querySelector<HTMLInputElement>('#new-issue input[type="date"]')!;
      expect(due.value).toBe('2026-12-01');
      fill(due, '');
      await submit();
      expect(updated).not.toHaveBeenCalled();
      expect(due.getAttribute('aria-invalid')).toBe('true');
    });
  });
});

describe('NewIssueDialog when a load fails', () => {
  const alert = () => [...document.querySelectorAll('[role="alert"]')].map((a) => a.textContent);
  const submitButton = () =>
    document.querySelector<HTMLButtonElement>('button[type="submit"][form="new-issue"]')!;

  for (const what of ['projects', 'project'] as const) {
    it(`shows the error instead of the form when the ${what} can't be loaded`, async () => {
      failing = what;
      await open();
      expect(document.querySelector('form#new-issue')).toBeNull();
      expect(alert()).toContain('The server can’t be reached right now.');
      expect(notes).toEqual([]);
    });
  }

  it('says why it can’t create when the custom fields can’t be loaded', async () => {
    failing = 'fields';
    await open();
    expect(document.querySelector('form#new-issue')).not.toBeNull();
    expect(alert()).toContain('The server can’t be reached right now.');
    expect(submitButton().disabled).toBe(true);
  });
});
