/**
 * A theme package kept in the user's browser: its manifest as far as the
 * interface can use it, and its stylesheet with the package's files inlined,
 * since nothing serves them. Nothing here judges the package: the user chose to
 * trust it.
 */

import type { Scheme, SchemeSwatch, ThemeManifest } from './appearance.js';

/** The files of an unpacked zip, by path. */
export type PackageFiles = Record<string, Uint8Array>;

const MEDIA_TYPES: Record<string, string> = {
  svg: 'image/svg+xml',
  png: 'image/png',
  jpg: 'image/jpeg',
  jpeg: 'image/jpeg',
  webp: 'image/webp',
  gif: 'image/gif',
  woff2: 'font/woff2',
  woff: 'font/woff',
  ttf: 'font/ttf',
  otf: 'font/otf',
};

const URL_CALL = String.raw`(?<![\w-])url\(\s*("[^"]*"|'[^']*'|[^'")\s]*)\s*\)`;
const STRING = String.raw`"(?:[^"\\]|\\.)*"|'(?:[^'\\]|\\.)*'`;
/** A comment or a string left alone, a url(), or a function that loads bare strings. */
const CSS_TOKENS = new RegExp(
  String.raw`\/\*[\s\S]*?\*\/|(?<![\w-])(?:-webkit-)?(?:image-set|image|src)\((?:[^()]|\([^()]*\))*\)|${URL_CALL}|${STRING}`,
  'gi',
);
/** Inside such a function: type() names a format, the rest loads a file. */
const LOADER_TOKENS = new RegExp(String.raw`(?<![\w-])type\([^)]*\)|${URL_CALL}|${STRING}`, 'gi');

function base64(bytes: Uint8Array): string {
  const chunk = 0x8000;
  let binary = '';
  for (let i = 0; i < bytes.length; i += chunk) {
    binary += String.fromCharCode(...bytes.subarray(i, i + chunk));
  }
  return btoa(binary);
}

/** Replaces each file of the package a stylesheet loads with a data: URL; other targets stay. */
export function inlineFiles(css: string, files: PackageFiles): string {
  const inlined = new Map<string, string>();
  const dataUrl = (name: string, bytes: Uint8Array): string => {
    let url = inlined.get(name);
    if (url === undefined) {
      const type = MEDIA_TYPES[name.slice(name.lastIndexOf('.') + 1).toLowerCase()];
      url = `data:${type ?? 'application/octet-stream'};base64,${base64(bytes)}`;
      inlined.set(name, url);
    }
    return url;
  };
  const target = (value: string): string => {
    const hash = value.indexOf('#');
    const name = (hash < 0 ? value : value.slice(0, hash)).replace(/^(?:\.\/)+/, '');
    const bytes = Object.hasOwn(files, name) ? files[name] : undefined;
    return bytes ? dataUrl(name, bytes) + (hash < 0 ? '' : value.slice(hash)) : value;
  };
  const inQuotes = (token: string): string =>
    token[0] + target(token.slice(1, -1)) + token[token.length - 1];
  const urlCall = (url: string): string =>
    /^["']/.test(url) ? `url(${inQuotes(url)})` : `url(${target(url)})`;

  return css.replace(CSS_TOKENS, (token, url?: string) => {
    if (url !== undefined) return urlCall(url);
    if (!token.endsWith(')') || token.startsWith('/*')) return token;
    const open = token.indexOf('(');
    const args = token.slice(open).replace(LOADER_TOKENS, (inner, innerUrl?: string) => {
      if (innerUrl !== undefined) return urlCall(innerUrl);
      return /^["']/.test(inner) ? inQuotes(inner) : inner;
    });
    return token.slice(0, open) + args;
  });
}

function isTexts(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function texts(value: unknown): Record<string, string> {
  if (!isTexts(value)) return {};
  return Object.fromEntries(
    Object.entries(value).filter(
      (entry): entry is [string, string] => typeof entry[1] === 'string',
    ),
  );
}

const COLOUR = /^#[0-9a-f]{6}$/i;

function swatch(value: unknown): SchemeSwatch | undefined {
  if (!isTexts(value)) return undefined;
  const { bar, paper, ink, accent } = value;
  const colours = [bar, paper, ink, accent];
  if (!colours.every((c) => typeof c === 'string' && COLOUR.test(c))) return undefined;
  return { bar, paper, ink, accent } as SchemeSwatch;
}

/**
 * The manifest as the interface can draw it, whatever the package says: null
 * only without an id to serve the theme under, as the rest has a default.
 */
export function parseLocalManifest(source: string): ThemeManifest | null {
  let raw: unknown;
  try {
    raw = JSON.parse(source);
  } catch {
    return null;
  }
  if (!isTexts(raw) || typeof raw.id !== 'string' || !/^[a-z0-9-]{1,32}$/.test(raw.id)) return null;
  const name = texts(raw.name);
  const schemes: Scheme[] =
    Array.isArray(raw.schemes) && raw.schemes.includes('dark') ? ['light', 'dark'] : ['light'];
  const preview: ThemeManifest['preview'] = {};
  if (isTexts(raw.preview)) {
    for (const scheme of schemes) {
      const colours = swatch(raw.preview[scheme]);
      if (colours) preview[scheme] = colours;
    }
  }
  return {
    id: raw.id,
    contract: typeof raw.contract === 'number' ? raw.contract : 0,
    version: typeof raw.version === 'string' ? raw.version : '',
    name: name.en ? name : { ...name, en: raw.id },
    description: texts(raw.description),
    ...(isTexts(raw.about) ? { about: texts(raw.about) } : {}),
    author: typeof raw.author === 'string' ? raw.author : '',
    schemes,
    preview,
  };
}

/** The manifest and inlined stylesheet of an unpacked zip; null when either is missing. */
export function readPackage(files: PackageFiles): { manifest: ThemeManifest; css: string } | null {
  const manifestFile = files['theme.json'];
  const cssFile = files['theme.css'];
  if (!manifestFile || !cssFile) return null;
  const decoder = new TextDecoder();
  const manifest = parseLocalManifest(decoder.decode(manifestFile));
  return manifest && { manifest, css: inlineFiles(decoder.decode(cssFile), files) };
}
