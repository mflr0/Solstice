// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest';
import { flushSync, mount, tick, unmount } from 'svelte';

import { ApiError } from '../../src/lib/api.js';
import { notes } from '../../src/lib/ui.svelte.js';
import AdminOverview from '../../src/ui/admin/AdminOverview.svelte';

vi.mock('../../src/lib/admin-api.js', () => ({
  adminApi: {
    onboarding: () => Promise.reject(new ApiError('The database is not answering.', 500)),
  },
}));

let component: ReturnType<typeof mount> | undefined;

afterEach(() => {
  if (component) unmount(component);
  component = undefined;
  notes.splice(0);
});

describe('an admin page whose load fails', () => {
  it('raises an error toast with the server’s message', async () => {
    component = mount(AdminOverview, { target: document.body });
    for (let i = 0; i < 4; i++) await tick();
    flushSync();
    expect(notes.map((n) => [n.kind, n.text])).toEqual([
      ['error', 'The database is not answering.'],
    ]);
  });
});
