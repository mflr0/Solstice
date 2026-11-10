// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { flushSync, mount, tick, unmount } from 'svelte';

import type { Meta } from '../../src/core/types.js';
import { t } from '../../src/i18n/t.js';
import { session } from '../../src/lib/session.svelte.js';
import ProjectDialog from '../../src/ui/admin/ProjectDialog.svelte';

vi.mock('../../src/lib/admin-api.js', () => ({
  adminApi: {
    categories: () => Promise.resolve([]),
    groups: () => Promise.resolve([]),
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
