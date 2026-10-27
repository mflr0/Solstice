// @vitest-environment jsdom
import { afterEach, describe, expect, it } from 'vitest';
import { flushSync, mount, unmount } from 'svelte';

import { t } from '../../src/i18n/t.js';
import MultiPick from '../../src/ui/MultiPick.svelte';

const options = [
  { id: 1, name: 'Backend' },
  { id: 2, name: 'Frontend' },
  { id: 3, name: 'Docs' },
];

let component: ReturnType<typeof mount> | undefined;

afterEach(() => {
  if (component) unmount(component);
  component = undefined;
});

function open(oncreate?: (name: string) => Promise<number>, onchange = (_: number[]) => {}) {
  component = mount(MultiPick, {
    target: document.body,
    props: { options, selected: [], label: t.issue.components, onchange, oncreate },
  });
  flushSync();
  (document.querySelector('button.current') as HTMLButtonElement).click();
  flushSync();
}

function type(text: string) {
  const input = document.querySelector('input.filter') as HTMLInputElement;
  input.value = text;
  input.dispatchEvent(new Event('input'));
  flushSync();
}

const names = () =>
  [...document.querySelectorAll('[role="menuitemcheckbox"]')].map((b) => b.textContent);
const createItem = () => document.querySelector('.item.create');

describe('MultiPick', () => {
  it('narrows the options by name, ignoring case', () => {
    open();
    expect(names()).toEqual(['Backend', 'Frontend', 'Docs']);
    type('END');
    expect(names()).toEqual(['Backend', 'Frontend']);
  });

  it('offers no Create without oncreate', () => {
    open();
    type('Mobile');
    expect(createItem()).toBeNull();
  });

  it('offers Create for a new name only', () => {
    open(() => Promise.resolve(4));
    type('docs');
    expect(createItem()).toBeNull();
    type('Mobile');
    expect(createItem()?.textContent?.trim()).toBe(t.common.createNamed('Mobile'));
  });

  it('selects the value it creates', async () => {
    let asked = '';
    let chosen: number[] = [];
    open(
      (name) => {
        asked = name;
        return Promise.resolve(4);
      },
      (ids) => (chosen = ids),
    );
    type('  Mobile ');
    (createItem() as HTMLButtonElement).click();
    await Promise.resolve();
    await Promise.resolve();
    flushSync();
    expect(asked).toBe('Mobile');
    expect((document.querySelector('input.filter') as HTMLInputElement).value).toBe('');
    (document.querySelector('button.current') as HTMLButtonElement).click();
    flushSync();
    expect(chosen).toEqual([4]);
  });
});
