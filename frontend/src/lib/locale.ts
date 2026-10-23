/**
 * Choosing the interface language. The choice is kept on this browser and on
 * the account, and the page reloads in it: nothing re-renders in place.
 */

import { locale, saveLocale } from '../i18n/index.js';
import { parseLocale, type Locale } from '../i18n/locales.js';
import { accountApi } from './admin-api.js';

export async function chooseLocale(next: Locale): Promise<void> {
  if (next === locale) return;
  saveLocale(next);
  try {
    await accountApi.updateProfile({ locale: next });
  } catch {
    // Offline, or the server refused it: this browser still remembers it.
  }
  location.reload();
}

/**
 * Once signed in, the account's language wins over this browser's. Returns
 * true when the page is reloading into it.
 */
export function adoptAccountLocale(value: string | null | undefined): boolean {
  const l = parseLocale(value);
  if (!l || l === locale) return false;
  saveLocale(l);
  location.reload();
  return true;
}
