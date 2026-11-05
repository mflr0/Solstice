import { describe, expect, it } from 'vitest';

import { focusAfterRemoval, focusOrderAfterRemoval } from '../../src/core/focus.js';

describe('focusAfterRemoval', () => {
  it('takes the next row', () => {
    expect(focusAfterRemoval([1, 2, 3], 1)).toBe(2);
    expect(focusAfterRemoval([1, 2, 3], 2)).toBe(3);
  });

  it('takes the previous row after the last one', () => {
    expect(focusAfterRemoval([1, 2, 3], 3)).toBe(2);
  });

  it('takes none when the row was alone', () => {
    expect(focusAfterRemoval(['a'], 'a')).toBeUndefined();
  });

  it('takes none when the row is not among those with the control', () => {
    expect(focusAfterRemoval([1, 2], 3)).toBeUndefined();
  });

  it('keeps a falsy neighbour', () => {
    expect(focusAfterRemoval([5, 0], 5)).toBe(0);
    expect(focusAfterRemoval([0, 5], 5)).toBe(0);
  });
});

describe('focusOrderAfterRemoval', () => {
  const idOf = (n: number) => `row-${n}`;

  it('tries the next rows, then the previous ones, nearest first, then the fallback', () => {
    expect(focusOrderAfterRemoval([1, 2, 3, 4], 2, idOf, 'new')).toEqual([
      'row-3',
      'row-4',
      'row-1',
      'new',
    ]);
  });

  it('starts with the previous row after the last one', () => {
    expect(focusOrderAfterRemoval([1, 2, 3], 3, idOf, 'new')).toEqual(['row-2', 'row-1', 'new']);
  });

  it('has only the fallback for the only row', () => {
    expect(focusOrderAfterRemoval([1], 1, idOf, 'new')).toEqual(['new']);
  });

  it('has only the fallback for a row no longer listed', () => {
    expect(focusOrderAfterRemoval([1, 2], 3, idOf, 'new')).toEqual(['new']);
  });

  it('finds the row by id, whatever object holds it', () => {
    const rows = [{ id: 1 }, { id: 2 }];
    expect(focusOrderAfterRemoval(rows, { id: 1 }, (r) => `r${r.id}`, 'new')).toEqual([
      'r2',
      'new',
    ]);
  });
});
