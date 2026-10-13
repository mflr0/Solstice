/**
 * Interface state shared across pages: the "new issue" dialog, short notes
 * shown at the bottom of the screen, the clock relative dates follow, and
 * the colour theme.
 */

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

export type Theme = 'system' | 'light' | 'dark';

function readTheme(): Theme {
  try {
    const v = localStorage.getItem('solstice.theme');
    return v === 'light' || v === 'dark' ? v : 'system';
  } catch {
    return 'system';
  }
}

export const theme = $state({ value: readTheme() });

export function applyTheme(value: Theme): void {
  theme.value = value;
  if (value === 'system') document.documentElement.removeAttribute('data-theme');
  else document.documentElement.setAttribute('data-theme', value);
  try {
    localStorage.setItem('solstice.theme', value);
  } catch {
    // Private browsing: the choice lasts for this page only.
  }
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
