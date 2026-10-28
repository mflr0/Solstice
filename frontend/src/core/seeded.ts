/**
 * Catalogue entries a new instance starts with (priorities, issue types,
 * resolutions, link types) are stored in the instance's language. While an
 * entry's text is still the seeded one, the server tags it with that
 * default's key, and the interface shows it in the viewer's language.
 * Renamed and imported entries show as stored. Pure functions, tested in
 * tests/core/seeded.test.ts.
 */

import type { LinkTypeRef, SeedKey } from './types.js';

/** The viewer's text of each seeded default, by key. */
export type SeedWords = Readonly<Record<SeedKey, string>>;

interface Seedable {
  id: string | number;
  name: string;
  seed?: SeedKey;
}

/**
 * The key of the default ref still is: its own tag, else the catalogue's
 * entry of the same id while it holds the same name (a ref built outside
 * the catalogue carries no tag).
 */
function seedOf<T extends Seedable>(
  ref: T,
  catalogue: readonly T[] | undefined,
): SeedKey | undefined {
  if (ref.seed) return ref.seed;
  const entry = catalogue?.find((x) => x.id === ref.id);
  return entry && entry.name === ref.name ? entry.seed : undefined;
}

function word(words: SeedWords, key: SeedKey | undefined, stored: string): string {
  return (key !== undefined && Object.hasOwn(words, key) ? words[key] : undefined) ?? stored;
}

/** A catalogue entry's name as shown. */
export function seededName<T extends Seedable>(
  ref: T,
  catalogue: readonly T[] | undefined,
  words: SeedWords,
): string {
  return word(words, seedOf(ref, catalogue), ref.name);
}

/** How a link type reads one way ("blocks", "is blocked by"), as shown. */
export function seededVerb(
  lt: LinkTypeRef,
  direction: 'inward' | 'outward',
  catalogue: readonly LinkTypeRef[] | undefined,
  words: SeedWords,
): string {
  const stored = lt[direction];
  let key = direction === 'inward' ? lt.inwardSeed : lt.outwardSeed;
  if (key === undefined) {
    const entry = catalogue?.find((x) => x.id === lt.id);
    if (entry && entry[direction] === stored)
      key = direction === 'inward' ? entry.inwardSeed : entry.outwardSeed;
  }
  return word(words, key, stored);
}
