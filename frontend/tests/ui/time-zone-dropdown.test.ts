// @vitest-environment jsdom
import { afterAll, afterEach, beforeAll, describe, expect, it } from 'vitest';
import { flushSync, mount, unmount } from 'svelte';

import { zoneChoices, zoneOffset } from '../../src/core/setup.js';
import TimeZoneDropdown from '../../src/ui/TimeZoneDropdown.svelte';

const zones = zoneChoices('en-US');

// jsdom has no showPopover: its popovers stay hidden, and nothing hidden takes the focus.
const shownPopovers = document.createElement('style');
shownPopovers.textContent = '[popover] { display: block !important; }';
beforeAll(() => document.head.append(shownPopovers));
afterAll(() => shownPopovers.remove());

let component: ReturnType<typeof mount> | undefined;
afterEach(() => {
  if (component) unmount(component);
  component = undefined;
});

function open(value: string, extra: Record<string, unknown> = {}) {
  const chosen: string[] = [];
  component = mount(TimeZoneDropdown, {
    target: document.body,
    props: { zones, value, label: 'Time zone', onchange: (v: string) => chosen.push(v), ...extra },
  });
  flushSync();
  const button = document.querySelector('button.dropdown') as HTMLButtonElement;
  button.focus();
  button.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true }));
  flushSync();
  return { button, chosen };
}

function type(text: string) {
  const input = document.querySelector('input.filter') as HTMLInputElement;
  input.value = text;
  input.dispatchEvent(new Event('input', { bubbles: true }));
  flushSync();
}

const names = () =>
  [...document.querySelectorAll('[role="option"] .name')].map((o) => o.textContent?.trim());

describe('TimeZoneDropdown', () => {
  it('shows the zone behind its offset badge', () => {
    const { button } = open('Europe/Paris');
    expect(button.querySelector('.offset')?.textContent).toBe(zoneOffset('Europe/Paris'));
    expect(button.querySelector('.name')?.textContent).toBe('Europe/Paris');
  });

  it('finds a zone by its offset, as UTC or GMT', () => {
    open('UTC');
    const paris = zoneOffset('Europe/Paris')
      .slice(3)
      .replace(/^([+-])0?(\d+):00$/, '$1$2');
    type(`UTC${paris}`);
    expect(names()).toContain('Europe/Paris');
    type(`GMT${paris}`);
    expect(names()).toContain('Europe/Paris');
    type('paris');
    expect(names()).toEqual(['Europe/Paris']);
  });

  it('keeps a value no zone has, and lists the first option before the zones', () => {
    open('Mars/Olympus', { first: { value: '', label: 'The server’s' } });
    expect(names().slice(0, 3)).toEqual(['The server’s', 'Mars/Olympus', 'UTC']);
  });
});
