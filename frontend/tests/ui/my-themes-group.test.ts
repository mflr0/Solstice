// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest';
import { flushSync, mount, tick, unmount, type Component } from 'svelte';

import { THEMES } from './themes-fixture.js';
import type { ThemeManifest } from '../../src/core/appearance.js';
import { t } from '../../src/i18n/t.js';
import { appearance, applySkin } from '../../src/lib/ui.svelte.js';
import AppearancePage from '../../src/ui/account/AppearancePage.svelte';
import ThemesPage from '../../src/ui/pages/ThemesPage.svelte';

const localThemes = vi.hoisted(() => ({ list: [] as ThemeManifest[] }));
vi.mock('../../src/lib/local-themes.js', () => ({
  localThemeManifests: () => Promise.resolve(localThemes.list),
  readLocalCss: () => Promise.resolve("[data-skin='x'] { }"),
}));
vi.mock('../../src/lib/api.js', () => ({
  api: {
    themes: () => import('./themes-fixture.js').then((f) => ({ contract: 1, themes: f.THEMES })),
  },
  errorMessage: String,
}));
vi.stubGlobal('fetch', () => Promise.resolve(new Response("[data-skin='x'] { }")));

const MINE = { ...THEMES[0]!, id: 'local-mine', name: { en: 'Mine' } };
const heading = t.account.sections.themes;

let component: ReturnType<typeof mount> | undefined;

afterEach(() => {
  if (component) unmount(component);
  component = undefined;
  localThemes.list = [];
  applySkin('carnet');
});

async function show(page: Component) {
  component = mount(page, { target: document.body });
  for (let i = 0; i < 5; i++) await tick();
  await new Promise((resolve) => setTimeout(resolve));
  flushSync();
}

const headingShown = () =>
  [...document.querySelectorAll('h2, h3')].some((e) => e.textContent?.trim() === heading);

describe.each([
  ['the gallery', ThemesPage, '[aria-labelledby="my-themes"] button.btn-primary'],
  ['Account → Appearance', AppearancePage, '[aria-labelledby="my-themes"] button.skin'],
])('%s', (_, page, choice) => {
  it('hides the group when this browser has no local theme', async () => {
    await show(page as Component);
    expect(headingShown()).toBe(false);
  });

  it('lists a local theme under its heading, after the shared themes, and can choose it', async () => {
    localThemes.list = [MINE];
    await show(page as Component);
    expect(headingShown()).toBe(true);
    const group = document.querySelector('[aria-labelledby="my-themes"]')!;
    expect(group.textContent).toContain('Mine');
    expect(group.textContent).not.toContain(THEMES[1]!.name.en);
    const shared = [...document.querySelectorAll('h2, .skin-name')].find(
      (e) => e.textContent?.trim() === THEMES[0]!.name.en,
    )!;
    expect(shared.compareDocumentPosition(group) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    document.querySelector<HTMLButtonElement>(choice)!.click();
    flushSync();
    expect(appearance.skin).toBe('local-mine');
  });
});
