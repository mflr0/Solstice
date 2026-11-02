// @vitest-environment jsdom
import { afterAll, afterEach, beforeAll, describe, expect, it } from 'vitest';
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

describe('Dropdown search', () => {
  const languages = [
    { value: 'en', label: 'English' },
    { value: 'fr', label: 'Français', keywords: ['fr', 'French'] },
    { value: 'de', label: 'Deutsch', keywords: ['de', 'German'] },
    { value: 'ft', label: 'Fête' },
  ];

  // jsdom has no showPopover: its popovers stay hidden, and nothing hidden takes the focus.
  const shownPopovers = document.createElement('style');
  shownPopovers.textContent = '[popover] { display: block !important; }';
  beforeAll(() => {
    document.head.append(shownPopovers);
  });
  afterAll(() => shownPopovers.remove());

  const field = () => document.querySelector('input.filter') as HTMLInputElement | null;
  const shown = () =>
    [...document.querySelectorAll('[role="option"]')].map((o) => o.textContent?.trim());
  const pointed = () =>
    document
      .getElementById(field()?.getAttribute('aria-activedescendant') ?? '')
      ?.textContent?.trim();

  function type(text: string) {
    const input = field()!;
    input.value = text;
    input.dispatchEvent(new Event('input', { bubbles: true }));
    flushSync();
  }

  function open(value = 'en') {
    const d = dropdown(value, { options: languages, search: true });
    press(d.button, 'ArrowDown');
    return d;
  }

  it('opens with the field first, focused, named and empty', () => {
    open();
    const input = field()!;
    expect(input).not.toBeNull();
    expect(document.activeElement).toBe(input);
    expect(input.value).toBe('');
    expect(input.getAttribute('aria-label')).toBe('Filter the options');
    expect(input.placeholder).toBe('Filter…');
    expect(input.parentElement?.firstElementChild).toBe(input);
    expect(shown()).toEqual(['English', 'Français', 'Deutsch', 'Fête']);
    expect(pointed()).toBe('English');
  });

  it('filters as the user types, ignoring case and accents', () => {
    open();
    type('FRANC');
    expect(shown()).toEqual(['Français']);
    type('fete');
    expect(shown()).toEqual(['Fête']);
    type('');
    expect(shown()).toHaveLength(4);
  });

  it('finds an option by its keywords without showing them', () => {
    open();
    type('german');
    expect(shown()).toEqual(['Deutsch']);
    expect(list()?.textContent).not.toContain('German');
  });

  it('moves through the filtered options with the arrows and picks one with Enter', () => {
    const { button, chosen } = open();
    type('f');
    expect(shown()).toEqual(['Français', 'Fête']);
    expect(pointed()).toBe('Français');
    press(field()!, 'ArrowDown');
    expect(pointed()).toBe('Fête');
    press(field()!, 'ArrowDown');
    expect(pointed()).toBe('Fête');
    press(field()!, 'ArrowUp');
    expect(pointed()).toBe('Français');
    expect(document.activeElement).toBe(field());
    press(field()!, 'Enter');
    expect(chosen).toEqual(['fr']);
    expect(list()).toBeNull();
    expect(document.activeElement).toBe(button);
  });

  it('closes on Escape, gives the focus back to the button and keeps the key', () => {
    const { button, chosen } = open();
    let outer = 0;
    const count = (e: KeyboardEvent) => {
      if (e.key === 'Escape') outer++;
    };
    document.body.addEventListener('keydown', count);
    type('de');
    press(field()!, 'Escape');
    expect(list()).toBeNull();
    expect(document.activeElement).toBe(button);
    expect(chosen).toEqual([]);
    expect(outer).toBe(0);
    document.body.removeEventListener('keydown', count);
  });

  it('says so when no option matches', () => {
    const { chosen } = open();
    expect(document.querySelector('.no-match')?.textContent).toBe('');
    type('zz');
    expect(shown()).toEqual([]);
    expect(document.querySelector('[role="status"]')?.textContent).toBe('No option matches.');
    expect(field()!.hasAttribute('aria-activedescendant')).toBe(false);
    press(field()!, 'Enter');
    expect(chosen).toEqual([]);
  });

  it('starts the search with a letter typed on the closed button', () => {
    const { button } = dropdown('en', { options: languages, search: true });
    press(button, 'd');
    expect(field()?.value).toBe('d');
    expect(shown()).toEqual(['Deutsch']);
  });

  it('stays open while the focus moves between the button and the field', () => {
    let blurred = 0;
    const { button } = dropdown('en', {
      options: languages,
      search: true,
      onblur: () => blurred++,
    });
    press(button, 'ArrowDown');
    expect(document.activeElement).toBe(field());
    expect(list()).not.toBeNull();
    field()!.blur();
    flushSync();
    expect(list()).toBeNull();
    expect(blurred).toBe(1);
  });

  it('takes the search texts it is given, for a page without the catalogue', () => {
    const { button } = dropdown('en', {
      options: languages,
      search: true,
      messages: { filter: 'Filtrer…', filterOptions: 'Filtrer les options', noMatch: 'Rien.' },
    });
    press(button, 'ArrowDown');
    expect(field()!.placeholder).toBe('Filtrer…');
    expect(field()!.getAttribute('aria-label')).toBe('Filtrer les options');
    field()!.value = 'zzz';
    field()!.dispatchEvent(new Event('input', { bubbles: true }));
    flushSync();
    expect(document.querySelector('.no-match')?.textContent).toBe('Rien.');
  });

  it('has no field without the option', () => {
    const { button } = dropdown();
    press(button, 'ArrowDown');
    expect(list()).not.toBeNull();
    expect(field()).toBeNull();
    expect(document.activeElement).toBe(button);
  });
});
