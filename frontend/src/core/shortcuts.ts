/**
 * Keyboard shortcut hints: a ProseMirror binding ("Mod-Shift-x") written the
 * way the platform's own menus write it.
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
