/**
 * Skin stylesheets: src/themes/<id>.css, each its own chunk, fetched the
 * first time the skin is shown (chosen, or previewed in the gallery) and
 * added to the end of <head>, so its rules win ties with the base ones.
 */

import type { Skin } from '../core/appearance.js';
import { hasDarkVariant, scopeSkinCss } from '../core/skin-css.js';

const sources = import.meta.glob<string>('../themes/*.css', {
  query: '?inline',
  import: 'default',
});

/**
 * What is known of each loaded skin: whether it draws a dark variant.
 * Carnet is the main stylesheet.
 */
export const skinSheets = $state<Partial<Record<Skin, { dark: boolean }>>>({
  carnet: { dark: true },
});

const pending: Partial<Record<Skin, Promise<void>>> = {};

const scopeSupported = typeof globalThis !== 'undefined' && 'CSSScopeRule' in globalThis;

export function isSkinLoaded(skin: Skin): boolean {
  return skinSheets[skin] !== undefined;
}

export function loadSkin(skin: Skin): Promise<void> {
  if (skin === 'carnet') return Promise.resolve();
  let p = pending[skin];
  if (!p) {
    const source = sources[`../themes/${skin}.css`];
    p = (source ? source() : Promise.resolve(''))
      .then((css) => {
        if (css) {
          const style = document.createElement('style');
          style.dataset.skinCss = skin;
          style.textContent = scopeSupported ? scopeSkinCss(css, skin) : css;
          document.head.append(style);
        }
        skinSheets[skin] = { dark: hasDarkVariant(css, skin) };
      })
      .catch(() => {
        // A failed chunk (network): try again next time.
        delete pending[skin];
      });
    pending[skin] = p;
  }
  return p;
}
