// @vitest-environment jsdom
import { afterEach, describe, expect, it } from 'vitest';
import { flushSync, mount, unmount } from 'svelte';

import Dropdown from '../../src/ui/Dropdown.svelte';

const options = [
  { value: 'hi', label: 'Highest' },
  { value: 'h', label: 'High' },
  { value: 'm', label: 'Medium' },
  { value: 'l', label: 'Low' },
];

let component: ReturnType<typeof mount> | undefined;

afterEach(() => {
  if (component) unmount(component);
  component = undefined;
});

function dropdown(value = 'm', extra: Record<string, unknown> = {}) {
  const chosen: string[] = [];
  component = mount(Dropdown, {
    target: document.body,
    props: { options, value, label: 'Priority', onchange: (v: string) => chosen.push(v), ...extra },
  });
  flushSync();
  const button = document.querySelector('button.dropdown') as HTMLButtonElement;
  button.focus();
  return { button, chosen };
}

function press(button: HTMLElement, key: string, init: KeyboardEventInit = {}) {
  button.dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true, ...init }));
  flushSync();
}

const list = () => document.querySelector('[role="listbox"]');
const highlighted = (button: HTMLElement) =>
  document.getElementById(button.getAttribute('aria-activedescendant') ?? '')?.textContent?.trim();

describe('Dropdown', () => {
  it('shows the value and opens on the selected option', () => {
    const { button } = dropdown();
    expect(button.textContent?.trim()).toBe('Medium');
    expect(list()).toBeNull();
    press(button, 'ArrowDown');
    expect(button.getAttribute('aria-expanded')).toBe('true');
    expect(highlighted(button)).toBe('Medium');
    expect(document.querySelector('[aria-selected="true"]')?.textContent?.trim()).toBe('Medium');
  });

  it('moves with the arrows, Home and End, without wrapping', () => {
    const { button } = dropdown();
    press(button, 'ArrowDown');
    press(button, 'ArrowDown');
    expect(highlighted(button)).toBe('Low');
    press(button, 'ArrowDown');
    expect(highlighted(button)).toBe('Low');
    press(button, 'Home');
    expect(highlighted(button)).toBe('Highest');
    press(button, 'ArrowUp');
    expect(highlighted(button)).toBe('Highest');
    press(button, 'End');
    expect(highlighted(button)).toBe('Low');
  });

  it('jumps to an option by typing its first letters', () => {
    const { button } = dropdown();
    press(button, 'ArrowDown');
    press(button, 'h');
    expect(highlighted(button)).toBe('Highest');
    press(button, 'h');
    expect(highlighted(button)).toBe('High');
    press(button, 'l');
    expect(highlighted(button)).toBe('High');
  });

  it('selects with Enter and Space, closing the list', () => {
    const { button, chosen } = dropdown();
    press(button, 'ArrowDown');
    press(button, 'ArrowUp');
    press(button, 'Enter');
    expect(chosen).toEqual(['h']);
    expect(list()).toBeNull();
    expect(document.activeElement).toBe(button);
    press(button, ' ');
    press(button, 'End');
    press(button, ' ');
    expect(chosen).toEqual(['h', 'l']);
  });

  it('selects an option on a click', () => {
    const { button, chosen } = dropdown();
    button.click();
    flushSync();
    (document.querySelectorAll('[role="option"]')[0] as HTMLElement).click();
    flushSync();
    expect(chosen).toEqual(['hi']);
    expect(list()).toBeNull();
  });

  it('closes on Escape without changing the value or letting the key through', () => {
    const { button, chosen } = dropdown();
    let outer = 0;
    const count = (e: KeyboardEvent) => {
      if (e.key === 'Escape') outer++;
    };
    document.body.addEventListener('keydown', count);
    press(button, 'ArrowDown');
    press(button, 'ArrowDown');
    press(button, 'Escape');
    expect(list()).toBeNull();
    expect(button.getAttribute('aria-expanded')).toBe('false');
    expect(chosen).toEqual([]);
    expect(document.activeElement).toBe(button);
    expect(outer).toBe(0);
    press(button, 'Escape');
    expect(outer).toBe(1);
    document.body.removeEventListener('keydown', count);
  });

  it('lists grouped options under their heading, which the keys skip', () => {
    const grouped = [
      { value: 'a', label: 'Alpha', group: 'Letters' },
      { value: 'b', label: 'Beta', group: 'Letters' },
      { value: '1', label: 'One', group: 'Numbers' },
    ];
    const { button, chosen } = dropdown('b', { options: grouped });
    press(button, 'ArrowDown');
    const groups = [...document.querySelectorAll('[role="group"]')];
    expect(
      groups.map((g) => document.getElementById(g.getAttribute('aria-labelledby')!)?.textContent),
    ).toEqual(['Letters', 'Numbers']);
    expect(groups[1]!.querySelectorAll('[role="option"]')).toHaveLength(1);
    press(button, 'ArrowDown');
    expect(highlighted(button)).toBe('One');
    press(button, 'Enter');
    expect(chosen).toEqual(['1']);
  });

  it('shows the placeholder when no option matches, without listing it', () => {
    const { button } = dropdown('', { placeholder: 'Choose…' });
    expect(button.textContent?.trim()).toBe('Choose…');
    press(button, 'ArrowDown');
    expect(highlighted(button)).toBe('Highest');
    expect(document.querySelectorAll('[role="option"]')).toHaveLength(options.length);
    expect(list()?.textContent).not.toContain('Choose…');
  });

  it('closes when the focus leaves it', () => {
    let blurred = 0;
    const { button } = dropdown('m', { onblur: () => blurred++ });
    press(button, 'ArrowDown');
    expect(list()).not.toBeNull();
    button.blur();
    flushSync();
    expect(list()).toBeNull();
    expect(blurred).toBe(1);
  });
});

describe('Dropdown required', () => {
  it('tells assistive technology the choice is required', () => {
    const { button } = dropdown('m', { required: true });
    expect(button.getAttribute('aria-required')).toBe('true');
  });

  it('says nothing when it is optional', () => {
    const { button } = dropdown();
    expect(button.hasAttribute('aria-required')).toBe(false);
  });
});
