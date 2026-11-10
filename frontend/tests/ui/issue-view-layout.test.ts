// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest';
import { flushSync, mount, tick, unmount } from 'svelte';

import type { LayoutField, LayoutForm } from '../../src/core/admin-types.js';
import type { IssueDetail, OfferedField } from '../../src/core/types.js';
import { t } from '../../src/i18n/t.js';
import Fiche from '../../src/ui/issue/Fiche.svelte';
import { FieldState } from '../../src/ui/issue/field-state.svelte.js';
import IssueField from '../../src/ui/issue/IssueField.svelte';

const steps: OfferedField = { id: 9, name: 'Steps', type: 'textarea', typeKey: '', options: [] };
const team: OfferedField = { id: 8, name: 'Team', type: 'text', typeKey: '', options: [] };

const side = (field: string, hidden = false): LayoutField => ({ field, zone: 'side', hidden });
const view: LayoutField[] = [
  { field: 'description', zone: 'main', hidden: false },
  { field: 'customfield_9', zone: 'main', hidden: false },
  side('duedate'),
  side('assignee'),
  side('customfield_8'),
  side('priority'),
  side('reporter', true),
  side('labels'),
];
const create: LayoutField[] = [{ field: 'duedate', hidden: false, required: true }];

vi.mock('../../src/lib/api.js', () => ({
  api: {
    project: () =>
      Promise.resolve({ issueTypes: [], components: [], versions: [], priorities: [], can: {} }),
    projectFields: () => Promise.resolve([steps, team]),
    layout: (_p: string, _t: string, form: LayoutForm) =>
      Promise.resolve({ form, source: 'type', fields: form === 'view' ? view : create }),
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
    type: { id: '1', name: 'Bug', subtask: false },
    description: 'It breaks.',
    environment: null,
    priority: null,
    assignee: null,
    reporter: { id: 3, username: 'hans', displayName: 'Hans' },
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
    customFields: [
      { id: 9, name: 'Steps', kind: 'wiki', values: ['Open the page'] },
      { id: 5, name: 'Legacy', kind: 'text', values: ['old'] },
    ],
    can: { edit },
  } as unknown as IssueDetail;
}

const mounted: ReturnType<typeof mount>[] = [];
let state: FieldState | undefined;

afterEach(() => {
  for (const c of mounted.splice(0)) unmount(c);
  state?.destroy();
  state = undefined;
  document.body.innerHTML = '';
});

async function settle() {
  for (let i = 0; i < 6; i++) await tick();
  flushSync();
}

// The side card, and the main zone as the issue page lays it out.
async function show(edit: boolean) {
  const issue = issueOf(edit);
  state = new FieldState(
    () => issue,
    () => Promise.resolve(true),
  );
  await settle();
  const main = document.createElement('div');
  main.className = 'main-zone';
  document.body.append(main);
  for (const field of state.zones.main) {
    mounted.push(
      mount(IssueField, { target: main, props: { issue, field, zone: 'main', fields: state } }),
    );
  }
  mounted.push(mount(Fiche, { target: document.body, props: { issue, fields: state } }));
  await settle();
}

const sideLabels = () =>
  [...document.querySelectorAll('.fiche dt')].map((d) =>
    (d.textContent ?? '').replace(t.issue.required, '').trim(),
  );
const mainTitles = () =>
  [...document.querySelectorAll('.main-zone h2')].map((h) =>
    h.firstElementChild?.textContent?.trim(),
  );

describe('issue view layout', () => {
  it('lists the side rows in the layout order, after the type', async () => {
    await show(true);
    expect(sideLabels()).toEqual([
      t.issue.type,
      t.issue.due,
      t.issue.assignee,
      'Team',
      t.issue.priority,
      t.issue.labels,
      'Legacy',
    ]);
  });

  it('renders a custom field placed in main there, like the description', async () => {
    await show(true);
    expect(mainTitles()).toEqual([t.issue.description, 'Steps']);
    const stepsSection = document.querySelector('.main-zone section[aria-label="Steps"]');
    expect(stepsSection?.textContent).toContain('Open the page');
    expect(stepsSection?.querySelector('button')?.textContent).toBe(t.common.edit);
    expect(document.querySelector('.fiche')?.textContent).not.toContain('Steps');
  });

  it('leaves a hidden field out, for readers too', async () => {
    await show(false);
    expect(sideLabels()).not.toContain(t.issue.reporter);
    expect(document.body.textContent).not.toContain('Hans');
  });

  it('has no custom fields heading', async () => {
    await show(true);
    expect(document.querySelector('.fiche .card-head.sub')).toBeNull();
    expect(document.body.textContent).not.toContain('Other fields');
  });

  it('marks an empty required field to editors only', async () => {
    await show(true);
    const due = () =>
      [...document.querySelectorAll('.fiche .line')].find((l) =>
        l.querySelector('dt')?.textContent?.includes(t.issue.due),
      );
    expect(due()?.querySelector('.req')?.textContent).toBe(t.issue.required);
    for (const c of mounted.splice(0)) unmount(c);
    state?.destroy();
    document.body.innerHTML = '';
    await show(false);
    expect(due()?.querySelector('.req')).toBeNull();
  });
});
