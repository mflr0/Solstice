/**
 * Theme packages, from the server: the list (GET /api/themes) and each one's
 * stylesheet (GET /api/themes/<id>/theme.css), fetched the first time the skin
 * is shown (chosen, or previewed in the gallery) and added to the end of
 * <head>, so its rules win ties with the base ones. public/theme-boot.js has
 * already started the fetch of the stored skin.
 */

import type { Skin, ThemeManifest } from '../core/appearance.js';
import { hasDarkVariant, rebaseSkinCss, scopeSkinCss } from '../core/skin-css.js';
import { api } from './api.js';

/** The themes the server offers, in the order the theme menus list them. */
export const themes = $state<{ list: ThemeManifest[] }>({ list: [] });

let listing: Promise<void> | undefined;

/** Fetches the list once; a failed attempt can be tried again. */
export function loadThemes(): Promise<void> {
  listing ??= api
    .themes()
    .then((index) => {
      themes.list = index.themes;
    })
    .catch(() => {
      listing = undefined;
    });
  return listing;
}

/** What is known of each loaded skin: whether it draws a dark variant. */
export const skinSheets = $state<Partial<Record<Skin, { dark: boolean }>>>({});

const pending: Partial<Record<Skin, Promise<boolean>>> = {};

const scopeSupported = typeof globalThis !== 'undefined' && 'CSSScopeRule' in globalThis;

declare global {
  interface Window {
    /** The fetch public/theme-boot.js started; null when it failed. */
    solsticeBootSkin?: { id: string; css: Promise<string | null> };
  }
}

function bootFetch(skin: Skin): Promise<string | null> | undefined {
  const boot = typeof window !== 'undefined' ? window.solsticeBootSkin : undefined;
  if (boot?.id !== skin) return undefined;
  delete window.solsticeBootSkin;
  return boot.css;
}

async function fetchCss(skin: Skin): Promise<string | null> {
  try {
    const res = await fetch(`/api/themes/${skin}/theme.css`, { credentials: 'same-origin' });
    return res.ok ? await res.text() : null;
  } catch {
    return null;
  }
}

export function isSkinLoaded(skin: Skin): boolean {
  return skinSheets[skin] !== undefined;
}

/** Resolves to whether the skin's stylesheet is in. */
export function loadSkin(skin: Skin): Promise<boolean> {
  let p = pending[skin];
  if (!p) {
    p = (bootFetch(skin) ?? fetchCss(skin)).then((source) => {
      if (source === null) {
        // Try again next time.
        delete pending[skin];
        return false;
      }
      const css = rebaseSkinCss(source, skin);
      const style = document.createElement('style');
      style.dataset.skinCss = skin;
      style.textContent = scopeSupported ? scopeSkinCss(css, skin) : css;
      document.head.append(style);
      skinSheets[skin] = { dark: hasDarkVariant(css, skin) };
      return true;
    });
    pending[skin] = p;
  }
  return p;
}
