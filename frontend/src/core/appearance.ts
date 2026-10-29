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
 * Colours of each skin's light mode, for the little swatches on the Appearance
 * page (drawn before the other skins' stylesheets are loaded).
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

/** The colours a theme-menu rectangle draws: top bar, paper, ink and accent. */
export interface SchemeSwatch {
  bar: string;
  paper: string;
  ink: string;
  accent: string;
}

/**
 * Each skin's own colours in light and in dark, from its stylesheet's tokens,
 * for the top bar's theme menu (drawn before the skin's stylesheet is loaded).
 */
export const SCHEME_SWATCHES: Record<Skin, Record<Scheme, SchemeSwatch>> = {
  carnet: {
    light: { bar: '#faf8f3', paper: '#f4f1ea', ink: '#1c1b19', accent: '#d9381e' },
    dark: { bar: '#22211d', paper: '#1a1916', ink: '#ebe6da', accent: '#ec5a3d' },
  },
  orbite: {
    light: { bar: '#0f1a33', paper: '#f3f6fb', ink: '#0e1830', accent: '#c9580c' },
    dark: { bar: '#070c19', paper: '#0b1122', ink: '#e6eaf3', accent: '#ff8a3d' },
  },
  crayon: {
    light: { bar: '#f7f5ee', paper: '#f7f5ee', ink: '#262522', accent: '#2c5daa' },
    dark: { bar: '#1f2b27', paper: '#1f2b27', ink: '#ecebe4', accent: '#9cc0f0' },
  },
  luna: {
    light: { bar: '#2253bd', paper: '#efebde', ink: '#1a1f2b', accent: '#2350b5' },
    dark: { bar: '#1a3a83', paper: '#1d2333', ink: '#e7ebf4', accent: '#7ea6ff' },
  },
  retro: {
    light: { bar: '#c6c5bf', paper: '#1a7a77', ink: '#111111', accent: '#0b2a84' },
    dark: { bar: '#3c3d40', paper: '#0f3735', ink: '#ecebe6', accent: '#91b3ff' },
  },
  classique: {
    light: { bar: '#163463', paper: '#ffffff', ink: '#1d2533', accent: '#1d5bd0' },
    dark: { bar: '#142540', paper: '#1a1f27', ink: '#e3e8ef', accent: '#5b9bf3' },
  },
  terminal: {
    light: { bar: '#fbfcf7', paper: '#fbfcf7', ink: '#232723', accent: '#1d6b38' },
    dark: { bar: '#0f1912', paper: '#0b120d', ink: '#7ef29d', accent: '#9dffb5' },
  },
  plan: {
    light: { bar: '#ffffff', paper: '#f5f8fa', ink: '#0f2e57', accent: '#c42b1c' },
    dark: { bar: '#0a2645', paper: '#0d2e52', ink: '#f2f6fb', accent: '#ff9a84' },
  },
};
