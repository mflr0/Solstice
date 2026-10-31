/**
 * Comparing what people type with names: one folding rule for every search
 * that ignores case and accents.
 */

/** Lower case, accents removed: "Été" finds "ete". */
export function fold(s: string): string {
  // Only Latin accents go: other scripts' marks are letters (Devanagari vowel signs).
  return s
    .normalize('NFD')
    .replace(/[\u0300-\u036f]/g, '')
    .toLowerCase();
}

/** Whether a row matches a typed search: every word somewhere in its texts, folded. */
export function matches(search: string, ...texts: (string | null | undefined)[]): boolean {
  const words = fold(search).split(/\s+/).filter(Boolean);
  if (words.length === 0) return true;
  const hay = fold(texts.filter(Boolean).join(' '));
  return words.every((w) => hay.includes(w));
}

/** A piece of a text, marked when it matched the search. */
export interface TextPart {
  text: string;
  match: boolean;
}

/**
 * The text folded, with for each folded character the span of the original
 * character it came from. Folding changes lengths ("é" → "e", "İ" → "i̇"), so
 * each character is folded alone and the matches are cut from the original.
 */
function folding(text: string): { folded: string; from: number[]; to: number[] } {
  let folded = '';
  const from: number[] = [];
  const to: number[] = [];
  let at = 0;
  for (const ch of text) {
    for (const f of fold(ch)) {
      folded += f;
      from.push(at);
      to.push(at + ch.length);
    }
    at += ch.length;
  }
  return { folded, from, to };
}

/** The text cut into plain and matched parts at the given spans of it. */
function cut(text: string, spans: [number, number][]): TextPart[] {
  if (spans.length === 0) return text ? [{ text, match: false }] : [];
  spans.sort((a, b) => a[0] - b[0]);

  const parts: TextPart[] = [];
  let plain = 0;
  let end = -1;
  let start = 0;
  const close = () => {
    // Accents that folded to nothing stay with the letter before them.
    while (end < text.length && fold(text.slice(end, end + 1)) === '') end++;
    if (start > plain) parts.push({ text: text.slice(plain, start), match: false });
    parts.push({ text: text.slice(start, end), match: true });
    plain = end;
  };
  for (const [s, e] of spans) {
    if (s > end) {
      if (end >= 0) close();
      start = s;
    }
    end = Math.max(end, e);
  }
  close();
  if (plain < text.length) parts.push({ text: text.slice(plain), match: false });
  return parts;
}

/** The text cut into plain and matched parts, by the rule of `matches`: each word anywhere, folded. */
export function highlight(text: string, search: string): TextPart[] {
  const { folded, from, to } = folding(text);
  const spans: [number, number][] = [];
  for (const word of search.split(/\s+/)) {
    const w = [...word].map(fold).join('');
    if (!w) continue;
    for (let i = folded.indexOf(w); i >= 0; i = folded.indexOf(w, i + 1)) {
      spans.push([from[i]!, to[i + w.length - 1]!]);
    }
  }
  return cut(text, spans);
}

/** The text cut into plain and matched parts, the typed prefix marked when the text starts with it, folded. */
export function highlightPrefix(text: string, prefix: string): TextPart[] {
  const { folded, from, to } = folding(text);
  const p = [...prefix].map(fold).join('');
  return cut(text, p && folded.startsWith(p) ? [[from[0]!, to[p.length - 1]!]] : []);
}

/** A letter, a digit or `_`: what MariaDB's full-text index keeps inside a word. */
const WORD_END = /[\p{L}\p{M}\p{N}_]$/u;

/** InnoDB's default stopwords of 3 characters or more, as in `ftStopwords` of backend/internal/jql/compile.go. */
const STOPWORDS = new Set(
  'about are com for from how that the this was what when where who will with und www'.split(' '),
);

/** Whether the server ignores a search word, punctuation around it aside. */
function stopword(w: string): boolean {
  return STOPWORDS.has(
    w.replace(/^[^\p{L}\p{M}\p{N}_]+|[^\p{L}\p{M}\p{N}_]+$/gu, '').toLowerCase(),
  );
}

/**
 * The text cut into plain and matched parts, after the server's rule for `text ~`:
 * its operators dropped, each word of 3 characters or more marked where a word
 * of the text starts with it, stopwords included though the server ignores
 * them; with only stopwords or shorter words, the whole search as a substring.
 */
export function highlightText(text: string, search: string): TextPart[] {
  let words = search
    .split(/\s+/)
    .map((w) => w.replace(/[+\-<>()~*"@]/g, ''))
    .filter((w) => [...w].length >= 3);
  if (words.every(stopword)) words = [];
  const { folded, from, to } = folding(text);
  const spans: [number, number][] = [];
  const mark = (w: string, start: (i: number) => boolean) => {
    for (let i = folded.indexOf(w); i >= 0; i = folded.indexOf(w, i + 1)) {
      if (start(i)) spans.push([from[i]!, to[i + w.length - 1]!]);
    }
  };
  if (words.length === 0) {
    const all = [...search.replace(/^\*+|\*+$/g, '')].map(fold).join('');
    if (all) mark(all, () => true);
    return cut(text, spans);
  }
  for (const word of words) {
    // The index splits at punctuation ("crash," finds "crash") and skips what is shorter than 3.
    for (const run of word.match(/[\p{L}\p{M}\p{N}_]+/gu) ?? []) {
      const w = [...run].map(fold).join('');
      if ([...w].length < 3) continue;
      mark(w, (i) => !WORD_END.test(folded.slice(Math.max(0, i - 2), i)));
    }
  }
  return cut(text, spans);
}
