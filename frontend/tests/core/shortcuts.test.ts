import { describe, expect, it } from 'vitest';

import {
  bindingFromPress,
  bindingsInUse,
  clashes,
  formatShortcut,
  isMac,
  isRefused,
  type KeyPress,
  matchesBinding,
  overridesFrom,
  parseBinding,
  type Shortcut,
  type ShortcutScope,
  SHORTCUTS,
  shortcutFor,
  shortcutsEnabled,
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

  it('keeps Meta off a Mac', () => {
    expect(formatShortcut('Meta-k', false, de)).toBe('Meta+K');
  });

  it('keeps a hyphen key and named keys as they are', () => {
    expect(formatShortcut('Mod--', false, fr)).toBe('Ctrl+-');
    expect(formatShortcut('Mod-Enter', false, fr)).toBe('Ctrl+Enter');
  });

  it('writes a key alone as it is typed', () => {
    expect(formatShortcut('c', false, fr)).toBe('c');
    expect(formatShortcut('c', true, fr)).toBe('c');
    expect(formatShortcut('Enter', true, fr)).toBe('Enter');
    expect(formatShortcut('Alt-k', false, de)).toBe('Alt+K');
  });
});

/** The action of a press with the default bindings, off a Mac. */
function on(
  press: string | KeyPress,
  scope: ShortcutScope,
  characterKeys: boolean,
  shortcuts: readonly Shortcut[] = SHORTCUTS,
  mac = false,
) {
  const e = typeof press === 'string' ? { key: press } : press;
  return shortcutFor(e, scope, characterKeys, shortcuts, mac);
}

describe('shortcutFor', () => {
  it('finds each key in its own scope only', () => {
    expect(on('/', 'global', true)).toBe('search');
    expect(on('c', 'global', true)).toBe('create');
    expect(on('?', 'global', true)).toBe('help');
    expect(on('j', 'results', true)).toBe('next');
    expect(on('k', 'results', true)).toBe('previous');
    expect(on('o', 'results', true)).toBe('open');
    expect(on('Enter', 'results', true)).toBe('open');
    expect(on('x', 'results', true)).toBe('select');
    expect(on('e', 'issue', true)).toBe('editSummary');
    expect(on('a', 'issue', true)).toBe('assignToMe');
    expect(on('m', 'issue', true)).toBe('comment');
    expect(on('t', 'issue', true)).toBe('status');
    expect(on('w', 'issue', true)).toBe('watch');
    expect(on('j', 'issue', true)).toBeNull();
    expect(on('e', 'results', true)).toBeNull();
    expect(on('Enter', 'global', true)).toBeNull();
  });

  it('is case-sensitive: Shift+J is not j', () => {
    expect(on('J', 'results', true)).toBeNull();
  });

  it('leaves only the non-character keys when turned off', () => {
    for (const s of SHORTCUTS) {
      for (const key of s.keys) {
        expect(on(key, s.scope, false)).toBe(key === 'Enter' ? s.action : null);
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

describe('bindings with modifiers', () => {
  const altK = bindingsInUse({ previous: ['Alt-k'] });

  it('matches the modifiers exactly', () => {
    expect(on({ key: 'k', altKey: true }, 'results', true, altK)).toBe('previous');
    expect(on({ key: 'k' }, 'results', true, altK)).toBeNull();
    expect(on({ key: 'k', altKey: true, ctrlKey: true }, 'results', true, altK)).toBeNull();
    expect(on({ key: 'j', altKey: true }, 'results', true)).toBeNull();
    expect(on({ key: 'c', ctrlKey: true }, 'global', true)).toBeNull();
  });

  it('reads the key code when the modifier changes the character', () => {
    expect(on({ key: '˚', altKey: true, keyCode: 75 }, 'results', true, altK, true)).toBe(
      'previous',
    );
    expect(
      on({ key: '', altKey: true, shiftKey: true, keyCode: 75 }, 'results', true, altK, true),
    ).toBeNull();
  });

  it('takes Mod as Cmd on a Mac and Ctrl elsewhere', () => {
    const mod = bindingsInUse({ create: ['Mod-Enter'] });
    expect(on({ key: 'Enter', ctrlKey: true }, 'global', true, mod)).toBe('create');
    expect(on({ key: 'Enter', metaKey: true }, 'global', true, mod)).toBeNull();
    expect(on({ key: 'Enter', metaKey: true }, 'global', true, mod, true)).toBe('create');
  });

  it('checks Shift only when the binding names it', () => {
    expect(on({ key: '?', shiftKey: true }, 'global', true)).toBe('help');
    expect(on({ key: 'Enter', shiftKey: true }, 'results', true)).toBeNull();
    const shiftJ = bindingsInUse({ next: ['Shift-j'] });
    expect(on({ key: 'J', shiftKey: true }, 'results', true, shiftJ)).toBe('next');
    expect(on({ key: 'j' }, 'results', true, shiftJ)).toBeNull();
  });

  it('keeps the bindings with a modifier when single keys are off', () => {
    expect(on({ key: 'k', altKey: true }, 'results', false, altK)).toBe('previous');
    const both = bindingsInUse({ previous: ['k', 'Alt-k'] });
    expect(on({ key: 'k' }, 'results', false, both)).toBeNull();
    expect(on({ key: 'k', altKey: true }, 'results', false, both)).toBe('previous');
  });
});

describe('bindingsInUse', () => {
  it('is the defaults without overrides', () => {
    expect(bindingsInUse({})).toEqual(SHORTCUTS);
  });

  it('replaces an overridden action’s whole list', () => {
    const inUse = bindingsInUse({ open: ['Alt-o'], watch: [] });
    expect(inUse.find((s) => s.action === 'open')?.keys).toEqual(['Alt-o']);
    expect(on('Enter', 'results', true, inUse)).toBeNull();
    expect(on('o', 'results', true, inUse)).toBeNull();
    expect(inUse.find((s) => s.action === 'watch')?.keys).toEqual([]);
    expect(on('w', 'issue', true, inUse)).toBeNull();
    expect(on('e', 'issue', true, inUse)).toBe('editSummary');
  });

  it('ignores names that are not actions', () => {
    expect(bindingsInUse({ toString: ['z'], nope: ['z'] })).toEqual(SHORTCUTS);
  });
});

describe('overridesFrom', () => {
  const defaults = Object.fromEntries(SHORTCUTS.map((s) => [s.action, s.keys]));

  it('keeps only the actions whose keys changed', () => {
    expect(overridesFrom({ ...defaults, create: ['Alt-n'], watch: [] })).toEqual({
      create: ['Alt-n'],
      watch: [],
    });
    expect(overridesFrom({ ...defaults, open: ['Enter', 'o'] })).toEqual({ open: ['Enter', 'o'] });
  });

  it('is empty once every action is back to its defaults', () => {
    expect(overridesFrom(defaults)).toEqual({});
    expect(overridesFrom({})).toEqual({});
  });

  it('gives back what bindingsInUse took', () => {
    const stored = { create: ['Alt-n'], next: ['j', 'ArrowDown'] };
    const inUse = Object.fromEntries(bindingsInUse(stored).map((s) => [s.action, s.keys]));
    expect(overridesFrom(inUse)).toEqual(stored);
  });
});

describe('bindingFromPress', () => {
  it('writes a key alone as it is typed, Shift included', () => {
    expect(bindingFromPress({ key: 'c' }, false)).toBe('c');
    expect(bindingFromPress({ key: '?', shiftKey: true }, false)).toBe('?');
    expect(bindingFromPress({ key: 'J', shiftKey: true }, false)).toBe('J');
    expect(bindingFromPress({ key: ' ' }, false)).toBe('Space');
    expect(bindingFromPress({ key: 'Enter', shiftKey: true }, false)).toBe('Shift-Enter');
  });

  it('writes Ctrl off a Mac and Cmd on a Mac as Mod', () => {
    expect(bindingFromPress({ key: 'k', ctrlKey: true, keyCode: 75 }, false)).toBe('Mod-k');
    expect(bindingFromPress({ key: 'k', metaKey: true, keyCode: 75 }, true)).toBe('Mod-k');
    expect(bindingFromPress({ key: 'k', ctrlKey: true, keyCode: 75 }, true)).toBe('Ctrl-k');
    expect(bindingFromPress({ key: 'k', metaKey: true, keyCode: 75 }, false)).toBe('Meta-k');
  });

  it('names the key under the fingers when a modifier changes the character', () => {
    expect(bindingFromPress({ key: 'Dead', altKey: true, keyCode: 78 }, true)).toBe('Alt-n');
    expect(bindingFromPress({ key: '˚', altKey: true, keyCode: 75 }, true)).toBe('Alt-k');
    expect(bindingFromPress({ key: 'K', altKey: true, shiftKey: true, keyCode: 75 }, false)).toBe(
      'Alt-Shift-k',
    );
  });

  it('waits for a key while only modifiers are down', () => {
    for (const key of ['Shift', 'Control', 'Alt', 'Meta', 'Unidentified'])
      expect(bindingFromPress({ key, altKey: key === 'Alt' }, false), key).toBeNull();
  });

  it('records what the same press then matches', () => {
    const presses: KeyPress[] = [
      { key: 'c' },
      { key: '?', shiftKey: true },
      { key: 'J', shiftKey: true },
      { key: ' ' },
      { key: 'ArrowDown', shiftKey: true },
      { key: 'n', altKey: true, keyCode: 78 },
      { key: '!', altKey: true, shiftKey: true, keyCode: 49 },
      { key: 'Enter', ctrlKey: true, keyCode: 13 },
    ];
    for (const mac of [false, true]) {
      for (const e of presses) {
        const b = parseBinding(bindingFromPress(e, mac) ?? '', mac);
        expect(b && matchesBinding(e, b), `${e.key} ${mac}`).toBe(true);
      }
    }
  });
});

describe('clashes', () => {
  it('finds none in the defaults', () => {
    expect(clashes(SHORTCUTS, false)).toEqual([]);
    expect(clashes(SHORTCUTS, true)).toEqual([]);
  });

  it('finds a global binding used in another scope', () => {
    expect(clashes(bindingsInUse({ editSummary: ['c'] }), false)).toEqual([
      [
        { scope: 'global', action: 'create', binding: 'c' },
        { scope: 'issue', action: 'editSummary', binding: 'c' },
      ],
    ]);
  });

  it('allows the same binding in two scopes that never meet', () => {
    expect(clashes(bindingsInUse({ editSummary: ['j'] }), false)).toEqual([]);
  });

  it('finds the same binding in the same scope, however it is written', () => {
    expect(clashes(bindingsInUse({ next: ['J'], previous: ['Shift-j'] }), false)).toHaveLength(1);
    expect(clashes(bindingsInUse({ next: ['Mod-k'], previous: ['Ctrl-k'] }), false)).toHaveLength(
      1,
    );
    expect(clashes(bindingsInUse({ next: ['Mod-k'], previous: ['Ctrl-k'] }), true)).toEqual([]);
  });
});

describe('isRefused', () => {
  it('refuses a modifier alone, Tab, Escape and browser combos', () => {
    for (const b of ['Shift', 'Ctrl', 'Alt', 'Meta', 'Tab', 'Shift-Tab', 'Escape']) {
      expect(isRefused(b), b).toBe(true);
    }
    for (const b of ['Ctrl-w', 'Meta-w', 'Mod-t', 'Ctrl-n', 'Meta-q', 'Ctrl-Shift-t']) {
      expect(isRefused(b), b).toBe(true);
    }
  });

  it('refuses what it can’t read', () => {
    for (const b of ['', 'Hyper-k', 'Cmd-k']) expect(isRefused(b), b).toBe(true);
  });

  it('accepts the defaults and other combos', () => {
    for (const s of SHORTCUTS) for (const k of s.keys) expect(isRefused(k), k).toBe(false);
    for (const b of ['Alt-k', 'Mod-Enter', 'Ctrl-Shift-k', 'Alt-w'])
      expect(isRefused(b), b).toBe(false);
  });
});

describe('shortcutsEnabled', () => {
  it('is on unless the account turned it off', () => {
    expect(shortcutsEnabled(undefined)).toBe(true);
    expect(shortcutsEnabled({})).toBe(true);
    expect(shortcutsEnabled({ shortcuts: { create: ['n'] } })).toBe(true);
    expect(shortcutsEnabled({ shortcutsEnabled: true })).toBe(true);
    expect(shortcutsEnabled({ shortcutsEnabled: false })).toBe(false);
  });
});
