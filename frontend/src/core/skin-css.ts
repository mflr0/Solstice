/**
 * Turns a skin's stylesheet (src/themes/<id>.css) into one that only reaches
 * the elements that skin owns.
 *
 * A skin file anchors every selector on [data-skin='<id>']. Where the browser
 * supports @scope, the rules are wrapped in
 *   @scope ([data-skin='<id>']) to ([data-skin]) { … }
 * with the anchor rewritten to :scope (same specificity), so they stop at a
 * nested element of another skin — the previews of the theme gallery.
 * At-rules that cannot live in @scope (fonts, keyframes…) stay outside.
 */

/** At-rules kept at the top level, outside @scope. */
const TOP_LEVEL =
  /^@(?:charset|import|namespace|font-face|(?:-webkit-)?keyframes|property|counter-style|font-feature-values|font-palette-values)\b|^@layer\b[^{]*;$/i;

/** Splits a stylesheet into its top-level statements and blocks. */
export function topLevelRules(css: string): string[] {
  const out: string[] = [];
  let depth = 0;
  let start = 0;
  let i = 0;
  while (i < css.length) {
    const c = css[i];
    if (c === '/' && css[i + 1] === '*') {
      const end = css.indexOf('*/', i + 2);
      i = end < 0 ? css.length : end + 2;
      continue;
    }
    if (c === '"' || c === "'") {
      i++;
      while (i < css.length && css[i] !== c) i += css[i] === '\\' ? 2 : 1;
      i++;
      continue;
    }
    if (c === '{') {
      depth++;
    } else if (c === '}') {
      depth--;
      if (depth === 0) {
        out.push(css.slice(start, i + 1));
        start = i + 1;
      }
    } else if (c === ';' && depth === 0) {
      out.push(css.slice(start, i + 1));
      start = i + 1;
    }
    i++;
  }
  out.push(css.slice(start));
  return out.map(stripComments).filter((r) => r !== '');
}

function stripComments(rule: string): string {
  return rule.replace(/^(?:\s*\/\*[\s\S]*?\*\/)*\s*/, '').trim();
}

function anchor(skin: string): RegExp {
  return new RegExp(`\\[data-skin\\s*=\\s*(['"]?)${skin}\\1\\s*\\]`, 'g');
}

export function scopeSkinCss(css: string, skin: string): string {
  const outside: string[] = [];
  const inside: string[] = [];
  for (const rule of topLevelRules(css)) {
    (TOP_LEVEL.test(rule) ? outside : inside).push(rule);
  }
  const scoped = inside.join('\n').replace(anchor(skin), ':scope');
  return [...outside, `@scope ([data-skin='${skin}']) to ([data-skin]) {\n${scoped}\n}`].join('\n');
}

/** Whether a skin's stylesheet draws a dark variant. */
export function hasDarkVariant(css: string, skin: string): boolean {
  return new RegExp(
    `\\[data-skin\\s*=\\s*(['"]?)${skin}\\1\\s*\\]\\[data-scheme\\s*=\\s*(['"]?)dark\\2\\s*\\]`,
  ).test(css);
}
