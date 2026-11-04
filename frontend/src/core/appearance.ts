/**
 * Appearance: a skin (the art direction) and a colour mode.
 *
 * The skin and the user's mode choice are stored in localStorage. The page
 * carries these attributes on <html>:
 *   data-skin          a theme's id
 *   data-theme         light | dark, absent when following the system (as before)
 *   data-scheme        light | dark — the mode actually in effect, which the CSS
 *                      tokens key on, so "system" needs no duplicated media query.
 *   data-skin-loading  present while a skin's stylesheet is on its way.
 *
 * Each skin is a theme package the server holds: its manifest and its
 * stylesheet are fetched at run time (src/lib/skins.svelte.ts).
 * public/theme-boot.js applies the same rules before the first paint; keep the
 * two in step.
 */

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

/** The answer of GET /api/themes (backend/internal/themes, Index). */
export interface ThemeIndex {
  contract: number;
  themes: ThemeManifest[];
}

/** The theme shown when none is stored, or the stored one can't be loaded. */
export const DEFAULT_SKIN: Skin = 'carnet';

/** What a theme's id looks like (backend/internal/themes, idPattern). */
const SKIN_ID = /^[a-z0-9-]{1,32}$/;

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
  return value && SKIN_ID.test(value) ? value : DEFAULT_SKIN;
}

export function parseMode(value: string | null | undefined): Mode {
  return value === 'light' || value === 'dark' ? value : 'system';
}

/** The scheme in effect: the user's choice, or the system's when following it. */
export function resolveScheme(mode: Mode, systemDark: boolean): Scheme {
  if (mode === 'system') return systemDark ? 'dark' : 'light';
  return mode;
}

/** The skin of a page load: ?skin=<id> in the address, else the stored one. */
export function startingSkin(search: string, stored: string | null | undefined): Skin {
  return parseSkin(new URLSearchParams(search).get('skin') || stored);
}

/** The skin, or the default when the server's list doesn't have it; an empty list says nothing. */
export function offeredSkin(skin: Skin, offered: readonly { id: Skin }[]): Skin {
  return offered.length === 0 || offered.some((theme) => theme.id === skin) ? skin : DEFAULT_SKIN;
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
