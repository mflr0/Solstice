import { describe, expect, it } from 'vitest';

import { hasDarkVariant, scopeSkinCss, topLevelRules } from '../../src/core/skin-css.js';

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
