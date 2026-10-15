/**
 * Jira (Server / Data Center) wiki markup → HTML.
 *
 * Descriptions and comments imported from Jira are written in this markup, and
 * Solstice keeps it as its storage format. The output is meant for Svelte's
 * `{@html}`, so it is safe by construction: every character of the source is
 * HTML-escaped, and the only tags in the output are the ones generated here.
 * Links are restricted to http(s), mailto, ftp and same-site URLs.
 *
 * Rendering happens in two passes:
 * 1. Multi-line macros ({code}, {noformat}, {quote}, {panel}) are rendered
 *    first and replaced by placeholders on their own line, so their content is
 *    never seen by the line-based parser.
 * 2. Lines are grouped into blocks (paragraphs, headings, lists, tables…) and
 *    the text of each block goes through the inline parser.
 *
 * The rich editor (wiki-doc.ts) reads markup with the same rules, so the
 * scanning helpers and patterns below are exported for it.
 */

/** Looks up what the markup refers to; the renderer itself has no data. */
export interface WikiResolver {
  /** `[~username]` → display name, or null when unknown. */
  user(username: string): string | null;
  /** `!capture.png!` / `[^spec.pdf]` → URL of the issue attachment with that file name, or null. */
  attachment(filename: string): string | null;
  /** `DEMO-12` → true when the issue exists (it is then linked). */
  issueExists(key: string): boolean;
}

/** What a text refers to, so a caller can load it in one go before rendering. */
export interface WikiRefs {
  users: string[];
  attachments: string[];
  issueKeys: string[];
}

/** Lists the users, attachments and issue keys a text mentions, deduplicated, in order. */
export function wikiReferences(src: string): WikiRefs {
  const users = new Set<string>();
  const attachments = new Set<string>();
  const issueKeys = new Set<string>();
  // Rendering with a recording resolver guarantees the references match
  // exactly what renderWiki will ask for (code blocks excluded, etc.).
  renderWiki(src, {
    user(username) {
      users.add(username);
      return null;
    },
    attachment(filename) {
      attachments.add(filename);
      return null;
    },
    issueExists(key) {
      issueKeys.add(key);
      return false;
    },
  });
  return { users: [...users], attachments: [...attachments], issueKeys: [...issueKeys] };
}

/** Renders wiki markup to safe HTML. */
export function renderWiki(src: string, resolver: WikiResolver): string {
  const ctx: Ctx = { resolver, blocks: [] };
  return renderDocument(normalize(src), ctx);
}

/** Plain-text excerpt: markup stripped, whitespace collapsed, cut with an ellipsis. */
export function wikiToText(src: string, maxLength?: number): string {
  const html = renderWiki(src, NO_RESOLVER);
  const text = decodeEntities(html.replace(BLOCK_TAG_RE, ' ').replace(/<[^>]*>/g, ''))
    .replace(/\s+/g, ' ')
    .trim();
  if (maxLength === undefined || text.length <= maxLength) return text;
  if (maxLength < 1) return '';
  let cut = text.slice(0, maxLength - 1);
  const lastSpace = cut.lastIndexOf(' ');
  if (lastSpace > maxLength * 0.6) cut = cut.slice(0, lastSpace);
  return cut.trimEnd() + '…';
}

/* ---------- shared ---------- */

interface Ctx {
  resolver: WikiResolver;
  /** Rendered multi-line macros, referenced by placeholders. */
  blocks: string[];
}

const NO_RESOLVER: WikiResolver = {
  user: () => null,
  attachment: () => null,
  issueExists: () => false,
};

// Private-use characters delimit placeholders; they are stripped from the
// input, so a user cannot forge one.
const PH_OPEN = '';
const PH_CLOSE = '';
const PLACEHOLDER_RE = new RegExp(`^${PH_OPEN}(\\d+)${PH_CLOSE}$`);

const ESCAPES: Record<string, string> = {
  '&': '&amp;',
  '<': '&lt;',
  '>': '&gt;',
  '"': '&quot;',
  "'": '&#39;',
};

function esc(s: string): string {
  return s.replace(/[&<>"']/g, (c) => ESCAPES[c] ?? c);
}

function decodeEntities(s: string): string {
  return s
    .replace(/&lt;/g, '<')
    .replace(/&gt;/g, '>')
    .replace(/&quot;/g, '"')
    .replace(/&#39;/g, "'")
    .replace(/&amp;/g, '&');
}

const BLOCK_TAG_RE =
  /<\/?(?:p|br|li|ul|ol|h[1-6]|table|tbody|tr|td|th|pre|blockquote|hr|div)\b[^>]*>/g;

export function normalize(src: string): string {
  return src.replace(/\r\n?/g, '\n').split(PH_OPEN).join('').split(PH_CLOSE).join('');
}

const WORD_RE = /[\p{L}\p{N}]/u;

export function isWord(c: string): boolean {
  return c !== '' && WORD_RE.test(c);
}

export function isSpace(c: string): boolean {
  return c === '' || /\s/.test(c);
}

const EXTERNAL_RE = /^(?:https?|ftp):\/\/[^\s]/i;
const MAILTO_RE = /^mailto:[^\s]/i;
// "/x" and "#x" stay on this site; "//x" and "/\x" are other hosts to a browser.
const LOCAL_RE = /^(?:\/(?![/\\])|#)/;

/** Whether a URL may appear in href/src. Control characters are never allowed. */
export function isSafeUrl(url: string): boolean {
  for (let k = 0; k < url.length; k++) {
    const code = url.charCodeAt(k);
    if (code < 0x20 || code === 0x7f) return false;
  }
  return EXTERNAL_RE.test(url) || MAILTO_RE.test(url) || LOCAL_RE.test(url);
}

function isExternal(url: string): boolean {
  return EXTERNAL_RE.test(url) || MAILTO_RE.test(url);
}

function anchor(url: string, inner: string, cls?: string): string {
  const classAttr = cls ? ` class="${cls}"` : '';
  const rel = isExternal(url) ? ' rel="nofollow noopener noreferrer"' : '';
  return `<a${classAttr} href="${esc(url.replace(/ /g, '%20'))}"${rel}>${inner}</a>`;
}

export const ISSUE_KEY_RE = /^[A-Z][A-Z0-9_]+-\d+$/;
export const IMAGE_EXT_RE = /\.(?:png|jpe?g|gif|webp|bmp|svg|avif)$/i;

/* ---------- pass 1: multi-line macros ---------- */

export const MACRO_OPEN_RE = /\{(code|noformat|quote|panel)(?::([^}\n]*))?\}/g;

function renderDocument(src: string, ctx: Ctx): string {
  return renderBlocks(extractMacros(src, ctx), ctx);
}

function extractMacros(src: string, ctx: Ctx): string {
  let out = '';
  let pos = 0;
  // Macros with no closing tag left in the text: no need to search again.
  const unclosed = new Set<string>();
  for (;;) {
    MACRO_OPEN_RE.lastIndex = pos;
    const m = MACRO_OPEN_RE.exec(src);
    if (!m) break;
    const name = m[1] ?? '';
    const contentStart = m.index + m[0].length;
    const closeTag = `{${name}}`;
    const closeIdx = unclosed.has(name) ? -1 : src.indexOf(closeTag, contentStart);
    if (closeIdx < 0) unclosed.add(name);
    // An escaped or unterminated opening stays as text.
    if (src.charAt(m.index - 1) === '\\' || closeIdx < 0) {
      out += src.slice(pos, contentStart);
      pos = contentStart;
      continue;
    }
    const body = src.slice(contentStart, closeIdx);
    const html = renderMacro(name, m[2] ?? '', body, ctx);
    ctx.blocks.push(html);
    out += `${src.slice(pos, m.index)}\n${PH_OPEN}${ctx.blocks.length - 1}${PH_CLOSE}\n`;
    pos = closeIdx + closeTag.length;
  }
  return out + src.slice(pos);
}

function parseParams(params: string): { bare: string[]; named: Map<string, string> } {
  const bare: string[] = [];
  const named = new Map<string, string>();
  for (const part of params.split('|')) {
    const p = part.trim();
    if (p === '') continue;
    const eq = p.indexOf('=');
    if (eq < 0) bare.push(p);
    else named.set(p.slice(0, eq).trim().toLowerCase(), p.slice(eq + 1).trim());
  }
  return { bare, named };
}

/** Strips the blank lines Jira users put around code. */
export function trimCode(body: string): string {
  return body.replace(/^[ \t]*\n/, '').replace(/\s+$/, '');
}

function renderMacro(name: string, params: string, body: string, ctx: Ctx): string {
  const { bare, named } = parseParams(params);
  switch (name) {
    case 'code': {
      const lang = named.get('language') ?? named.get('lang') ?? bare[0] ?? '';
      const langAttr = /^[A-Za-z0-9_+#.-]{1,30}$/.test(lang) ? ` data-lang="${esc(lang)}"` : '';
      const title = named.get('title');
      const titleHtml = title ? `<div class="code-title">${esc(title)}</div>` : '';
      return `${titleHtml}<pre class="code"${langAttr}><code>${esc(trimCode(body))}</code></pre>`;
    }
    case 'noformat':
      return `<pre class="noformat">${esc(trimCode(body))}</pre>`;
    case 'quote':
      return `<blockquote>${renderDocument(body, ctx)}</blockquote>`;
    default: {
      const title = named.get('title');
      const titleHtml = title ? `<div class="panel-title">${esc(title)}</div>` : '';
      return `<div class="panel">${titleHtml}${renderDocument(body, ctx)}</div>`;
    }
  }
}

/* ---------- pass 2: blocks ---------- */

export const HEADING_RE = /^\s*h([1-6])\.(?:\s+(.*))?$/;
export const BQ_RE = /^\s*bq\.\s+(.*)$/;
export const HR_RE = /^\s*-{4,}\s*$/;
export const LIST_RE = /^\s*([*#]+|-)\s+(.*)$/;

function renderBlocks(src: string, ctx: Ctx): string {
  const lines = src.split('\n');
  let out = '';
  let para: string[] = [];
  const flush = (): void => {
    if (para.length > 0) {
      const html = inline(para.join('\n'), ctx, true);
      // A line holding only {toc} or {anchor} leaves nothing to show.
      if (html !== '') out += `<p>${html}</p>`;
      para = [];
    }
  };

  let i = 0;
  while (i < lines.length) {
    const line = lines[i] ?? '';
    const trimmed = line.trim();
    if (trimmed === '') {
      flush();
      i++;
      continue;
    }

    const ph = PLACEHOLDER_RE.exec(trimmed);
    if (ph) {
      flush();
      out += ctx.blocks[Number(ph[1])] ?? '';
      i++;
      continue;
    }

    const heading = HEADING_RE.exec(line);
    if (heading) {
      flush();
      const level = heading[1] ?? '1';
      out += `<h${level}>${inline(heading[2] ?? '', ctx, true)}</h${level}>`;
      i++;
      continue;
    }

    const bq = BQ_RE.exec(line);
    if (bq) {
      flush();
      out += `<blockquote>${inline(bq[1] ?? '', ctx, true)}</blockquote>`;
      i++;
      continue;
    }

    if (HR_RE.test(line)) {
      flush();
      out += '<hr>';
      i++;
      continue;
    }

    if (LIST_RE.test(line)) {
      flush();
      const items: string[] = [];
      while (i < lines.length && LIST_RE.test(lines[i] ?? '')) items.push(lines[i++] ?? '');
      out += renderList(items, ctx);
      continue;
    }

    if (trimmed.startsWith('|')) {
      flush();
      const rows: string[] = [];
      while (i < lines.length && (lines[i] ?? '').trim().startsWith('|')) {
        rows.push((lines[i++] ?? '').trim());
      }
      out += renderTable(rows, ctx);
      continue;
    }

    para.push(line);
    i++;
  }
  flush();
  return out;
}

/** Nested lists: the marker string ("#*") gives the list type at each depth. */
function renderList(items: string[], ctx: Ctx): string {
  let out = '';
  const stack: string[] = [];
  for (const item of items) {
    const m = LIST_RE.exec(item);
    if (!m) continue;
    const types = [...(m[1] ?? '*')].map((c) => (c === '#' ? 'ol' : 'ul'));
    let common = 0;
    while (common < stack.length && common < types.length && stack[common] === types[common]) {
      common++;
    }
    // Close the levels that differ, then either continue at the same depth or
    // open the missing levels inside the current item.
    while (stack.length > common) out += `</li></${stack.pop()}>`;
    if (stack.length === types.length) {
      out += '</li><li>';
    } else {
      for (let d = stack.length; d < types.length; d++) {
        const tag = types[d] ?? 'ul';
        out += `<${tag}><li>`;
        stack.push(tag);
      }
    }
    out += inline(m[2] ?? '', ctx, true);
  }
  while (stack.length > 0) out += `</li></${stack.pop()}>`;
  return out;
}

export interface Cell {
  header: boolean;
  content: string;
}

/** Splits "||h1||h2||" / "|a|[x|http://y]|" into cells; "|" inside [...] does not split. */
export function splitRow(row: string): Cell[] {
  const cells: Cell[] = [];
  let i = 0;
  while (i < row.length && row.charAt(i) === '|') {
    const header = row.charAt(i + 1) === '|';
    i += header ? 2 : 1;
    const start = i;
    let depth = 0;
    while (i < row.length) {
      const c = row.charAt(i);
      if (c === '\\') {
        i += 2;
        continue;
      }
      if (c === '[') depth++;
      else if (c === ']') depth = Math.max(0, depth - 1);
      else if (c === '|' && depth === 0) break;
      i++;
    }
    const content = row.slice(start, Math.min(i, row.length));
    // The separator closing the row is not a cell.
    if (i >= row.length && content.trim() === '') break;
    cells.push({ header, content });
  }
  return cells;
}

function renderTable(rows: string[], ctx: Ctx): string {
  let out = '<table class="wiki-table"><tbody>';
  for (const row of rows) {
    out += '<tr>';
    for (const cell of splitRow(row)) {
      const tag = cell.header ? 'th' : 'td';
      out += `<${tag}>${inline(cell.content.trim(), ctx, true)}</${tag}>`;
    }
    out += '</tr>';
  }
  return out + '</tbody></table>';
}

/* ---------- inline ---------- */

const EFFECTS: Record<string, string> = {
  '*': 'strong',
  _: 'em',
  '-': 'del',
  '+': 'ins',
  '^': 'sup',
  '~': 'sub',
  '??': 'cite',
};

export const EMOTICONS: Record<string, [string, string]> = {
  '(/)': ['✓', 'ok'],
  '(x)': ['✗', 'no'],
  '(!)': ['!', 'warn'],
  '(?)': ['?', 'help'],
  '(i)': ['i', 'info'],
  '(y)': ['+', 'up'],
  '(n)': ['−', 'down'],
};

export const INLINE_MACRO_RE = /\{([a-zA-Z]+)(?::([^}\n]*))?\}/y;
export const URL_RE = /https?:\/\/[^\s<>"'[\]{}|\\^`]+/y;
export const KEY_AT_RE = /[A-Z][A-Z0-9_]+-\d+/y;

/**
 * Renders one block of text. `links` is false inside link labels, so a label
 * never contains a nested <a>.
 */
function inline(src: string, ctx: Ctx, links: boolean): string {
  let out = '';
  const scan: Scan = {
    noCloser: new Map(),
    monoMissingUntil: -1,
    colorMissing: false,
    lineFrom: -1,
    lineTo: -1,
  };
  let i = 0;

  while (i < src.length) {
    const c = src.charAt(i);
    const prev = src.charAt(i - 1);

    if (c === PH_OPEN || c === PH_CLOSE) {
      i++;
      continue;
    }

    if (c === '\\') {
      const next = src.charAt(i + 1);
      if (next === '\\') {
        out += '<br>';
        i += 2;
      } else if (next !== '' && !isWord(next) && !isSpace(next)) {
        out += esc(next);
        i += 2;
      } else {
        out += '\\';
        i++;
      }
      continue;
    }

    if (c === '\n') {
      out += '<br>';
      i++;
      continue;
    }

    if (c === '{') {
      const handled = inlineBrace(src, i, ctx, links, scan);
      out += handled.html;
      i = handled.next;
      continue;
    }

    if (c === '[' && links) {
      const close = findBracketClose(src, i);
      if (close > 0) {
        const html = renderBracket(src.slice(i + 1, close), ctx);
        if (html !== null) {
          out += html;
          i = close + 1;
          continue;
        }
      }
      out += '[';
      i++;
      continue;
    }

    if (c === '!') {
      const close = src.indexOf('!', i + 1);
      const lineEnd = lineEndOf(src, scan, i);
      if (close > i + 1 && close < lineEnd) {
        const content = src.slice(i + 1, close);
        if (!isSpace(content.charAt(0)) && !isSpace(content.charAt(content.length - 1))) {
          const html = renderImage(content, ctx, links);
          if (html !== null) {
            out += html;
            i = close + 1;
            continue;
          }
        }
      }
      out += '!';
      i++;
      continue;
    }

    if (c === '(') {
      const emo = EMOTICONS[src.slice(i, i + 3)];
      if (emo) {
        out += `<span class="emo emo-${emo[1]}">${emo[0]}</span>`;
        i += 3;
        continue;
      }
    }

    if (links && c === 'h' && !isWord(prev) && /^https?:\/\//.test(src.slice(i, i + 8))) {
      URL_RE.lastIndex = i;
      const m = URL_RE.exec(src);
      if (m) {
        const url = trimUrl(m[0]);
        out += anchor(url, esc(url));
        i += url.length;
        continue;
      }
    }

    if (links && c >= 'A' && c <= 'Z' && !isWord(prev) && prev !== '-' && prev !== '_') {
      KEY_AT_RE.lastIndex = i;
      const m = KEY_AT_RE.exec(src);
      if (m && !isWord(src.charAt(i + m[0].length))) {
        const key = m[0];
        out += ctx.resolver.issueExists(key)
          ? `<a class="issue-key" href="/browse/${key}">${key}</a>`
          : esc(key);
        i += key.length;
        continue;
      }
    }

    const marker = src.startsWith('??', i) ? '??' : c;
    const tag = EFFECTS[marker];
    if (tag && !isWord(prev) && !isSpace(src.charAt(i + marker.length))) {
      const lineEnd = lineEndOf(src, scan, i);
      const known = scan.noCloser.get(marker);
      const close = known === lineEnd ? -1 : findEffectClose(src, i, marker, lineEnd);
      if (close > 0) {
        const inner = inline(src.slice(i + marker.length, close), ctx, links);
        out += `<${tag}>${inner}</${tag}>`;
        i = close + marker.length;
        continue;
      }
      scan.noCloser.set(marker, lineEnd);
    }

    out += esc(c);
    i++;
  }
  return out;
}

/**
 * What one inline pass already knows is missing further on, so that a text
 * full of unmatched markers is still scanned in linear time.
 */
interface Scan {
  /** Effect marker → end of the line on which it has no closer. */
  noCloser: Map<string, number>;
  /** End of the line on which "{{" has no "}}". */
  monoMissingUntil: number;
  /** No "{color}" left in the text. */
  colorMissing: boolean;
  /** The current line, found once: a long line full of markers would otherwise be scanned again for each. */
  lineFrom: number;
  lineTo: number;
}

/** The end of the line `at` is on, remembered while the scan stays on it. */
function lineEndOf(src: string, scan: Scan, at: number): number {
  if (at < scan.lineFrom || at > scan.lineTo) {
    scan.lineFrom = at;
    scan.lineTo = endOfLine(src, at);
  }
  return scan.lineTo;
}

export function endOfLine(src: string, from: number): number {
  const nl = src.indexOf('\n', from);
  return nl < 0 ? src.length : nl;
}

/** First valid closing marker: after non-space, before a non-word character. */
export function findEffectClose(
  src: string,
  open: number,
  marker: string,
  lineEnd: number,
): number {
  let j = src.indexOf(marker, open + marker.length + 1);
  while (j >= 0 && j < lineEnd) {
    if (
      !isSpace(src.charAt(j - 1)) &&
      src.charAt(j - 1) !== '\\' &&
      !isWord(src.charAt(j + marker.length))
    ) {
      return j;
    }
    j = src.indexOf(marker, j + 1);
  }
  return -1;
}

/** Index of the "]" closing the "[" at `open`, on the same line, or -1. */
export function findBracketClose(src: string, open: number): number {
  for (let j = open + 1; j < src.length; j++) {
    const c = src.charAt(j);
    if (c === ']') return j;
    if (c === '[' || c === '\n') return -1;
  }
  return -1;
}

/** Drops trailing punctuation from an autolinked URL, keeping balanced parentheses. */
export function trimUrl(url: string): string {
  let u = url;
  for (;;) {
    const last = u.charAt(u.length - 1);
    if (/[.,;:!?]/.test(last)) {
      u = u.slice(0, -1);
    } else if (last === ')' && (u.match(/\(/g) ?? []).length < (u.match(/\)/g) ?? []).length) {
      u = u.slice(0, -1);
    } else {
      return u;
    }
  }
}

const NAMED_COLORS = new Set(
  (
    'aliceblue antiquewhite aqua aquamarine azure beige bisque black blanchedalmond blue ' +
    'blueviolet brown burlywood cadetblue chartreuse chocolate coral cornflowerblue cornsilk ' +
    'crimson cyan darkblue darkcyan darkgoldenrod darkgray darkgreen darkgrey darkkhaki ' +
    'darkmagenta darkolivegreen darkorange darkorchid darkred darksalmon darkseagreen ' +
    'darkslateblue darkslategray darkslategrey darkturquoise darkviolet deeppink deepskyblue ' +
    'dimgray dimgrey dodgerblue firebrick floralwhite forestgreen fuchsia gainsboro ghostwhite ' +
    'gold goldenrod gray green greenyellow grey honeydew hotpink indianred indigo ivory khaki ' +
    'lavender lavenderblush lawngreen lemonchiffon lightblue lightcoral lightcyan ' +
    'lightgoldenrodyellow lightgray lightgreen lightgrey lightpink lightsalmon lightseagreen ' +
    'lightskyblue lightslategray lightslategrey lightsteelblue lightyellow lime limegreen linen ' +
    'magenta maroon mediumaquamarine mediumblue mediumorchid mediumpurple mediumseagreen ' +
    'mediumslateblue mediumspringgreen mediumturquoise mediumvioletred midnightblue mintcream ' +
    'mistyrose moccasin navajowhite navy oldlace olive olivedrab orange orangered orchid ' +
    'palegoldenrod palegreen paleturquoise palevioletred papayawhip peachpuff peru pink plum ' +
    'powderblue purple rebeccapurple red rosybrown royalblue saddlebrown salmon sandybrown ' +
    'seagreen seashell sienna silver skyblue slateblue slategray slategrey snow springgreen ' +
    'steelblue tan teal thistle tomato turquoise violet wheat white whitesmoke yellow yellowgreen'
  ).split(' '),
);

/** A CSS color we can put in a style attribute verbatim, or null. */
function safeColor(value: string): string | null {
  const v = value.trim().toLowerCase();
  if (NAMED_COLORS.has(v)) return v;
  if (/^#(?:[0-9a-f]{3}|[0-9a-f]{6})$/.test(v)) return v;
  return null;
}

/** Handles "{…}" at `i`: monospace, {color}, dropped macros, or literal text. */
function inlineBrace(
  src: string,
  i: number,
  ctx: Ctx,
  links: boolean,
  scan: Scan,
): { html: string; next: number } {
  if (src.startsWith('{{', i)) {
    const lineEnd = lineEndOf(src, scan, i);
    if (scan.monoMissingUntil !== lineEnd) {
      const close = src.slice(i + 2, lineEnd).indexOf('}}');
      if (close > 0) {
        return {
          html: `<code>${esc(src.slice(i + 2, i + 2 + close))}</code>`,
          next: i + close + 4,
        };
      }
      if (close < 0) scan.monoMissingUntil = lineEnd;
    }
    return { html: '{', next: i + 1 };
  }

  INLINE_MACRO_RE.lastIndex = i;
  const m = INLINE_MACRO_RE.exec(src);
  if (!m) return { html: '{', next: i + 1 };
  const name = (m[1] ?? '').toLowerCase();
  const after = i + m[0].length;

  if (name === 'anchor' || name === 'toc') return { html: '', next: after };

  if (name === 'color' && m[2] !== undefined && !scan.colorMissing) {
    const close = src.indexOf('{color}', after);
    if (close < 0) scan.colorMissing = true;
    if (close >= 0) {
      const inner = inline(src.slice(after, close), ctx, links);
      const color = safeColor(m[2]);
      const html = color ? `<span style="color: ${color}">${inner}</span>` : inner;
      return { html, next: close + '{color}'.length };
    }
  }

  // Unknown macro: shown as written.
  return { html: esc(m[0]), next: after };
}

/** "[…]" content: a link, a mention, an attachment or an issue key; null when it is plain text. */
function renderBracket(content: string, ctx: Ctx): string | null {
  if (content.trim() === '') return null;
  if (content.startsWith('~')) return mention(content.slice(1).trim(), null, ctx);
  if (content.startsWith('^')) return attachmentLink(content.slice(1).trim(), null, ctx);

  const bar = content.indexOf('|');
  const label = bar >= 0 ? content.slice(0, bar) : null;
  // "[label|url|tooltip]": the tooltip is ignored.
  const target = (bar >= 0 ? (content.slice(bar + 1).split('|')[0] ?? '') : content).trim();

  if (target.startsWith('~')) return mention(target.slice(1).trim(), label, ctx);
  if (target.startsWith('^')) return attachmentLink(target.slice(1).trim(), label, ctx);

  if (ISSUE_KEY_RE.test(target)) {
    if (!ctx.resolver.issueExists(target)) return null;
    const text = label !== null ? inline(label, ctx, false) : target;
    return `<a class="issue-key" href="/browse/${target}">${text}</a>`;
  }

  if (!isSafeUrl(target)) return null;
  const text =
    label !== null && label.trim() !== ''
      ? inline(label, ctx, false)
      : esc(target.replace(/^mailto:/i, '').replace(/^#/, ''));
  return anchor(target, text);
}

function mention(username: string, label: string | null, ctx: Ctx): string | null {
  if (username === '') return null;
  const name = ctx.resolver.user(username);
  const text =
    label !== null && label.trim() !== '' ? inline(label, ctx, false) : esc(name ?? `@${username}`);
  return `<a class="mention" href="/people/${esc(encodeURIComponent(username))}">${text}</a>`;
}

function attachmentLink(filename: string, label: string | null, ctx: Ctx): string | null {
  if (filename === '') return null;
  const url = ctx.resolver.attachment(filename);
  if (url === null || !isSafeUrl(url)) return null;
  const text = label !== null && label.trim() !== '' ? inline(label, ctx, false) : esc(filename);
  return anchor(url, text, 'attachment');
}

/** "!…!" content: an attachment (image or file) or an external image; null when plain text. */
function renderImage(content: string, ctx: Ctx, links: boolean): string | null {
  const [rawName = '', ...params] = content.split('|');
  const name = rawName.trim();
  if (name === '' || name.includes('\n')) return null;

  // External images are not loaded (privacy, mixed content): they become links.
  if (/^https?:\/\//i.test(name)) return links && isSafeUrl(name) ? anchor(name, esc(name)) : null;
  if (/^[a-z][a-z0-9+.-]*:/i.test(name)) return null;

  const url = ctx.resolver.attachment(name);
  if (url === null || !isSafeUrl(url)) return null;
  if (!IMAGE_EXT_RE.test(name)) return links ? anchor(url, esc(name), 'attachment') : null;

  let attrs = '';
  let thumbnail = false;
  for (const p of params) {
    const param = p.trim().toLowerCase();
    if (param === 'thumbnail') thumbnail = true;
    const size = /^(width|height)=(\d{1,4})$/.exec(param);
    if (size) attrs += ` ${size[1]}="${size[2]}"`;
  }
  const cls = thumbnail ? ' class="thumbnail"' : '';
  return `<img${cls} src="${esc(url)}" alt="${esc(name)}"${attrs} loading="lazy">`;
}
