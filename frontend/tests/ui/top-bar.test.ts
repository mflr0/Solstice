// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { flushSync, mount, tick, unmount } from 'svelte';

import { MODES, SKINS } from '../../src/core/appearance.js';
import { paths } from '../../src/core/route.js';
import { t } from '../../src/i18n/t.js';
import { session } from '../../src/lib/session.svelte.js';
import { appearance, applyMode, dialogs } from '../../src/lib/ui.svelte.js';
import TopBar from '../../src/ui/TopBar.svelte';

vi.mock('../../src/lib/api.js', () => ({
  api: {
    projects: () =>
      Promise.resolve([
        { id: 1, key: 'ALPHA', name: 'Alpha' },
        { id: 2, key: 'BETA', name: 'Beta' },
      ]),
    issues: () => Promise.resolve({ issues: [] }),
    users: () => Promise.resolve([]),
  },
  errorMessage: String,
  pictureUrl: (id: number) => `/api/users/${id}/picture`,
}));

vi.stubGlobal(
  'ResizeObserver',
  class {
    observe() {}
    disconnect() {}
  },
);

let component: ReturnType<typeof mount> | undefined;

beforeEach(() => {
  session.me = {
    id: 1,
    username: 'alice',
    name: 'Alice',
    email: 'alice@example.com',
    admin: false,
    project: { id: 2, key: 'BETA', name: 'Beta' },
  };
  component = mount(TopBar, { target: document.body, props: { project: undefined } });
  flushSync();
});

afterEach(() => {
  if (component) unmount(component);
  component = undefined;
  session.me = null;
  dialogs.create = false;
  applyMode('system');
});

/** What Tab reaches, in order: what has no tabindex of -1 and is not disabled. */
function tabbable(root: ParentNode): HTMLElement[] {
  return [
    ...root.querySelectorAll<HTMLElement>('a[href], button, input, select, textarea, [tabindex]'),
  ].filter((el) => el.tabIndex >= 0 && !(el as HTMLButtonElement).disabled);
}

const button = (name: string) =>
  document.querySelector<HTMLButtonElement>(`button[aria-label^="${name}"]`)!;
const focused = () => document.activeElement as HTMLElement;

function key(el: Element, k: string) {
  el.dispatchEvent(new KeyboardEvent('keydown', { key: k, bubbles: true, cancelable: true }));
  flushSync();
}

/** Lets the menu mount and take focus. */
async function settle() {
  for (let i = 0; i < 3; i++) await tick();
  flushSync();
}

describe('TopBar keyboard', () => {
  it('reaches every item with Tab, in reading order', () => {
    const names = tabbable(document.querySelector('header')!).map(
      (el) =>
        (el as HTMLInputElement).labels?.[0]?.textContent ??
        el.getAttribute('aria-label') ??
        el.textContent?.trim(),
    );
    expect(names).toEqual([
      t.appName,
      t.nav.scopeLabel('Beta'),
      t.nav.work,
      t.nav.projects,
      t.nav.boards,
      t.nav.search,
      t.quick.label,
      t.nav.themes,
      expect.stringContaining(t.nav.create),
      t.nav.account,
    ]);
  });

  // Enter and Space activate native links and buttons: no item is a div with a click handler.
  it('makes every item a native link, button or field', () => {
    for (const el of tabbable(document.querySelector('header')!)) {
      expect(['A', 'BUTTON', 'INPUT']).toContain(el.tagName);
    }
    expect(document.querySelector('header [onclick], header div[role="button"]')).toBeNull();
  });

  it('links each page to its address', () => {
    const hrefs = [...document.querySelectorAll('.topbar-nav a')].map((a) =>
      a.getAttribute('href'),
    );
    expect(hrefs).toEqual([paths.home(), paths.projects(), paths.boards(), paths.search()]);
    expect(document.querySelector('.brand')?.getAttribute('href')).toBe(paths.home());
  });

  it('opens the creation dialog', () => {
    document.querySelector<HTMLButtonElement>('.topbar-create')!.click();
    expect(dialogs.create).toBe(true);
  });

  it('focuses the search field with its label', () => {
    const input = document.querySelector<HTMLInputElement>('#quick-search')!;
    expect(document.querySelector(`label[for="quick-search"]`)?.textContent).toBe(t.quick.label);
    input.focus();
    expect(focused()).toBe(input);
  });

  it('walks the account menu with the arrows and leaves it with Escape', async () => {
    const account = button(t.nav.account);
    account.click();
    await settle();
    expect(account.getAttribute('aria-expanded')).toBe('true');
    const items = [...document.querySelectorAll<HTMLElement>('.account-menu [role="menuitem"]')];
    expect(items.map((i) => i.textContent?.trim())).toEqual([
      t.nav.myPage,
      t.account.menu,
      t.nav.sourceCode,
      t.nav.licenses,
      t.nav.logout,
    ]);
    expect(focused()).toBe(items[0]);
    // Tab leaves the menu instead of walking it.
    expect(items.every((i) => i.tabIndex === -1)).toBe(true);

    key(focused(), 'ArrowDown');
    expect(focused()).toBe(items[1]);
    key(focused(), 'End');
    expect(focused()).toBe(items[4]);
    key(focused(), 'ArrowDown');
    expect(focused()).toBe(items[0]);
    key(focused(), 'ArrowUp');
    expect(focused()).toBe(items[4]);
    key(focused(), 'Home');
    expect(focused()).toBe(items[0]);

    key(focused(), 'Escape');
    expect(document.querySelector('.account-menu')).toBeNull();
    expect(focused()).toBe(account);
  });

  it('switches the mode from the theme menu and links to the gallery', async () => {
    const themes = button(t.nav.themes);
    themes.click();
    await settle();
    expect(themes.getAttribute('aria-expanded')).toBe('true');
    const radios = [
      ...document.querySelectorAll<HTMLElement>('.theme-menu [role="menuitemradio"]'),
    ];
    expect(radios.map((i) => i.textContent?.trim())).toEqual([
      ...SKINS.map((s) => t.nav.skins[s]),
      ...MODES.map((m) => t.nav.modes[m]),
    ]);
    const checked = () =>
      radios
        .filter((i) => i.getAttribute('aria-checked') === 'true')
        .map((i) => i.textContent?.trim());
    expect(checked()).toEqual([t.nav.skins[appearance.skin], t.nav.modes.system]);
    expect(focused()).toBe(radios[SKINS.indexOf(appearance.skin)]);

    const dark = radios[SKINS.length + MODES.indexOf('dark')]!;
    dark.click();
    flushSync();
    expect(appearance.mode).toBe('dark');
    expect(document.documentElement.dataset.theme).toBe('dark');
    // The menu stays open and shows the new choice.
    expect(checked()).toEqual([t.nav.skins[appearance.skin], t.nav.modes.dark]);

    key(focused(), 'End');
    const gallery = document.querySelector<HTMLAnchorElement>('.theme-menu [role="menuitem"]')!;
    expect(focused()).toBe(gallery);
    expect(gallery.getAttribute('href')).toBe(paths.themes());
    expect(gallery.textContent?.trim()).toBe(t.nav.allThemes);

    key(focused(), 'Escape');
    expect(document.querySelector('.theme-menu')).toBeNull();
    expect(focused()).toBe(themes);
  });

  it('starts the scope menu on the current project and closes it when focus moves on', async () => {
    const scope = button(t.nav.scopeLabel('Beta'));
    scope.click();
    await settle();
    const items = [...document.querySelectorAll<HTMLElement>('.scope-menu [role="menuitemradio"]')];
    expect(items).toHaveLength(3);
    expect(focused()).toBe(items[2]);
    key(focused(), 'ArrowUp');
    expect(focused()).toBe(items[1]);

    // What Tab does from inside the menu: focus goes to the next item of the bar.
    document.querySelector<HTMLElement>('.topbar-nav a')!.focus();
    flushSync();
    expect(document.querySelector('.scope-menu')).toBeNull();
    expect(scope.getAttribute('aria-expanded')).toBe('false');
  });
});
