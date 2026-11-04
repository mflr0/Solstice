import { readFileSync } from 'node:fs';
import { runInNewContext } from 'node:vm';
import { describe, expect, it } from 'vitest';

import {
  htmlAttributes,
  localized,
  MODES,
  parseMode,
  parseSkin,
  resolveScheme,
  type Mode,
  type Skin,
} from '../../src/core/appearance.js';

describe('parsing stored values', () => {
  it('keeps known skins and falls back to Carnet', () => {
    expect(parseSkin('orbite')).toBe('orbite');
    expect(parseSkin('crayon')).toBe('crayon');
    expect(parseSkin('retro')).toBe('retro');
    expect(parseSkin('plan')).toBe('plan');
    expect(parseSkin('carnet')).toBe('carnet');
    expect(parseSkin('a-theme-2')).toBe('a-theme-2');
    expect(parseSkin(null)).toBe('carnet');
    expect(parseSkin('../neon')).toBe('carnet');
    expect(parseSkin('Neon')).toBe('carnet');
  });

  it('keeps light and dark, anything else follows the system', () => {
    expect(parseMode('light')).toBe('light');
    expect(parseMode('dark')).toBe('dark');
    expect(parseMode('system')).toBe('system');
    expect(parseMode(undefined)).toBe('system');
    expect(parseMode('sepia')).toBe('system');
  });
});

describe('the scheme in effect', () => {
  it('follows the system only in system mode', () => {
    expect(resolveScheme('system', true)).toBe('dark');
    expect(resolveScheme('system', false)).toBe('light');
    expect(resolveScheme('light', true)).toBe('light');
    expect(resolveScheme('dark', false)).toBe('dark');
  });

  it('sets data-theme only for an explicit choice, as before', () => {
    expect(htmlAttributes('orbite', 'system', true)).toEqual({
      'data-skin': 'orbite',
      'data-theme': null,
      'data-scheme': 'dark',
    });
    expect(htmlAttributes('carnet', 'light', true)).toEqual({
      'data-skin': 'carnet',
      'data-theme': 'light',
      'data-scheme': 'light',
    });
  });
});

describe('manifest texts', () => {
  it('are in the locale, else in English', () => {
    const name = { en: 'Orbit', fr: 'Orbite' };
    expect(localized(name, 'fr')).toBe('Orbite');
    expect(localized(name, 'de')).toBe('Orbit');
  });
});

const SKINS = ['carnet', 'orbite', 'a-theme-2'];

/** Runs public/theme-boot.js against a fake page and returns <html>'s attributes. */
function boot(
  stored: Record<string, string>,
  systemDark: boolean,
  fetched: string[] = [],
): Record<string, string> {
  const attrs: Record<string, string> = {};
  const script = readFileSync(new URL('../../public/theme-boot.js', import.meta.url), 'utf8');
  runInNewContext(script, {
    document: {
      documentElement: {
        setAttribute: (k: string, v: string) => (attrs[k] = v),
        removeAttribute: (k: string) => delete attrs[k],
      },
    },
    localStorage: { getItem: (k: string) => stored[k] ?? null },
    window: { matchMedia: () => ({ matches: systemDark }) },
    fetch: (url: string) => {
      fetched.push(url);
      return Promise.resolve({ ok: true, text: () => Promise.resolve('') });
    },
    setTimeout: () => 0,
  });
  return attrs;
}

describe('the boot script', () => {
  it('starts the fetch of the stored skin', () => {
    const fetched: string[] = [];
    boot({ 'solstice.skin': 'a-theme-2' }, false, fetched);
    expect(fetched).toEqual(['/api/themes/a-theme-2/theme.css']);
  });

  it('falls back to Carnet on a value that is no id', () => {
    const fetched: string[] = [];
    const got = boot({ 'solstice.skin': '../../etc/passwd' }, false, fetched);
    expect(got['data-skin']).toBe('carnet');
    expect(fetched).toEqual(['/api/themes/carnet/theme.css']);
  });

  it('applies the same rules as the application', () => {
    for (const skin of SKINS) {
      for (const mode of MODES) {
        for (const systemDark of [false, true]) {
          const expected = htmlAttributes(skin as Skin, mode as Mode, systemDark);
          const got = boot({ 'solstice.skin': skin, 'solstice.theme': mode }, systemDark);
          for (const [name, value] of Object.entries(expected)) {
            expect(got[name] ?? null, `${skin}/${mode}/${systemDark} ${name}`).toBe(value);
          }
          expect('data-skin-loading' in got).toBe(true);
        }
      }
    }
  });

  it('survives garbage in storage', () => {
    expect(boot({ 'solstice.skin': '<script>', 'solstice.theme': 'x' }, false)).toEqual({
      'data-skin': 'carnet',
      'data-scheme': 'light',
      'data-skin-loading': '',
    });
  });
});
