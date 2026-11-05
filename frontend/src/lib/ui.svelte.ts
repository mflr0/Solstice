/**
 * Interface state shared across pages: the "new issue" dialog, short notes
 * shown at the bottom of the screen, the clock relative dates follow, and
 * the appearance (skin and colour mode).
 */

import { tick } from 'svelte';
import {
  htmlAttributes,
  parseMode,
  DEFAULT_SKIN,
  offeredSkin,
  resolveScheme,
  startingSkin,
  STORAGE_KEYS,
  type Mode,
  type Scheme,
  type Skin,
} from '../core/appearance.js';
import {
  bindingsInUse,
  isMac,
  type Shortcut,
  type ShortcutOverrides,
  shortcutsEnabled,
} from '../core/shortcuts.js';
import { t } from '../i18n/t.js';
import { accountApi } from './admin-api.js';
import { session } from './session.svelte.js';
import { isSkinLoaded, loadSkin, loadThemes, themes } from './skins.svelte.js';

export interface CreateDefaults {
  project?: string;
  parent?: string;
}

export const dialogs = $state<{
  create: boolean;
  createDefaults: CreateDefaults;
  shortcuts: boolean;
}>({
  create: false,
  createDefaults: {},
  shortcuts: false,
});

export function openCreate(defaults: CreateDefaults = {}): void {
  dialogs.createDefaults = defaults;
  dialogs.create = true;
}

/** The board the page shows, once loaded: New issue offers its project. */
export const shownBoard = $state<{ id: number; projects: { key: string }[] }>({
  id: 0,
  projects: [],
});

export interface Note {
  id: number;
  text: string;
  kind: 'error' | 'info';
}

export const notes = $state<Note[]>([]);
let nextNote = 1;

export function notify(text: string, kind: Note['kind'] = 'info'): void {
  const id = nextNote++;
  notes.push({ id, text, kind });
  setTimeout(() => dismiss(id), kind === 'error' ? 8000 : 4000);
}

export function dismiss(id: number): void {
  const i = notes.findIndex((n) => n.id === id);
  if (i >= 0) notes.splice(i, 1);
}

/** Ticks every 30 s so "il y a 3 min" stays true without reloading. */
// The Date is replaced, never mutated, so plain Date objects are fine here.
// eslint-disable-next-line svelte/prefer-svelte-reactivity
export const clock = $state({ now: new Date() });
setInterval(() => (clock.now = new Date()), 30_000);

function readStored(key: string): string | null {
  try {
    return localStorage.getItem(key);
  } catch {
    return null;
  }
}

function store(key: string, value: string): void {
  try {
    localStorage.setItem(key, value);
  } catch {
    // Private browsing: the choice lasts for this page only.
  }
}

const darkQuery =
  typeof matchMedia === 'function' ? matchMedia('(prefers-color-scheme: dark)') : null;

class Appearance {
  skin = $state<Skin>(
    startingSkin(
      typeof location !== 'undefined' ? location.search : '',
      readStored(STORAGE_KEYS.skin),
    ),
  );
  mode = $state<Mode>(parseMode(readStored(STORAGE_KEYS.mode)));
  scheme = $state<Scheme>('light');

  /** Whether single-key shortcuts work, as the signed-in account says. */
  get shortcuts(): boolean {
    return shortcutsEnabled(session.me?.preferences);
  }

  /** The shortcuts in use: the defaults, with the account's overrides. */
  readonly bindings: readonly Shortcut[] = $derived(
    bindingsInUse(session.me?.preferences?.shortcuts ?? {}),
  );
}

/** Whether Mod means Cmd on this device. */
export const mac = isMac(typeof navigator !== 'undefined' ? navigator.platform : '');

/** The skin (art direction) and colour mode in use, and whether single-key shortcuts work. */
export const appearance = new Appearance();

/** What takes focus once the page shows: a hidden page can't keep it. */
let focusOnReveal: (() => HTMLElement | null) | null = null;

function reveal(): void {
  document.documentElement.removeAttribute('data-skin-loading');
  const target = focusOnReveal;
  focusOnReveal = null;
  if (!target) return;
  // After the update: a fallback brings back what the chosen skin had replaced.
  void tick().then(() => {
    if (document.activeElement === document.body) target()?.focus();
  });
}

/** How long a skin's stylesheet may take; shorter than the boot script's guard. */
const SKIN_TIMEOUT_MS = 5000;

/** Shows the default skin for this page load; the stored choice stays for next time. */
function fallBack(skin: Skin): void {
  if (appearance.skin !== skin) return;
  if (skin === DEFAULT_SKIN) return reveal();
  appearance.skin = DEFAULT_SKIN;
  render();
  notify(t.account.themeFallback, 'error');
}

function render(): void {
  appearance.scheme = resolveScheme(appearance.mode, darkQuery?.matches ?? false);
  const attrs = htmlAttributes(appearance.skin, appearance.mode, darkQuery?.matches ?? false);
  for (const [name, value] of Object.entries(attrs)) {
    if (value === null) document.documentElement.removeAttribute(name);
    else document.documentElement.setAttribute(name, value);
  }
  const skin = appearance.skin;
  if (isSkinLoaded(skin)) return reveal();
  // A blank page until the skin's stylesheet is in, rather than a flash of an
  // unstyled page.
  const focused = document.activeElement;
  if (focused instanceof HTMLElement && focused !== document.body) {
    focusOnReveal ??= () => focused;
  }
  document.documentElement.setAttribute('data-skin-loading', '');
  const timer = setTimeout(() => fallBack(skin), SKIN_TIMEOUT_MS);
  void loadSkin(skin).then((loaded) => {
    clearTimeout(timer);
    if (appearance.skin !== skin) return;
    if (loaded) reveal();
    else fallBack(skin);
  });
}

/** Applies the skin; `focusAfter` finds what takes focus when the focused element goes away with it. */
export function applySkin(skin: Skin, focusAfter?: () => HTMLElement | null): void {
  appearance.skin = skin;
  store(STORAGE_KEYS.skin, skin);
  if (focusAfter) focusOnReveal = focusAfter;
  render();
}

export function applyMode(mode: Mode): void {
  appearance.mode = mode;
  store(STORAGE_KEYS.mode, mode);
  render();
}

/** Flips between light and dark, keeping the skin; from the system's mode, stores the other one. */
export function toggleScheme(): void {
  applyMode(appearance.scheme === 'dark' ? 'light' : 'dark');
}

/** Turns single-key shortcuts on or off on the account. */
export async function applyShortcuts(on: boolean): Promise<void> {
  if (session.me) session.me = await accountApi.setPreference('shortcutsEnabled', on);
}

/** Stores the bindings that differ from the defaults on the account; none puts the defaults back. */
export async function applyBindings(overrides: ShortcutOverrides): Promise<void> {
  if (!session.me) return;
  session.me = Object.keys(overrides).length
    ? await accountApi.setPreference(
        'shortcuts',
        Object.fromEntries(Object.entries(overrides).map(([a, k]) => [a, [...k]])),
      )
    : await accountApi.deletePreference('shortcuts');
}

/** Applies the stored appearance and follows the system's light/dark changes. */
export function initAppearance(): () => void {
  render();
  void loadThemes().then(() => {
    if (offeredSkin(appearance.skin, themes.list) !== appearance.skin) fallBack(appearance.skin);
  });
  const onChange = () => {
    if (appearance.mode === 'system') render();
  };
  darkQuery?.addEventListener('change', onChange);
  return () => darkQuery?.removeEventListener('change', onChange);
}

/** Inputs a letter does nothing in, so shortcuts still work from them. */
const UNTYPED_INPUTS = new Set(['checkbox', 'radio', 'button', 'submit', 'reset']);

/** Whether a keyboard shortcut should be ignored because the user is typing. */
export function isTyping(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false;
  if (target instanceof HTMLInputElement) return !UNTYPED_INPUTS.has(target.type);
  return target.isContentEditable || target.tagName === 'TEXTAREA' || target.tagName === 'SELECT';
}

/**
 * Whether a key press is not for the page's shortcuts: while typing, or while
 * a dialog is open.
 */
export function ignoreShortcut(e: KeyboardEvent): boolean {
  return (
    e.defaultPrevented ||
    e.isComposing ||
    dialogs.create ||
    dialogs.shortcuts ||
    isTyping(e.target) ||
    document.querySelector('dialog[open]') !== null
  );
}
