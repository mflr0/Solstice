// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { flushSync, mount, tick, unmount } from 'svelte';

import type { Meta } from '../../src/core/types.js';
import { t } from '../../src/i18n/t.js';
import { session } from '../../src/lib/session.svelte.js';
import { ApiError } from '../../src/lib/api.js';
import { notes } from '../../src/lib/ui.svelte.js';
import ProjectDialog from '../../src/ui/admin/ProjectDialog.svelte';

const createProject = vi.hoisted(() => vi.fn());

vi.mock('../../src/lib/admin-api.js', () => ({
  adminApi: {
    categories: () => Promise.resolve([]),
    groups: () => Promise.resolve([]),
    workflows: () =>
      Promise.resolve([
        { id: 5, name: 'Software', default: true, statuses: [{ id: '1', name: 'To Do' }] },
        { id: 6, name: 'Simple', default: false, statuses: [{ id: '1', name: 'To Do' }] },
      ]),
    checkKey: () => Promise.resolve({ available: true }),
    createProject,
  },
}));

let component: ReturnType<typeof mount> | undefined;

beforeEach(() => {
  // jsdom has no modal dialogs.
  HTMLDialogElement.prototype.showModal = function (this: HTMLDialogElement) {
    this.open = true;
  };
  HTMLDialogElement.prototype.close = function (this: HTMLDialogElement) {
    this.open = false;
  };
  session.me = { id: 1, username: 'chef', name: 'Chef', email: '', admin: true };
});

afterEach(() => {
  notes.splice(0);
  createProject.mockReset();
  if (component) unmount(component);
  component = undefined;
  session.me = null;
  session.meta = null;
});

async function open(issueTypes: Meta['issueTypes']) {
  session.meta = {
    issueTypes,
    priorities: [],
    resolutions: [],
    statuses: [],
    linkTypes: [],
  } as unknown as Meta;
  component = mount(ProjectDialog, {
    target: document.body,
    props: { open: true, onclose: () => {}, oncreated: () => {} },
  });
  for (let i = 0; i < 4; i++) await tick();
  flushSync();
}

describe('ProjectDialog issue types', () => {
  it('says there are none, with a link to add some, when the catalogue is empty', async () => {
    await open([]);
    expect(document.body.textContent).toContain(t.projectsAdmin.noTypes);
    const link = [...document.querySelectorAll('a')].find(
      (a) => a.textContent === t.projectsAdmin.noTypesLink,
    );
    expect(link?.getAttribute('href')).toBe('/admin/issue-types');
  });

  it('lists the types to choose from otherwise', async () => {
    await open([{ id: '1', name: 'Task', subtask: false }] as Meta['issueTypes']);
    expect(document.body.textContent).not.toContain(t.projectsAdmin.noTypes);
    expect(document.querySelector('fieldset.choices')?.textContent).toContain('Task');
  });
});

const task = [{ id: '1', name: 'Task', subtask: false }] as Meta['issueTypes'];

async function settle() {
  for (let i = 0; i < 4; i++) await tick();
  flushSync();
}

/** Types a name (the key follows it) and sends the form. */
async function submit(name: string) {
  const input = document.querySelector<HTMLInputElement>('.project-form input')!;
  input.value = name;
  input.dispatchEvent(new Event('input', { bubbles: true }));
  flushSync();
  // Sent from elsewhere in the form, as after a scroll down to the buttons.
  document.querySelectorAll<HTMLInputElement>('.project-form input')[1]!.focus();
  document.querySelector<HTMLFormElement>('.project-form')!.requestSubmit();
  await settle();
}

describe('ProjectDialog workflow', () => {
  it('lists the global workflows and sends the default one, preselected', async () => {
    createProject.mockResolvedValue({ id: 1, key: 'DEMO' });
    await open(task);
    expect(document.body.textContent).toContain('Software');
    await submit('Demo');
    expect(createProject).toHaveBeenCalledWith(expect.objectContaining({ workflow: 5 }));
  });
});

const errorNotes = () => notes.filter((n) => n.kind === 'error').map((n) => n.text);

describe('ProjectDialog failures', () => {
  it('raises a toast when the create fails, and keeps the inline error', async () => {
    createProject.mockRejectedValue(new ApiError('The key DEMO is taken.', 409));
    await open(task);
    await submit('Demo');
    expect(createProject).toHaveBeenCalled();
    expect(errorNotes()).toEqual(['The key DEMO is taken.']);
    expect(document.querySelector('.project-form .error-text')?.textContent).toBe(
      'The key DEMO is taken.',
    );
  });

  it('says what is wrong when the field in error is not shown', async () => {
    await open([]);
    await submit('Demo');
    expect(createProject).not.toHaveBeenCalled();
    expect(errorNotes()).toEqual([t.projectsAdmin.problems.typesRequired]);
  });

  it('focuses the first field in error when it is shown', async () => {
    // jsdom lays nothing out: every element counts as hidden unless it has a box.
    const rects = vi
      .spyOn(HTMLElement.prototype, 'getClientRects')
      .mockReturnValue([{}] as unknown as DOMRectList);
    HTMLElement.prototype.scrollIntoView = () => {};
    try {
      await open(task);
      await submit('');
      expect(createProject).not.toHaveBeenCalled();
      expect(document.activeElement).toBe(document.querySelector('.project-form input'));
      expect(errorNotes()).toEqual([]);
    } finally {
      rects.mockRestore();
    }
  });
});
