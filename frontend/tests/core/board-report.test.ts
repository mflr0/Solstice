import { describe, expect, it } from 'vitest';

import {
  burndownLine,
  byOutcome,
  dayTicks,
  guideline,
  scale,
  timeDomain,
  valueTicks,
  type ReportIssue,
  type SprintReport,
} from '../../src/core/board-report.js';

function report(extra: Partial<SprintReport> = {}): SprintReport {
  return {
    sprint: {
      id: 1,
      name: 'Sprint 1',
      state: 'closed',
      goal: null,
      start: '2026-10-01T09:00:00Z',
      end: '2026-10-15T09:00:00Z',
      completed: '2026-10-14T17:00:00Z',
    },
    end: '2026-10-14T17:00:00Z',
    estimate: 'points',
    totals: { committed: 8, added: 0, removed: 0, completed: 0, notCompleted: 0 },
    startRemaining: 8,
    steps: [],
    issues: [],
    ...extra,
  };
}

const ms = (iso: string) => Date.parse(iso);

describe('burndownLine', () => {
  it('draws a staircase from the start to the end', () => {
    const r = report({
      steps: [
        { at: '2026-10-03T10:00:00Z', issue: 'A-1', remaining: 5 },
        { at: '2026-10-06T10:00:00Z', issue: 'A-2', remaining: 7 },
      ],
    });
    expect(burndownLine(r)).toEqual([
      { t: ms('2026-10-01T09:00:00Z'), v: 8 },
      { t: ms('2026-10-03T10:00:00Z'), v: 8 },
      { t: ms('2026-10-03T10:00:00Z'), v: 5 },
      { t: ms('2026-10-06T10:00:00Z'), v: 5 },
      { t: ms('2026-10-06T10:00:00Z'), v: 7 },
      { t: ms('2026-10-14T17:00:00Z'), v: 7 },
    ]);
  });

  it('joins steps at the same time', () => {
    const at = '2026-10-03T10:00:00Z';
    const r = report({
      steps: [
        { at, issue: 'A-1', remaining: 5 },
        { at, issue: 'A-2', remaining: 4 },
      ],
    });
    expect(burndownLine(r)).toEqual([
      { t: ms('2026-10-01T09:00:00Z'), v: 8 },
      { t: ms(at), v: 8 },
      { t: ms(at), v: 5 },
      { t: ms(at), v: 4 },
      { t: ms('2026-10-14T17:00:00Z'), v: 4 },
    ]);
  });
});

describe('axes', () => {
  it('runs the guideline to the planned end, and the time axis to the later end', () => {
    const r = report();
    expect(guideline(r)).toEqual([
      { t: ms('2026-10-01T09:00:00Z'), v: 8 },
      { t: ms('2026-10-15T09:00:00Z'), v: 0 },
    ]);
    expect(timeDomain(r)).toEqual([ms('2026-10-01T09:00:00Z'), ms('2026-10-15T09:00:00Z')]);
    const late = report({ end: '2026-10-20T09:00:00Z' });
    expect(timeDomain(late)[1]).toBe(ms('2026-10-20T09:00:00Z'));
  });

  it('picks round value ticks', () => {
    expect(valueTicks(8)).toEqual([0, 2, 4, 6, 8]);
    expect(valueTicks(13)).toEqual([0, 5, 10, 15]);
    expect(valueTicks(0.5)).toEqual([0, 0.2, 0.4, 0.6]);
    expect(valueTicks(0)).toEqual([0, 1]);
  });

  it('ticks local midnights, thinned', () => {
    const from = new Date(2026, 9, 1, 9).getTime();
    const to = new Date(2026, 9, 5, 9).getTime();
    expect(dayTicks(from, to)).toEqual([2, 3, 4, 5].map((d) => new Date(2026, 9, d).getTime()));
    const long = dayTicks(from, new Date(2026, 9, 31, 9).getTime(), 8);
    expect(long.length).toBeLessThanOrEqual(8);
    expect(long[0]).toBe(new Date(2026, 9, 2).getTime());
  });

  it('scales linearly', () => {
    const x = scale([10, 20], [0, 100]);
    expect(x(15)).toBe(50);
    expect(scale([5, 5], [0, 100])(5)).toBe(0);
  });
});

describe('byOutcome', () => {
  it('groups issues by where they ended up, in board order', () => {
    const issue = (key: string, outcome: ReportIssue['outcome']) =>
      ({ key, outcome }) as ReportIssue;
    const r = report({
      issues: [issue('A-1', 'completed'), issue('A-2', 'removed'), issue('A-3', 'completed')],
    });
    const g = byOutcome(r);
    expect(g.completed.map((i) => i.key)).toEqual(['A-1', 'A-3']);
    expect(g.removed.map((i) => i.key)).toEqual(['A-2']);
    expect(g.notCompleted).toEqual([]);
  });
});
