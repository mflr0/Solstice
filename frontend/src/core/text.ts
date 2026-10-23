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
