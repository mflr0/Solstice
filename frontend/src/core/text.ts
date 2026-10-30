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
