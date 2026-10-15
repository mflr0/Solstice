/**
 * Board rules that do not touch the page: grouping cards into swimlanes,
 * where a dropped card lands, WIP limits, sprint arithmetic, old board
 * addresses. Pure functions, tested in tests/core/board.test.ts.
 */

import type { BoardCard, CardMove, Swimlane, SwimlaneStrategy } from './board-types.js';
import type { IssueBrief, UserRef } from './types.js';
import { formatLocale } from '../i18n/format.js';

/** A horizontal band of the board. Wording is left to the interface. */
export type Lane =
  | { id: string; kind: 'all'; cards: BoardCard[] }
  | { id: string; kind: 'custom'; name: string; cards: BoardCard[] }
  | { id: string; kind: 'assignee'; user: UserRef | null; cards: BoardCard[] }
  | { id: string; kind: 'epic'; issue: IssueBrief | null; cards: BoardCard[] }
  | { id: string; kind: 'parent'; issue: IssueBrief; cards: BoardCard[] }
  /** Cards no other lane takes. */
  | { id: string; kind: 'other'; cards: BoardCard[] };

/**
 * Splits cards (in rank order) into swimlanes. Custom lanes follow the
 * board's order (the server tells each card its lane); people are sorted by
 * name, epics and parents come in the order of their first card; the lane
 * for "nobody", "no epic" or "everything else" is last. With parent lanes, a
 * parent heads its lane rather than appearing as a card.
 */
export function groupLanes(
  cards: BoardCard[],
  strategy: SwimlaneStrategy,
  lanes: Swimlane[],
): Lane[] {
  switch (strategy) {
    case 'custom': {
      const known = new Set(lanes.map((l) => l.id));
      const out: Lane[] = lanes.map((l) => ({
        id: `lane-${l.id}`,
        kind: 'custom',
        name: l.name,
        cards: cards.filter((c) => c.lane === l.id),
      }));
      const rest = cards.filter((c) => c.lane === undefined || !known.has(c.lane));
      if (rest.length) out.push({ id: 'other', kind: 'other', cards: rest });
      return out;
    }
    case 'assignee': {
      const people = new Map<number, { user: UserRef; cards: BoardCard[] }>();
      const nobody: BoardCard[] = [];
      for (const c of cards) {
        if (!c.assignee) {
          nobody.push(c);
          continue;
        }
        const entry = people.get(c.assignee.id);
        if (entry) entry.cards.push(c);
        else people.set(c.assignee.id, { user: c.assignee, cards: [c] });
      }
      const out: Lane[] = [...people.values()]
        .sort((a, b) => a.user.name.localeCompare(b.user.name, formatLocale()))
        .map((p) => ({ id: `user-${p.user.id}`, kind: 'assignee', user: p.user, cards: p.cards }));
      if (nobody.length) out.push({ id: 'user-none', kind: 'assignee', user: null, cards: nobody });
      return out;
    }
    case 'epic': {
      const epics = new Map<string, { issue: IssueBrief; cards: BoardCard[] }>();
      const none: BoardCard[] = [];
      for (const c of cards) {
        if (!c.epic) {
          none.push(c);
          continue;
        }
        const entry = epics.get(c.epic.key);
        if (entry) entry.cards.push(c);
        else epics.set(c.epic.key, { issue: c.epic, cards: [c] });
      }
      const out: Lane[] = [...epics.values()].map((e) => ({
        id: `epic-${e.issue.key}`,
        kind: 'epic',
        issue: e.issue,
        cards: e.cards,
      }));
      if (none.length) out.push({ id: 'epic-none', kind: 'epic', issue: null, cards: none });
      return out;
    }
    case 'parent': {
      const parents = new Map<string, { issue: IssueBrief; cards: BoardCard[] }>();
      for (const c of cards) {
        if (!c.parent) continue;
        const entry = parents.get(c.parent.key);
        if (entry) entry.cards.push(c);
        else parents.set(c.parent.key, { issue: c.parent, cards: [c] });
      }
      const out: Lane[] = [...parents.values()].map((p) => ({
        id: `parent-${p.issue.key}`,
        kind: 'parent',
        issue: p.issue,
        cards: p.cards,
      }));
      const rest = cards.filter((c) => !c.parent && !parents.has(c.key));
      if (rest.length) out.push({ id: 'other', kind: 'other', cards: rest });
      return out;
    }
    default:
      return [{ id: 'all', kind: 'all', cards }];
  }
}

/**
 * Where a card dropped at height y lands among the cards of a list, given
 * the vertical middles of those cards (the dragged one left out): the
 * number of cards above the pointer.
 */
export function dropIndex(middles: number[], y: number): number {
  let i = 0;
  while (i < middles.length && (middles[i] ?? 0) < y) i++;
  return i;
}

/**
 * The neighbours a card gets when inserted at index among keys (the list it
 * lands in, which may still hold the card itself): the card just above
 * (after) and just below (before).
 */
export function neighbours(
  keys: string[],
  card: string,
  index: number,
): { after?: string; before?: string } {
  const rest = keys.filter((k) => k !== card);
  const i = Math.max(0, Math.min(index, rest.length));
  const out: { after?: string; before?: string } = {};
  const above = rest[i - 1];
  const below = rest[i];
  if (above !== undefined) out.after = above;
  if (below !== undefined) out.before = below;
  return out;
}

/** Whether inserting card at index (among the others) leaves it where it was. */
export function staysInPlace(keys: string[], card: string, index: number): boolean {
  const at = keys.indexOf(card);
  return at >= 0 && at === Math.max(0, Math.min(index, keys.length - 1));
}

/**
 * The move that shifts a card one place up (-1) or down (+1) among keys, or
 * null at either end.
 */
export function stepMove(keys: string[], card: string, step: -1 | 1): CardMove | null {
  const at = keys.indexOf(card);
  const target = at + step;
  if (at < 0 || target < 0 || target >= keys.length) return null;
  return { issue: card, ...neighbours(keys, card, target) };
}

/**
 * The cards after a move, for showing it before the server confirms: the
 * card takes its new column or sprint and goes right after its new upper
 * neighbour (or right before its lower one); without either it keeps its
 * place in the order.
 */
export function applyMove(cards: BoardCard[], move: CardMove): BoardCard[] {
  const at = cards.findIndex((c) => c.key === move.issue);
  const card = cards[at];
  if (!card) return cards;
  const moved: BoardCard = { ...card };
  if (move.column !== undefined) moved.column = move.column;
  if (move.sprint !== undefined) {
    if (move.sprint === 0) delete moved.sprint;
    else moved.sprint = move.sprint;
  }
  const rest = cards.filter((c) => c.key !== move.issue);
  let index = at;
  if (move.after !== undefined) {
    const i = rest.findIndex((c) => c.key === move.after);
    if (i >= 0) index = i + 1;
  } else if (move.before !== undefined) {
    const i = rest.findIndex((c) => c.key === move.before);
    if (i >= 0) index = i;
  }
  rest.splice(index, 0, moved);
  return rest;
}

export type WipState = 'ok' | 'over' | 'under';

/** A column's work-in-progress state: over its maximum, under its minimum, or fine. */
export function wipState(count: number, min: number | null, max: number | null): WipState {
  if (max !== null && count > max) return 'over';
  if (min !== null && count < min) return 'under';
  return 'ok';
}

/** The sum of the cards' estimates, or null when none has one. */
export function totalEstimate(cards: BoardCard[]): number | null {
  let total: number | null = null;
  for (const c of cards) {
    if (c.estimate !== null && c.estimate !== undefined) total = (total ?? 0) + c.estimate;
  }
  return total === null ? null : Math.round(total * 100) / 100;
}

/**
 * Calendar days from today to a sprint's last day: 0 on that day, negative
 * once it is past. Counted on the local calendar, so a clock change does not
 * add a day.
 */
export function daysLeft(end: string, now: Date = new Date()): number {
  const e = new Date(end);
  const a = Date.UTC(now.getFullYear(), now.getMonth(), now.getDate());
  const b = Date.UTC(e.getFullYear(), e.getMonth(), e.getDate());
  return Math.round((b - a) / (24 * 60 * 60 * 1000));
}

/**
 * The end a sprint gets when started for a given last day ("YYYY-MM-DD"):
 * that day, at the time it is now, as an ISO instant.
 */
export function sprintEnd(day: string, now: Date = new Date()): string | null {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(day);
  if (!m) return null;
  const end = new Date(
    Number(m[1]),
    Number(m[2]) - 1,
    Number(m[3]),
    now.getHours(),
    now.getMinutes(),
  );
  return Number.isNaN(end.getTime()) ? null : end.toISOString();
}

/** A local day ("YYYY-MM-DD") some days after now. */
export function dayAfter(days: number, now: Date = new Date()): string {
  const d = new Date(now.getFullYear(), now.getMonth(), now.getDate() + days);
  const mm = String(d.getMonth() + 1).padStart(2, '0');
  const dd = String(d.getDate()).padStart(2, '0');
  return `${d.getFullYear()}-${mm}-${dd}`;
}

/** Quick filter ids as kept in the address ("1,3"), cleaned up. */
export function parseQuick(param: string | null): number[] {
  if (!param) return [];
  const ids = param
    .split(',')
    .map((s) => Number(s.trim()))
    .filter((n) => Number.isInteger(n) && n > 0);
  return [...new Set(ids)];
}

/** Turns a quick filter on or off; the address form, '' when none is left. */
export function toggleQuick(selected: number[], id: number): string {
  const next = selected.includes(id) ? selected.filter((x) => x !== id) : [...selected, id];
  return next.join(',');
}

/**
 * Where an old board address leads: /secure/RapidBoard.jspa?rapidView=ID
 * opens the board (its backlog with view=planning…), RapidView.jspa its
 * settings; without a board id, the list of boards. Null for any other
 * address.
 */
export function legacyBoardPath(pathname: string, search: string): string | null {
  const page = pathname.replace(/\/+$/, '').toLowerCase();
  if (page !== '/secure/rapidboard.jspa' && page !== '/secure/rapidview.jspa') return null;
  const params = new URLSearchParams(search);
  const id = Number(params.get('rapidView'));
  if (!Number.isInteger(id) || id <= 0) return '/boards';
  if (page === '/secure/rapidview.jspa') return `/boards/${id}/settings`;
  const view = params.get('view') ?? '';
  return view.startsWith('planning') ? `/boards/${id}/backlog` : `/boards/${id}`;
}
