import { describe, expect, it } from 'vitest';

import type { StatusRef } from '../../src/core/types.js';
import {
  arcHeight,
  diagramEdges,
  diagramOrder,
  layout,
  levels,
  margins,
  type Box,
} from '../../src/core/workflow-diagram.js';

const A: StatusRef = { id: 'a', name: 'Open', category: 'todo' };
const B: StatusRef = { id: 'b', name: 'In Progress', category: 'doing' };
const C: StatusRef = { id: 'c', name: 'Closed', category: 'done' };
const R: StatusRef = { id: 'r', name: 'Reopened', category: 'todo' };

/** Boxes 80 px wide, 20 px high, 40 px apart, on a row at y = 100. */
function row(ids: string[]): Record<string, Box> {
  return Object.fromEntries(ids.map((id, i) => [id, { x: i * 120, y: 100, w: 80, h: 20 }]));
}

/** The numbers of an SVG path, in order. */
const nums = (d: string) => (d.match(/-?\d+(\.\d+)?/g) ?? []).map(Number);

describe('diagramOrder', () => {
  it('lays statuses out by category, keeping the workflow order inside each', () => {
    expect(diagramOrder([C, A, B, R]).map((s) => s.id)).toEqual(['a', 'r', 'b', 'c']);
  });
});

describe('diagramEdges', () => {
  it('merges transitions with the same ends and drops the ones it cannot draw', () => {
    const edges = diagramEdges(
      [
        { from: 'a', to: 'c', name: 'Fermer' },
        { from: 'a', to: 'c', name: 'Abandonner' },
        { from: null, to: 'c', name: 'Clore' },
        { from: 'a', to: 'gone', name: 'X' },
        { from: 'b', to: 'b', name: 'Y' },
      ],
      ['a', 'b', 'c'],
    );
    expect(edges).toEqual([
      { from: 'a', to: 'c', names: ['Fermer', 'Abandonner'] },
      { from: null, to: 'c', names: ['Clore'] },
    ]);
  });
});

describe('levels', () => {
  const order = ['a', 'b', 'c', 'd'];
  const e = (from: string, to: string) => ({ from, to, names: [`${from}${to}`] });

  it('keeps arcs inside others lower', () => {
    const inner = e('b', 'c');
    const outer = e('a', 'd');
    const lv = levels(order, [outer, inner]);
    expect(lv.get(inner)).toBe(1);
    expect(lv.get(outer)).toBe(2);
  });

  it('gives crossing arcs of the same length different heights', () => {
    const left = e('a', 'c');
    const right = e('b', 'd');
    const lv = levels(order, [right, left]);
    expect(lv.get(left)).toBe(1);
    expect(lv.get(right)).toBe(2);
  });

  it('lets arcs that only touch, or sit on the other side, share the lowest level', () => {
    const ab = e('a', 'b');
    const bc = e('b', 'c');
    const back = e('c', 'a');
    const lv = levels(order, [ab, bc, back]);
    expect([lv.get(ab), lv.get(bc), lv.get(back)]).toEqual([1, 1, 1]);
  });
});

describe('margins', () => {
  const order = ['a', 'b', 'c'];

  it('grows with the arcs stacked on each side', () => {
    const one = margins(order, [{ from: 'a', to: 'c', names: ['x'] }]);
    const two = margins(order, [
      { from: 'a', to: 'c', names: ['x'] },
      { from: 'a', to: 'b', names: ['y'] },
    ]);
    expect(two.above).toBeGreaterThan(one.above);
    expect(two.below).toBe(one.below);
    const back = margins(order, [{ from: 'c', to: 'a', names: ['x'] }]);
    expect(back.below).toBe(one.above);
    expect(arcHeight(2)).toBeGreaterThan(arcHeight(1));
  });

  it('keeps room for the "any status" line', () => {
    const plain = margins(order, [{ from: 'a', to: 'b', names: ['x'] }]);
    const any = margins(order, [
      { from: 'a', to: 'b', names: ['x'] },
      { from: null, to: 'c', names: ['y'] },
    ]);
    expect(any.above).toBeGreaterThan(plain.above);
  });
});

describe('layout', () => {
  const order = ['a', 'b', 'c'];
  const boxes = row(order);

  it('draws forward moves above the row and backward ones below', () => {
    const { arcs } = layout(
      order,
      boxes,
      [
        { from: 'a', to: 'b', names: ['Commencer'] },
        { from: 'b', to: 'a', names: ['Arrêter'] },
      ],
      null,
    );
    const [fwd, back] = arcs;
    expect(fwd?.kind).toBe('forward');
    expect(back?.kind).toBe('backward');
    const f = nums(fwd?.d ?? '');
    expect(f[1]).toBe(100); // leaves the top of "a"
    expect(Math.max(...f.filter((_, i) => i % 2 === 1))).toBeLessThanOrEqual(100);
    const b = nums(back?.d ?? '');
    expect(b[1]).toBe(120); // leaves the bottom of "b"
    expect(Math.min(...b.filter((_, i) => i % 2 === 1))).toBeGreaterThanOrEqual(120);
  });

  it('ends each arrow on its target, the head touching it', () => {
    const { arcs } = layout(order, boxes, [{ from: 'a', to: 'c', names: ['x'] }], null);
    const head = nums(arcs[0]?.head ?? '');
    expect(head[0]).toBeGreaterThan(240);
    expect(head[0]).toBeLessThan(320);
    expect(head[1]).toBe(99);
  });

  it('nests arcs that meet the same status instead of crossing them', () => {
    // a→c and b→c both land on top of c: the longer one lands further right.
    const { arcs } = layout(
      order,
      boxes,
      [
        { from: 'a', to: 'c', names: ['long'] },
        { from: 'b', to: 'c', names: ['short'] },
      ],
      null,
    );
    const endX = (i: number) => nums(arcs[i]?.d ?? '').at(-2) ?? 0;
    expect(endX(0)).toBeGreaterThan(endX(1));
    const startX = (i: number) => nums(arcs[i]?.d ?? '')[0] ?? 0;
    expect(startX(0)).toBeLessThan(startX(1));
  });

  it('keeps arrows to the left and to the right of a status apart', () => {
    // On b: a→b arrives from the left, b→c leaves to the right.
    const { arcs } = layout(
      order,
      boxes,
      [
        { from: 'b', to: 'c', names: ['out'] },
        { from: 'a', to: 'b', names: ['in'] },
      ],
      null,
    );
    const leaving = nums(arcs[0]?.d ?? '')[0] ?? 0;
    const arriving = nums(arcs[1]?.d ?? '').at(-2) ?? 0;
    expect(arriving).toBeLessThan(leaving);
  });

  it('drops "any status" transitions from one line above every arc', () => {
    const { arcs, bus } = layout(
      order,
      boxes,
      [
        { from: 'a', to: 'c', names: ['x'] },
        { from: null, to: 'c', names: ['Clore'] },
      ],
      null,
    );
    const drop = arcs.find((a) => a.kind === 'any');
    expect(bus).not.toBeNull();
    const arcTop = Math.min(...nums(arcs[0]?.d ?? '').filter((_, i) => i % 2 === 1));
    expect(bus?.y).toBeLessThan(arcTop);
    expect(nums(drop?.d ?? '')[1]).toBe(bus?.y);
    expect(bus?.x1).toBe(0);
    expect(bus?.x2).toBe(nums(drop?.d ?? '')[0]);
  });

  it('has no line when nothing comes from any status', () => {
    expect(layout(order, boxes, [{ from: 'a', to: 'b', names: ['x'] }], null).bus).toBeNull();
  });

  it('marks the start status from its left', () => {
    const { start } = layout(order, boxes, [], 'b');
    expect(start?.cy).toBe(110);
    expect(start?.cx).toBeLessThan(120);
    expect(nums(start?.head ?? '')[0]).toBe(118);
  });

  it('skips statuses it has no box for', () => {
    const { arcs } = layout(order, row(['a', 'b']), [{ from: 'a', to: 'c', names: ['x'] }], null);
    expect(arcs).toEqual([]);
  });
});
