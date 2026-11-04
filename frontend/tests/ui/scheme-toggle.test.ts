// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest';

// The system's mode is read once, when ui.svelte.ts loads.
const system = vi.hoisted(() => {
  const state = { dark: true };
  globalThis.matchMedia = ((query: string) => ({
    get matches() {
      return query === '(prefers-color-scheme: dark)' && state.dark;
    },
    addEventListener() {},
    removeEventListener() {},
  })) as unknown as typeof matchMedia;
  return state;
});

import { appearance, applyMode, applySkin, toggleScheme } from '../../src/lib/ui.svelte.js';

afterEach(() => {
  applyMode('system');
  applySkin('carnet');
  system.dark = true;
});

describe('toggleScheme', () => {
  it.each([
    ['light', true, 'dark'],
    ['dark', false, 'light'],
    ['system', true, 'light'],
    ['system', false, 'dark'],
  ] as const)('from %s (system dark: %s) stores %s', (mode, dark, stored) => {
    system.dark = dark;
    applyMode(mode);
    toggleScheme();
    expect(appearance.mode).toBe(stored);
    expect(appearance.scheme).toBe(stored);
    expect(localStorage.getItem('solstice.theme')).toBe(stored);
  });

  it('keeps the skin', () => {
    applySkin('orbite');
    applyMode('light');
    toggleScheme();
    expect(appearance.skin).toBe('orbite');
  });
});
