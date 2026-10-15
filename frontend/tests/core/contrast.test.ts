import { existsSync, readdirSync, readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';

import { SKINS, type Scheme, type Skin } from '../../src/core/appearance.js';

/**
 * Reads the colour tokens of every skin × scheme — Carnet from app.css, the
 * others from src/themes/<id>.css — resolving them the way the cascade does,
 * and checks WCAG AA contrast on the pairs the interface draws, plus any pair
 * a theme declares in a comment: `/* contrast: --a on --b [minimum] *\/`.
 * See src/themes/ANATOMY.md.
 */

const themesDir = new URL('../../src/themes/', import.meta.url);

function source(skin: Skin): string | null {
  if (skin === 'carnet') return readFileSync(new URL('../../src/app.css', import.meta.url), 'utf8');
  const file = new URL(`${skin}.css`, themesDir);
  return existsSync(file) ? readFileSync(file, 'utf8') : null;
}

/** The text between a block's braces, given the index of its "{". */
function blockBody(css: string, open: number): string {
  let depth = 0;
  for (let i = open; i < css.length; i++) {
    if (css[i] === '{') depth++;
    else if (css[i] === '}' && --depth === 0) return css.slice(open + 1, i);
  }
  throw new Error('unclosed block');
}

/** The custom properties declared directly in a block (not in nested rules). */
function declarations(body: string): Record<string, string> {
  const out: Record<string, string> = {};
  let depth = 0;
  let quote = '';
  let start = 0;
  const flush = (end: number) => {
    const text = body.slice(start, end).replace(/\/\*[\s\S]*?\*\//g, '');
    const m = /^\s*--([\w-]+)\s*:([\s\S]*)$/.exec(text);
    if (m) out[m[1]!] = m[2]!.trim();
    start = end + 1;
  };
  for (let i = 0; i < body.length; i++) {
    const c = body[i]!;
    if (quote) {
      if (c === '\\') i++;
      else if (c === quote) quote = '';
    } else if (c === '"' || c === "'") quote = c;
    else if (c === '/' && body[i + 1] === '*') {
      const end = body.indexOf('*/', i + 2);
      if (end < 0) break;
      i = end + 1;
    } else if (c === '(' || c === '{') depth++;
    else if (c === ')') depth--;
    else if (c === '}') {
      if (--depth === 0) start = i + 1;
    } else if (c === ';' && depth === 0) flush(i);
  }
  flush(body.length);
  return out;
}

function tokenBlock(css: string, skin: Skin, dark: boolean): Record<string, string> | null {
  const q = `(['"])`;
  const selector = dark
    ? `\\[data-skin=${q}${skin}\\1\\]\\[data-scheme=${q}dark\\2\\]`
    : `\\[data-skin=${q}${skin}\\1\\]`;
  const m = new RegExp(`(?:^|,)\\s*${selector}\\s*\\{`, 'm').exec(css);
  return m ? declarations(blockBody(css, m.index + m[0].length - 1)) : null;
}

type Rgb = [number, number, number];

function parseColour(value: string): Rgb[] {
  const out: Rgb[] = [];
  for (const m of value.matchAll(/#([0-9a-f]{3}|[0-9a-f]{6})\b/gi)) {
    const h = m[1]!.length === 3 ? [...m[1]!].map((c) => c + c).join('') : m[1]!;
    out.push([0, 2, 4].map((i) => parseInt(h.slice(i, i + 2), 16)) as Rgb);
  }
  for (const m of value.matchAll(/rgba?\(\s*(\d+)[\s,]+(\d+)[\s,]+(\d+)/gi)) {
    out.push([Number(m[1]), Number(m[2]), Number(m[3])]);
  }
  return out;
}

function luminance([r, g, b]: Rgb): number {
  const lin = (c: number) => {
    const s = c / 255;
    return s <= 0.03928 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4;
  };
  return 0.2126 * lin(r) + 0.7152 * lin(g) + 0.0722 * lin(b);
}

function contrast(a: Rgb, b: Rgb): number {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x) as [number, number];
  return (hi + 0.05) / (lo + 0.05);
}

/** A colour laid over another at the given opacity (tinted pills). */
function over(top: Rgb, bottom: Rgb, alpha: number): Rgb {
  return top.map((c, i) => Math.round(c * alpha + bottom[i]! * (1 - alpha))) as Rgb;
}

const extraPair = /\/\*\s*contrast:\s*--([\w-]+)\s+on\s+--([\w-]+)(?:\s+([\d.]+))?\s*\*\//g;

/** Every failing pair of a skin × scheme, as "foreground/background ratio". */
function failures(css: string, t: Record<string, string>, label: string): string[] {
  const colours = (name: string, seen: string[] = []): Rgb[] => {
    const v = t[name];
    if (v === undefined) throw new Error(`${label}: --${name} is not defined`);
    const ref = /^var\(--([\w-]+)\)$/.exec(v);
    if (ref) {
      if (seen.includes(ref[1]!)) throw new Error(`${label}: --${name} refers to itself`);
      return colours(ref[1]!, [...seen, name]);
    }
    const found = parseColour(v);
    if (found.length === 0) throw new Error(`${label}: --${name} = ${v} is not a colour`);
    return found;
  };
  const one = (name: string): Rgb => {
    const [c, ...rest] = colours(name);
    if (rest.length > 0) throw new Error(`${label}: --${name} must be a single colour`);
    return c!;
  };
  const out: string[] = [];
  const check = (fg: string, bg: string, min = 4.5, bgColours = colours(bg)) => {
    for (const f of colours(fg)) {
      for (const b of bgColours) {
        const c = contrast(f, b);
        if (c < min) out.push(`${fg}/${bg} ${c.toFixed(2)}`);
      }
    }
  };

  for (const bg of ['paper', 'paper-raised', 'paper-sunk']) {
    for (const fg of ['ink', 'ink-soft', 'muted']) check(fg, bg);
  }
  const fill = parseFloat(t['stamp-fill'] ?? '0') / 100;
  for (const bg of ['paper', 'paper-raised']) {
    check('accent-text', bg);
    for (const fg of ['stamp-todo', 'stamp-doing', 'stamp-done']) {
      check(fg, bg);
      check(fg, `${bg} tinted`, 4.5, [over(one(fg), one(bg), fill)]);
    }
  }
  for (const fg of ['tone-bug', 'tone-story', 'tone-task', 'tone-subtask']) {
    check(fg, 'paper-raised');
  }
  check('paper', 'tone-epic');
  check('primary-ink', 'primary');
  check('bar-ink', 'bar');
  check('bar-ink-soft', 'bar');
  // Focus rings and frames only need 3:1.
  check('focus', 'paper', 3);
  check('accent', 'paper', 3);
  for (const m of css.matchAll(extraPair)) check(m[1]!, m[2]!, m[3] ? Number(m[3]) : 4.5);
  return [...new Set(out)];
}

const carnet = tokenBlock(source('carnet')!, 'carnet', false)!;

describe('theme files', () => {
  it('are all known skins', () => {
    const files = readdirSync(themesDir).filter((f) => f.endsWith('.css'));
    for (const f of files) expect(SKINS, f).toContain(f.replace(/\.css$/, ''));
  });
});

describe('contrast (WCAG AA)', () => {
  for (const skin of SKINS) {
    const css = source(skin);
    const missing = css === null;
    const base = css ? tokenBlock(css, skin, false) : null;
    const dark = css ? tokenBlock(css, skin, true) : null;

    it.skipIf(missing)(`${skin} declares every token`, () => {
      expect(base, `no [data-skin='${skin}'] block`).not.toBeNull();
      const absent = Object.keys(carnet).filter((name) => !(name in base!));
      expect(absent).toEqual([]);
    });

    const schemes: Scheme[] = dark ? ['light', 'dark'] : ['light'];
    for (const scheme of schemes) {
      it.skipIf(missing)(`${skin} ${scheme}`, () => {
        const t = { ...base, ...(scheme === 'dark' ? dark : {}) };
        expect(failures(css!, t, `${skin}/${scheme}`)).toEqual([]);
      });
    }
  }
});
