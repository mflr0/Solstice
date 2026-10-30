import { describe, expect, it } from 'vitest';
import { matches } from '../../src/core/text.js';

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
