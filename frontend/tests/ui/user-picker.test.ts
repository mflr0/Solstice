// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { flushSync, mount, unmount } from 'svelte';

import type { UserRef } from '../../src/core/types.js';
import { t } from '../../src/i18n/t.js';
import UserPicker from '../../src/ui/UserPicker.svelte';

const alice: UserRef = { id: 3, username: 'alice', name: 'Alice Martin', active: true };

const bob: UserRef = { id: 4, username: 'bob', name: 'Bob Weber', active: true };

let component: ReturnType<typeof mount> | undefined;
let shown: HTMLElement[] = [];

beforeEach(() => {
  // jsdom has no top layer.
  HTMLElement.prototype.showPopover = function (this: HTMLElement) {
    shown.push(this);
  };
});

afterEach(() => {
  if (component) unmount(component);
  component = undefined;
  shown = [];
  delete (HTMLElement.prototype as Partial<HTMLElement>).showPopover;
});

function picker(onpick: (user: UserRef | null) => void = () => {}) {
  component = mount(UserPicker, {
    target: document.body,
    props: {
      value: alice,
      label: t.issue.assignee,
      search: () => Promise.resolve([alice, bob]),
      onpick,
    },
  });
  flushSync();
  return document.querySelector('button.current') as HTMLButtonElement;
}

describe('UserPicker', () => {
  it('shows the person without a link inside the button', () => {
    const button = picker();
    expect(button.querySelector('a')).toBeNull();
    expect(button.textContent).toContain(alice.name);
  });

  it('opens the list when the person is clicked', () => {
    const button = picker();
    (button.querySelector('.name') as HTMLElement).click();
    flushSync();
    expect(document.querySelector('[role="listbox"]')).not.toBeNull();
  });

  it('opens the list in the top layer', () => {
    const button = picker();
    button.click();
    flushSync();
    const popover = document.querySelector('.popover') as HTMLElement;
    expect(popover.getAttribute('popover')).toBe('manual');
    expect(shown).toEqual([popover]);
    expect(popover.contains(document.querySelector('[role="listbox"]'))).toBe(true);
  });

  it('closes on a pick, Escape and a press outside', async () => {
    const picked: (UserRef | null)[] = [];
    const button = picker((u) => picked.push(u));
    const open = async () => {
      button.click();
      await new Promise((r) => setTimeout(r));
      flushSync();
      expect(document.querySelector('.popover')).not.toBeNull();
    };
    const closed = () => document.querySelector('.popover') === null;

    await open();
    const item = [...document.querySelectorAll('button.item')].find((b) =>
      b.textContent?.includes(bob.name),
    ) as HTMLButtonElement;
    item.click();
    flushSync();
    expect(closed()).toBe(true);
    expect(picked).toEqual([bob]);

    await open();
    const filter = document.querySelector('input.filter') as HTMLInputElement;
    const escape = new KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true });
    filter.dispatchEvent(escape);
    flushSync();
    expect(closed()).toBe(true);
    // A dialog around it stays open.
    expect(escape.defaultPrevented).toBe(true);

    await open();
    document.body.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true }));
    flushSync();
    expect(closed()).toBe(true);
  });
});
