import { describe, expect, it } from 'vitest';

import { LOCALES, localeOptions } from '../../src/i18n/locales.js';

describe('localeOptions', () => {
  it('lists the five languages in LOCALES order, with their codes and native names', () => {
    const options = localeOptions('en');
    expect(options.map((o) => o.value)).toEqual(LOCALES);
    expect(options.map((o) => o.code)).toEqual(['EN', 'FR', 'DE', 'IT', 'HI']);
    expect(options.map((o) => o.label)).toEqual([
      'English',
      'Français',
      'Deutsch',
      'Italiano',
      'हिन्दी',
    ]);
    expect(options.every((o) => o.lang === o.value)).toBe(true);
  });

  it('finds a language by its name in the interface language and by its code', () => {
    const de = (display: string) => localeOptions(display).find((o) => o.value === 'de')!;
    expect(de('en').keywords).toEqual(['de', 'German']);
    expect(de('fr').keywords).toEqual(['de', 'allemand']);
  });

  it('keeps only the languages asked for, in LOCALES order', () => {
    expect(localeOptions('en', ['de', 'en', 'xx']).map((o) => o.value)).toEqual(['en', 'de']);
  });
});
