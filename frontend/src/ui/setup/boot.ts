/**
 * Starts the setup wizard (/setup) instead of the application. Its language
 * is picked as the application's is (src/i18n/locales.ts), from its own
 * catalogues.
 */

import { mount } from 'svelte';
import { LOCALE_STORAGE_KEY, negotiate } from '../../i18n/locales.js';
import SetupApp from './SetupApp.svelte';
import './setup.css';

export function startSetup(target: HTMLElement): void {
  let saved: string | null = null;
  try {
    saved = localStorage.getItem(LOCALE_STORAGE_KEY);
  } catch {
    // Storage unavailable: the browser's languages decide.
  }
  const initialLocale = negotiate(
    saved,
    typeof navigator !== 'undefined' ? navigator.languages : [],
  );
  mount(SetupApp, { target, props: { initialLocale } });
}
