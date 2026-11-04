/**
 * The administration's themes page: what the server says about a theme and
 * what can be done to it. Wording lives in i18n.
 */

import { DEFAULT_SKIN, type ThemeManifest } from './appearance.js';

export type ThemeOrigin = 'builtin' | 'imported';

/** A theme as administrators see it (backend/internal/themes, Entry). */
export interface AdminTheme extends ThemeManifest {
  manifestId: string;
  origin: ThemeOrigin;
  enabled: boolean;
  importedBy?: string;
  importedAt?: string;
}

/** The answer of GET /api/admin/themes. */
export interface AdminThemes {
  contract: number;
  themes: AdminTheme[];
}

/** Why a package was refused (backend/internal/themes, Finding). */
export interface ThemeFinding {
  rule: string;
  message: string;
  file: string;
  /** From 1; absent when it concerns the whole file. */
  line?: number;
}

/** The findings in a refused import's error data; none for any other error. */
export function themeFindings(data: unknown): ThemeFinding[] {
  if (!data || typeof data !== 'object' || !('findings' in data)) return [];
  return Array.isArray(data.findings) ? (data.findings as ThemeFinding[]) : [];
}

/** Where a finding is: theme.css:12, or the file alone. */
export function findingPlace(f: Pick<ThemeFinding, 'file' | 'line'>): string {
  return f.line ? `${f.file}:${f.line}` : f.file;
}

/**
 * What the server lets an administrator do to a theme, beyond previewing and
 * exporting: built-in ones stay, and the default can't be turned off since
 * everyone falls back to it.
 */
export function themeActions(theme: Pick<AdminTheme, 'id' | 'origin'>): {
  switchOff: boolean;
  replace: boolean;
  remove: boolean;
} {
  const imported = theme.origin === 'imported';
  return { switchOff: theme.id !== DEFAULT_SKIN, replace: imported, remove: imported };
}
