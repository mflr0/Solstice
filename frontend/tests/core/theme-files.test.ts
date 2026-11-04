import { existsSync, readdirSync, readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';

/**
 * A skin carries everything it needs in its own folder, so it can be served
 * as a package at run time: no @import (no npm package to resolve), and every
 * url() points into src/themes/<id>/ or is a data: URI.
 */

const themes = new URL('../../src/themes/', import.meta.url);

function stripComments(css: string): string {
  return css.replace(/\/\*[\s\S]*?\*\//g, '');
}

/** The targets of a stylesheet's url() calls, unquoted. */
function urls(css: string): string[] {
  return [...stripComments(css).matchAll(/url\(\s*(['"]?)(.*?)\1\s*\)/g)].map((m) => m[2]!);
}

describe('url() targets', () => {
  it('are found, quoted or not', () => {
    expect(
      urls(`.a { background: url('./x/a.svg'), url("data:image/svg+xml,%3Csvg fill='url(%23g)'/%3E"); }
/* url('./y/commented.svg') */
.b { mask: url( ./x/b.svg#draw ); }`),
    ).toEqual(['./x/a.svg', "data:image/svg+xml,%3Csvg fill='url(%23g)'/%3E", './x/b.svg#draw']);
  });
});

describe.each(
  readdirSync(themes)
    .filter((f) => f.endsWith('.css'))
    .map((f) => [f.slice(0, -'.css'.length)]),
)('skin %s', (id) => {
  const css = readFileSync(new URL(`${id}.css`, themes), 'utf8');

  it('has no @import', () => {
    expect(stripComments(css)).not.toMatch(/@import\b/);
  });

  it('only loads files from its own folder', () => {
    for (const url of urls(css)) {
      if (url.startsWith('data:')) continue;
      expect(url, url).toMatch(new RegExp(`^\\./${id}/`));
      expect(existsSync(new URL(url.replace(/#.*/, ''), themes)), url).toBe(true);
    }
  });
});
