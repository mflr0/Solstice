/**
 * The workflow diagram's geometry. Statuses sit on one row, left to right
 * by category (to do, doing, done); a transition that moves right arcs
 * above the row, one that moves back arcs below it, and transitions from
 * any status drop from a dashed line along the top. Arcs that meet a status
 * share its edge in an order that keeps nested arcs from crossing.
 */

import type { StatusRef } from './types.js';
import { byCategory } from './workflow.js';

/** Transitions between the same two statuses, drawn as one arrow. */
export interface DiagramEdge {
  /** null: from any status. */
  from: string | null;
  to: string;
  names: string[];
}

/** A status as laid out on the page, in the diagram's pixels. */
export interface Box {
  x: number;
  y: number;
  w: number;
  h: number;
}

export interface Arc {
  key: string;
  edge: DiagramEdge;
  kind: 'forward' | 'backward' | 'any';
  d: string;
  /** The arrowhead, as polygon points. */
  head: string;
}

export interface Layout {
  arcs: Arc[];
  /** The line "any status" transitions drop from, with its label's anchor. */
  bus: { y: number; x1: number; x2: number } | null;
  /** The start marker before the initial status: a dot and a short arrow. */
  start: { cx: number; cy: number; d: string; head: string } | null;
}

const ARC_BASE = 24;
const ARC_STEP = 14;
const HEAD = 7;
const HEAD_HALF = 4;
/** Between the highest arc and the bus line, and the label above it. */
const BUS_GAP = 14;
const BUS_LABEL = 18;
const PAD = 6;
const START = 22;

/** The order statuses are drawn in. */
export function diagramOrder(statuses: StatusRef[]): StatusRef[] {
  return byCategory(statuses).flatMap((g) => g.statuses);
}

/** Merges transitions with the same ends; drops those touching unknown statuses. */
export function diagramEdges(
  transitions: { from: string | null; to: string; name: string }[],
  ids: string[],
): DiagramEdge[] {
  const out: DiagramEdge[] = [];
  for (const t of transitions) {
    if (!ids.includes(t.to) || (t.from !== null && !ids.includes(t.from)) || t.from === t.to) {
      continue;
    }
    const same = out.find((e) => e.from === t.from && e.to === t.to);
    if (same) same.names.push(t.name);
    else out.push({ from: t.from, to: t.to, names: [t.name] });
  }
  return out;
}

/** The control height of an arc on `level` (1 is the lowest). */
export function arcHeight(level: number): number {
  return ARC_BASE + ARC_STEP * Math.max(0, level - 1);
}

/** A cubic whose two control points sit at height h rises to 3/4 h. */
const peak = (h: number) => Math.ceil(h * 0.75);

function span(order: string[], e: DiagramEdge): number {
  return e.from === null ? 0 : order.indexOf(e.to) - order.indexOf(e.from);
}

/**
 * Stacks the arcs of each side: shorter arcs first, each one level above
 * the highest arc it overlaps. Arcs inside others stay lower, and two arcs
 * that cross never share a height. Arcs that only touch at a status do not
 * count as overlapping.
 */
export function levels(order: string[], edges: DiagramEdge[]): Map<DiagramEdge, number> {
  const out = new Map<DiagramEdge, number>();
  for (const dir of [1, -1]) {
    const arcs = edges
      .filter((e) => Math.sign(span(order, e)) === dir)
      .map((e) => {
        const a = order.indexOf(e.from ?? '');
        const b = order.indexOf(e.to);
        return { e, lo: Math.min(a, b), hi: Math.max(a, b) };
      })
      .sort((x, y) => x.hi - x.lo - (y.hi - y.lo) || x.lo - y.lo);
    const placed: { lo: number; hi: number; level: number }[] = [];
    for (const a of arcs) {
      const under = placed.filter((p) => Math.max(p.lo, a.lo) < Math.min(p.hi, a.hi));
      const level = 1 + Math.max(0, ...under.map((p) => p.level));
      placed.push({ lo: a.lo, hi: a.hi, level });
      out.set(a.e, level);
    }
  }
  return out;
}

/** Room needed above and below the row of statuses. */
export function margins(order: string[], edges: DiagramEdge[]): { above: number; below: number } {
  const level = levels(order, edges);
  let up = 0;
  let down = 0;
  for (const e of edges) {
    const s = span(order, e);
    const h = peak(arcHeight(level.get(e) ?? 1));
    if (s > 0) up = Math.max(up, h);
    if (s < 0) down = Math.max(down, h);
  }
  const any = edges.some((e) => e.from === null);
  return {
    above: up + PAD + (any ? BUS_GAP + BUS_LABEL : 0),
    below: down + PAD,
  };
}

const n = (v: number) => String(Math.round(v * 10) / 10);
const pt = (x: number, y: number) => `${n(x)},${n(y)}`;

/** An arrowhead whose tip is (x, y), pointing down (dir 1) or up (dir -1). */
function headAt(x: number, y: number, dir: 1 | -1): string {
  const base = y - dir * HEAD;
  return [pt(x, y), pt(x - HEAD_HALF, base), pt(x + HEAD_HALF, base)].join(' ');
}

interface Port {
  edge: DiagramEdge;
  /** 0: the partner is on the left, 1: on the right, 0.5: a drop from the bus. */
  group: number;
  partner: number;
}

/**
 * Which arcs meet each side of each status, and where along it: left
 * partners first, then right ones, the farther right the partner the further
 * left the port, so nested arcs never cross. Returns the x of the port an
 * edge uses on a status side.
 */
function portsOf(
  order: string[],
  boxes: Record<string, Box>,
  edges: DiagramEdge[],
): (id: string, side: 'top' | 'bottom', e: DiagramEdge) => number {
  const sides = new Map<string, Port[]>();
  const meet = (id: string, side: 'top' | 'bottom', p: Port) => {
    const k = `${id}|${side}`;
    sides.set(k, [...(sides.get(k) ?? []), p]);
  };
  for (const e of edges) {
    const to = order.indexOf(e.to);
    if (e.from === null) {
      meet(e.to, 'top', { edge: e, group: 0.5, partner: to });
      continue;
    }
    const from = order.indexOf(e.from);
    const side = to > from ? 'top' : 'bottom';
    meet(e.from, side, { edge: e, group: to > from ? 1 : 0, partner: to });
    meet(e.to, side, { edge: e, group: from > to ? 1 : 0, partner: from });
  }
  return (id, side, e) => {
    const box = boxes[id] as Box;
    const ports = [...(sides.get(`${id}|${side}`) ?? [])].sort(
      (a, b) => a.group - b.group || b.partner - a.partner,
    );
    const i = ports.findIndex((p) => p.edge === e);
    const k = ports.length;
    return box.x + box.w * (0.12 + (0.76 * (i + 0.5)) / k);
  };
}

/** A drop from the bus line to the top of the target status. */
function anyArc(key: string, e: DiagramEdge, x: number, busY: number, to: Box): Arc {
  const tip = to.y - 1;
  return {
    key,
    edge: e,
    kind: 'any',
    d: `M${pt(x, busY)}V${n(tip - HEAD + 1)}`,
    head: headAt(x, tip, 1),
  };
}

/** Where an arc between two statuses starts and ends, and how high it rises. */
interface ArcEnds {
  from: Box;
  to: Box;
  /** The ports on `from` and on `to`. */
  x1: number;
  x2: number;
  h: number;
}

/** An arc above the row, from the top of `from` to the top of `to`. */
function forwardArc(key: string, e: DiagramEdge, { from, to, x1, x2, h }: ArcEnds): Arc {
  const yc = Math.min(from.y, to.y) - h;
  const tip = to.y - 1;
  return {
    key,
    edge: e,
    kind: 'forward',
    d: `M${pt(x1, from.y)}C${pt(x1, yc)} ${pt(x2, yc)} ${pt(x2, tip - HEAD + 1)}`,
    head: headAt(x2, tip, 1),
  };
}

/** An arc below the row, from the bottom of `from` to the bottom of `to`. */
function backwardArc(key: string, e: DiagramEdge, { from, to, x1, x2, h }: ArcEnds): Arc {
  const fromBottom = from.y + from.h;
  const toBottom = to.y + to.h;
  const yc = Math.max(fromBottom, toBottom) + h;
  const tip = toBottom + 1;
  return {
    key,
    edge: e,
    kind: 'backward',
    d: `M${pt(x1, fromBottom)}C${pt(x1, yc)} ${pt(x2, yc)} ${pt(x2, tip + HEAD - 1)}`,
    head: headAt(x2, tip, -1),
  };
}

/** The line "any status" transitions drop from, or null when none of them do. */
function busLine(
  order: string[],
  boxes: Record<string, Box>,
  y: number,
  end: number,
): Layout['bus'] {
  if (end === -Infinity) return null;
  const xs = order.filter((id) => boxes[id]).map((id) => (boxes[id] as Box).x);
  return { y, x1: Math.min(...xs), x2: Math.max(end, Math.min(...xs)) };
}

/** The start marker before the initial status: a dot and a short arrow. */
function startMarker(first: Box): Layout['start'] {
  const cy = first.y + first.h / 2;
  const tip = first.x - 2;
  const cx = tip - START;
  return {
    cx,
    cy,
    d: `M${pt(cx, cy)}H${n(tip - HEAD + 1)}`,
    head: [pt(tip, cy), pt(tip - HEAD, cy - HEAD_HALF), pt(tip - HEAD, cy + HEAD_HALF)].join(' '),
  };
}

/**
 * Lays the arrows out over measured boxes. `order` is the drawing order of
 * the statuses (see diagramOrder), `boxes` their positions by id.
 */
export function layout(
  order: string[],
  boxes: Record<string, Box>,
  edges: DiagramEdge[],
  initial: string | null,
): Layout {
  const shown = edges.filter(
    (e) => boxes[e.to] && (e.from === null || boxes[e.from]) && order.includes(e.to),
  );
  const ports = portsOf(order, boxes, shown);
  const level = levels(order, shown);
  const tops = order.filter((id) => boxes[id]).map((id) => (boxes[id] as Box).y);
  const rowTop = tops.length ? Math.min(...tops) : 0;
  let up = 0;
  for (const e of shown) {
    if (span(order, e) > 0) up = Math.max(up, peak(arcHeight(level.get(e) ?? 1)));
  }
  const busY = rowTop - up - BUS_GAP;

  const arcs: Arc[] = [];
  let busEnd = -Infinity;
  for (const e of shown) {
    const to = boxes[e.to] as Box;
    const key = `${e.from ?? '*'}>${e.to}`;
    const s = span(order, e);
    if (e.from === null) {
      const x = ports(e.to, 'top', e);
      busEnd = Math.max(busEnd, x);
      arcs.push(anyArc(key, e, x, busY, to));
      continue;
    }
    const side = s > 0 ? 'top' : 'bottom';
    const ends: ArcEnds = {
      from: boxes[e.from] as Box,
      to,
      x1: ports(e.from, side, e),
      x2: ports(e.to, side, e),
      h: arcHeight(level.get(e) ?? 1),
    };
    arcs.push(s > 0 ? forwardArc(key, e, ends) : backwardArc(key, e, ends));
  }

  const bus = busLine(order, boxes, busY, busEnd);

  const first = initial ? boxes[initial] : undefined;
  const start = first ? startMarker(first) : null;

  return { arcs, bus, start };
}
