// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { flushSync, mount, unmount } from 'svelte';

import type { CustomFieldType, OfferedField, UserRef } from '../../src/core/types.js';
import { emptyDraft, restValue, type CustomDraft } from '../../src/core/custom-fields.js';
import CustomFieldInput from '../../src/ui/CustomFieldInput.svelte';

const bob: UserRef = { id: 4, username: 'bob', name: 'Bob Weber', active: true };

vi.mock('../../src/lib/api.js', () => ({
  api: { users: () => Promise.resolve([bob]) },
  errorMessage: String,
  pictureUrl: (id: number) => `/api/users/${id}/picture`,
  hasPicture: () => Promise.resolve(false),
}));

const options = [
  { id: 10, value: 'Red' },
  { id: 11, value: 'Green' },
];

function fieldOf(type: CustomFieldType): OfferedField {
  return { id: 7, name: 'Colour', type, typeKey: '', options };
}

let component: ReturnType<typeof mount> | undefined;
let sent: unknown;

beforeEach(() => {
  // jsdom has no top layer.
  HTMLElement.prototype.showPopover = function () {};
  sent = 'nothing';
});

afterEach(() => {
  if (component) unmount(component);
  component = undefined;
  delete (HTMLElement.prototype as Partial<HTMLElement>).showPopover;
});

function editor(type: CustomFieldType, value?: CustomDraft) {
  const field = fieldOf(type);
  component = mount(CustomFieldInput, {
    target: document.body,
    props: {
      field,
      id: 'cf',
      value: value ?? emptyDraft(field),
      onchange: (v: CustomDraft) => (sent = restValue(field, v)),
    },
  });
  flushSync();
}

function type(value: string) {
  const input = document.querySelector('#cf') as HTMLInputElement;
  input.value = value;
  input.dispatchEvent(new Event('input', { bubbles: true }));
  flushSync();
}

const settle = async () => {
  await new Promise((r) => setTimeout(r));
  flushSync();
};

describe('CustomFieldInput', () => {
  it.each([
    ['text', 'text', 'Hello', 'Hello'],
    ['url', 'url', 'https://example.org', 'https://example.org'],
    ['number', 'number', '3.5', 3.5],
    ['date', 'date', '2026-11-09', '2026-11-09'],
  ] as const)('%s: a %s input gives %j', (kind, input, typed, value) => {
    editor(kind);
    expect((document.querySelector('#cf') as HTMLInputElement).type).toBe(input);
    type(typed);
    expect(sent).toEqual(value);
  });

  it('datetime: gives the local time typed as an ISO instant', () => {
    editor('datetime');
    expect((document.querySelector('#cf') as HTMLInputElement).type).toBe('datetime-local');
    type('2026-11-09T14:30');
    expect(sent).toBe(new Date('2026-11-09T14:30').toISOString());
  });

  it('an emptied input sends nothing', () => {
    editor('text', 'Hello');
    type('  ');
    expect(sent).toBeUndefined();
  });

  it('textarea: gives the wiki markup', () => {
    expect(restValue(fieldOf('textarea'), '*bold*')).toBe('*bold*');
    expect(restValue(fieldOf('textarea'), '')).toBeUndefined();
  });

  it('select: gives the chosen option by id, nothing for None', () => {
    editor('select');
    (document.querySelector('[role="combobox"]') as HTMLButtonElement).click();
    flushSync();
    const items = [...document.querySelectorAll<HTMLElement>('[role="option"]')];
    expect(items.map((i) => i.textContent?.trim())).toEqual(['None', 'Red', 'Green']);
    items[2]!.click();
    flushSync();
    expect(sent).toEqual({ id: '11' });
  });

  it('radio: gives the chosen option by id', () => {
    editor('radio');
    const radios = [...document.querySelectorAll<HTMLInputElement>('input[type="radio"]')];
    expect(radios).toHaveLength(3);
    radios[1]!.click();
    flushSync();
    expect(sent).toEqual({ id: '10' });
  });

  it('checkboxes: gives the ticked options by id', () => {
    editor('checkboxes', [10]);
    const boxes = [...document.querySelectorAll<HTMLInputElement>('input[type="checkbox"]')];
    expect(boxes.map((b) => b.checked)).toEqual([true, false]);
    boxes[1]!.click();
    flushSync();
    expect(sent).toEqual([{ id: '10' }, { id: '11' }]);
  });

  it('multiselect: gives the chosen options by id', () => {
    editor('multiselect');
    (document.querySelector('button.current') as HTMLButtonElement).click();
    flushSync();
    (document.querySelectorAll('[role="menuitemcheckbox"]')[1] as HTMLButtonElement).click();
    flushSync();
    (document.querySelector('button.current') as HTMLButtonElement).click();
    flushSync();
    expect(sent).toEqual([{ id: '11' }]);
  });

  it('labels: gives the labels', () => {
    editor('labels');
    const input = document.querySelector('#cf') as HTMLInputElement;
    input.value = 'urgent';
    input.dispatchEvent(new Event('input', { bubbles: true }));
    input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
    flushSync();
    expect(sent).toEqual(['urgent']);
  });

  it('user: gives the person by username', async () => {
    editor('user');
    (document.querySelector('button.current') as HTMLButtonElement).click();
    await settle();
    const item = [...document.querySelectorAll<HTMLButtonElement>('button.item')].find((b) =>
      b.textContent?.includes(bob.name),
    );
    item!.click();
    flushSync();
    expect(sent).toEqual({ name: 'bob' });
  });

  it('multiuser: adds people by username, removes them', async () => {
    const alice: UserRef = { id: 3, username: 'alice', name: 'Alice Martin', active: true };
    editor('multiuser', [alice]);
    (document.querySelector('button.current') as HTMLButtonElement).click();
    await settle();
    (document.querySelector('button.item') as HTMLButtonElement).click();
    flushSync();
    expect(sent).toEqual([{ name: 'alice' }, { name: 'bob' }]);

    (document.querySelector('.chip button') as HTMLButtonElement).click();
    flushSync();
    expect(sent).toBeUndefined();
  });
});
