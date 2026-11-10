// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { flushSync, mount, tick, unmount } from 'svelte';

import type { IssueDetail, OfferedField } from '../../src/core/types.js';
import { t } from '../../src/i18n/t.js';
import Fiche from '../../src/ui/issue/Fiche.svelte';
import { FieldState } from '../../src/ui/issue/field-state.svelte.js';

const colour: OfferedField = {
  id: 7,
  name: 'Colour',
  type: 'select',
  typeKey: '',
  options: [
    { id: 10, value: 'Red' },
    { id: 11, value: 'Green' },
  ],
};
const team: OfferedField = { id: 8, name: 'Team', type: 'text', typeKey: '', options: [] };

const fields = vi.fn(() => Promise.resolve([colour, team]));

vi.mock('../../src/lib/api.js', () => ({
  api: {
    project: () =>
      Promise.resolve({
        issueTypes: [],
        components: [],
        versions: [],
        priorities: [],
        can: {},
      }),
    projectFields: () => fields(),
    layout: () => Promise.reject(new Error('offline')),
    users: () => Promise.resolve([]),
    assignable: () => Promise.resolve([]),
  },
  errorMessage: String,
  pictureUrl: (id: number) => `/api/users/${id}/picture`,
  hasPicture: () => Promise.resolve(false),
}));

vi.stubGlobal(
  'ResizeObserver',
  class {
    observe() {}
    disconnect() {}
  },
);

function issueOf(edit: boolean): IssueDetail {
  return {
    id: 1,
    key: 'ALPHA-1',
    project: { id: 1, key: 'ALPHA', name: 'Alpha' },
    type: { id: '1', name: 'Task', subtask: false },
    priority: null,
    assignee: null,
    reporter: null,
    components: [],
    fixVersions: [],
    affectsVersions: [],
    labels: [],
    due: null,
    securityLevel: null,
    securityLevels: [],
    created: '2026-11-01T10:00:00Z',
    updated: '2026-11-01T10:00:00Z',
    resolved: null,
    resolution: null,
    time: { original: null, spent: null, remaining: null },
    attachments: [],
    customFields: [{ id: 7, name: 'Colour', kind: 'option', values: ['Red'] }],
    can: { edit },
  } as unknown as IssueDetail;
}

let component: ReturnType<typeof mount> | undefined;
let state: FieldState | undefined;
let saved: unknown[];

beforeEach(() => {
  // jsdom has no top layer.
  HTMLElement.prototype.showPopover = function () {};
  saved = [];
});

afterEach(() => {
  fields.mockClear();
  if (component) unmount(component);
  component = undefined;
  state?.destroy();
  state = undefined;
  delete (HTMLElement.prototype as Partial<HTMLElement>).showPopover;
});

async function settle() {
  for (let i = 0; i < 6; i++) await tick();
  flushSync();
}

async function show(edit: boolean) {
  const issue = issueOf(edit);
  state = new FieldState(
    () => issue,
    (f) => {
      saved.push(f);
      return Promise.resolve(true);
    },
  );
  component = mount(Fiche, { target: document.body, props: { issue, fields: state } });
  await settle();
}

const control = (label: string) =>
  [...document.querySelectorAll<HTMLElement>('[role="combobox"]')].find(
    (c) => c.getAttribute('aria-label') === label,
  );

async function choose(label: string, option: string) {
  control(label)!.click();
  flushSync();
  [...document.querySelectorAll<HTMLElement>('[role="option"]')]
    .find((o) => o.textContent?.trim() === option)!
    .click();
  await settle();
}

const teamInput = () => {
  const label = [...document.querySelectorAll('label')].find((l) => l.textContent === 'Team');
  return document.getElementById(label?.htmlFor ?? '') as HTMLInputElement | null;
};

describe('Fiche custom fields', () => {
  it('saves a changed select with customFields', async () => {
    await show(true);
    await choose('Colour', 'Green');
    expect(saved).toEqual([{ customFields: { '7': { id: '11' } } }]);
  });

  it('clearing a select sends null', async () => {
    await show(true);
    await choose('Colour', t.common.none);
    expect(saved).toEqual([{ customFields: { '7': null } }]);
  });

  it('lists offered fields without a value and saves text when left', async () => {
    await show(true);
    const input = teamInput()!;
    input.value = 'Core';
    input.dispatchEvent(new Event('input', { bubbles: true }));
    flushSync();
    expect(saved).toEqual([]);
    input.dispatchEvent(new FocusEvent('focusout', { bubbles: true }));
    await settle();
    expect(saved).toEqual([{ customFields: { '8': 'Core' } }]);
  });

  it('gives a viewer without edit no control', async () => {
    await show(false);
    expect(fields).not.toHaveBeenCalled();
    expect(control('Colour')).toBeUndefined();
    expect(teamInput()).toBeNull();
    expect(document.querySelector('.fiche')?.textContent).toContain('Red');
  });
});
