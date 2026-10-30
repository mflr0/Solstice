import { describe, expect, it } from 'vitest';
import { highlight, matches } from '../../src/core/text.js';

describe('matches', () => {
  it('takes the words in any order', () => {
    expect(matches('smith alice', 'Alice Smith')).toBe(true);
  });

  it('ignores accents and case', () => {
    expect(matches('ete', 'Été')).toBe(true);
  });

  it('wants every word', () => {
    expect(matches('ali bob', 'Alice')).toBe(false);
  });

  it('lets an empty or blank query match everything', () => {
    expect(matches('', 'Alice')).toBe(true);
    expect(matches('   ', 'Alice')).toBe(true);
  });
});

describe('highlight', () => {
  const marked = (text: string, search: string) =>
    highlight(text, search)
      .filter((p) => p.match)
      .map((p) => p.text);

  it('marks the original letters of a folded match', () => {
    expect(highlight('Été 2024', 'ete')).toEqual([
      { text: 'Été', match: true },
      { text: ' 2024', match: false },
    ]);
  });

  it('marks every word, in any order', () => {
    expect(marked('Alice Smith', 'smith ali')).toEqual(['Ali', 'Smith']);
  });

  it('keeps the original text when folding changes its length', () => {
    const parts = highlight('İstanbul', 'ist');
    expect(parts.map((p) => p.text).join('')).toBe('İstanbul');
    expect(marked('İstanbul', 'ist')).toEqual(['İst']);
  });

  it('keeps a decomposed accent with its letter', () => {
    expect(marked('Cafe\u0301 noir', 'cafe')).toEqual(['Cafe\u0301']);
  });

  it('merges overlapping and adjacent matches', () => {
    expect(marked('Alice Smith', 'alic lice')).toEqual(['Alice']);
    expect(marked('Alice Smith', 'ali ce')).toEqual(['Alice']);
  });

  it('returns the text as one plain part without a match', () => {
    expect(highlight('Alice', 'bob')).toEqual([{ text: 'Alice', match: false }]);
    expect(highlight('Alice', '  ')).toEqual([{ text: 'Alice', match: false }]);
  });
});
