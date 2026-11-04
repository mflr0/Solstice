import { existsSync, readdirSync, readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';

/**
 * A skin carries everything it needs in its package, so it can be served at
 * run time: no @import (no npm package to resolve), and every url() points
 * into the package's assets/ or fonts/, relative to theme.css, or is a data:
 * URI.
 */

const themes = new URL('../../../backend/internal/themes/builtin/', import.meta.url);

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
  readdirSync(themes, { withFileTypes: true })
    .filter((e) => e.isDirectory())
    .map((e) => [e.name]),
)('skin %s', (id) => {
  const dir = new URL(`${id}/`, themes);
  const css = readFileSync(new URL('theme.css', dir), 'utf8');

  it('has no @import', () => {
    expect(stripComments(css)).not.toMatch(/@import\b/);
  });

  it('only loads files from its own package', () => {
    for (const url of urls(css)) {
      if (url.startsWith('data:')) continue;
      expect(url, url).toMatch(/^(assets|fonts)\/(?!.*\.\.)/);
      expect(existsSync(new URL(url.replace(/#.*/, ''), dir)), url).toBe(true);
    }
  });
});
