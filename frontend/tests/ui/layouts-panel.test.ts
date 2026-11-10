// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { flushSync, mount, tick, unmount } from 'svelte';

import type { IssueLayout, LayoutForm } from '../../src/core/admin-types.js';
import type { ProjectSettings } from '../../src/core/project-admin.js';
import { ApiError } from '../../src/lib/api.js';
import LayoutsPanel from '../../src/ui/project-settings/LayoutsPanel.svelte';
import ProjectSettingsPage from '../../src/ui/pages/ProjectSettingsPage.svelte';

const detail = {
  key: 'ALPHA',
  issueTypes: [
    { id: '1', name: 'Task', subtask: false },
    { id: '2', name: 'Bug', subtask: false },
  ],
} as unknown as ProjectSettings;

const inherited = (form: LayoutForm): IssueLayout => ({
  form,
  source: 'type',
  fields: [
    { field: 'description', hidden: false },
    { field: 'priority', hidden: false },
  ],
});

let overrides: Partial<Record<string, IssueLayout>> = {};
// The page tests open on the General tab, which needs the full settings.
let settings: () => Promise<unknown> = () => Promise.resolve(detail);
const saved = vi.fn();
const reset = vi.fn();
const pending = () => new Promise<never>(() => {});

vi.mock('../../src/lib/admin-api.js', async (original) => ({
  ...(await original<object>()),
  projectAdminApi: new Proxy(
    {
      settings: () => settings(),
      layout: (_key: string, type: string, form: LayoutForm) =>
        Promise.resolve(overrides[`${type}:${form}`] ?? inherited(form)),
      saveLayout: (...args: unknown[]) => {
        saved(...args);
        return Promise.resolve({
          form: args[2],
          source: 'project',
          fields: args[3],
        } as IssueLayout);
      },
      resetLayout: (key: string, type: string, form: LayoutForm) => {
        reset(key, type, form);
        delete overrides[`${type}:${form}`];
        return Promise.resolve();
      },
    },
    { get: (target, k) => (target as Record<string | symbol, unknown>)[k] ?? pending },
  ),
}));

let permissions: () => Promise<unknown> = pending;

vi.mock('../../src/lib/api.js', async (original) => ({
  ...(await original<object>()),
  api: new Proxy(
    {
      projectFields: () => Promise.resolve([]),
      permissions: () => permissions(),
    },
    { get: (target, k) => (target as Record<string | symbol, unknown>)[k] ?? pending },
  ),
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
  // jsdom has no modal dialogs, popovers or top layer.
  HTMLDialogElement.prototype.showModal = function (this: HTMLDialogElement) {
    this.open = true;
  };
  HTMLDialogElement.prototype.close = function (this: HTMLDialogElement) {
    this.open = false;
  };
  HTMLElement.prototype.showPopover = function () {};
  HTMLElement.prototype.hidePopover = () => {};
});

afterEach(() => {
  if (component) unmount(component);
  component = undefined;
  overrides = {};
  permissions = pending;
  settings = () => Promise.resolve(detail);
  saved.mockReset();
  reset.mockReset();
  document.body.innerHTML = '';
});

async function settle() {
  for (let i = 0; i < 5; i++) await tick();
  flushSync();
}

async function panel() {
  component = mount(LayoutsPanel, {
    target: document.body,
    props: { projectKey: 'ALPHA', guard: (go: () => void) => go(), ondirty: () => {} },
  });
  await settle();
}

const button = (text: string) =>
  [...document.querySelectorAll<HTMLButtonElement>('button')].find(
    (b) => b.textContent?.trim() === text,
  );

async function click(el: HTMLElement | undefined) {
  if (!el) throw new Error('no such button');
  el.click();
  await settle();
}

describe('LayoutsPanel', () => {
  it('says a type without override follows the type’s layout', async () => {
    await panel();
    expect(document.body.textContent).toContain('Follows the type’s layout');
    expect(document.querySelector('form.layout-editor')).toBeNull();
    expect(button('Reset to the type’s layout')).toBeUndefined();
  });

  it('customises from the inherited template and saves the override', async () => {
    await panel();
    await click(button('Customise'));
    expect(document.querySelector('form.layout-editor')).not.toBeNull();
    await click(document.getElementById('order-down-layout-create-description') ?? undefined);
    const form = document.querySelector('form.layout-editor') as HTMLFormElement;
    form.dispatchEvent(new SubmitEvent('submit', { bubbles: true, cancelable: true }));
    await settle();
    expect(saved).toHaveBeenCalledTimes(1);
    const [key, type, f, fields] = saved.mock.calls[0] as [
      string,
      string,
      string,
      { field: string }[],
    ];
    expect([key, type, f]).toEqual(['ALPHA', '1', 'create']);
    expect(fields.slice(0, 2).map((x) => x.field)).toEqual(['priority', 'description']);
    expect(document.body.textContent).toContain('Customised for this project');
  });

  it('resets an override after confirmation', async () => {
    overrides['1:create'] = { ...inherited('create'), source: 'project' };
    await panel();
    expect(document.body.textContent).toContain('Customised for this project');
    await click(button('Reset to the type’s layout'));
    expect(reset).not.toHaveBeenCalled();
    const dialog = document.querySelector('dialog[open]') as HTMLElement;
    const confirm = [...dialog.querySelectorAll<HTMLButtonElement>('button')].find(
      (b) => b.textContent?.trim() === 'Reset to the type’s layout',
    );
    await click(confirm);
    expect(reset).toHaveBeenCalledWith('ALPHA', '1', 'create');
    expect(document.body.textContent).toContain('Follows the type’s layout');
  });
});

describe('ProjectSettingsPage, issue layouts', () => {
  beforeEach(() => {
    settings = pending;
  });

  it('has no layouts tab for someone who doesn’t administer the project', async () => {
    permissions = () => Promise.reject(new ApiError('forbidden', 403, null));
    component = mount(ProjectSettingsPage, {
      target: document.body,
      props: { projectKey: 'ALPHA' },
    });
    await settle();
    expect(document.getElementById('tab-layouts')).toBeNull();
    expect(document.getElementById('panel-layouts')).toBeNull();
  });

  it('has the layouts tab for a project administrator', async () => {
    permissions = () =>
      Promise.resolve({ permissions: [], grants: [], groups: [], users: [], holders: [] });
    component = mount(ProjectSettingsPage, {
      target: document.body,
      props: { projectKey: 'ALPHA' },
    });
    await settle();
    expect(document.getElementById('tab-layouts')?.textContent).toBe('Issue layouts');
  });
});
