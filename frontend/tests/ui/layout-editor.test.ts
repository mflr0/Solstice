// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest';
import { flushSync, mount, tick, unmount } from 'svelte';

import type { LayoutField, LayoutForm } from '../../src/core/admin-types.js';
import { layoutRows, type LayoutChoice } from '../../src/ui/layouts/layout-rows.js';
import LayoutEditor from '../../src/ui/layouts/LayoutEditor.svelte';

vi.stubGlobal(
  'ResizeObserver',
  class {
    observe() {}
    disconnect() {}
  },
);

const createChoices: LayoutChoice[] = [
  { id: 'priority', name: 'Priority' },
  { id: 'description', name: 'Description' },
  { id: 'assignee', name: 'Assignee' },
  { id: 'customfield_7', name: 'Team', note: 'Only in 2 projects' },
];

const viewChoices: LayoutChoice[] = [
  { id: 'description', name: 'Description' },
  { id: 'assignee', name: 'Assignee' },
  { id: 'priority', name: 'Priority' },
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

function click(id: string) {
  byId(id).click();
  flushSync();
}

/** The names of the movable rows, top to bottom, by zone in the view. */
function order(zone?: 'main' | 'side') {
  const scope = zone ? `.layout-list[data-zone="${zone}"]` : '.layout-list';
  return [...document.querySelectorAll<HTMLElement>(`${scope} .layout-row[data-field]`)].map(
    (r) => r.dataset['field'],
  );
}

async function save() {
  const form = document.querySelector('form.layout-editor') as HTMLFormElement;
  form.dispatchEvent(new SubmitEvent('submit', { bubbles: true, cancelable: true }));
  await tick();
}

const createTemplate: LayoutField[] = [
  { field: 'priority', hidden: false },
  { field: 'description', hidden: false, required: true },
  { field: 'assignee', hidden: true },
];

describe('LayoutEditor, create form', () => {
  it('lists the template, then the custom fields it misses', () => {
    editor('create', createTemplate, createChoices);
    expect(order()).toEqual(['priority', 'description', 'assignee', 'customfield_7']);
    expect(document.body.textContent).toContain('Only in 2 projects');
  });

  it('moves rows with the arrows', () => {
    editor('create', createTemplate, createChoices);
    click('order-down-layout-create-priority');
    expect(order()).toEqual(['description', 'priority', 'assignee', 'customfield_7']);
    click('order-up-layout-create-customfield_7');
    expect(order()).toEqual(['description', 'priority', 'customfield_7', 'assignee']);
    expect(byId<HTMLButtonElement>('order-up-layout-create-description').disabled).toBe(true);
  });

  it('keeps the fixed fields first, without arrows or switches', () => {
    editor('create', createTemplate, createChoices);
    const fixed = [...document.querySelectorAll('.layout-row.is-fixed')];
    expect(fixed.map((r) => r.querySelector('.label')?.textContent)).toEqual([
      'Project',
      'Issue type',
      'Summary',
    ]);
    for (const r of fixed) {
      expect(r.querySelector('button, input')).toBeNull();
      expect(r.getAttribute('draggable')).toBeNull();
    }
    const rows = [...document.querySelectorAll('.layout-list > li')];
    expect(rows.slice(0, 3).every((r) => r.classList.contains('is-fixed'))).toBe(true);
    for (const id of ['project', 'issuetype', 'summary']) {
      expect(document.getElementById(`shown-layout-create-${id}`)).toBeNull();
    }
  });

  it('offers Required, not on a hidden field', () => {
    editor('create', createTemplate, createChoices);
    expect(byId<HTMLInputElement>('required-layout-create-description').checked).toBe(true);
    expect(byId<HTMLInputElement>('required-layout-create-assignee').disabled).toBe(true);
  });

  it('saves the rows as the template body', async () => {
    const saved = editor('create', createTemplate, createChoices);
    expect(document.querySelector('.savebar')).toBeNull();
    click('order-down-layout-create-priority');
    click('shown-layout-create-description');
    click('shown-layout-create-assignee');
    click('required-layout-create-assignee');
    expect(document.querySelector('.savebar')).not.toBeNull();
    await save();
    expect(saved).toEqual([
      [
        { field: 'description', hidden: true, required: false },
        { field: 'priority', hidden: false, required: false },
        { field: 'assignee', hidden: false, required: true },
        { field: 'customfield_7', hidden: false, required: false },
      ],
    ]);
  });
});

const viewTemplate: LayoutField[] = [
  { field: 'description', zone: 'main', hidden: false },
  { field: 'assignee', zone: 'side', hidden: false },
  { field: 'priority', zone: 'side', hidden: false },
];

async function chooseZone(field: string, key: 'Home' | 'End') {
  const button = byId(`zone-layout-view-${field}`);
  button.focus();
  for (const k of ['ArrowDown', key, 'Enter']) {
    button.dispatchEvent(new KeyboardEvent('keydown', { key: k, bubbles: true }));
    flushSync();
  }
  await tick();
}

describe('LayoutEditor, issue view', () => {
  it('moves rows within their zone with the arrows', () => {
    editor('view', viewTemplate, viewChoices);
    expect(order('main')).toEqual(['description']);
    expect(order('side')).toEqual(['assignee', 'priority']);
    expect(byId<HTMLButtonElement>('order-down-layout-view-description').disabled).toBe(true);
    click('order-up-layout-view-priority');
    expect(order('side')).toEqual(['priority', 'assignee']);
  });

  it('moves a row to the other zone with its zone select', async () => {
    editor('view', viewTemplate, viewChoices);
    await chooseZone('priority', 'Home');
    expect(order('main')).toEqual(['description', 'priority']);
    expect(order('side')).toEqual(['assignee']);
    await chooseZone('description', 'End');
    expect(order('main')).toEqual(['priority']);
    expect(order('side')).toEqual(['assignee', 'description']);
  });

  it('has no Required switch', () => {
    editor('view', viewTemplate, viewChoices);
    expect(document.querySelector('[id^="required-"]')).toBeNull();
    expect(document.querySelectorAll('[id^="shown-"]')).toHaveLength(3);
  });

  it('saves zones, not required', async () => {
    const saved = editor('view', viewTemplate, viewChoices);
    await chooseZone('assignee', 'Home');
    click('shown-layout-view-priority');
    await save();
    expect(saved).toEqual([
      [
        { field: 'description', zone: 'main', hidden: false },
        { field: 'priority', zone: 'side', hidden: true },
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
