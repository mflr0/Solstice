// @vitest-environment jsdom
import { afterEach, describe, expect, it } from 'vitest';
import { flushSync, mount, unmount } from 'svelte';

import { t } from '../../src/i18n/t.js';
import ThemeContractFlag from '../../src/ui/ThemeContractFlag.svelte';

describe('ThemeContractFlag', () => {
  let shown: ReturnType<typeof mount> | undefined;

  afterEach(() => {
    if (shown) void unmount(shown);
    shown = undefined;
    document.body.replaceChildren();
  });

  function show(theme: number, server: number) {
    shown = mount(ThemeContractFlag, { target: document.body, props: { theme, server } });
    flushSync();
    return document.body.textContent?.trim();
  }

  it('names both versions when the theme is on an older contract', () => {
    expect(show(0, 1)).toBe(t.themes.outdated(0, 1));
  });

  it('shows nothing on the server’s contract', () => {
    expect(show(1, 1)).toBe('');
  });
});
