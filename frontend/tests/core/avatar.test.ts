import { describe, expect, it } from 'vitest';

import { AVATAR_SIZE, avatarUrl, squareCrop } from '../../src/core/avatar.js';

describe('squareCrop', () => {
  it('takes the middle of a landscape image', () => {
    expect(squareCrop(1000, 600)).toEqual({ sx: 200, sy: 0, side: 600 });
  });

  it('takes the middle of a portrait image', () => {
    expect(squareCrop(600, 1000)).toEqual({ sx: 0, sy: 200, side: 600 });
  });

  it('keeps a square image whole', () => {
    expect(squareCrop(300, 300)).toEqual({ sx: 0, sy: 0, side: 300 });
  });

  it('stays on whole pixels', () => {
    expect(squareCrop(101, 50)).toEqual({ sx: 25, sy: 0, side: 50 });
    expect(squareCrop(50.5, 50.5)).toEqual({ sx: 0, sy: 0, side: 50 });
  });

  it('refuses an image without pixels', () => {
    expect(squareCrop(0, 100)).toBeNull();
    expect(squareCrop(100, 0)).toBeNull();
    expect(squareCrop(Number.NaN, 100)).toBeNull();
  });

  it('is scaled to the size the server expects at most', () => {
    expect(AVATAR_SIZE).toBe(256);
  });
});

describe('avatarUrl', () => {
  it('addresses a person by id', () => {
    expect(avatarUrl(42)).toBe('/api/avatars/42');
  });

  it('adds the version after a change', () => {
    expect(avatarUrl(42, 1700000000000)).toBe('/api/avatars/42?v=1700000000000');
  });
});
