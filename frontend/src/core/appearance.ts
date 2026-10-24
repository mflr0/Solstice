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
 * Carnet is drawn by src/app.css; every other skin by src/themes/<id>.css,
 * fetched when chosen (src/lib/skins.svelte.ts). public/theme-boot.js applies the
 * same rules before the first paint; keep the two in step.
 */

export type Skin =
  'carnet' | 'orbite' | 'crayon' | 'luna' | 'retro' | 'classique' | 'terminal' | 'plan';
export type Mode = 'system' | 'light' | 'dark';
export type Scheme = 'light' | 'dark';

/** In the order the theme menus list them. */
export const SKINS: readonly Skin[] = [
  'carnet',
  'orbite',
  'crayon',
  'luna',
  'retro',
  'classique',
  'terminal',
  'plan',
];
export const MODES: readonly Mode[] = ['system', 'light', 'dark'];

export const STORAGE_KEYS = {
  skin: 'solstice.skin',
  mode: 'solstice.theme',
  shortcuts: 'solstice.shortcuts',
} as const;

export function parseSkin(value: string | null | undefined): Skin {
  return SKINS.includes(value as Skin) ? (value as Skin) : 'carnet';
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

/**
 * Colours of each skin's light mode, for the little swatches in the account
 * menu (drawn before the other skins' stylesheets are loaded).
 */
export const SWATCHES: Record<Skin, { paper: string; ink: string; accent: string }> = {
  carnet: { paper: '#f4f1ea', ink: '#1c1b19', accent: '#d9381e' },
  orbite: { paper: '#0b1020', ink: '#e6eaf2', accent: '#ff8a3d' },
  crayon: { paper: '#f8f6f0', ink: '#2b2b2b', accent: '#2c5daa' },
  luna: { paper: '#ece9d8', ink: '#0a246a', accent: '#3a9a3a' },
  retro: { paper: '#008080', ink: '#c0c0c0', accent: '#000080' },
  classique: { paper: '#ffffff', ink: '#172b4d', accent: '#0052cc' },
  terminal: { paper: '#0a0f0a', ink: '#5cf08a', accent: '#5cf08a' },
  plan: { paper: '#1b4b82', ink: '#ffffff', accent: '#ffd166' },
};
