/**
 * Appearance: a skin (the art direction) and a colour mode.
 *
 * The skin and the user's mode choice are stored in localStorage. The page
 * carries these attributes on <html>:
 *   data-skin          one of SKINS
 *   data-theme         light | dark, absent when following the system (as before)
 *   data-scheme        light | dark — the mode actually in effect, which the CSS
 *                      tokens key on, so "system" needs no duplicated media query.
 *   data-skin-loading  present while a skin's stylesheet is on its way
 *                      (Carnet's is the main one, so never for Carnet).
 *
 * Each skin is a theme package, backend/internal/themes/builtin/<id>/: its
 * theme.json (THEMES) and theme.css. Carnet's stylesheet is part of the main
 * one, the others are fetched when chosen (src/lib/skins.svelte.ts).
 * public/theme-boot.js applies the same rules before the first paint; keep the
 * two in step.
 */

import order from '../../../backend/internal/themes/builtin/order.json';

/** A theme package's id. */
export type Skin = string;
export type Mode = 'system' | 'light' | 'dark';
export type Scheme = 'light' | 'dark';

/** The colours a theme's thumbnail draws: top bar, paper, ink and accent. */
export interface SchemeSwatch {
  bar: string;
  paper: string;
  ink: string;
  accent: string;
}

/** A theme package's theme.json (backend/internal/themes, Manifest). */
export interface ThemeManifest {
  id: Skin;
  contract: number;
  version: string;
  /** Locale → text; English always present. */
  name: Record<string, string>;
  /** Locale → a few words on the look. */
  description: Record<string, string>;
  /** Locale → the theme gallery's longer sentence, optional. */
  about?: Record<string, string>;
  author: string;
  schemes: Scheme[];
  preview: Partial<Record<Scheme, SchemeSwatch>>;
}

const manifests = import.meta.glob<ThemeManifest>(
  '../../../backend/internal/themes/builtin/*/theme.json',
  { eager: true, import: 'default' },
);

/** The built-in themes, in the order the theme menus list them. */
export const THEMES: readonly ThemeManifest[] = order.map((id) => {
  const m = manifests[`../../../backend/internal/themes/builtin/${id}/theme.json`];
  if (!m) throw new Error(`theme ${id}: no theme.json`);
  return m;
});

export const SKINS: readonly Skin[] = THEMES.map((m) => m.id);

const ids = new Set(SKINS);

/** A manifest text in the locale, else in English. */
export function localized(texts: Record<string, string>, locale: string): string {
  return texts[locale] ?? texts.en ?? '';
}

export const MODES: readonly Mode[] = ['system', 'light', 'dark'];

export const STORAGE_KEYS = {
  skin: 'solstice.skin',
  mode: 'solstice.theme',
} as const;

export function parseSkin(value: string | null | undefined): Skin {
  return value && ids.has(value) ? value : 'carnet';
}

export function parseMode(value: string | null | undefined): Mode {
  return value === 'light' || value === 'dark' ? value : 'system';
}

/** The scheme in effect: the user's choice, or the system's when following it. */
export function resolveScheme(mode: Mode, systemDark: boolean): Scheme {
  if (mode === 'system') return systemDark ? 'dark' : 'light';
  return mode;
}

/** The attributes <html> should carry; null removes the attribute. */
export function htmlAttributes(
  skin: Skin,
  mode: Mode,
  systemDark: boolean,
): Record<'data-skin' | 'data-theme' | 'data-scheme', string | null> {
  return {
    'data-skin': skin,
    'data-theme': mode === 'system' ? null : mode,
    'data-scheme': resolveScheme(mode, systemDark),
  };
}
