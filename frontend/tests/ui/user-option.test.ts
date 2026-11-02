// @vitest-environment jsdom
import { afterEach, describe, expect, it } from 'vitest';
import { flushSync, mount, unmount } from 'svelte';

import type { UserRef } from '../../src/core/types.js';
import { t } from '../../src/i18n/t.js';
import UserOption from '../../src/ui/UserOption.svelte';

let component: ReturnType<typeof mount> | undefined;

afterEach(() => {
  if (component) unmount(component);
  component = undefined;
  document.body.innerHTML = '';
});

function row(user: UserRef, search = '') {
  const target = document.createElement('div');
  document.body.append(target);
  component = mount(UserOption, {
    target,
    props: { user, name: user.name, username: user.username, search },
  });
  flushSync();
  return target;
}

describe('UserOption', () => {
  it('shows the avatar, the name and the username of an active user', () => {
    const el = row({ id: 3, username: 'alice', name: 'Alice Martin', active: true }, 'mar');
    expect(el.querySelector('.avatar img')?.getAttribute('src')).toBe('/api/avatars/3');
    expect(el.querySelector('.name')?.textContent).toBe('Alice Martin');
    expect(el.querySelector('.name mark.match')?.textContent).toBe('Mar');
    expect(el.querySelector('.name')?.classList.contains('inactive')).toBe(false);
    expect(el.querySelector('.user')?.textContent).toBe('alice');
  });

  it('marks an inactive user as such', () => {
    const el = row({ id: 4, username: 'bob', name: 'Bob Weber', active: false });
    expect(el.querySelector('.avatar img')).not.toBeNull();
    expect(el.querySelector('.name')?.textContent).toBe('Bob Weber');
    expect(el.querySelector('.name')?.classList.contains('inactive')).toBe(true);
    expect(el.querySelector('.user')?.textContent).toBe(`bob · ${t.common.inactive}`);
  });
});
