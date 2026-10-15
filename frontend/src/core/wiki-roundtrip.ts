/**
 * Checks that wiki markup survives the editor: parsed into a document, then
 * written back as if every block had been edited (canonical markup, no
 * original text reused). Used by the tests and by the statistics script that
 * runs over real data; the application itself does not import it.
 */

import type { Node as PMNode } from 'prosemirror-model';
import { renderWiki, type WikiResolver } from './wiki.js';
import { parseWiki, serializeWiki } from './wiki-doc.js';

/** Every reference resolves: images show, keys link, names display. */
export const ALL_EXIST: WikiResolver = {
  user: (username) => `User ${username}`,
  attachment: (filename) => `/files/${encodeURIComponent(filename)}`,
  issueExists: () => true,
};

/** Nothing resolves: references stay as written. */
export const NONE_EXIST: WikiResolver = {
  user: () => null,
  attachment: () => null,
  issueExists: () => false,
};

/**
 * - identical: the canonical markup is the source itself;
 * - same-html: different markup, same HTML whatever the data;
 * - same-meaning: same text and formatting, only the nesting order of inline
 *   tags differs (<strong><em> for <em><strong>);
 * - different: anything else.
 */
export type Verdict = 'identical' | 'same-html' | 'same-meaning' | 'different';

export interface RoundTrip {
  verdict: Verdict;
  canonical: string;
  /** Kinds of content met in the text (node, mark and raw kinds). */
  constructs: string[];
  /** For a "different" verdict: whether the meaning holds when every reference exists. */
  sameWithAllRefs: boolean;
}

const INLINE_TAGS = new Set([
  'strong',
  'em',
  'del',
  'ins',
  'sup',
  'sub',
  'cite',
  'code',
  'span',
  'a',
]);
const TOKEN_RE = /<(\/?)([a-z][a-z0-9]*)([^>]*)>|([^<]+)/g;

/**
 * HTML with inline formatting flattened: each run of text is written with the
 * sorted set of inline tags around it, so nesting order no longer matters.
 */
export function flattenInline(html: string): string {
  let out = '';
  const open: string[] = [];
  let pre = 0;
  let run = '';
  let runKey = '';
  const flushRun = (): void => {
    if (run !== '') out += `{${runKey}}${run}`;
    run = '';
  };
  for (const m of html.matchAll(TOKEN_RE)) {
    const [, closing = '', name = '', attrs = '', text] = m;
    if (text !== undefined) {
      // Adjacent runs with the same formatting are one run (two links to the same place, one link).
      const key = [...open].sort().join(',');
      if (key !== runKey) flushRun();
      runKey = key;
      run += text;
      continue;
    }
    if (name === 'pre') pre += closing ? -1 : 1;
    if (INLINE_TAGS.has(name) && !(name === 'code' && pre > 0)) {
      if (closing) {
        const at = open.map((t) => t.split(' ')[0]).lastIndexOf(name);
        if (at >= 0) open.splice(at, 1);
      } else {
        open.push(`${name}${attrs}`);
      }
      continue;
    }
    flushRun();
    runKey = '';
    out += `<${closing}${name}${attrs}>`;
  }
  flushRun();
  return out;
}

function constructsOf(doc: PMNode): string[] {
  const found = new Set<string>();
  doc.descendants((node, _pos, parent) => {
    const type = node.type.name;
    if (type === 'raw_inline' || type === 'raw_block') {
      found.add(`${type}:${node.attrs.kind as string}`);
    } else if (type !== 'text' && type !== 'paragraph') {
      found.add(type);
    }
    if ((type === 'bullet_list' || type === 'ordered_list') && parent?.type.name === 'list_item') {
      found.add('nested_list');
    }
    if (type === 'hard_break') found.add(`hard_break:${node.attrs.form as string}`);
    for (const mark of node.marks) found.add(`mark:${mark.type.name}`);
  });
  return [...found].sort();
}

/** The markup saving writes when every block has been edited. */
export function canonicalWiki(src: string): string {
  return serializeWiki(parseWiki(src).doc);
}

/** Parses, writes back canonically, and compares. */
export function roundTrip(src: string): RoundTrip {
  const parsed = parseWiki(src);
  const canonical = serializeWiki(parsed.doc);
  const constructs = constructsOf(parsed.doc);
  if (canonical === src) {
    return { verdict: 'identical', canonical, constructs, sameWithAllRefs: true };
  }
  // Each text rendered once per resolver: [all exist, none exist].
  const pairs = [ALL_EXIST, NONE_EXIST].map((r) => [renderWiki(src, r), renderWiki(canonical, r)]);
  const same = (flat: boolean, k: number): boolean => {
    const [x = '', y = ''] = pairs[k] ?? [];
    return flat ? flattenInline(x) === flattenInline(y) : x === y;
  };
  let verdict: Verdict = 'different';
  if (same(false, 0) && same(false, 1)) verdict = 'same-html';
  else if (same(true, 0) && same(true, 1)) verdict = 'same-meaning';
  const sameWithAllRefs = verdict !== 'different' || same(true, 0);
  return { verdict, canonical, constructs, sameWithAllRefs };
}

/** One step of a rendering: a block tag, or a character with the inline tags around it. */
interface Step {
  /** What is compared: the tag with its attributes, or the inline tags and the character. */
  key: string;
  /** The character, or '' for a tag. */
  ch: string;
  /** What may be shown: tag names only ("<li>", "text[a,strong]"). */
  kind: string;
  /** The block tags open around it ("ul>li"). */
  where: string;
}

function steps(html: string): Step[] {
  const out: Step[] = [];
  const blocks: string[] = [];
  const inline: { name: string; tag: string }[] = [];
  let pre = 0;
  for (const m of html.matchAll(TOKEN_RE)) {
    const [, closing = '', name = '', attrs = '', text] = m;
    const where = blocks.join('>');
    if (text !== undefined) {
      const key = inline
        .map((t) => t.tag)
        .sort()
        .join(',');
      const names = inline
        .map((t) => t.name)
        .sort()
        .join(',');
      const kind = names ? `text[${names}]` : 'text';
      for (const ch of text) out.push({ key: `${key}|${ch}`, ch, kind, where });
      continue;
    }
    if (name === 'pre') pre += closing ? -1 : 1;
    if (INLINE_TAGS.has(name) && !(name === 'code' && pre > 0)) {
      const at = inline.map((t) => t.name).lastIndexOf(name);
      if (!closing) inline.push({ name, tag: `${name}${attrs}` });
      else if (at >= 0) inline.splice(at, 1);
      continue;
    }
    const at = blocks.lastIndexOf(name);
    if (closing && at >= 0) blocks.splice(at, 1);
    else if (!closing && name !== 'br' && name !== 'hr' && name !== 'img') blocks.push(name);
    out.push({ key: `<${closing}${name}${attrs}>`, ch: '', kind: `<${closing}${name}>`, where });
  }
  return out;
}

/** One rendering against another, in words that tell nothing of the text. */
function describe(html: string, other: string): string {
  const a = steps(html);
  const b = steps(other);
  const shape = (list: Step[], br: boolean): string =>
    list.map((t) => (t.ch === '' && (br || t.kind !== '<br>') ? t.kind : '')).join('');
  const chars = (list: Step[]): string => list.map((t) => t.ch).join('');
  let kind = 'inline formatting';
  if (shape(a, false) !== shape(b, false)) kind = 'blocks';
  else if (shape(a, true) !== shape(b, true)) kind = 'line breaks';
  else if (chars(a) !== chars(b)) kind = 'text';
  let d = 0;
  while (d < a.length && d < b.length && a[d]?.key === b[d]?.key) d++;
  const x = a[d];
  const y = b[d];
  const where = x?.where ?? y?.where ?? '';
  // Same tags and character: what differs is an attribute (a link target…).
  const attrs = x && y && x.kind === y.kind && x.ch === y.ch ? ', attributes' : '';
  return `${kind} in ${where || 'top'} (${x?.kind ?? 'end'} / ${y?.kind ?? 'end'}${attrs})`;
}

/**
 * How two markup texts render differently, for reports over private data:
 * which references matter, whether blocks, text or inline formatting differ,
 * and the tags around the first difference, never any of the text itself.
 * Empty when they mean the same.
 */
export function differenceKind(a: string, b: string): string {
  const found: string[] = [];
  for (const [name, resolver] of [
    ['all refs', ALL_EXIST],
    ['no refs', NONE_EXIST],
  ] as const) {
    const x = renderWiki(a, resolver);
    const y = renderWiki(b, resolver);
    if (flattenInline(x) !== flattenInline(y)) found.push(`${name}: ${describe(x, y)}`);
  }
  return found.join('; ');
}
