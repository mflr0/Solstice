/**
 * The interface language: chosen once at boot, before the application mounts,
 * and never changed while a page lives (switching reloads it). French ships
 * with the application; any other catalogue is fetched on its own, so a
 * visit downloads one language only.
 *
 * Components read strings from `t` (src/i18n/fr.ts), a live binding that
 * setLocale() points at the catalogue in use. Read `t` inside functions and
 * components, never at the top level of a module: that would run before boot
 * and keep French.
 */

import { en } from './en.js';
import { setFormatLocale } from './format.js';
import { formatTag, LOCALE_STORAGE_KEY, negotiate, type Locale } from './locales.js';
import { useMessages } from './t.js';
import type { Messages } from './types.js';

export { LOCALE_NAMES, LOCALES, parseLocale, type Locale } from './locales.js';

/** The language in use. */
export let locale: Locale = 'en';

const catalogues: Record<Exclude<Locale, 'en'>, () => Promise<Messages>> = {
  fr: () => import('./fr.js').then((m) => m.fr),
  de: () => import('./de.js').then((m) => m.de),
  it: () => import('./it.js').then((m) => m.it),
  hi: () => import('./hi.js').then((m) => m.hi),
};

/** What a language needs besides words: Hindi, a Devanagari face in every skin. */
const extras: Partial<Record<Locale, () => Promise<unknown>>> = {
  hi: () => import('./devanagari.css'),
};

/** Loads a language and makes it the interface's. */
export async function setLocale(next: Locale, languages: readonly string[] = []): Promise<void> {
  const [messages] = await Promise.all([next === 'en' ? en : catalogues[next](), extras[next]?.()]);
  useMessages(messages);
  setFormatLocale(formatTag(next, languages));
  locale = next;
  if (typeof document !== 'undefined') document.documentElement.lang = next;
}

/** The language chosen on this browser, if any. */
export function savedLocale(): string | null {
  try {
    return localStorage.getItem(LOCALE_STORAGE_KEY);
  } catch {
    return null;
  }
}

export function saveLocale(l: Locale): void {
  try {
    localStorage.setItem(LOCALE_STORAGE_KEY, l);
  } catch {
    // Private browsing: the account still remembers it.
  }
}

/**
 * Boot: the language chosen here, else the browser's, else French. A
 * catalogue that fails to load (offline) leaves French rather than no page.
 */
export async function initLocale(): Promise<Locale> {
  const languages = typeof navigator !== 'undefined' ? (navigator.languages ?? []) : [];
  try {
    await setLocale(negotiate(savedLocale(), languages), languages);
  } catch {
    await setLocale('en');
  }
  return locale;
}
