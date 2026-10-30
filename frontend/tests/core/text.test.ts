import { describe, expect, it } from 'vitest';
import { highlight, highlightPrefix, highlightText, matches } from '../../src/core/text.js';

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

describe('highlightPrefix', () => {
  it('marks the typed start, folded', () => {
    expect(highlightPrefix('Été 2024', 'ete')).toEqual([
      { text: 'Été', match: true },
      { text: ' 2024', match: false },
    ]);
  });

  it('marks nothing elsewhere in the text', () => {
    expect(highlightPrefix('assignee', 'sig')).toEqual([{ text: 'assignee', match: false }]);
  });

  it('marks nothing for an empty prefix', () => {
    expect(highlightPrefix('status', '')).toEqual([{ text: 'status', match: false }]);
  });

  it('keeps the original text when folding changes its length', () => {
    expect(highlightPrefix('İstanbul', 'is')).toEqual([
      { text: 'İs', match: true },
      { text: 'tanbul', match: false },
    ]);
  });
});

describe('highlightText', () => {
  const marked = (text: string, search: string) =>
    highlightText(text, search)
      .filter((p) => p.match)
      .map((p) => p.text);

  it('marks each word where a word of the text starts with it', () => {
    expect(marked('Login page crashes', 'crash log')).toEqual(['Log', 'crash']);
    expect(marked('Fix the API', 'api')).toEqual(['API']);
  });

  it('marks nothing inside a word', () => {
    expect(marked('rapid', 'api')).toEqual([]);
  });

  it('falls back to the substring without a word of 3 characters', () => {
    expect(marked('Go up', 'up')).toEqual(['up']);
    expect(marked('Go up', 'go u')).toEqual(['Go u']);
  });

  it('ignores the operators in the search', () => {
    expect(marked('Login page crashes', '+crash* -"log" (pa)')).toEqual(['Log', 'crash']);
  });

  it('follows the word boundaries of the index', () => {
    expect(marked("l'école d'été", 'ecole ete')).toEqual(['école', 'été']);
    expect(marked('e-mail sender', 'mail')).toEqual(['mail']);
    expect(marked('foo_bar', 'bar')).toEqual([]);
    expect(marked('Login crashes', 'crash,')).toEqual(['crash']);
    expect(marked('हिंदी अनुवाद', 'अनु')).toEqual(['अनु']);
    expect(marked('अनुवाद', 'वाद')).toEqual([]);
  });

  it('returns the text as one plain part for a blank search', () => {
    expect(highlightText('Alice', '  ')).toEqual([{ text: 'Alice', match: false }]);
  });
});
