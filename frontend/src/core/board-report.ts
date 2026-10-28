/**
 * The sprint and velocity reports (backend/internal/api/sprint_report.go)
 * and the geometry of their charts: pure functions, tested in
 * tests/core/board-report.test.ts.
 */

import type { Sprint } from './board-types.js';
import type { IssueSummary } from './types.js';

export type Outcome = 'completed' | 'notCompleted' | 'removed';

export interface ReportIssue extends IssueSummary {
  /** In the sprint when it started; otherwise added later. */
  committed: boolean;
  outcome: Outcome;
  /** When the sprint started (or the issue joined), and when the report ends. */
  startEstimate: number | null;
  endEstimate: number | null;
}

export interface ReportTotals {
  committed: number;
  added: number;
  removed: number;
  completed: number;
  notCompleted: number;
}

export interface SprintReport {
  sprint: Sprint;
  /** Where the report stops: the sprint's completion, or now. */
  end: string;
  /** '' when the board has no estimate: issues are counted. */
  estimate: '' | 'points' | 'time';
  totals: ReportTotals;
  startRemaining: number;
  /** Each change of the remaining work, in order. */
  steps: { at: string; issue: string; remaining: number }[];
  /** In board order. */
  issues: ReportIssue[];
}

/** A closed sprint of the velocity report. */
export interface VelocitySprint {
  sprint: Sprint;
  /** Work in the sprint when it started. */
  committed: number;
  /** Work done when it completed, added issues included. */
  completed: number;
}

export interface VelocityReport {
  /** '' when the board has no estimate: issues are counted. */
  estimate: '' | 'points' | 'time';
  /** The last closed sprints, oldest first. */
  sprints: VelocitySprint[];
}

/** The work completed per sprint on average; null without a sprint. */
export function averageCompleted(r: VelocityReport): number | null {
  if (r.sprints.length === 0) return null;
  const sum = r.sprints.reduce((n, s) => n + s.completed, 0);
  return Math.round((sum / r.sprints.length) * 100) / 100;
}

/**
 * Labels for names under a chart's bars: the words every name shares at the
 * start and at the end left out ("Web Sprint 4", "Web Sprint 5" give "4", "5"),
 * then cut to max characters.
 */
export function shortLabels(names: string[], max: number): string[] {
  let words = names.map((n) => n.split(/\s+/).filter(Boolean));
  if (words.length > 1) {
    const shortest = Math.min(...words.map((w) => w.length));
    const same = (pick: (w: string[]) => string) => words.every((w) => pick(w) === pick(words[0]!));
    let head = 0;
    while (head < shortest - 1 && same((w) => w[head]!)) head++;
    let tail = 0;
    while (head + tail < shortest - 1 && same((w) => w[w.length - 1 - tail]!)) tail++;
    words = words.map((w) => w.slice(head, w.length - tail));
  }
  return words.map((w) => {
    const label = w.join(' ');
    return label.length <= max ? label : `${label.slice(0, Math.max(1, max - 1)).trimEnd()}…`;
  });
}

/**
 * Splits a range into count equal bands, gap being the share of each left
 * empty around it. Bands wider than maxStep shrink to it, centred.
 */
export function bands(
  count: number,
  [r0, r1]: [number, number],
  gap = 0.3,
  maxStep = Infinity,
): { x: number; w: number }[] {
  if (count <= 0) return [];
  const step = Math.min((r1 - r0) / count, maxStep);
  const from = (r0 + r1 - step * count) / 2;
  return Array.from({ length: count }, (_, i) => ({
    x: from + i * step + (step * gap) / 2,
    w: step * (1 - gap),
  }));
}

/** A point of a chart: a time (ms) and a value. */
export interface Point {
  t: number;
  v: number;
}

/**
 * The remaining work as a staircase: flat until a change, then straight to
 * its new value. A step at the same time as the previous one joins it.
 */
export function burndownLine(r: SprintReport): Point[] {
  const start = Date.parse(r.sprint.start ?? r.end);
  const pts: Point[] = [{ t: start, v: r.startRemaining }];
  let v = r.startRemaining;
  for (const s of r.steps) {
    const t = Date.parse(s.at);
    pts.push({ t, v }, { t, v: s.remaining });
    v = s.remaining;
  }
  pts.push({ t: Date.parse(r.end), v });
  return pts.filter((p, i) => i === 0 || p.t !== pts[i - 1]!.t || p.v !== pts[i - 1]!.v);
}

/** The guideline: from the work at start down to nothing on the planned end. */
export function guideline(r: SprintReport): [Point, Point] {
  const start = Date.parse(r.sprint.start ?? r.end);
  return [
    { t: start, v: r.startRemaining },
    { t: Date.parse(r.sprint.end ?? r.end), v: 0 },
  ];
}

/** The time axis: from the start to the planned end or the report's end, whichever is later. */
export function timeDomain(r: SprintReport): [number, number] {
  const start = Date.parse(r.sprint.start ?? r.end);
  const end = Math.max(Date.parse(r.end), Date.parse(r.sprint.end ?? r.end));
  return [start, Math.max(end, start + 1)];
}

/** Round ticks from 0 to at least max, about count of them. */
export function valueTicks(max: number, count = 4): number[] {
  if (!(max > 0)) return [0, 1];
  const raw = max / count;
  const mag = 10 ** Math.floor(Math.log10(raw));
  const step = ([1, 2, 5, 10].find((m) => m * mag >= raw) ?? 10) * mag;
  const top = Math.ceil(max / step) * step;
  const out: number[] = [];
  for (let v = 0; v <= top + step / 2; v += step) out.push(Math.round(v * 1e6) / 1e6);
  return out;
}

/** Local midnights from the day after from to to, thinned to at most max of them. */
export function dayTicks(from: number, to: number, max = 8): number[] {
  const days: number[] = [];
  const d = new Date(from);
  d.setHours(0, 0, 0, 0);
  for (d.setDate(d.getDate() + 1); d.getTime() <= to; d.setDate(d.getDate() + 1)) {
    days.push(d.getTime());
  }
  const every = Math.max(1, Math.ceil(days.length / max));
  return days.filter((_, i) => i % every === 0);
}

/** The report's issues by where they ended up. */
export function byOutcome(r: SprintReport): Record<Outcome, ReportIssue[]> {
  const out: Record<Outcome, ReportIssue[]> = { completed: [], notCompleted: [], removed: [] };
  for (const is of r.issues) out[is.outcome].push(is);
  return out;
}

/** A linear scale from a domain onto a range. */
export function scale(
  [d0, d1]: [number, number],
  [r0, r1]: [number, number],
): (x: number) => number {
  const k = d1 === d0 ? 0 : (r1 - r0) / (d1 - d0);
  return (x) => r0 + (x - d0) * k;
}
