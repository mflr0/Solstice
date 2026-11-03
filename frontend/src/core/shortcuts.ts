/**
 * Keyboard shortcuts: the app's own bindings ("c", "Alt-k", "Mod-Shift-x", in
 * ProseMirror's notation), matched against key presses and written the way the
 * platform's own menus write them.
 */

import type { Preferences } from './types.js';

/** The modifier names a catalogue gives for non-Apple keyboards. */
export interface KeyNames {
  readonly ctrl: string;
  readonly alt: string;
  readonly shift: string;
}

/** Whether `Mod` means Cmd, with the test prosemirror-keymap applies to `navigator.platform`. */
export function isMac(platform: string): boolean {
  return /Mac|iP(hone|[oa]d)/.test(platform);
}

/** "Mod-Shift-x": "Ctrl+Shift+X", or "⇧⌘X" on a Mac. A key alone stays as typed: "c". */
export function formatShortcut(binding: string, mac: boolean, names: KeyNames): string {
  const parts = binding.split(/-(?!$)/);
  const key = parts.pop() ?? '';
  const mods = new Set(parts.map((p) => (p === 'Mod' ? (mac ? 'Meta' : 'Ctrl') : p)));
  if (!parts.length) return key === ' ' ? 'Space' : key;
  const shown = key.length === 1 ? key.toUpperCase() : key;
  if (mac) {
    const glyphs: [string, string][] = [
      ['Ctrl', '⌃'],
      ['Alt', '⌥'],
      ['Shift', '⇧'],
      ['Meta', '⌘'],
    ];
    return glyphs.map(([m, g]) => (mods.has(m) ? g : '')).join('') + shown;
  }
  const words: [string, string][] = [
    ['Ctrl', names.ctrl],
    ['Alt', names.alt],
    ['Shift', names.shift],
  ];
  return [...words.filter(([m]) => mods.has(m)).map(([, w]) => w), shown].join('+');
}

/** Where a shortcut works: everywhere, in a search's results, or on an issue. */
export type ShortcutScope = 'global' | 'results' | 'issue';

export type ShortcutAction =
  | 'search'
  | 'create'
  | 'help'
  | 'next'
  | 'previous'
  | 'open'
  | 'select'
  | 'editSummary'
  | 'assignToMe'
  | 'comment'
  | 'status'
  | 'watch';

export interface Shortcut {
  readonly scope: ShortcutScope;
  readonly action: ShortcutAction;
  /** Bindings ("c", "Alt-k", "Mod-Enter"), the first one shown first. */
  readonly keys: readonly string[];
}

/** Every shortcut with its default bindings, in the order the help lists them. */
export const SHORTCUTS: readonly Shortcut[] = [
  { scope: 'global', action: 'search', keys: ['/'] },
  { scope: 'global', action: 'create', keys: ['c'] },
  { scope: 'global', action: 'help', keys: ['?'] },
  { scope: 'results', action: 'next', keys: ['j'] },
  { scope: 'results', action: 'previous', keys: ['k'] },
  { scope: 'results', action: 'open', keys: ['o', 'Enter'] },
  { scope: 'results', action: 'select', keys: ['x'] },
  { scope: 'issue', action: 'editSummary', keys: ['e'] },
  { scope: 'issue', action: 'assignToMe', keys: ['a'] },
  { scope: 'issue', action: 'comment', keys: ['m'] },
  { scope: 'issue', action: 'status', keys: ['t'] },
  { scope: 'issue', action: 'watch', keys: ['w'] },
];

/** An account's bindings by action; each list replaces the action's defaults. */
export type ShortcutOverrides = Readonly<Record<string, readonly string[]>>;

/** The shortcuts in use: the defaults, with each overridden action's list replaced. */
export function bindingsInUse(overrides: ShortcutOverrides): readonly Shortcut[] {
  return SHORTCUTS.map((s) =>
    Object.hasOwn(overrides, s.action) ? { ...s, keys: overrides[s.action] ?? [] } : s,
  );
}

/** A binding taken apart, with Mod resolved for the platform. */
export interface Binding {
  readonly key: string;
  readonly ctrl: boolean;
  readonly alt: boolean;
  readonly shift: boolean;
  readonly meta: boolean;
}

/** "Alt-k" taken apart, or null when it names no key or an unknown modifier. */
export function parseBinding(binding: string, mac: boolean): Binding | null {
  const parts = binding.split(/-(?!$)/);
  let key = parts.pop() ?? '';
  if (!key) return null;
  if (key === 'Space') key = ' ';
  const mods = { ctrl: false, alt: false, shift: false, meta: false };
  for (const p of parts) {
    if (p === 'Ctrl') mods.ctrl = true;
    else if (p === 'Alt') mods.alt = true;
    else if (p === 'Shift') mods.shift = true;
    else if (p === 'Meta') mods.meta = true;
    else if (p === 'Mod') mods[mac ? 'meta' : 'ctrl'] = true;
    else return null;
  }
  // "J" is what Shift+j types: the same press as "Shift-j".
  if (isTypedKey(key) && key !== key.toLowerCase())
    return { ...mods, shift: true, key: key.toLowerCase() };
  return { ...mods, key: mods.shift && isTypedKey(key) ? key.toLowerCase() : key };
}

/** A single character other than space, which Shift can change ("/" to "?"). */
function isTypedKey(key: string): boolean {
  return isCharacterKey(key) && key !== ' ';
}

function sameBinding(a: Binding, b: Binding): boolean {
  return (
    a.key === b.key &&
    a.ctrl === b.ctrl &&
    a.alt === b.alt &&
    a.shift === b.shift &&
    a.meta === b.meta
  );
}

/** The parts of a KeyboardEvent a binding is matched against. */
export interface KeyPress {
  readonly key: string;
  readonly ctrlKey?: boolean;
  readonly altKey?: boolean;
  readonly shiftKey?: boolean;
  readonly metaKey?: boolean;
  readonly keyCode?: number;
}

/**
 * The letter or digit a press's key code stands for, for a modified press
 * whose key is something else (Alt+K types "˚" on a Mac).
 */
function baseKey(e: KeyPress): string | null {
  const code = e.keyCode ?? 0;
  if (code >= 48 && code <= 57) return String.fromCharCode(code);
  if (code < 65 || code > 90) return null;
  const letter = String.fromCharCode(code);
  return e.shiftKey ? letter : letter.toLowerCase();
}

/** Whether a press is a binding's key with exactly its modifiers. */
export function matchesBinding(e: KeyPress, binding: Binding): boolean {
  if (!!e.ctrlKey !== binding.ctrl || !!e.altKey !== binding.alt || !!e.metaKey !== binding.meta)
    return false;
  const names = [e.key];
  const base = e.ctrlKey || e.altKey || e.metaKey ? baseKey(e) : null;
  if (base && base !== e.key) names.push(base);
  return names.some((name) => {
    // Shift is part of what a character key types ("?" is Shift+/), so only a
    // binding that names Shift checks it.
    if (!isTypedKey(name)) return name === binding.key && !!e.shiftKey === binding.shift;
    if (binding.shift) return !!e.shiftKey && name.toLowerCase() === binding.key;
    return name === binding.key;
  });
}

/** A key that types a character (WCAG 2.1.4): what the off switch turns off. */
export function isCharacterKey(key: string): boolean {
  return [...key].length === 1;
}

/** A binding without Ctrl, Alt or Meta on a character key: what the off switch turns off. */
export function isBareCharacter(binding: Binding): boolean {
  return !binding.ctrl && !binding.alt && !binding.meta && isCharacterKey(binding.key);
}

/** Whether a binding works with single-key shortcuts on or off. */
export function bindingWorks(binding: string, characterKeys: boolean, mac: boolean): boolean {
  const b = parseBinding(binding, mac);
  return b !== null && (characterKeys || !isBareCharacter(b));
}

/**
 * The action a key press means in a scope, or null. With character keys
 * turned off, only the bindings with a modifier or a named key are left.
 */
export function shortcutFor(
  e: KeyPress,
  scope: ShortcutScope,
  characterKeys: boolean,
  shortcuts: readonly Shortcut[],
  mac: boolean,
): ShortcutAction | null {
  for (const s of shortcuts) {
    if (s.scope !== scope) continue;
    for (const key of s.keys) {
      const b = parseBinding(key, mac);
      if (!b || (!characterKeys && isBareCharacter(b))) continue;
      if (matchesBinding(e, b)) return s.action;
    }
  }
  return null;
}

/** One binding of one action. */
export interface BoundKey {
  readonly scope: ShortcutScope;
  readonly action: ShortcutAction;
  readonly binding: string;
}

/**
 * Pairs of bindings of different actions that the same press would trigger:
 * the same binding in the same scope, or in any scope when either is global.
 */
export function clashes(shortcuts: readonly Shortcut[], mac: boolean): [BoundKey, BoundKey][] {
  const bound = shortcuts.flatMap((s) =>
    s.keys.flatMap((binding) => {
      const parsed = parseBinding(binding, mac);
      return parsed ? [{ scope: s.scope, action: s.action, binding, parsed }] : [];
    }),
  );
  const found: [BoundKey, BoundKey][] = [];
  bound.forEach((a, i) => {
    for (const b of bound.slice(i + 1)) {
      if (a.action === b.action || !sameBinding(a.parsed, b.parsed)) continue;
      if (a.scope !== b.scope && a.scope !== 'global' && b.scope !== 'global') continue;
      found.push([
        { scope: a.scope, action: a.action, binding: a.binding },
        { scope: b.scope, action: b.action, binding: b.binding },
      ]);
    }
  });
  return found;
}

/** Keys a binding can't be: a modifier alone, and keys that move focus or close things. */
const REFUSED_KEYS = new Set([
  'Shift',
  'Control',
  'Ctrl',
  'Alt',
  'AltGraph',
  'Meta',
  'Mod',
  'Tab',
  'Escape',
]);

/** Browsers keep these for their own windows and tabs and never pass them to the page. */
const BROWSER_BINDINGS = [
  'Mod-w',
  'Mod-t',
  'Mod-n',
  'Mod-q',
  'Mod-Shift-w',
  'Mod-Shift-t',
  'Mod-Shift-n',
];

/** Whether a binding can't be used: unreadable, a refused key, or a browser's own combo. */
export function isRefused(binding: string): boolean {
  return [false, true].some((mac) => {
    const b = parseBinding(binding, mac);
    if (!b || REFUSED_KEYS.has(b.key)) return true;
    return BROWSER_BINDINGS.some((r) => {
      const reserved = parseBinding(r, mac);
      return reserved !== null && sameBinding(b, reserved);
    });
  });
}

/** Whether single-key shortcuts work: on unless the account turned them off. */
export function shortcutsEnabled(preferences: Preferences | undefined): boolean {
  return preferences?.shortcutsEnabled !== false;
}
