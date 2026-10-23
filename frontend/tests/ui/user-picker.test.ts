// @vitest-environment jsdom
import { afterEach, describe, expect, it } from 'vitest';
import { flushSync, mount, unmount } from 'svelte';

import type { UserRef } from '../../src/core/types.js';
import { t } from '../../src/i18n/t.js';
import UserPicker from '../../src/ui/UserPicker.svelte';

const alice: UserRef = { id: 3, username: 'alice', name: 'Alice Martin', active: true };

let component: ReturnType<typeof mount> | undefined;

afterEach(() => {
  if (component) unmount(component);
  component = undefined;
});

function picker() {
  component = mount(UserPicker, {
    target: document.body,
    props: {
      value: alice,
      label: t.issue.assignee,
      search: () => Promise.resolve([alice]),
      onpick: () => {},
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
});
