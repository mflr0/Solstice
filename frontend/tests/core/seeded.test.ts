import { readFileSync } from 'node:fs';

import { describe, expect, it } from 'vitest';

import { seededName, seededVerb } from '../../src/core/seeded.js';
import type { LinkTypeRef, PriorityRef } from '../../src/core/types.js';
import { de } from '../../src/i18n/de.js';
import { en } from '../../src/i18n/en.js';
import { fr } from '../../src/i18n/fr.js';
import { hi } from '../../src/i18n/hi.js';
import { it as italian } from '../../src/i18n/it.js';
import { LOCALES } from '../../src/i18n/locales.js';

const catalogs = { en, fr, de, it: italian, hi };

describe('the seeded texts', () => {
  // Every key the server tags entries with (the seeded* lists of
  // backend/internal/api/server.go), with the text the server seeds.
  it.each(LOCALES)('are the server’s in %s', (l) => {
    const source = readFileSync(
      new URL(`../../../backend/internal/i18n/${l}.go`, import.meta.url),
      'utf8',
    );
    const server = Object.fromEntries(
      [
        ...source.matchAll(/^\s+(Default(?:Type|Priority|Resolution|Link)\w+):\s+"([^"]+)",$/gm),
      ].map(([, k, v]) => [k, v!.replaceAll("'", '’')]),
    );
    expect(Object.keys(server)).toHaveLength(25);
    expect(catalogs[l].seeded).toEqual(server);
  });
});

const words = {
  DefaultPriorityHigh: 'Haute',
  DefaultLinkBlocksInward: 'est bloqué par',
  DefaultLinkBlocksOutward: 'bloque',
};

describe('seededName', () => {
  const priorities: PriorityRef[] = [
    { id: '1', name: 'High', seed: 'DefaultPriorityHigh' },
    { id: '2', name: 'Urgent' },
  ];

  it('shows a tagged entry in the viewer’s language', () => {
    expect(
      seededName({ id: '1', name: 'High', seed: 'DefaultPriorityHigh' }, priorities, words),
    ).toBe('Haute');
  });

  it('shows an untagged entry as stored', () => {
    expect(seededName({ id: '2', name: 'Urgent' }, priorities, words)).toBe('Urgent');
  });

  it('takes the tag from the catalogue for a ref built without one', () => {
    expect(seededName({ id: '1', name: 'High' }, priorities, words)).toBe('Haute');
  });

  it('keeps a name the catalogue no longer holds', () => {
    expect(seededName({ id: '1', name: 'Hot' }, priorities, words)).toBe('Hot');
  });

  it('keeps the stored name for a key the viewer’s language lacks', () => {
    expect(seededName({ id: '9', name: 'Low', seed: 'DefaultPriorityLow' }, [], words)).toBe('Low');
    expect(seededName({ id: '9', name: 'Low', seed: 'toString' }, [], words)).toBe('Low');
  });
});

describe('seededVerb', () => {
  const blocks: LinkTypeRef = {
    id: 1,
    name: 'Blocks',
    inward: 'is blocked by',
    outward: 'holds back',
    seed: 'DefaultLinkBlocksName',
    inwardSeed: 'DefaultLinkBlocksInward',
  };

  it('translates each direction on its own', () => {
    expect(seededVerb(blocks, 'inward', [blocks], words)).toBe('est bloqué par');
    expect(seededVerb(blocks, 'outward', [blocks], words)).toBe('holds back');
  });

  it('takes the tags from the catalogue for a ref built without them', () => {
    const bare = { id: 1, name: 'Blocks', inward: 'is blocked by', outward: 'holds back' };
    expect(seededVerb(bare, 'inward', [blocks], words)).toBe('est bloqué par');
    expect(seededVerb({ ...bare, inward: 'waits for' }, 'inward', [blocks], words)).toBe(
      'waits for',
    );
  });
});
