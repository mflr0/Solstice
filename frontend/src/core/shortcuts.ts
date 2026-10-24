/**
 * Keyboard shortcuts: the app's own single keys, and hints for a ProseMirror
 * binding ("Mod-Shift-x") written the way the platform's own menus write it.
 */

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

/** "Mod-Shift-x": "Ctrl+Shift+X", or "⇧⌘X" on a Mac. */
export function formatShortcut(binding: string, mac: boolean, names: KeyNames): string {
  const parts = binding.split(/-(?!$)/);
  const key = parts.pop() ?? '';
  const mods = new Set(parts.map((p) => (p === 'Mod' ? (mac ? 'Meta' : 'Ctrl') : p)));
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
  /** KeyboardEvent.key values, the first one shown first. */
  readonly keys: readonly string[];
}

/** Every shortcut, in the order the help lists them. */
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

/** A key that types a character (WCAG 2.1.4): what the off switch turns off. */
export function isCharacterKey(key: string): boolean {
  return [...key].length === 1;
}

/**
 * The action a key press means in a scope, or null. With character keys
 * turned off, only the others (Enter) are left.
 */
export function shortcutFor(
  key: string,
  scope: ShortcutScope,
  characterKeys: boolean,
): ShortcutAction | null {
  if (!characterKeys && isCharacterKey(key)) return null;
  return SHORTCUTS.find((s) => s.scope === scope && s.keys.includes(key))?.action ?? null;
}

/** The stored setting: on unless turned off. */
export function parseShortcutsSetting(value: string | null | undefined): boolean {
  return value !== 'off';
}
