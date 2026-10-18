/**
 * Interface state shared across pages: the "new issue" dialog, short notes
 * shown at the bottom of the screen, the clock relative dates follow, and
 * the appearance (skin and colour mode).
 */

import { getContext, setContext } from 'svelte';
import {
  htmlAttributes,
  parseMode,
  parseSkin,
  resolveScheme,
  STORAGE_KEYS,
  type Mode,
  type Scheme,
  type Skin,
} from '../core/appearance.js';
import { isSkinLoaded, loadSkin } from './skins.svelte.js';

export interface CreateDefaults {
  project?: string;
  parent?: string;
}

export const dialogs = $state<{ create: boolean; createDefaults: CreateDefaults }>({
  create: false,
  createDefaults: {},
});

export function openCreate(defaults: CreateDefaults = {}): void {
  dialogs.createDefaults = defaults;
  dialogs.create = true;
}

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

/** The skin (art direction) and colour mode in use. */
export const appearance = $state<{ skin: Skin; mode: Mode; scheme: Scheme }>({
  skin: parseSkin(readStored(STORAGE_KEYS.skin)),
  mode: parseMode(readStored(STORAGE_KEYS.mode)),
  scheme: 'light',
});

function reveal(): void {
  document.documentElement.removeAttribute('data-skin-loading');
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
  // A blank page until the skin's stylesheet is in, rather than a flash of
  // another skin; never for long.
  document.documentElement.setAttribute('data-skin-loading', '');
  const timer = setTimeout(reveal, 1200);
  void loadSkin(skin).finally(() => {
    if (appearance.skin !== skin) return;
    clearTimeout(timer);
    reveal();
  });
}

export function applySkin(skin: Skin): void {
  appearance.skin = skin;
  store(STORAGE_KEYS.skin, skin);
  render();
}

export function applyMode(mode: Mode): void {
  appearance.mode = mode;
  store(STORAGE_KEYS.mode, mode);
  render();
}

/** Applies the stored appearance and follows the system's light/dark changes. */
export function initAppearance(): () => void {
  render();
  const onChange = () => {
    if (appearance.mode === 'system') render();
  };
  darkQuery?.addEventListener('change', onChange);
  return () => darkQuery?.removeEventListener('change', onChange);
}

const PREVIEW_SKIN = Symbol('preview-skin');

/** Marks a component's subtree as a preview of a skin (theme gallery). Call during setup. */
export function setPreviewSkin(skin: () => Skin): void {
  setContext(PREVIEW_SKIN, skin);
}

/** The skin a component is drawn in: its preview's, else the page's. Call during setup. */
export function skinInEffect(): () => Skin {
  return getContext<(() => Skin) | undefined>(PREVIEW_SKIN) ?? (() => appearance.skin);
}

/** Whether a keyboard shortcut should be ignored because the user is typing. */
export function isTyping(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false;
  return (
    target.isContentEditable ||
    target.tagName === 'INPUT' ||
    target.tagName === 'TEXTAREA' ||
    target.tagName === 'SELECT'
  );
}
