import { readdirSync, readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';

import { topLevelRules } from '../../src/core/skin-css.js';

const MIN_RING_PX = 2;

// What script focuses or what draws its ring elsewhere: not a stop that loses its ring.
const NO_RING_OF_THEIR_OWN = [
  /^\.app-main(?: h1)?:focus-visible$/,
  /^\.filter-popover:focus$/,
  /^\.summary:focus-visible$/,
];

function declarations(body: string): [string, string][] {
  return body.split(';').flatMap((declaration) => {
    const colon = declaration.indexOf(':');
    if (colon < 0) return [];
    return [[declaration.slice(0, colon).trim(), declaration.slice(colon + 1).trim()]];
  });
}

function ringWidth(value: string): number | undefined {
  const px = /(?:^|\s)(\d*\.?\d+)px\b/.exec(value);
  return px ? Number(px[1]) : undefined;
}

function hasReplacementRing(body: string): boolean {
  return declarations(body).some(([name, value]) => {
    const spread = /\b0 0 0 (\d*\.?\d+)px/.exec(value);
    return name === 'box-shadow' && spread !== null && Number(spread[1]) >= MIN_RING_PX;
  });
}

/** The focus rules of a stylesheet whose outline is too thin or has no replacement. */
function thinRings(css: string): string[] {
  const out: string[] = [];
  const visit = (source: string) => {
    for (const rule of topLevelRules(source)) {
      const open = rule.indexOf('{');
      if (open < 0) continue;
      const selector = rule.slice(0, open).trim();
      const body = rule.slice(open + 1, rule.lastIndexOf('}'));
      if (selector.startsWith('@')) {
        visit(body);
        continue;
      }
      const focused = selector
        .split(',')
        .map((part) => part.trim())
        .filter((part) => /:focus(?!-within)/.test(part));
      if (focused.every((part) => NO_RING_OF_THEIR_OWN.some((allowed) => allowed.test(part)))) {
        continue;
      }
      for (const [name, value] of declarations(body)) {
        if (name !== 'outline' && name !== 'outline-width') continue;
        const none = value === 'none' || value === '0';
        const fine = none ? hasReplacementRing(body) : (ringWidth(value) ?? 0) >= MIN_RING_PX;
        if (!fine) out.push(`${selector.replace(/\s+/g, ' ')} { ${name}: ${value} }`);
      }
    }
  };
  visit(css.replace(/\/\*[\s\S]*?\*\//g, ''));
  return out;
}

function svelteFiles(dir: URL): URL[] {
  return readdirSync(dir, { withFileTypes: true }).flatMap((entry) => {
    if (entry.isDirectory()) return svelteFiles(new URL(`${entry.name}/`, dir));
    return entry.name.endsWith('.svelte') ? [new URL(entry.name, dir)] : [];
  });
}

function svelteStyle(file: URL): string {
  return /<style[^>]*>([\s\S]*)<\/style>/.exec(readFileSync(file, 'utf8'))?.[1] ?? '';
}

const builtin = new URL('../../../backend/internal/themes/builtin/', import.meta.url);
const skins = readdirSync(builtin, { withFileTypes: true })
  .filter((entry) => entry.isDirectory())
  .map((entry) => entry.name);

describe('the check itself', () => {
  it('reports a thin ring, inside an at-rule too', () => {
    expect(thinRings('.a:focus-visible { outline: 1px dotted red; }')).toHaveLength(1);
    expect(thinRings('@media print { .a:focus { outline-width: 1px; } }')).toHaveLength(1);
    expect(thinRings('.a:focus-visible { outline: 2px dashed red; }')).toEqual([]);
  });

  it('takes none only with a 2px ring in the same rule', () => {
    expect(thinRings('.a:focus { outline: none; box-shadow: 0 0 0 1px red; }')).toHaveLength(1);
    expect(thinRings('.a:focus { outline: none; box-shadow: 0 0 0 2px red; }')).toEqual([]);
  });

  it('leaves other rules alone', () => {
    expect(thinRings('.a:hover { outline: 1px solid red; }')).toEqual([]);
    expect(thinRings('.a:focus-within { outline: 1px solid red; }')).toEqual([]);
  });
});

describe('the focus ring is at least 2px', () => {
  it('finds the skins', () => {
    expect(skins).toContain('carnet');
    expect(skins.length).toBeGreaterThanOrEqual(8);
  });

  it('in app.css', () => {
    const css = readFileSync(new URL('../../src/app.css', import.meta.url), 'utf8');
    expect(thinRings(css)).toEqual([]);
  });

  it.each(skins)('in %s', (skin) => {
    expect(thinRings(readFileSync(new URL(`${skin}/theme.css`, builtin), 'utf8'))).toEqual([]);
  });

  it('in the components', () => {
    const found = svelteFiles(new URL('../../src/', import.meta.url)).flatMap((file) =>
      thinRings(svelteStyle(file)).map((rule) => `${file.pathname.split('/src/')[1]}: ${rule}`),
    );
    expect(found).toEqual([]);
  });
});
