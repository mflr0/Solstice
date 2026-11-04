import { readdirSync, readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

import {
  hasDarkVariant,
  rebaseSkinCss,
  scopeSkinCss,
  topLevelRules,
} from '../../src/core/skin-css.js';

describe('fitting a stylesheet to where it is served', () => {
  it('turns the anchor into the served id, quoted or not', () => {
    const css = `[data-skin='retro'] { --a: 1; }
[data-skin="retro"][data-scheme='dark'] .x { --a: 2; }
@media (width < 1px) { [data-skin=retro] .y { --a: 3; } }`;
    const out = rebaseSkinCss(css, 'th-7');
    expect(out).toContain("[data-skin='th-7'] { --a: 1; }");
    expect(out).toContain("[data-skin='th-7'][data-scheme='dark'] .x { --a: 2; }");
    expect(out).toContain("[data-skin='th-7'] .y { --a: 3; }");
    expect(out).not.toContain('retro');
  });

  it('leaves another anchor alone', () => {
    expect(rebaseSkinCss("[data-skin='a'] { } [data-skin='b'] { }", 'c')).toBe(
      "[data-skin='c'] { } [data-skin='b'] { }",
    );
  });

  it('points package-relative url()s at the served path', () => {
    const out = rebaseSkinCss(
      `.a { background: url('assets/a.svg'); }
@font-face { src: url("fonts/b.woff2") format('woff2'); }
.c { mask: url(./assets/c.svg); }`,
      'th-7',
    );
    expect(out).toContain("url('/api/themes/th-7/assets/a.svg')");
    expect(out).toContain('url("/api/themes/th-7/fonts/b.woff2")');
    expect(out).toContain('url(/api/themes/th-7/assets/c.svg)');
  });

  it('leaves data:, absolute addresses and fragments alone', () => {
    const css = `.a { background: url("data:image/svg+xml,%3Csvg fill='url(#g)'/%3E"); }
.b { background: url(data:image/png;base64,AAAA); }
.c { background: url('/x.svg'); }
.d { background: url(https://example.com/x.svg); }
.e { filter: url(#rough); }`;
    expect(rebaseSkinCss(css, 'th-7')).toBe(css);
  });

  it('skips comments and strings', () => {
    const css = `/* don't [data-skin='x'] url(a.svg) */
.a::before { content: "url(a.svg)"; }
[data-skin='y'] { }`;
    expect(rebaseSkinCss(css, 'z')).toBe(`/* don't [data-skin='x'] url(a.svg) */
.a::before { content: "url(a.svg)"; }
[data-skin='z'] { }`);
  });
});

describe('the build', () => {
  it('bundles no theme: nothing under src imports a package', () => {
    const src = fileURLToPath(new URL('../../src/', import.meta.url));
    const files = readdirSync(src, { recursive: true, encoding: 'utf8' }).filter((f) =>
      /\.(?:ts|svelte|css)$/.test(f),
    );
    expect(files.length).toBeGreaterThan(0);
    const importing =
      /(?:from\s+|import\s*\(?\s*|glob[^(]*\(\s*\[?\s*|@import\s+)['"][^'"]*themes\/builtin/;
    expect(files.filter((f) => importing.test(readFileSync(src + f, 'utf8')))).toEqual([]);
  });
});

describe('splitting a stylesheet', () => {
  it('keeps braces and semicolons inside strings and comments', () => {
    const css = `@import 'a.css';
/* a { b } ; */
.x::before { content: '}' ; }
.y { background: url("data:image/svg+xml;utf8,<svg>{}</svg>"); }`;
    expect(topLevelRules(css)).toEqual([
      "@import 'a.css';",
      ".x::before { content: '}' ; }",
      '.y { background: url("data:image/svg+xml;utf8,<svg>{}</svg>"); }',
    ]);
  });
});

describe('scoping a skin', () => {
  const css = `@import '@fontsource/x/latin-400.css';
@font-face { font-family: X; src: url(x.woff2); }
[data-skin='luna'] { --paper: #ece9d8; }
[data-skin="luna"][data-scheme='dark'] .btn { color: red; }
@keyframes blink { to { opacity: 0; } }
@media (max-width: 700px) { [data-skin=luna] .topbar { height: 40px; } }`;
  const out = scopeSkinCss(css, 'luna');

  it('wraps the rules in @scope, stopping at a nested skin', () => {
    expect(out).toContain("@scope ([data-skin='luna']) to ([data-skin]) {");
    expect(out).toContain(':scope { --paper: #ece9d8; }');
    expect(out).toContain(":scope[data-scheme='dark'] .btn { color: red; }");
    expect(out).toContain('@media (max-width: 700px) { :scope .topbar { height: 40px; } }');
    expect(out.match(/data-skin=['"]?luna/g)).toHaveLength(1);
  });

  it('keeps fonts, imports and keyframes at the top level', () => {
    const scope = out.indexOf('@scope');
    for (const rule of ['@import', '@font-face', '@keyframes blink']) {
      expect(out.indexOf(rule), rule).toBeGreaterThanOrEqual(0);
      expect(out.indexOf(rule), rule).toBeLessThan(scope);
    }
  });

  it('leaves other skins alone', () => {
    expect(scopeSkinCss("[data-skin='plan'] .x { color: red; }", 'luna')).toContain(
      "[data-skin='plan'] .x",
    );
  });
});

describe('dark variants', () => {
  it('are found with or without quotes', () => {
    expect(hasDarkVariant("[data-skin='plan'][data-scheme='dark'] { }", 'plan')).toBe(true);
    expect(hasDarkVariant('[data-skin=plan][data-scheme=dark]{}', 'plan')).toBe(true);
    expect(hasDarkVariant("[data-skin='plan'] { }", 'plan')).toBe(false);
    expect(hasDarkVariant("[data-skin='luna'][data-scheme='dark'] { }", 'plan')).toBe(false);
  });
});
