import { describe, expect, it } from 'vitest';

import { reconnectDelay } from '../../src/lib/live.svelte.js';

describe('reconnectDelay', () => {
  it('starts at 5 s and doubles', () => {
    expect([0, 1, 2, 3].map(reconnectDelay)).toEqual([5000, 10000, 20000, 40000]);
  });

  it('never goes past 60 s', () => {
    expect(reconnectDelay(4)).toBe(60000);
    expect(reconnectDelay(50)).toBe(60000);
  });
});
