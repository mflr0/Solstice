/**
 * The interface languages and how one is picked. Pure functions, tested;
 * public/theme-boot.js repeats negotiate() to set <html lang> before the
 * first paint — keep the two in step.
 */

export type Locale = 'fr' | 'en' | 'de' | 'it' | 'hi';

/** In the order the language menu lists them; French, the source, first. */
export const LOCALES: readonly Locale[] = ['fr', 'en', 'de', 'it', 'hi'];

/** Each language in its own name, as the menu shows it whatever the interface language. */
export const LOCALE_NAMES: Record<Locale, string> = {
  fr: 'Français',
  en: 'English',
  de: 'Deutsch',
  it: 'Italiano',
  hi: 'हिन्दी',
};

export const LOCALE_STORAGE_KEY = 'solstice.locale';

export function parseLocale(value: unknown): Locale | null {
  return LOCALES.includes(value as Locale) ? (value as Locale) : null;
}

/** "de-CH" → "de", "pt_BR" → "pt". */
function languageOf(tag: string): string {
  return tag.split(/[-_]/)[0]?.toLowerCase() ?? '';
}

/**
 * The language to show: the one chosen on this browser, else the first of
 * the browser's languages Solstice speaks (regional variants count as their
 * language), else French.
 */
export function negotiate(saved: string | null | undefined, languages: readonly string[]): Locale {
  const chosen = parseLocale(saved);
  if (chosen) return chosen;
  for (const tag of languages) {
    const l = parseLocale(languageOf(tag));
    if (l) return l;
  }
  return 'en';
}

/**
 * The tag dates and numbers follow: the browser's own variant of the
 * language when it lists one ("en-GB" writes 12 March, "en-US" March 12),
 * else the language itself.
 */
export function formatTag(locale: Locale, languages: readonly string[]): string {
  for (const tag of languages) {
    if (languageOf(tag) !== locale) continue;
    try {
      return Intl.getCanonicalLocales(tag.replace('_', '-'))[0] ?? locale;
    } catch {
      return locale;
    }
  }
  return locale;
}
