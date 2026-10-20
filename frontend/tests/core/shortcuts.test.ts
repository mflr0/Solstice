import { describe, expect, it } from 'vitest';

import { formatShortcut, isMac } from '../../src/core/shortcuts.js';

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
