import { describe, expect, it } from 'vitest';

import { backdropClick, outside } from '../../src/core/modal.js';

const rect = { left: 100, top: 50, right: 500, bottom: 350 };

describe('outside', () => {
  it('finds a point beyond each side', () => {
    expect(outside(rect, { x: 99, y: 200 })).toBe(true);
    expect(outside(rect, { x: 501, y: 200 })).toBe(true);
    expect(outside(rect, { x: 300, y: 49 })).toBe(true);
    expect(outside(rect, { x: 300, y: 351 })).toBe(true);
  });

  it('keeps a point inside or on the edge', () => {
    expect(outside(rect, { x: 300, y: 200 })).toBe(false);
    expect(outside(rect, { x: 100, y: 50 })).toBe(false);
    expect(outside(rect, { x: 500, y: 350 })).toBe(false);
    expect(outside(rect, { x: 100, y: 200 })).toBe(false);
  });
});

describe('backdropClick', () => {
  const out = { x: 20, y: 20 };
  const inside = { x: 300, y: 200 };
  const edge = { x: 500, y: 200 };

  it('closes on a press and release outside', () => {
    expect(backdropClick(rect, out, { x: 600, y: 400 })).toBe(true);
  });

  it('stays open on a click inside or on the edge', () => {
    expect(backdropClick(rect, inside, inside)).toBe(false);
    expect(backdropClick(rect, edge, edge)).toBe(false);
  });

  it('stays open on a press that started inside and was released outside', () => {
    expect(backdropClick(rect, inside, out)).toBe(false);
  });

  it('stays open on a press that started outside and was released inside', () => {
    expect(backdropClick(rect, out, inside)).toBe(false);
  });
});
