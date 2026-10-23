// @vitest-environment jsdom
import { afterEach, describe, expect, it } from 'vitest';
import { flushSync, mount, unmount } from 'svelte';

import { t } from '../../src/i18n/t.js';
import NotFoundPage from '../../src/ui/pages/NotFoundPage.svelte';

let component: ReturnType<typeof mount> | undefined;

afterEach(() => {
  if (component) unmount(component);
  component = undefined;
});

describe('NotFoundPage', () => {
  it('names the page in the tab title', () => {
    component = mount(NotFoundPage, { target: document.body });
    flushSync();
    expect(document.title).toBe(`${t.notFound.title} — ${t.appName}`);
  });
});
