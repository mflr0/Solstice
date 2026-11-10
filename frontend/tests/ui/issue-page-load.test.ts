// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest';
import { flushSync, mount, tick, unmount } from 'svelte';

import { t } from '../../src/i18n/t.js';
import { notes } from '../../src/lib/ui.svelte.js';
import IssuePage from '../../src/ui/issue/IssuePage.svelte';

vi.mock('../../src/lib/api.js', () => ({
  ApiError: class extends Error {
    constructor(
      message: string,
      readonly status: number,
    ) {
      super(message);
    }
  },
  api: {
    issue: () => Promise.reject(new Error('The server can’t be reached right now.')),
  },
  isVisible: () => true,
  errorMessage: (e: Error) => e.message,
  pictureUrl: (id: number) => `/api/users/${id}/picture`,
}));

let component: ReturnType<typeof mount> | undefined;

afterEach(() => {
  if (component) unmount(component);
  component = undefined;
  notes.splice(0);
});

describe('an issue page whose load fails', () => {
  it('shows the error in the page, not a loading line that never ends', async () => {
    component = mount(IssuePage, { target: document.body, props: { issueKey: 'ALPHA-1' } });
    for (let i = 0; i < 4; i++) await tick();
    flushSync();
    expect(document.querySelector('[role="alert"]')?.textContent).toBe(
      'The server can’t be reached right now.',
    );
    expect(document.body.textContent).not.toContain(t.common.loading);
    expect(notes).toEqual([]);
  });
});
