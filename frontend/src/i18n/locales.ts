/**
 * The interface languages and how one is picked. Pure functions, tested;
 * public/theme-boot.js repeats negotiate() to set <html lang> before the
 * first paint — keep the two in step.
 */

export type Locale = 'fr' | 'en' | 'de' | 'it' | 'hi';

/** In the order the language menu lists them. */
export const LOCALES: readonly Locale[] = ['en', 'fr', 'de', 'it', 'hi'];

/** Each language in its own name, as the menu shows it whatever the interface language. */
export const LOCALE_NAMES: Record<Locale, string> = {
  fr: 'Français',
  en: 'English',
  de: 'Deutsch',
  it: 'Italiano',
  hi: 'हिन्दी',
};

/** A language as the language choosers list it: its code badge, its own name, and what finds it. */
export interface LocaleOption {
  value: Locale;
  /** The badge: "EN", "FR"… */
  code: string;
  label: string;
  lang: Locale;
  /** The code and the language's name in the interface language ("German" for de in English). */
  keywords: string[];
}

/**
 * The options of every language chooser, in LOCALES order, keeping only
 * those of `only` when given. `display` is the interface language.
 */
export function localeOptions(display: string, only?: readonly string[]): LocaleOption[] {
  let names: Intl.DisplayNames | null = null;
  try {
    names = new Intl.DisplayNames([display], { type: 'language' });
  } catch {
    // No display names here: the code and the native name still find it.
  }
  return LOCALES.filter((l) => !only || only.includes(l)).map((l) => ({
    value: l,
    code: l.toUpperCase(),
    label: LOCALE_NAMES[l],
    lang: l,
    keywords: [l, names?.of(l) ?? ''].filter(Boolean),
  }));
}

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
 * language), else English.
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
