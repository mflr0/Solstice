// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { flushSync, mount, tick, unmount } from 'svelte';

import type { ProjectDetail, UserRef } from '../../src/core/types.js';
import { t } from '../../src/i18n/t.js';
import { session } from '../../src/lib/session.svelte.js';
import { dialogs } from '../../src/lib/ui.svelte.js';
import NewIssueDialog from '../../src/ui/NewIssueDialog.svelte';

const alice: UserRef = { id: 1, username: 'alice', name: 'Alice Martin', active: true };

const detail = {
  project: { id: 1, key: 'ALPHA', name: 'Alpha' },
  issueTypes: [{ id: '1', name: 'Task', subtask: false }],
  components: [],
  versions: [],
  securityLevels: [],
  defaultAssignee: 'unassigned',
  can: { create: true },
} as unknown as ProjectDetail;

let assignable: UserRef[] = [];

vi.mock('../../src/lib/api.js', () => ({
  api: {
    projects: () => Promise.resolve([detail.project]),
    project: () => Promise.resolve(detail),
    assignable: () => Promise.resolve(assignable),
  },
  errorMessage: String,
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
  session.me = { id: 1, username: 'alice', name: 'Alice Martin', email: '', admin: false };
});

afterEach(() => {
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

function link(): HTMLButtonElement | undefined {
  return [...document.querySelectorAll<HTMLButtonElement>('button')].find(
    (b) => b.textContent === t.common.assignToMe,
  );
}

describe('NewIssueDialog', () => {
  it('assigns the issue to the viewer from the Assignee label', async () => {
    assignable = [alice];
    await open();
    link()!.click();
    flushSync();
    expect(document.querySelector('.assignee .picker')?.textContent).toContain(alice.name);
    expect(link()).toBeUndefined();
  });

  it('offers no link to a viewer who cannot be assigned in the project', async () => {
    assignable = [];
    await open();
    expect(document.querySelector('.assignee .picker')).not.toBeNull();
    expect(link()).toBeUndefined();
  });
});
