import { describe, expect, it } from 'vitest';

import { focusAfterRemoval } from '../../src/core/focus.js';

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
