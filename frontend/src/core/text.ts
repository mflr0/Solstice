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
 * The text cut into plain and matched parts, by the rule of `matches`: each
 * word anywhere, folded. Folding changes lengths ("é" → "e", "İ" → "i̇"), so
 * each character is folded alone and the matches are cut from the original.
 */
export function highlight(text: string, search: string): TextPart[] {
  let folded = '';
  // For each folded character, the span of the original character it came from.
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

  const spans: [number, number][] = [];
  for (const word of search.split(/\s+/)) {
    const w = [...word].map(fold).join('');
    if (!w) continue;
    for (let i = folded.indexOf(w); i >= 0; i = folded.indexOf(w, i + 1)) {
      spans.push([from[i]!, to[i + w.length - 1]!]);
    }
  }
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
