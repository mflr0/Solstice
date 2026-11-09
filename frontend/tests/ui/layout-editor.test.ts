// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest';
import { flushSync, mount, tick, unmount } from 'svelte';

import type { LayoutField, LayoutForm } from '../../src/core/admin-types.js';
import { dropRow, layoutRows, type LayoutChoice } from '../../src/ui/layouts/layout-rows.js';
import LayoutEditor from '../../src/ui/layouts/LayoutEditor.svelte';

vi.stubGlobal(
  'ResizeObserver',
  class {
    observe() {}
    disconnect() {}
  },
);
// jsdom has no popovers: the menus open by their beforetoggle event below.
HTMLElement.prototype.hidePopover = () => {};

const createChoices: LayoutChoice[] = [
  { id: 'priority', name: 'Priority' },
  { id: 'description', name: 'Description' },
  { id: 'assignee', name: 'Assignee' },
  { id: 'environment', name: 'Environment' },
  { id: 'customfield_7', name: 'Team', note: 'Only in 2 projects' },
];

const viewChoices: LayoutChoice[] = [
  { id: 'description', name: 'Description' },
  { id: 'assignee', name: 'Assignee' },
  { id: 'priority', name: 'Priority' },
  { id: 'labels', name: 'Labels' },
];

let component: ReturnType<typeof mount> | undefined;

afterEach(() => {
  if (component) unmount(component);
  component = undefined;
});

function editor(form: LayoutForm, template: LayoutField[], fields: LayoutChoice[]) {
  const saved: LayoutField[][] = [];
  component = mount(LayoutEditor, {
    target: document.body,
    props: {
      form,
      fields,
      template,
      onsave: (f: LayoutField[]) => {
        saved.push(f);
        return Promise.resolve();
      },
    },
  });
  flushSync();
  return saved;
}

const byId = <T extends HTMLElement>(id: string) => document.getElementById(id) as T;

function click(el: HTMLElement) {
  el.click();
  flushSync();
}

/** The shown rows, top to bottom, of a column of the view or of the create form. */
function order(zone?: 'main' | 'side') {
  const scope = zone ? `.layout-list[data-zone="${zone}"]` : '.layout-list';
  return [...document.querySelectorAll<HTMLElement>(`${scope} .layout-row`)].map(
    (r) => r.dataset['field'],
  );
}

const tray = () =>
  [...document.querySelectorAll<HTMLElement>('.tray .tray-chip')].map((c) => c.dataset['field']);

/** Opens the menu labelled `label` and picks its entry `entry`. */
async function pick(label: string, entry: string) {
  const button = document.querySelector<HTMLElement>(`button[aria-label="${label}"]`);
  const menu = document.getElementById(button?.getAttribute('popovertarget') ?? '');
  if (!menu) throw new Error(`no menu ${label}`);
  menu.dispatchEvent(Object.assign(new Event('beforetoggle'), { newState: 'open' }));
  flushSync();
  const item = [...menu.querySelectorAll<HTMLButtonElement>('[role="menuitem"]')].find(
    (b) => b.textContent?.trim() === entry,
  );
  if (!item) throw new Error(`no entry ${entry} in ${label}`);
  click(item);
  await tick();
}

async function save() {
  const form = document.querySelector('form.layout-editor') as HTMLFormElement;
  form.dispatchEvent(new SubmitEvent('submit', { bubbles: true, cancelable: true }));
  await tick();
}

const createTemplate: LayoutField[] = [
  { field: 'priority', hidden: false },
  { field: 'description', hidden: false, required: true },
  { field: 'assignee', hidden: false },
  { field: 'environment', hidden: true },
];

describe('LayoutEditor, create form', () => {
  it('lists the shown fields, then the hidden ones in the tray by name', () => {
    editor('create', createTemplate, createChoices);
    expect(order()).toEqual(['priority', 'description', 'assignee', 'customfield_7']);
    expect(tray()).toEqual(['environment']);
    expect(document.body.textContent).toContain('Only in 2 projects');
  });

  it('moves rows with the arrows', () => {
    editor('create', createTemplate, createChoices);
    click(byId('order-down-layout-create-priority'));
    expect(order()).toEqual(['description', 'priority', 'assignee', 'customfield_7']);
    click(byId('order-up-layout-create-customfield_7'));
    expect(order()).toEqual(['description', 'priority', 'customfield_7', 'assignee']);
    expect(byId<HTMLButtonElement>('order-up-layout-create-description').disabled).toBe(true);
  });

  it('keeps the fixed fields in the header, where nothing moves or hides them', () => {
    editor('create', createTemplate, createChoices);
    const head = document.querySelector('.mock-head') as HTMLElement;
    expect(head.textContent).toContain('Project · Issue type · Summary');
    expect(head.querySelector('button, input, [draggable]')).toBeNull();
    for (const id of ['project', 'issuetype', 'summary']) {
      expect(document.querySelector(`[data-field="${id}"]`)).toBeNull();
    }
  });

  it('hides a field from its menu and shows it again from the tray', async () => {
    editor('create', createTemplate, createChoices);
    await pick('Actions for Description', 'Hide');
    expect(order()).toEqual(['priority', 'assignee', 'customfield_7']);
    expect(tray()).toEqual(['description', 'environment']);
    click(document.querySelector('button[aria-label="Show Environment"]') as HTMLElement);
    expect(order()).toEqual(['priority', 'assignee', 'customfield_7', 'environment']);
  });

  it('saves the rows as the template body; hiding clears Required', async () => {
    const saved = editor('create', createTemplate, createChoices);
    expect(document.querySelector('.savebar')).toBeNull();
    const required = (field: string) =>
      byId(`layout-create-${field}`).querySelector('.req') as HTMLButtonElement;
    expect(required('description').getAttribute('aria-pressed')).toBe('true');
    click(byId('order-down-layout-create-priority'));
    click(required('assignee'));
    await pick('Actions for Description', 'Hide');
    expect(document.querySelector('.savebar')).not.toBeNull();
    await save();
    expect(saved).toEqual([
      [
        { field: 'description', hidden: true, required: false },
        { field: 'priority', hidden: false, required: false },
        { field: 'assignee', hidden: false, required: true },
        { field: 'environment', hidden: true, required: false },
        { field: 'customfield_7', hidden: false, required: false },
      ],
    ]);
  });
});

const viewTemplate: LayoutField[] = [
  { field: 'description', zone: 'main', hidden: false },
  { field: 'assignee', zone: 'side', hidden: false },
  { field: 'priority', zone: 'side', hidden: false },
  { field: 'labels', zone: 'side', hidden: true },
];

describe('LayoutEditor, issue view', () => {
  it('moves rows within their column with the arrows', () => {
    editor('view', viewTemplate, viewChoices);
    expect(order('main')).toEqual(['description']);
    expect(order('side')).toEqual(['assignee', 'priority']);
    expect(byId<HTMLButtonElement>('order-down-layout-view-description').disabled).toBe(true);
    click(byId('order-up-layout-view-priority'));
    expect(order('side')).toEqual(['priority', 'assignee']);
  });

  it('moves a row to the other column from its menu', async () => {
    editor('view', viewTemplate, viewChoices);
    await pick('Actions for Priority', 'Main');
    expect(order('main')).toEqual(['description', 'priority']);
    expect(order('side')).toEqual(['assignee']);
  });

  it('shows a hidden field in the column chosen from the tray', async () => {
    editor('view', viewTemplate, viewChoices);
    expect(tray()).toEqual(['labels']);
    await pick('Show Labels', 'Main');
    expect(order('main')).toEqual(['description', 'labels']);
    expect(tray()).toEqual([]);
  });

  it('has no Required', () => {
    editor('view', viewTemplate, viewChoices);
    expect(document.querySelector('.req')).toBeNull();
  });

  it('saves zones, not required', async () => {
    const saved = editor('view', viewTemplate, viewChoices);
    await pick('Actions for Assignee', 'Main');
    await pick('Actions for Priority', 'Hide');
    await save();
    expect(saved).toEqual([
      [
        { field: 'description', zone: 'main', hidden: false },
        { field: 'priority', zone: 'side', hidden: true },
        { field: 'labels', zone: 'side', hidden: true },
        { field: 'assignee', zone: 'main', hidden: false },
      ],
    ]);
  });
});

describe('layoutRows', () => {
  it('drops unknown fields and appends the missing ones as the server does', () => {
    const rows = layoutRows(
      'view',
      [
        { field: 'customfield_9', zone: 'main', hidden: false },
        { field: 'assignee', zone: 'side', hidden: false, required: true },
      ],
      [
        { id: 'assignee', name: 'Assignee' },
        { id: 'description', name: 'Description' },
        { id: 'customfield_7', name: 'Team' },
      ],
    );
    expect(rows).toEqual([
      { field: 'assignee', zone: 'side', hidden: false },
      { field: 'customfield_7', zone: 'side', hidden: false },
      { field: 'description', zone: 'main', hidden: true },
    ]);
  });
});

describe('dropRow', () => {
  const rows: LayoutField[] = [
    { field: 'a', zone: 'main', hidden: false },
    { field: 'b', zone: 'side', hidden: false },
    { field: 'c', zone: 'side', hidden: false },
    { field: 'h', zone: 'side', hidden: true },
  ];
  const ids = (r: LayoutField[]) => r.map((x) => `${x.field}:${x.hidden ? 'hidden' : x.zone}`);

  it('takes the place of the row it lands on', () => {
    expect(ids(dropRow(rows, 'c', { field: 'b', zone: 'side' }))).toEqual([
      'a:main',
      'c:side',
      'b:side',
      'h:hidden',
    ]);
  });

  it('goes before a row of another column', () => {
    expect(ids(dropRow(rows, 'b', { field: 'a', zone: 'main' }))).toEqual([
      'b:main',
      'a:main',
      'c:side',
      'h:hidden',
    ]);
  });

  it('hides in the tray and comes back from it', () => {
    expect(ids(dropRow(rows, 'a', { tray: true }))[0]).toBe('a:hidden');
    expect(ids(dropRow(rows, 'h', { field: 'a', zone: 'main' }))).toEqual([
      'h:main',
      'a:main',
      'b:side',
      'c:side',
    ]);
  });
});
