/**
 * Turns a skin's stylesheet (its package's theme.css) into one that only reaches
 * the elements that skin owns.
 *
 * A skin file anchors every selector on [data-skin='<id>']. Where the browser
 * supports @scope, the rules are wrapped in
 *   @scope ([data-skin='<id>']) to ([data-skin]) { … }
 * with the anchor rewritten to :scope (same specificity), so they stop at a
 * nested element of another skin — the previews of the theme gallery.
 * At-rules that cannot live in @scope (fonts, keyframes…) stay outside.
 */

const TOKENS =
  /\/\*[\s\S]*?\*\/|url\(\s*("[^"]*"|'[^']*'|[^'")\s]*)\s*\)|\[data-skin\s*=\s*("[^"]*"|'[^']*'|[\w-]*)\s*\]|"(?:[^"\\]|\\.)*"|'(?:[^'\\]|\\.)*'/g;

/** Leaves absolute addresses alone: data:, https:, /path and #fragment. */
const ABSOLUTE_URL = /^(?:[a-z][a-z0-9+.-]*:|\/|#)/i;

function unquoted(token: string): string {
  return token.replace(/^(['"])([\s\S]*)\1$/, '$2');
}

/**
 * Makes a package's stylesheet fit the place it is served from: its anchor
 * (the first one it holds, the manifest's id) becomes the served id, and its
 * package-relative url()s point at /api/themes/<id>/.
 */
export function rebaseSkinCss(css: string, id: string): string {
  let manifestId: string | undefined;
  return css.replace(TOKENS, (token, url?: string, anchor?: string) => {
    if (anchor !== undefined) {
      const name = unquoted(anchor);
      manifestId ??= name;
      return name === manifestId ? `[data-skin='${id}']` : token;
    }
    if (url === undefined) return token;
    const target = unquoted(url);
    if (target === '' || ABSOLUTE_URL.test(target)) return token;
    const quote = url.length > target.length ? url[0] : '';
    return `url(${quote}/api/themes/${id}/${target.replace(/^(?:\.\/)+/, '')}${quote})`;
  });
}

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
