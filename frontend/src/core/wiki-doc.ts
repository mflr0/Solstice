/**
 * Wiki markup ⇄ rich document, for the WYSIWYG editor.
 *
 * Text stays stored as Jira wiki markup; the editor works on a ProseMirror
 * document. The parser reads markup with exactly the rules of the renderer
 * (wiki.ts), so what the editor shows is what will be saved. What it cannot
 * model faithfully ({panel}, {color}, emoticons, ragged tables, attachments
 * that may or may not exist…) becomes an opaque "raw" node written back
 * character for character.
 *
 * Saving is careful in two ways:
 * - top-level blocks left untouched are written back from their original
 *   source (ProseMirror keeps unchanged nodes as the very same objects);
 * - edited blocks get canonical markup, escaped as little as possible: each
 *   block's inline markup is parsed back, and escapes are added until it
 *   means exactly what the document holds.
 */

import {
  Fragment,
  Mark,
  Schema,
  type DOMOutputSpec,
  type Node as PMNode,
  type MarkSpec,
  type NodeSpec,
} from 'prosemirror-model';
import {
  BQ_RE,
  EMOTICONS,
  endOfLine,
  findBracketClose,
  findEffectClose,
  HEADING_RE,
  HR_RE,
  IMAGE_EXT_RE,
  INLINE_MACRO_RE,
  isSafeUrl,
  isSpace,
  isWord,
  ISSUE_KEY_RE,
  KEY_AT_RE,
  KEY_IN_TEXT,
  LIST_RE,
  MACRO_OPEN_RE,
  normalize,
  splitRow,
  trimCode,
  trimUrl,
  URL_RE,
} from './wiki.js';

/* ---------- schema ---------- */

const cellAttrs = {
  colspan: { default: 1 },
  rowspan: { default: 1 },
  colwidth: { default: null },
};

function cellFromDom(dom: HTMLElement): Record<string, unknown> {
  const span = (name: string): number => Math.max(1, Number(dom.getAttribute(name) ?? 1) || 1);
  return { colspan: span('colspan'), rowspan: span('rowspan'), colwidth: null };
}

function cellToDom(node: PMNode): Record<string, string> {
  const attrs: Record<string, string> = {};
  if (node.attrs.colspan !== 1) attrs.colspan = String(node.attrs.colspan);
  if (node.attrs.rowspan !== 1) attrs.rowspan = String(node.attrs.rowspan);
  return attrs;
}

/** The language of a {code} block: "java", "language=java" or "lang=java". */
export function codeLanguage(params: string | null): string {
  if (!params) return '';
  for (const part of params.split('|')) {
    const p = part.trim();
    const eq = p.indexOf('=');
    if (eq < 0) {
      if (p !== '') return p;
    } else if (/^(?:language|lang)$/i.test(p.slice(0, eq).trim())) {
      return p.slice(eq + 1).trim();
    }
  }
  return '';
}

/** {code} parameters with the language replaced (or removed when empty), others kept. */
export function withCodeLanguage(params: string | null, language: string): string | null {
  const lang = language.trim().replace(/[|}\n]/g, '');
  const parts = (params ?? '').split('|').filter((p) => p.trim() !== '');
  const at = parts.findIndex((p) => !p.includes('=') || /^\s*(?:language|lang)\s*=/i.test(p));
  if (at >= 0) {
    if (lang === '') parts.splice(at, 1);
    else parts[at] = parts[at]?.includes('=') ? `language=${lang}` : lang;
  } else if (lang !== '') {
    parts.unshift(lang);
  }
  return parts.length > 0 ? parts.join('|') : null;
}

const nodes = {
  doc: { content: 'block+' },
  paragraph: {
    content: 'inline*',
    group: 'block',
    parseDOM: [{ tag: 'p' }],
    toDOM: () => ['p', 0],
  },
  heading: {
    attrs: { level: { default: 1 } },
    content: 'inline*',
    group: 'block',
    defining: true,
    parseDOM: [1, 2, 3, 4, 5, 6].map((level) => ({ tag: `h${level}`, attrs: { level } })),
    toDOM: (node) => [`h${node.attrs.level}`, 0],
  },
  blockquote: {
    // form: "bq" for a one-line `bq.`, "macro" for {quote}…{quote}.
    attrs: { form: { default: 'macro' }, params: { default: null } },
    content: 'block+',
    group: 'block',
    defining: true,
    parseDOM: [{ tag: 'blockquote' }],
    toDOM: () => ['blockquote', 0],
  },
  code_block: {
    // macro: "code" or "noformat"; params: what follows the colon, verbatim.
    attrs: { macro: { default: 'code' }, params: { default: null } },
    content: 'text*',
    marks: '',
    group: 'block',
    code: true,
    defining: true,
    parseDOM: [
      {
        tag: 'pre',
        preserveWhitespace: 'full',
        getAttrs: (dom: HTMLElement) => {
          const lang = dom.getAttribute('data-lang');
          return {
            macro: dom.classList.contains('noformat') ? 'noformat' : 'code',
            params: lang && /^[A-Za-z0-9_+#.-]{1,30}$/.test(lang) ? lang : null,
          };
        },
      },
    ],
    toDOM: (node) => {
      const lang = codeLanguage(node.attrs.params as string | null);
      const cls = node.attrs.macro === 'noformat' ? 'noformat' : 'code';
      return ['pre', { class: cls, 'data-lang': lang || null }, ['code', 0]];
    },
  },
  horizontal_rule: {
    group: 'block',
    parseDOM: [{ tag: 'hr' }],
    toDOM: () => ['hr'],
  },
  bullet_list: {
    attrs: { marker: { default: '*' } },
    content: 'list_item+',
    group: 'block',
    parseDOM: [{ tag: 'ul' }],
    toDOM: () => ['ul', 0],
  },
  ordered_list: {
    content: 'list_item+',
    group: 'block',
    parseDOM: [{ tag: 'ol' }],
    toDOM: () => ['ol', 0],
  },
  // One line of wiki markup per item, so one paragraph, then nested lists.
  list_item: {
    content: 'paragraph (bullet_list | ordered_list)*',
    defining: true,
    parseDOM: [{ tag: 'li' }],
    toDOM: () => ['li', 0],
  },
  table: {
    content: 'table_row+',
    tableRole: 'table',
    isolating: true,
    group: 'block',
    parseDOM: [{ tag: 'table' }],
    toDOM: () => ['table', { class: 'wiki-table' }, ['tbody', 0]],
  },
  table_row: {
    content: '(table_cell | table_header)+',
    tableRole: 'row',
    parseDOM: [{ tag: 'tr' }],
    toDOM: () => ['tr', 0],
  },
  table_cell: {
    content: 'paragraph',
    attrs: cellAttrs,
    tableRole: 'cell',
    isolating: true,
    parseDOM: [{ tag: 'td', getAttrs: cellFromDom }],
    toDOM: (node) => ['td', cellToDom(node), 0],
  },
  table_header: {
    content: 'paragraph',
    attrs: cellAttrs,
    tableRole: 'header_cell',
    isolating: true,
    parseDOM: [{ tag: 'th', getAttrs: cellFromDom }],
    toDOM: (node) => ['th', cellToDom(node), 0],
  },
  // Markup the editor shows but does not edit: written back verbatim.
  raw_block: {
    attrs: { wiki: { default: '' }, kind: { default: 'raw' } },
    group: 'block',
    atom: true,
    selectable: true,
    parseDOM: [
      {
        tag: 'div[data-wiki]',
        getAttrs: (dom: HTMLElement) => ({
          wiki: dom.getAttribute('data-wiki') ?? '',
          kind: dom.getAttribute('data-kind') ?? 'raw',
        }),
      },
    ],
    toDOM: (node) => [
      'div',
      { class: 'editor-raw', 'data-wiki': node.attrs.wiki, 'data-kind': node.attrs.kind },
      node.attrs.wiki as string,
    ],
  },
  text: { group: 'inline' },
  hard_break: {
    // form: "newline" (a line break in the source) or "backslash" (\\).
    attrs: { form: { default: 'newline' } },
    inline: true,
    group: 'inline',
    selectable: false,
    parseDOM: [{ tag: 'br' }],
    toDOM: () => ['br'],
  },
  mention: {
    // wiki: the markup it was read from ("[~ bob]"), written back as is.
    attrs: { username: {}, wiki: { default: null } },
    inline: true,
    group: 'inline',
    atom: true,
    selectable: true,
    parseDOM: [
      {
        tag: 'a.mention',
        priority: 60,
        getAttrs: (dom: HTMLElement) => {
          const direct = dom.getAttribute('data-username');
          const href = /^\/people\/([^/?#]+)$/.exec(dom.getAttribute('href') ?? '');
          const username = direct ?? (href?.[1] ? decodeURIComponent(href[1]) : '');
          return username ? { username } : false;
        },
      },
    ],
    toDOM: (node) => [
      'a',
      { class: 'mention editor-mention', 'data-username': node.attrs.username },
      `@${node.attrs.username as string}`,
    ],
  },
  raw_inline: {
    attrs: { wiki: {}, kind: { default: 'raw' } },
    inline: true,
    group: 'inline',
    atom: true,
    selectable: true,
    parseDOM: [
      {
        tag: 'span[data-wiki]',
        getAttrs: (dom: HTMLElement) => ({
          wiki: dom.getAttribute('data-wiki') ?? '',
          kind: dom.getAttribute('data-kind') ?? 'raw',
        }),
      },
      {
        // An image copied from rendered wiki text: its alt is the file name.
        tag: 'img[alt]',
        getAttrs: (dom: HTMLElement) => {
          const alt = (dom.getAttribute('alt') ?? '').trim();
          return alt && !/[!|\n]/.test(alt) ? { wiki: `!${alt}!`, kind: 'image' } : false;
        },
      },
    ],
    toDOM: (node) => [
      'span',
      { class: 'editor-raw', 'data-wiki': node.attrs.wiki, 'data-kind': node.attrs.kind },
      node.attrs.wiki as string,
    ],
  },
} satisfies Record<string, NodeSpec>;

const tag = (name: string) => (): DOMOutputSpec => [name, 0];

/**
 * `part` tells apart two spans of the same kind written side by side in the
 * source ("[a|u][b|u]", "-x--y-"): merged, they would not read the same.
 */
const partAttrs = { part: { default: null } };

const marks = {
  // Outermost first: this order is also the nesting order when serializing.
  link: {
    // form: how the source wrote it, "bare" (http://x), "bracket" ([http://x])
    // or "label" ([text|http://x]); null for a link made in the editor.
    attrs: { href: {}, title: { default: null }, form: { default: null }, ...partAttrs },
    inclusive: false,
    parseDOM: [
      // Issue keys and mentions are links in rendered text, plain text here.
      { tag: 'a.issue-key', skip: true, priority: 60 },
      {
        tag: 'a[href]',
        getAttrs: (dom: HTMLElement) => ({ href: dom.getAttribute('href') ?? '', title: null }),
      },
    ],
    toDOM: (mark) => {
      const href = mark.attrs.href as string;
      return ['a', { href: isSafeUrl(href) ? href : null }, 0];
    },
  },
  strong: {
    attrs: partAttrs,
    parseDOM: [
      { tag: 'strong' },
      { tag: 'b', getAttrs: (dom: HTMLElement) => dom.style.fontWeight !== 'normal' && null },
      { style: 'font-weight=400', clearMark: (m) => m.type.name === 'strong' },
      {
        style: 'font-weight',
        getAttrs: (v: string) => /^(?:bold(?:er)?|[5-9]\d{2,})$/.test(v) && null,
      },
    ],
    toDOM: tag('strong'),
  },
  em: {
    attrs: partAttrs,
    parseDOM: [
      { tag: 'em' },
      { tag: 'i' },
      { style: 'font-style=italic' },
      { style: 'font-style=normal', clearMark: (m) => m.type.name === 'em' },
    ],
    toDOM: tag('em'),
  },
  underline: {
    attrs: partAttrs,
    parseDOM: [{ tag: 'ins' }, { tag: 'u' }, { style: 'text-decoration=underline' }],
    toDOM: tag('ins'),
  },
  strike: {
    attrs: partAttrs,
    parseDOM: [
      { tag: 'del' },
      { tag: 's' },
      { tag: 'strike' },
      { style: 'text-decoration=line-through' },
    ],
    toDOM: tag('del'),
  },
  sup: { attrs: partAttrs, parseDOM: [{ tag: 'sup' }], toDOM: tag('sup') },
  sub: { attrs: partAttrs, parseDOM: [{ tag: 'sub' }], toDOM: tag('sub') },
  cite: { attrs: partAttrs, parseDOM: [{ tag: 'cite' }], toDOM: tag('cite') },
  code: { parseDOM: [{ tag: 'code' }], toDOM: tag('code') },
  // A character the source wrote with a backslash (\\*): written so again.
  escape: {
    inclusive: false,
    parseDOM: [],
    toDOM: () => ['span', { class: 'wiki-escape' }, 0],
  },
} satisfies Record<string, MarkSpec>;

export const schema = new Schema({ nodes, marks });

const N = schema.nodes;
const M = schema.marks;

/** Effect marker → mark name, as the renderer reads them. */
const EFFECT_MARKS: Record<string, string> = {
  '*': 'strong',
  _: 'em',
  '-': 'strike',
  '+': 'underline',
  '^': 'sup',
  '~': 'sub',
  '??': 'cite',
};

const MARKERS: Record<string, string> = Object.fromEntries(
  Object.entries(EFFECT_MARKS).map(([marker, name]) => [name, marker]),
);

/** What `[http://x]` shows: the target, without mailto: or #. */
function linkDisplay(href: string): string {
  return href.replace(/^mailto:/i, '').replace(/^#/, '');
}

/* ---------- parsing: inline ---------- */

const BLOCK_MACRO_RE = /\{(?:code|noformat|quote|panel)(?::[^}\n]*)?\}/;
const BLOCK_MACRO_ONLY_RE = new RegExp(`^${BLOCK_MACRO_RE.source}$`);

type Marks = readonly Mark[];

/** Numbers the spans read, see `partAttrs`. */
let parts = 0;

function raw(wiki: string, kind: string, marks: Marks): PMNode {
  return N.raw_inline.create({ wiki, kind }, null, marks);
}

/**
 * Mirror of the renderer's inline pass: same scanning, same decisions, but
 * producing document nodes. Constructs whose output depends on server data
 * (attachments, issue existence) become raw nodes, so that they are written
 * back as they were whatever the data says.
 */
function parseInline(src: string, links: boolean, marks: Marks, out: PMNode[]): void {
  let text = '';
  const flush = (): void => {
    if (text !== '') {
      out.push(schema.text(text, marks));
      text = '';
    }
  };
  const push = (node: PMNode): void => {
    flush();
    out.push(node);
  };
  const noCloser = new Map<string, number>();
  let monoMissingUntil = -1;
  let colorMissing = false;
  // The end of the current line, found once per line: a long line full of
  // markers would otherwise be scanned again for each of them.
  let lineFrom = -1;
  let lineTo = -1;
  const eol = (at: number): number => {
    if (at < lineFrom || at > lineTo) {
      lineFrom = at;
      lineTo = endOfLine(src, at);
    }
    return lineTo;
  };
  let i = 0;

  while (i < src.length) {
    const c = src.charAt(i);
    const prev = src.charAt(i - 1);

    if (c === '\\') {
      const next = src.charAt(i + 1);
      if (next === '\\') {
        push(N.hard_break.create({ form: 'backslash' }, null, marks));
        i += 2;
      } else if (next !== '' && !isWord(next) && !isSpace(next)) {
        push(schema.text(next, M.escape.create().addToSet(marks)));
        i += 2;
      } else {
        text += '\\';
        i++;
      }
      continue;
    }

    if (c === '\n') {
      push(N.hard_break.create({ form: 'newline' }, null, marks));
      i++;
      continue;
    }

    if (c === '{') {
      if (src.startsWith('{{', i)) {
        const lineEnd = eol(i);
        if (monoMissingUntil !== lineEnd) {
          const close = src.slice(i + 2, lineEnd).indexOf('}}');
          if (close > 0) {
            const mono = src.slice(i + 2, i + 2 + close);
            // A {code} inside mono only stays text while nothing closes it.
            push(
              BLOCK_MACRO_RE.test(mono)
                ? raw(`{{${mono}}}`, 'mono', marks)
                : schema.text(mono, M.code.create().addToSet(marks)),
            );
            i += close + 4;
            continue;
          }
          if (close < 0) monoMissingUntil = lineEnd;
        }
        text += '{';
        i++;
        continue;
      }
      INLINE_MACRO_RE.lastIndex = i;
      const m = INLINE_MACRO_RE.exec(src);
      if (!m) {
        text += '{';
        i++;
        continue;
      }
      const name = (m[1] ?? '').toLowerCase();
      const after = i + m[0].length;
      if (name === 'anchor' || name === 'toc') {
        push(raw(m[0], 'macro', marks));
        i = after;
        continue;
      }
      if (name === 'color' && m[2] !== undefined && !colorMissing) {
        const close = src.indexOf('{color}', after);
        if (close < 0) colorMissing = true;
        if (close >= 0) {
          push(raw(src.slice(i, close + 7), 'color', marks));
          i = close + 7;
          continue;
        }
      }
      // Unknown macro: shown as written, and nothing inside it is read. A
      // {code} or {quote} left open stays text, to be escaped when saved.
      if (BLOCK_MACRO_ONLY_RE.test(m[0])) text += m[0];
      else push(raw(m[0], 'macro', marks));
      i = after;
      continue;
    }

    if (c === '[' && links) {
      const close = findBracketClose(src, i);
      if (close > 0) {
        flush();
        if (parseBracket(src.slice(i + 1, close), marks, out)) {
          i = close + 1;
          continue;
        }
      }
      text += '[';
      i++;
      continue;
    }

    if (c === '!') {
      const close = src.indexOf('!', i + 1);
      if (close > i + 1 && close < eol(i)) {
        const content = src.slice(i + 1, close);
        if (
          !isSpace(content.charAt(0)) &&
          !isSpace(content.charAt(content.length - 1)) &&
          isImage(content, links)
        ) {
          push(raw(src.slice(i, close + 1), 'image', marks));
          i = close + 1;
          continue;
        }
      }
      text += '!';
      i++;
      continue;
    }

    if (c === '(' && EMOTICONS[src.slice(i, i + 3)]) {
      push(raw(src.slice(i, i + 3), 'emoticon', marks));
      i += 3;
      continue;
    }

    if (links && c === 'h' && !isWord(prev) && /^https?:\/\//.test(src.slice(i, i + 8))) {
      URL_RE.lastIndex = i;
      const m = URL_RE.exec(src);
      if (m) {
        const url = trimUrl(m[0]);
        const link = M.link.create({ href: url, title: null, form: 'bare', part: ++parts });
        push(schema.text(url, link.addToSet(marks)));
        i += url.length;
        continue;
      }
    }

    if (links && c >= 'A' && c <= 'Z' && !isWord(prev) && prev !== '-' && prev !== '_') {
      KEY_AT_RE.lastIndex = i;
      const m = KEY_AT_RE.exec(src);
      if (m && !isWord(src.charAt(i + m[0].length))) {
        text += m[0];
        i += m[0].length;
        continue;
      }
    }

    const marker = src.startsWith('??', i) ? '??' : c;
    const effect = EFFECT_MARKS[marker];
    if (effect && !isWord(prev) && !isSpace(src.charAt(i + marker.length))) {
      const lineEnd = eol(i);
      const known = noCloser.get(marker);
      const close = known === lineEnd ? -1 : findEffectClose(src, i, marker, lineEnd);
      if (close > 0) {
        flush();
        const inner = M[effect]?.create({ part: ++parts }).addToSet(marks) ?? marks;
        parseInline(src.slice(i + marker.length, close), links, inner, out);
        i = close + marker.length;
        continue;
      }
      noCloser.set(marker, lineEnd);
    }

    text += c;
    i++;
  }
  flush();
}

/** Whether "!content!" is taken as an image or attachment by the renderer, for some data. */
function isImage(content: string, links: boolean): boolean {
  const name = (content.split('|')[0] ?? '').trim();
  if (name === '' || name.includes('\n')) return false;
  if (/^https?:\/\//i.test(name)) return links && isSafeUrl(name);
  if (/^[a-z][a-z0-9+.-]*:/i.test(name)) return false;
  return links || IMAGE_EXT_RE.test(name);
}

/** "[…]": pushes what it stands for and returns true, or false when it is plain text. */
function parseBracket(content: string, marks: Marks, out: PMNode[]): boolean {
  if (content.trim() === '') return false;
  const whole = `[${content}]`;
  if (content.startsWith('~')) {
    const username = content.slice(1).trim();
    if (username === '') return false;
    out.push(N.mention.create({ username, wiki: whole }, null, marks));
    return true;
  }
  if (content.startsWith('^')) {
    if (content.slice(1).trim() === '') return false;
    out.push(raw(whole, 'file', marks));
    return true;
  }

  const bar = content.indexOf('|');
  const label = bar >= 0 ? content.slice(0, bar) : null;
  const rest = bar >= 0 ? content.slice(bar + 1).split('|') : [content];
  const target = (rest[0] ?? '').trim();

  if (target.startsWith('~')) {
    const username = target.slice(1).trim();
    if (username === '') return false;
    if (label === null || label.trim() === '') {
      out.push(N.mention.create({ username, wiki: whole }, null, marks));
    } else {
      out.push(raw(whole, 'mention', marks));
    }
    return true;
  }
  if (target.startsWith('^')) {
    if (target.slice(1).trim() === '') return false;
    out.push(raw(whole, 'file', marks));
    return true;
  }
  // Linked only when the issue exists: kept as written.
  if (ISSUE_KEY_RE.test(target)) {
    out.push(raw(whole, 'issue', marks));
    return true;
  }
  if (!isSafeUrl(target)) return false;

  const title = rest.length > 1 ? rest.slice(1).join('|') : null;
  const shown = label !== null && label.trim() !== '' ? label : null;
  const form = shown === null ? 'bracket' : 'label';
  const inner = M.link.create({ href: target, title, form, part: ++parts }).addToSet(marks);
  const before = out.length;
  if (shown !== null) parseInline(shown, false, inner, out);
  else if (linkDisplay(target) !== '') out.push(schema.text(linkDisplay(target), inner));
  // A link with no text ([#]) cannot be held by a mark.
  if (out.length === before) out.push(raw(whole, 'link', marks));
  return true;
}

function inlineContent(src: string): Fragment {
  const out: PMNode[] = [];
  parseInline(src, true, [], out);
  return Fragment.fromArray(out);
}

/* ---------- parsing: blocks ---------- */

const PH_OPEN = '\uE000';
const PH_CLOSE = '\uE001';
const PLACEHOLDER_RE = /^\uE000(\d+)\uE001$/;

interface Macro {
  name: string;
  params: string | null;
  body: string;
  start: number;
  end: number;
}

/** A top-level block and the range of source it came from. */
interface Placed {
  node: PMNode;
  start: number;
  end: number;
}

/**
 * Mirror of the renderer's first pass: closed {code}, {noformat}, {quote} and
 * {panel} macros are replaced by placeholder lines. `origin` maps each kept
 * character back to its offset in `src`.
 */
function extractMacros(src: string): { text: string; origin: number[]; macros: Macro[] } {
  let text = '';
  const origin: number[] = [];
  const macros: Macro[] = [];
  const copy = (from: number, to: number): void => {
    text += src.slice(from, to);
    for (let k = from; k < to; k++) origin.push(k);
  };
  const insert = (s: string, at: number): void => {
    text += s;
    for (let k = 0; k < s.length; k++) origin.push(at);
  };
  const unclosed = new Set<string>();
  let pos = 0;
  for (;;) {
    MACRO_OPEN_RE.lastIndex = pos;
    const m = MACRO_OPEN_RE.exec(src);
    if (!m) break;
    const name = m[1] ?? '';
    const contentStart = m.index + m[0].length;
    const closeTag = `{${name}}`;
    const closeIdx = unclosed.has(name) ? -1 : src.indexOf(closeTag, contentStart);
    if (closeIdx < 0) unclosed.add(name);
    if (src.charAt(m.index - 1) === '\\' || closeIdx < 0) {
      copy(pos, contentStart);
      pos = contentStart;
      continue;
    }
    const end = closeIdx + closeTag.length;
    copy(pos, m.index);
    macros.push({
      name,
      params: m[2] ?? null,
      body: src.slice(contentStart, closeIdx),
      start: m.index,
      end,
    });
    insert(`\n${PH_OPEN}${macros.length - 1}${PH_CLOSE}\n`, m.index);
    pos = end;
  }
  copy(pos, src.length);
  return { text, origin, macros };
}

function paragraph(content?: Fragment): PMNode {
  return N.paragraph.create(null, content);
}

function macroNode(m: Macro, src: string): PMNode {
  switch (m.name) {
    case 'code':
    case 'noformat': {
      const body = trimCode(m.body);
      return N.code_block.create(
        { macro: m.name, params: m.params },
        body ? schema.text(body) : null,
      );
    }
    case 'quote': {
      const inner = parseBlocks(m.body).map((p) => p.node);
      return N.blockquote.create(
        { form: 'macro', params: m.params },
        inner.length > 0 ? inner : [paragraph()],
      );
    }
    default:
      return N.raw_block.create({ wiki: src.slice(m.start, m.end), kind: m.name });
  }
}

interface ListTree {
  ordered: boolean;
  marker: string;
  items: { text: string | null; children: ListTree[] }[];
}

function listNode(list: ListTree): PMNode {
  const items = list.items.map((item) =>
    N.list_item.create(null, [
      paragraph(item.text ? inlineContent(item.text) : undefined),
      ...item.children.map(listNode),
    ]),
  );
  return list.ordered
    ? N.ordered_list.create(null, items)
    : N.bullet_list.create({ marker: list.marker }, items);
}

/** Mirror of the renderer's lists: one tree per top-level list, with its first and last line. */
function listTrees(lines: string[]): { tree: ListTree; first: number; last: number }[] {
  const tops: { tree: ListTree; first: number; last: number }[] = [];
  const stack: ListTree[] = [];
  lines.forEach((line, k) => {
    const m = LIST_RE.exec(line);
    if (!m) return;
    const markers = m[1] ?? '*';
    const text = m[2] ?? '';
    const types = [...markers].map((c) => c === '#');
    let common = 0;
    while (
      common < stack.length &&
      common < types.length &&
      stack[common]?.ordered === types[common]
    ) {
      common++;
    }
    stack.length = common;
    if (stack.length === types.length) {
      stack[stack.length - 1]?.items.push({ text, children: [] });
    } else {
      for (let d = stack.length; d < types.length; d++) {
        const tree: ListTree = {
          ordered: types[d] === true,
          marker: markers === '-' ? '-' : '*',
          items: [{ text: d === types.length - 1 ? text : null, children: [] }],
        };
        if (d === 0) tops.push({ tree, first: k, last: k });
        else stack[d - 1]?.items[stack[d - 1]!.items.length - 1]?.children.push(tree);
        stack.push(tree);
      }
    }
    const top = tops[tops.length - 1];
    if (top) top.last = k;
  });
  return tops;
}

/** A table, or a raw block when its rows do not all have the same number of cells. */
function tableNode(rows: string[], wiki: string): PMNode {
  const cells = rows.map((row) => splitRow(row.trim()));
  const width = cells[0]?.length ?? 0;
  if (width === 0 || cells.some((r) => r.length !== width)) {
    return N.raw_block.create({ wiki, kind: 'table' });
  }
  return N.table.create(
    null,
    cells.map((row) =>
      N.table_row.create(
        null,
        row.map((cell) =>
          (cell.header ? N.table_header : N.table_cell).create(
            null,
            paragraph(inlineContent(cell.content.trim())),
          ),
        ),
      ),
    ),
  );
}

/** Mirror of the renderer's block pass, keeping each block's source range. */
function parseBlocks(src: string): Placed[] {
  const { text, origin, macros } = extractMacros(src);
  const lines = text.split('\n');
  const starts: number[] = [];
  let acc = 0;
  for (const l of lines) {
    starts.push(acc);
    acc += l.length + 1;
  }
  const at = (k: number): number => origin[k] ?? 0;
  const range = (a: number, b: number): { start: number; end: number } => ({
    start: at(starts[a] ?? 0),
    end: at((starts[b] ?? 0) + (lines[b]?.length ?? 1) - 1) + 1,
  });

  const out: Placed[] = [];
  let para: number[] = [];
  const flush = (): void => {
    const a = para[0];
    const b = para[para.length - 1];
    if (a === undefined || b === undefined) return;
    const content = para.map((k) => lines[k] ?? '').join('\n');
    out.push({ node: paragraph(inlineContent(content)), ...range(a, b) });
    para = [];
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
      const m = macros[Number(ph[1])];
      if (m) out.push({ node: macroNode(m, src), start: m.start, end: m.end });
      i++;
      continue;
    }

    const heading = HEADING_RE.exec(line);
    if (heading) {
      flush();
      const level = Number(heading[1] ?? '1');
      out.push({
        node: N.heading.create({ level }, inlineContent(heading[2] ?? '')),
        ...range(i, i),
      });
      i++;
      continue;
    }

    const bq = BQ_RE.exec(line);
    if (bq) {
      flush();
      const content = paragraph(inlineContent(bq[1] ?? ''));
      out.push({ node: N.blockquote.create({ form: 'bq' }, content), ...range(i, i) });
      i++;
      continue;
    }

    if (HR_RE.test(line)) {
      flush();
      out.push({ node: N.horizontal_rule.create(), ...range(i, i) });
      i++;
      continue;
    }

    if (LIST_RE.test(line)) {
      flush();
      const first = i;
      while (i < lines.length && LIST_RE.test(lines[i] ?? '')) i++;
      for (const t of listTrees(lines.slice(first, i))) {
        out.push({ node: listNode(t.tree), ...range(first + t.first, first + t.last) });
      }
      continue;
    }

    if (trimmed.startsWith('|')) {
      flush();
      const first = i;
      while (i < lines.length && (lines[i] ?? '').trim().startsWith('|')) i++;
      const r = range(first, i - 1);
      out.push({ node: tableNode(lines.slice(first, i), src.slice(r.start, r.end)), ...r });
      continue;
    }

    para.push(i);
    i++;
  }
  flush();
  return out;
}

/* ---------- public parsing API ---------- */

interface BlockSource {
  /** The block's exact source. */
  text: string;
  /** What separated it from the block before it in the source. */
  sep: string;
  /** That block. */
  prev: PMNode | null;
}

/** A parsed text: the document, and what is needed to write untouched parts back as they were. */
export interface ParsedWiki {
  doc: PMNode;
  source: string;
  blocks: Map<PMNode, BlockSource>;
}

/** Whether a macro opens without closing: such a block depends on what follows it. */
function hasDanglingMacro(text: string): boolean {
  let pos = 0;
  for (;;) {
    MACRO_OPEN_RE.lastIndex = pos;
    const m = MACRO_OPEN_RE.exec(text);
    if (!m) return false;
    const after = m.index + m[0].length;
    if (text.charAt(m.index - 1) === '\\') {
      pos = after;
      continue;
    }
    const close = text.indexOf(`{${m[1] ?? ''}}`, after);
    if (close < 0) return true;
    pos = close + (m[1] ?? '').length + 2;
  }
}

/** Reads wiki markup into an editor document. */
export function parseWiki(source: string): ParsedWiki {
  const src = normalize(source);
  const placed = parseBlocks(src);
  const blocks = new Map<PMNode, BlockSource>();
  let prevEnd = 0;
  let prev: PMNode | null = null;
  for (const p of placed) {
    const text = src.slice(p.start, p.end);
    if (!hasDanglingMacro(text))
      blocks.set(p.node, { text, sep: src.slice(prevEnd, p.start), prev });
    prevEnd = p.end;
    prev = p.node;
  }
  const content = placed.map((p) => p.node);
  const doc = N.doc.create(null, content.length > 0 ? content : [paragraph()]);
  return { doc, source, blocks };
}

/** Reads wiki markup as a fragment of blocks, e.g. for pasting. */
export function parseWikiBlocks(source: string): Fragment {
  return Fragment.fromArray(parseBlocks(normalize(source)).map((p) => p.node));
}

/* ---------- serializing: inline ---------- */

/** Where inline markup goes: a paragraph, a one-line block (heading, list item, bq.) or a table cell. */
type InlineContext = 'para' | 'line' | 'cell';

interface Unit {
  kind: 'c' | 'br' | 'atom';
  /** The character, for kind "c". */
  ch: string;
  /** The node, for "br" and "atom". */
  node: PMNode | null;
  marks: Marks;
  /** Written with a backslash in the source. */
  escaped?: boolean;
}

function units(nodes: Iterable<PMNode>): Unit[] {
  const out: Unit[] = [];
  for (const n of nodes) {
    if (n.isText) {
      const text = n.text ?? '';
      for (let k = 0; k < text.length; k++) {
        out.push({ kind: 'c', ch: text.charAt(k), node: null, marks: n.marks });
      }
    } else {
      out.push({ kind: n.type === N.hard_break ? 'br' : 'atom', ch: '', node: n, marks: n.marks });
    }
  }
  return out;
}

function children(fragment: Fragment): PMNode[] {
  const list: PMNode[] = [];
  fragment.forEach((n) => list.push(n));
  return list;
}

const WS_RE = /\s/;
const ESCAPABLE_RE = /^[!-/:-@[\]^_`{-~]$/;
const EFFECT_NAMES = Object.values(EFFECT_MARKS);

// Ties between marks spanning the same text: "[^x^|url]" would read as an
// attachment, so superscript and subscript go outside links.
const MARK_ORDER = ['sup', 'sub', ...Object.keys(marks).filter((n) => n !== 'sup' && n !== 'sub')];

/**
 * Makes the document's inline content expressible in markup: whitespace moves
 * out of effects, effects stuck to a word are dropped (the markup cannot
 * write them), mono text that cannot be delimited loses its mono, links lose
 * the characters a label cannot hold. The result is what the markup will mean.
 */
function prepare(input: Unit[], ctx: InlineContext, monoBars = false): Unit[] {
  let list = input
    .filter((u) => u.kind !== 'c' || (u.ch !== '\r' && u.ch !== PH_OPEN && u.ch !== PH_CLOSE))
    .map((u): Unit => {
      if (u.kind === 'c' && u.ch === '\n') {
        return { ...u, kind: 'br', ch: '', node: N.hard_break.create() };
      }
      // The escape is a flag on the character, not markup around it.
      return M.escape.isInSet(u.marks)
        ? { ...u, marks: M.escape.removeFromSet(u.marks), escaped: true }
        : u;
    });

  const setMarks = (k: number, marks: Marks): void => {
    const u = list[k];
    if (u) list[k] = { ...u, marks };
  };

  // Links: a label holds neither brackets nor bars, nor mentions.
  list.forEach((u, k) => {
    const link = M.link.isInSet(u.marks);
    if (!link) return;
    const href = link.attrs.href as string;
    if (!isSafeUrl(href)) {
      setMarks(k, link.removeFromSet(u.marks));
      return;
    }
    const encoded = href.replace(/\[/g, '%5B').replace(/\]/g, '%5D').replace(/\|/g, '%7C');
    let marks = u.marks;
    if (encoded !== href) {
      marks = M.link.create({ ...link.attrs, href: encoded }).addToSet(link.removeFromSet(marks));
    }
    if (u.kind === 'atom') {
      const wiki = u.node?.type === N.raw_inline ? (u.node.attrs.wiki as string) : '[';
      if (/[[\]|\n]/.test(wiki)) marks = M.link.removeFromSet(marks);
    }
    const ch = u.kind === 'c' ? ({ '[': '(', ']': ')', '|': '/' }[u.ch] ?? u.ch) : u.ch;
    list[k] = { ...u, ch, marks };
  });

  // Mono: {{…}} holds plain characters only, cannot contain "}}" nor end with "}".
  let k = 0;
  while (k < list.length) {
    const u = list[k]!;
    if (!M.code.isInSet(u.marks)) {
      k++;
      continue;
    }
    if (u.kind !== 'c') {
      setMarks(k, M.code.removeFromSet(u.marks));
      k++;
      continue;
    }
    let end = k;
    while (list[end + 1]?.kind === 'c' && M.code.isInSet(list[end + 1]!.marks)) end++;
    const text = list
      .slice(k, end + 1)
      .map((x) => x.ch)
      .join('');
    const ok =
      !text.includes('}}') &&
      !text.endsWith('}') &&
      !BLOCK_MACRO_RE.test(text) &&
      (ctx !== 'cell' || monoBars || !text.includes('|'));
    if (!ok) for (let j = k; j <= end; j++) setMarks(j, M.code.removeFromSet(list[j]!.marks));
    k = end + 1;
  }

  // The renderer drops the spaces after "h1." or "*", and around cells.
  const loose = (u: Unit | undefined): boolean =>
    u?.kind === 'c' && WS_RE.test(u.ch) && u.marks.length === 0;
  if (ctx !== 'para') {
    while (loose(list[0])) list = list.slice(1);
  }
  if (ctx === 'cell') {
    while (loose(list[list.length - 1])) list = list.slice(0, -1);
  }

  // Effects: *x* needs a non-word character outside and no space inside.
  for (const name of EFFECT_NAMES) {
    const type = M[name];
    if (!type) continue;
    let a = 0;
    while (a < list.length) {
      if (!type.isInSet(list[a]!.marks)) {
        a++;
        continue;
      }
      // One span at a time: two spans side by side stay two.
      const span = type.isInSet(list[a]!.marks)!;
      let b = a;
      while (b + 1 < list.length && span.isInSet(list[b + 1]!.marks)) b++;
      const next = b + 1;
      // A space against a marker breaks it; so does "\\" before a closing one.
      // Spaces and breaks inside mono or a link label sit against "{{" or "["
      // instead.
      const space = (u: Unit): boolean =>
        u.kind === 'c' && WS_RE.test(u.ch) && !M.code.isInSet(u.marks) && !M.link.isInSet(u.marks);
      const isBlank = (u: Unit): boolean =>
        (u.kind === 'br' && !M.link.isInSet(u.marks)) || space(u);
      while (a <= b && space(list[a]!)) {
        setMarks(a, type.removeFromSet(list[a]!.marks));
        a++;
      }
      while (b >= a && isBlank(list[b]!)) {
        setMarks(b, type.removeFromSet(list[b]!.marks));
        b--;
      }
      a = next;
    }
  }
  return unstick(list, ctx);
}

/**
 * Drops the effects whose markers, where they get written, would not be read
 * as markers: stuck to a word ("a*b*"), against a space or after a backslash.
 */
function unstick(input: Unit[], ctx: InlineContext): Unit[] {
  let list = input;
  for (let round = 0; round < 20; round++) {
    const { text, owner, markers } = emit(list, ctx, noFlags());
    const backslash = (p: number): boolean => {
      const u = list[owner[p] ?? -1];
      return u?.kind === 'c' && u.ch === '\\';
    };
    const stuck = markers.filter((m) => {
      const before = text.charAt(m.at - 1);
      const after = text.charAt(m.at + m.length);
      return m.open
        ? isWord(before) || isSpace(after) || backslash(m.at - 1)
        : isWord(after) || isSpace(before) || before === '\\';
    });
    if (stuck.length === 0) break;
    list = [...list];
    for (const { mark, unit } of stuck) {
      let a = unit;
      while (a > 0 && mark.isInSet(list[a - 1]!.marks)) a--;
      for (let j = a; j < list.length && mark.isInSet(list[j]!.marks); j++) {
        list[j] = { ...list[j]!, marks: mark.removeFromSet(list[j]!.marks) };
      }
    }
  }
  return list;
}

interface Flags {
  /** Units written with a backslash before them. */
  esc: Set<number>;
  /** Line breaks written as \\ rather than a new line. */
  bs: Set<number>;
  /** Links (by first unit) written in brackets even though a bare address would do. */
  bracket: Set<number>;
}

const noFlags = (): Flags => ({ esc: new Set(), bs: new Set(), bracket: new Set() });

/** An effect marker as written: where, and for which span (one of its units). */
interface Marker {
  at: number;
  length: number;
  open: boolean;
  mark: Mark;
  unit: number;
}

interface Emitted {
  text: string;
  /** For each character of the text, the unit it writes, or -1 for syntax. */
  owner: number[];
  /** Links written as bare addresses, as [first, last] unit ranges. */
  bare: [number, number][];
  /** Effect markers (*, _, +…) written. */
  markers: Marker[];
}

/**
 * Marks the markup nests as syntax; code is always innermost. Spans of the
 * same length keep the nesting they had in the source (read first, outer).
 */
function openOrder(list: Unit[], k: number, toOpen: Mark[]): Mark[] {
  const extent = (m: Mark): number => {
    let e = k;
    while (e < list.length && m.isInSet(list[e]!.marks)) e++;
    return e - k;
  };
  const rank = (m: Mark): number => (m.type === M.code ? 1 : 0);
  const read = (x: Mark, y: Mark): number => {
    const a: unknown = x.attrs.part;
    const b: unknown = y.attrs.part;
    return typeof a === 'number' && typeof b === 'number' ? a - b : 0;
  };
  return [...toOpen].sort(
    (x, y) =>
      rank(x) - rank(y) ||
      extent(y) - extent(x) ||
      read(x, y) ||
      MARK_ORDER.indexOf(x.type.name) - MARK_ORDER.indexOf(y.type.name),
  );
}

function emit(list: Unit[], ctx: InlineContext, flags: Flags): Emitted {
  let text = '';
  const owner: number[] = [];
  const write = (s: string, who: number): void => {
    text += s;
    for (let j = 0; j < s.length; j++) owner.push(who);
  };
  const active: Mark[] = [];
  const closers: string[] = [];
  /** For each active mark, the unit it opened at. */
  const opened: number[] = [];
  let skipUntil = -1;
  const bare: [number, number][] = [];
  const markers: Marker[] = [];

  const closeTo = (depth: number): void => {
    while (active.length > depth) {
      const mark = active.pop()!;
      const closer = closers.pop() ?? '';
      const unit = opened.pop() ?? 0;
      if (MARKERS[mark.type.name]) {
        markers.push({ at: text.length, length: closer.length, open: false, mark, unit });
      }
      write(closer, -1);
    }
  };

  for (let k = 0; k < list.length; k++) {
    if (k <= skipUntil) continue;
    const u = list[k]!;
    const marks = u.marks;
    let keep = 0;
    while (keep < active.length && active[keep]!.isInSet(marks)) keep++;
    let toOpen = marks.filter((m) => !m.isInSet(active.slice(0, keep)));
    // Nothing opens inside mono.
    if (toOpen.length > 0 && keep > 0 && active[keep - 1]!.type === M.code) {
      keep--;
      toOpen = marks.filter((m) => !m.isInSet(active.slice(0, keep)));
    }
    closeTo(keep);

    const order = openOrder(list, k, toOpen);
    // A link whose text is its address is written as the bare address (the
    // renderer links it), or [address]; either way outside other marks. A
    // link read from markup keeps the form it was written in.
    const link = order.find((m) => m.type === M.link);
    const written = link?.attrs.form as string | null | undefined;
    // "[|address|title]" shows the address too, whatever it holds.
    const linkTitle = link?.attrs.title as string | null | undefined;
    let plainEnd = -1;
    let form: 'auto' | 'bracket' | 'label' = 'label';
    const plain = linkTitle === null || written === 'bracket';
    if (link && plain && written !== 'label' && !M.code.isInSet(marks)) {
      const href = link.attrs.href as string;
      let end = k;
      while (end + 1 < list.length && link.isInSet(list[end + 1]!.marks)) end++;
      const run = list.slice(k, end + 1);
      if (run.every((x) => x.kind === 'c' && Mark.sameSet(x.marks, marks))) {
        const label = run.map((x) => x.ch).join('');
        const auto = linkTitle === null && written !== 'bracket' && !flags.bracket.has(k);
        if (label === href && /^https?:\/\//i.test(href) && auto) form = 'auto';
        else if (label === linkDisplay(href)) form = 'bracket';
        if (form !== 'label') plainEnd = end;
      }
    }
    for (const m of order) {
      if (m.type === M.link) {
        if (plainEnd >= 0) continue;
        const title = m.attrs.title as string | null;
        write('[', -1);
        active.push(m);
        closers.push(`|${m.attrs.href as string}${title === null ? '' : `|${title}`}]`);
      } else if (m.type === M.code) {
        write('{{', -1);
        active.push(m);
        closers.push('}}');
      } else {
        const marker = MARKERS[m.type.name] ?? '';
        markers.push({ at: text.length, length: marker.length, open: true, mark: m, unit: k });
        write(marker, -1);
        active.push(m);
        closers.push(marker);
      }
      opened.push(k);
    }
    if (link && form === 'bracket') {
      const href = link.attrs.href as string;
      write(linkTitle == null ? `[${href}]` : `[|${href}|${linkTitle}]`, -1);
      skipUntil = plainEnd;
      continue;
    }
    if (form === 'auto') {
      for (let j = k; j <= plainEnd; j++) write(list[j]!.ch, j);
      bare.push([k, plainEnd]);
      skipUntil = plainEnd;
      continue;
    }

    if (u.kind === 'c') {
      if (flags.esc.has(k)) write('\\', -1);
      write(u.ch, k);
    } else if (u.kind === 'br') {
      const newline =
        ctx === 'para' &&
        u.marks.length === 0 &&
        u.node?.attrs.form !== 'backslash' &&
        !flags.bs.has(k);
      write(newline ? '\n' : '\\\\', k);
    } else if (u.node?.type === N.mention) {
      const wiki = u.node.attrs.wiki as string | null;
      write(wiki ?? `[~${u.node.attrs.username as string}]`, k);
    } else {
      write((u.node?.attrs.wiki as string | undefined) ?? '', k);
    }
  }
  closeTo(0);
  return { text, owner, bare, markers };
}

/**
 * The units as the renderer will see them: addresses in plain text become
 * links, as they do when the markup is displayed. An escaped character ends
 * an address.
 */
function withAutolinks(list: Unit[], esc: Set<number>): Unit[] {
  const plain = list
    .map((u, k) =>
      u.kind === 'c' && !M.link.isInSet(u.marks) && !M.code.isInSet(u.marks) && !esc.has(k)
        ? u.ch
        : '\uFFFF',
    )
    .join('');
  let out = list;
  for (const m of plain.matchAll(/https?:\/\/[^\s<>"'[\]{}|\\^`\uFFFF]+/g)) {
    if (isWord(plain.charAt(m.index - 1)) || !/^https?:\/\//.test(m[0])) continue;
    const url = trimUrl(m[0]);
    const link = M.link.create({ href: url, title: null });
    if (out === list) out = [...list];
    for (let j = m.index; j < m.index + url.length; j++) {
      const u = out[j]!;
      out[j] = { ...u, marks: link.addToSet(u.marks) };
    }
  }
  return out;
}

/** Same marks, whatever their `part` and however a link is written. */
function sameMarks(a: Marks, b: Marks): boolean {
  if (a.length !== b.length) return false;
  return a.every((m, i) => {
    const o = b[i];
    if (!o || o.type !== m.type) return false;
    return Object.keys(m.attrs).every(
      (k) => k === 'part' || k === 'form' || m.attrs[k] === o.attrs[k],
    );
  });
}

function sameUnit(a: Unit, b: Unit): boolean {
  if (a.kind !== b.kind || !sameMarks(a.marks, b.marks)) return false;
  if (a.kind === 'c') return a.ch === b.ch;
  if (a.kind === 'br') return true;
  const x = a.node;
  const y = b.node;
  if (!x || !y || x.type !== y.type) return false;
  return x.type === N.mention
    ? x.attrs.username === y.attrs.username
    : x.attrs.wiki === y.attrs.wiki;
}

/** Offset of the character that makes a paragraph line a block (heading, list…), or -1. */
function blockTrigger(line: string): number {
  const heading = /^(\s*h[1-6])\.(?:\s|$)/.exec(line);
  if (heading && HEADING_RE.test(line)) return heading[1]?.length ?? -1;
  const bq = /^(\s*bq)\.\s/.exec(line);
  if (bq && BQ_RE.test(line)) return bq[1]?.length ?? -1;
  if (HR_RE.test(line)) return line.search(/-/);
  const list = /^(\s*)(?:[*#]+|-)\s/.exec(line);
  if (list && LIST_RE.test(line)) return list[1]?.length ?? -1;
  if (line.trim().startsWith('|')) return line.search(/\|/);
  return -1;
}

function escapable(u: Unit | undefined): boolean {
  return u?.kind === 'c' && ESCAPABLE_RE.test(u.ch) && u.ch !== '\\' && !M.code.isInSet(u.marks);
}

/**
 * Line problems of a paragraph: a line must not be blank nor read as a block.
 * Fixes them all; returns how many, or -1 when one cannot be fixed.
 */
function lineFixes(list: Unit[], out: Emitted, ctx: InlineContext, flags: Flags): number {
  if (ctx !== 'para') return 0;
  const { text, owner } = out;
  let fixes = 0;
  let lineStart = 0;
  for (let p = 0; p <= text.length; p++) {
    if (p < text.length && text.charAt(p) !== '\n') continue;
    const line = text.slice(lineStart, p);
    if (lineStart === 0) {
      const t = blockTrigger(line);
      if (t >= 0) {
        const who = owner[t] ?? -1;
        if (!escapable(list[who]) || flags.esc.has(who)) return -1;
        flags.esc.add(who);
        fixes++;
      }
    }
    if (p < text.length && list[owner[p] ?? -1]?.kind === 'br') {
      const br = owner[p] ?? -1;
      const nextEnd = text.indexOf('\n', p + 1);
      const nextLine = text.slice(p + 1, nextEnd < 0 ? text.length : nextEnd);
      const t = blockTrigger(nextLine);
      if (t >= 0) {
        const who = owner[p + 1 + t] ?? -1;
        if (escapable(list[who]) && !flags.esc.has(who)) flags.esc.add(who);
        else flags.bs.add(br);
        fixes++;
      } else if ((line.trim() === '' || nextLine.trim() === '') && !flags.bs.has(br)) {
        // One at a time: the next break may be fine once this one is "\\".
        flags.bs.add(br);
        return fixes + 1;
      }
    }
    lineStart = p + 1;
  }
  return fixes;
}

/** Block macros are found before anything else: escapes those written as text. */
function macroFixes(list: Unit[], out: Emitted, flags: Flags): number {
  let fixes = 0;
  for (const m of out.text.matchAll(new RegExp(MACRO_OPEN_RE.source, 'g'))) {
    if (out.text.charAt(m.index - 1) === '\\') continue;
    const who = out.owner[m.index] ?? -1;
    if (escapable(list[who]) && !flags.esc.has(who)) {
      flags.esc.add(who);
      fixes++;
    }
  }
  return fixes;
}

/** Splits units into lines: the ranges between line breaks. */
function lines(list: Unit[]): [number, number][] {
  const out: [number, number][] = [];
  let start = 0;
  list.forEach((u, k) => {
    if (u.kind === 'br') {
      out.push([start, k]);
      start = k + 1;
    }
  });
  out.push([start, list.length]);
  return out;
}

/**
 * What is wrong with one rendering of the units: fixes are recorded in
 * `flags` and true returned, or null when the markup means exactly the units,
 * or false when no escape can help.
 */
function nextFix(
  list: Unit[],
  out: Emitted,
  ctx: InlineContext,
  flags: Flags,
  last: boolean,
): boolean | null {
  const { text } = out;
  const lf = lineFixes(list, out, ctx, flags);
  if (lf < 0) return false;
  if (lf > 0 || macroFixes(list, out, flags) > 0) return true;

  if (ctx === 'cell') {
    // A "[" left open takes the rest of the row: only the last cell may.
    const open = openBracket(text);
    if (open && !last) return false;
    const row = `|${text}${text.endsWith('\\') ? ' ' : ''}${open ? '' : '|'}`;
    const cells = splitRow(row);
    if (cells.length !== 1 || cells[0]?.header || cells[0]?.content.trim() !== text.trim()) {
      return false;
    }
  }

  const back: PMNode[] = [];
  parseInline(text, true, [], back);
  const got = units(Fragment.fromArray(back).content).map((u) =>
    M.escape.isInSet(u.marks) ? { ...u, marks: M.escape.removeFromSet(u.marks) } : u,
  );
  const want = withAutolinks(list, flags.esc);

  /** The fix for a divergence at unit d (got aligned with want at offset `shift`). */
  const fixAt = (d: number, shift: number): boolean => {
    // An address that does not come out as written goes in brackets; one that
    // swallows what follows is stopped by escaping the next character.
    const inside = out.bare.find(([a, b]) => a <= d && d <= b);
    if (inside) {
      const next = inside[1] + 1;
      const after = got[next + shift];
      const swallowed = after !== undefined && M.link.isInSet(after.marks);
      if (swallowed && escapable(list[next]) && !flags.esc.has(next)) flags.esc.add(next);
      else flags.bracket.add(inside[0]);
      return true;
    }
    const before = out.bare.find(([, b]) => b === d - 1);
    if (before && !(escapable(list[d]) && !flags.esc.has(d))) {
      flags.bracket.add(before[0]);
      return true;
    }
    for (let j = d; j >= Math.max(0, d - 3); j--) {
      if (list[j]?.kind === 'br') break;
      if (escapable(list[j]) && !flags.esc.has(j)) {
        flags.esc.add(j);
        return true;
      }
    }
    return false;
  };

  const wantLines = lines(want);
  const gotLines = lines(got);
  if (wantLines.length !== gotLines.length) {
    // Something spans lines: fix the first difference only.
    let d = 0;
    while (d < want.length && d < got.length && sameUnit(want[d]!, got[d]!)) d++;
    if (d === want.length && d === got.length) return null;
    return fixAt(d, 0);
  }
  // Lines are read independently: one fix per line and per round.
  let differs = false;
  let fixed = false;
  wantLines.forEach(([a, b], i) => {
    const [ga, gb] = gotLines[i] ?? [0, 0];
    let d = 0;
    while (a + d < b && ga + d < gb && sameUnit(want[a + d]!, got[ga + d]!)) d++;
    if (a + d === b && ga + d === gb) return;
    differs = true;
    if (fixAt(a + d, ga - a)) fixed = true;
  });
  if (!differs) {
    // Lines agree; so must the breaks between them.
    for (let k = 0; k < want.length; k++) if (!sameUnit(want[k]!, got[k]!)) return fixAt(k, 0);
    return null;
  }
  return fixed;
}

/** Last resort: every punctuation character escaped, links in brackets. */
function escapeEverything(list: Unit[]): Flags {
  const flags = noFlags();
  const plain = list
    .map((u) => (u.kind === 'c' && !M.link.isInSet(u.marks) ? u.ch : '\uFFFF'))
    .join('');
  const keepAsIs = new Set<number>();
  for (const m of plain.matchAll(new RegExp(KEY_IN_TEXT, 'g'))) {
    const prev = plain.charAt(m.index - 1);
    if (
      !isWord(prev) &&
      prev !== '-' &&
      prev !== '_' &&
      !isWord(plain.charAt(m.index + m[0].length))
    ) {
      for (let j = m.index; j < m.index + m[0].length; j++) keepAsIs.add(j);
    }
  }
  list.forEach((u, k) => {
    if (escapable(u) && !keepAsIs.has(k)) flags.esc.add(k);
    if (M.link.isInSet(u.marks)) flags.bracket.add(k);
  });
  return flags;
}

/** Inline markup for a list of units, escaped as little as possible; null if it cannot be verified. */
function writeUnits(list: Unit[], ctx: InlineContext, bars = false, last = true): string | null {
  const flags = noFlags();
  // In a cell, "[" and "|" would change where the cell ends; with `bars`, a
  // "[" is left open on purpose, for bars that cannot be escaped.
  const risky = bars ? '|' : '[|';
  list.forEach((u, k) => {
    if (!escapable(u)) return;
    if (u.escaped || (ctx === 'cell' && risky.includes(u.ch))) flags.esc.add(k);
  });
  for (let round = 0; round < 60; round++) {
    const out = emit(list, ctx, flags);
    const fix = nextFix(list, out, ctx, flags, last);
    if (fix === null) return out.text;
    if (fix === false) break;
  }
  return null;
}

/** `last`: the inline content is the last cell of its row. */
function serializeInline(content: Fragment, ctx: InlineContext, last = true): string {
  const input = units(children(content));
  // In a cell, a bar in mono or in raw markup reads right only after an open
  // "[": try that first.
  const bar = (u: Unit): boolean =>
    (u.ch === '|' && M.code.isInSet(u.marks) !== undefined) ||
    (u.node?.type === N.raw_inline && (u.node.attrs.wiki as string).includes('|'));
  if (ctx === 'cell' && input.some(bar)) {
    const kept = writeUnits(prepare(input, ctx, true), ctx, true, last);
    if (kept !== null) return kept;
  }
  const list = prepare(input, ctx);
  if (list.length === 0) return '';
  const text = writeUnits(list, ctx, false, last);
  if (text !== null) return text;
  // Last resort: everything escaped; lines still checked.
  const flags = escapeEverything(list);
  for (let round = 0; round < 60; round++) {
    const out = emit(list, ctx, flags);
    if (lineFixes(list, out, ctx, flags) <= 0) return out.text;
  }
  return emit(list, ctx, flags).text;
}

/* ---------- serializing: blocks ---------- */

function codeText(node: PMNode, inQuote: boolean): string {
  const macro = node.attrs.macro === 'noformat' ? 'noformat' : 'code';
  const params = node.attrs.params as string | null;
  // The closing tag cannot be escaped inside code: a zero-width space breaks it.
  const guard = (s: string, closeTag: string): string =>
    s.split(closeTag).join(`${closeTag.slice(0, -1)}\u200b}`);
  let text = node.textContent.replace(/\s+$/, '');
  text = guard(text, `{${macro}}`);
  if (inQuote) text = guard(text, '{quote}');
  return `{${macro}${params === null ? '' : `:${params}`}}\n${text}\n{${macro}}`;
}

function listLines(list: PMNode, prefix: string, out: string[]): void {
  const ordered = list.type === N.ordered_list;
  const ch = ordered ? '#' : prefix === '' && list.attrs.marker === '-' ? '-' : '*';
  const marker = prefix + ch;
  const inner = ch === '-' ? '*' : marker;
  list.forEach((item, _offset, index) => {
    const para = item.firstChild;
    const text = para ? serializeInline(para.content, 'line') : '';
    // "** x" opens the empty first item itself, but cannot say it is a "-" list.
    const implied = text === '' && item.childCount > 1 && index === 0 && ch !== '-';
    if (!implied) out.push(`${marker} ${text}`);
    item.forEach((child, _o, ci) => {
      if (ci > 0) listLines(child, inner, out);
    });
  });
}

/** Whether a cell leaves a "[" open: the renderer then reads the rest of the row as that cell. */
function openBracket(cell: string): boolean {
  let depth = 0;
  for (let i = 0; i < cell.length; i++) {
    const c = cell.charAt(i);
    if (c === '\\') i++;
    else if (c === '[') depth++;
    else if (c === ']') depth = Math.max(0, depth - 1);
  }
  return depth > 0;
}

function tableText(table: PMNode): string {
  const rows: string[] = [];
  table.forEach((row) => {
    let line = '';
    let lastHeader = false;
    let lastOpen = false;
    row.forEach((cell, _offset, index) => {
      const header = cell.type === N.table_header;
      const bar = header ? '||' : '|';
      const last = index === row.childCount - 1 && Number(cell.attrs.colspan) === 1;
      const para = cell.firstChild;
      const text = para ? serializeInline(para.content, 'cell', last) : '';
      // "x\\|" would escape the bar: "x\\ |" reads the same once trimmed.
      line += bar + (text === '' ? ' ' : text.endsWith('\\') ? `${text} ` : text);
      for (let s = 1; s < Number(cell.attrs.colspan); s++) line += `${bar} `;
      lastHeader = header;
      lastOpen = openBracket(text);
    });
    if (line !== '') rows.push(line + (lastOpen ? '' : lastHeader ? '||' : '|'));
  });
  return rows.join('\n');
}

function quoteText(node: PMNode, inQuote: boolean): string {
  const only = node.childCount === 1 ? node.firstChild : null;
  if ((node.attrs.form === 'bq' || inQuote) && only?.type === N.paragraph) {
    const text = serializeInline(only.content, 'line');
    if (text !== '' || node.attrs.form === 'bq') return `bq. ${text}`;
  }
  // {quote} cannot be nested: an inner quote keeps its content only.
  if (inQuote) return blocksText(node.content, true);
  const params = node.attrs.params as string | null;
  const open = `{quote${params === null ? '' : `:${params}`}}`;
  return `${open}\n${blocksText(node.content, true)}\n{quote}`;
}

function blockText(node: PMNode, inQuote: boolean): string {
  switch (node.type) {
    case N.paragraph:
      return serializeInline(node.content, 'para');
    case N.heading: {
      const text = serializeInline(node.content, 'line');
      return `h${node.attrs.level}.${text === '' ? '' : ` ${text}`}`;
    }
    case N.blockquote:
      return quoteText(node, inQuote);
    case N.code_block:
      return codeText(node, inQuote);
    case N.horizontal_rule:
      return '----';
    case N.bullet_list:
    case N.ordered_list: {
      const lines: string[] = [];
      listLines(node, '', lines);
      return lines.join('\n');
    }
    case N.table:
      return tableText(node);
    case N.raw_block:
      return node.attrs.wiki as string;
    default:
      return blocksText(node.content, inQuote);
  }
}

function blocksText(content: Fragment, inQuote: boolean): string {
  const parts: string[] = [];
  content.forEach((child) => {
    const text = blockText(child, inQuote);
    if (text.trim() !== '') parts.push(text);
  });
  return parts.join('\n\n');
}

const canonicalCache = new WeakMap<PMNode, string>();

/** Canonical markup of one top-level block (cached: nodes never change). */
function canonical(node: PMNode): string {
  let text = canonicalCache.get(node);
  if (text === undefined) {
    text = blockText(node, false);
    canonicalCache.set(node, text);
  }
  return text;
}

/** The original blocks by canonical markup: an edit may rebuild a block without changing it. */
const byMarkupCache = new WeakMap<ParsedWiki, Map<string, PMNode>>();

function byMarkup(base: ParsedWiki): Map<string, PMNode> {
  let map = byMarkupCache.get(base);
  if (!map) {
    map = new Map();
    for (const node of base.blocks.keys()) {
      const key = canonical(node);
      if (!map.has(key)) map.set(key, node);
    }
    byMarkupCache.set(base, map);
  }
  return map;
}

/**
 * Writes an editor document as wiki markup. With `base` (what it was parsed
 * from), an unchanged document gives back its source exactly, and top-level
 * blocks whose content did not change keep their original markup.
 */
export function serializeWiki(doc: PMNode, base?: ParsedWiki | null): string {
  if (base && doc.eq(base.doc)) return base.source;
  let out = '';
  let prev: PMNode | null = null;
  doc.forEach((child) => {
    let original: PMNode | null = base?.blocks.has(child) ? child : null;
    const text = original ? '' : canonical(child);
    if (!original && base) original = byMarkup(base).get(text) ?? null;
    const kept = original ? base?.blocks.get(original) : undefined;
    const written = kept ? kept.text : text;
    if (written.trim() === '') return;
    if (out !== '') out += kept && prev !== null && kept.prev === prev ? kept.sep : '\n\n';
    out += written;
    prev = original;
  });
  return out;
}
