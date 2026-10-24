import { describe, expect, it } from 'vitest';

import {
  formatShortcut,
  isMac,
  parseShortcutsSetting,
  SHORTCUTS,
  shortcutFor,
} from '../../src/core/shortcuts.js';

const fr = { ctrl: 'Ctrl', alt: 'Alt', shift: 'Maj' };
const de = { ctrl: 'Strg', alt: 'Alt', shift: 'Umschalt' };

describe('isMac', () => {
  it('is true on Apple platforms', () => {
    for (const p of ['MacIntel', 'iPhone', 'iPad', 'iPod']) expect(isMac(p)).toBe(true);
  });

  it('is false elsewhere', () => {
    for (const p of ['Win32', 'Linux x86_64', 'Linux armv81', '']) expect(isMac(p)).toBe(false);
  });
});

describe('formatShortcut', () => {
  it('writes Mod as Ctrl, with the catalogue’s modifier names', () => {
    expect(formatShortcut('Mod-b', false, fr)).toBe('Ctrl+B');
    expect(formatShortcut('Mod-Shift-x', false, fr)).toBe('Ctrl+Maj+X');
    expect(formatShortcut('Shift-Mod-z', false, de)).toBe('Strg+Umschalt+Z');
    expect(formatShortcut('Mod-Alt-c', false, de)).toBe('Strg+Alt+C');
    expect(formatShortcut('Mod-Shift-8', false, fr)).toBe('Ctrl+Maj+8');
  });

  it('writes Mod as Cmd on a Mac, in Apple’s symbol order', () => {
    expect(formatShortcut('Mod-b', true, fr)).toBe('⌘B');
    expect(formatShortcut('Mod-Shift-x', true, fr)).toBe('⇧⌘X');
    expect(formatShortcut('Shift-Mod-z', true, fr)).toBe('⇧⌘Z');
    expect(formatShortcut('Mod-Alt-c', true, fr)).toBe('⌥⌘C');
    expect(formatShortcut('Ctrl-Shift-Enter', true, fr)).toBe('⌃⇧Enter');
  });

  it('keeps a hyphen key and named keys as they are', () => {
    expect(formatShortcut('Mod--', false, fr)).toBe('Ctrl+-');
    expect(formatShortcut('Mod-Enter', false, fr)).toBe('Ctrl+Enter');
  });
});

describe('shortcutFor', () => {
  it('finds each key in its own scope only', () => {
    expect(shortcutFor('/', 'global', true)).toBe('search');
    expect(shortcutFor('c', 'global', true)).toBe('create');
    expect(shortcutFor('?', 'global', true)).toBe('help');
    expect(shortcutFor('j', 'results', true)).toBe('next');
    expect(shortcutFor('k', 'results', true)).toBe('previous');
    expect(shortcutFor('o', 'results', true)).toBe('open');
    expect(shortcutFor('Enter', 'results', true)).toBe('open');
    expect(shortcutFor('x', 'results', true)).toBe('select');
    expect(shortcutFor('e', 'issue', true)).toBe('editSummary');
    expect(shortcutFor('a', 'issue', true)).toBe('assignToMe');
    expect(shortcutFor('m', 'issue', true)).toBe('comment');
    expect(shortcutFor('t', 'issue', true)).toBe('status');
    expect(shortcutFor('w', 'issue', true)).toBe('watch');
    expect(shortcutFor('j', 'issue', true)).toBeNull();
    expect(shortcutFor('e', 'results', true)).toBeNull();
    expect(shortcutFor('Enter', 'global', true)).toBeNull();
  });

  it('is case-sensitive: Shift+J is not j', () => {
    expect(shortcutFor('J', 'results', true)).toBeNull();
  });

  it('leaves only the non-character keys when turned off', () => {
    for (const s of SHORTCUTS) {
      for (const key of s.keys) {
        expect(shortcutFor(key, s.scope, false)).toBe(key === 'Enter' ? s.action : null);
      }
    }
  });

  it('gives each key one action per scope', () => {
    for (const scope of ['global', 'results', 'issue'] as const) {
      const keys = SHORTCUTS.filter((s) => s.scope === scope).flatMap((s) => s.keys);
      expect(new Set(keys).size).toBe(keys.length);
    }
  });
});

describe('parseShortcutsSetting', () => {
  it('is on unless stored as off', () => {
    expect(parseShortcutsSetting(null)).toBe(true);
    expect(parseShortcutsSetting('on')).toBe(true);
    expect(parseShortcutsSetting('junk')).toBe(true);
    expect(parseShortcutsSetting('off')).toBe(false);
  });
});
