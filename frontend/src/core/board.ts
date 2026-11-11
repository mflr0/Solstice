/**
 * Board rules that do not touch the page: grouping cards into sections,
 * where a dropped card lands, sprint arithmetic, card filters,
 * old board addresses. Pure functions, tested in tests/core/board.test.ts.
 */

import type {
  BoardCard,
  CardMove,
  Sprint,
  SprintPatch,
  BoardSection,
  SectionStrategy,
} from './board-types.js';
import type { Facets, IssueBrief, UserRef } from './types.js';
import {
  emptyFilters,
  filtersToJql,
  jqlToFilters,
  QUICK_FIELDS,
  withoutField,
  type FilterState,
} from './filters.js';
import { isoDay, parseDay } from './dates.js';
import { formatLocale } from '../i18n/format.js';

/** A horizontal band of the board. Wording is left to the interface. */
export type Section =
  | { id: string; kind: 'all'; cards: BoardCard[] }
  | { id: string; kind: 'custom'; name: string; cards: BoardCard[] }
  | { id: string; kind: 'assignee'; user: UserRef | null; cards: BoardCard[] }
  | { id: string; kind: 'epic'; issue: IssueBrief | null; cards: BoardCard[] }
  | { id: string; kind: 'parent'; issue: IssueBrief; cards: BoardCard[] }
  /** Cards no other section takes. */
  | { id: string; kind: 'other'; cards: BoardCard[] };

/**
 * Splits cards (in rank order) into sections. Custom sections follow the
 * board's order (the server tells each card its section); people are sorted by
 * name, epics and parents come in the order of their first card; the section
 * for "nobody", "no epic" or "everything else" is last. With parent sections, a
 * parent heads its section rather than appearing as a card.
 */
export function groupSections(
  cards: BoardCard[],
  strategy: SectionStrategy,
  sections: BoardSection[],
): Section[] {
  switch (strategy) {
    case 'custom': {
      const known = new Set(sections.map((l) => l.id));
      const out: Section[] = sections.map((l) => ({
        id: `section-${l.id}`,
        kind: 'custom',
        name: l.name,
        cards: cards.filter((c) => c.section === l.id),
      }));
      const rest = cards.filter((c) => c.section === undefined || !known.has(c.section));
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
      const out: Section[] = [...people.values()]
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
      const out: Section[] = [...epics.values()].map((e) => ({
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
      const out: Section[] = [...parents.values()].map((p) => ({
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
 * How far to scroll, per frame, while a dragged card is held at pos between
 * the edges start and end of a scroller: more the deeper into the zone along
 * an edge, negative towards start, 0 away from both.
 */
export function edgeScroll(pos: number, start: number, end: number, zone = 48, max = 14): number {
  const depth = (d: number) => Math.min(1, Math.max(0, (zone - d) / zone));
  return Math.round(max * (depth(end - pos) - depth(pos - start)));
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
  const d = parseDay(day);
  if (!d) return null;
  const end = new Date(d.y, d.m - 1, d.d, now.getHours(), now.getMinutes());
  return Number.isNaN(end.getTime()) ? null : end.toISOString();
}

/** A local day ("YYYY-MM-DD") some days after now. */
export function dayAfter(days: number, now: Date = new Date()): string {
  return isoDay(new Date(now.getFullYear(), now.getMonth(), now.getDate() + days));
}

/** The local day ("YYYY-MM-DD") of an instant; empty for none. */
export function localDay(iso: string | null): string {
  if (!iso) return '';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return '';
  return isoDay(d);
}

/**
 * The instant of a local day, at the time of day an earlier instant had, or
 * at hour:minute when there was none. Null for a day that is not one.
 */
export function atDay(day: string, base: string | null, hour: number, minute = 0): string | null {
  const d = parseDay(day);
  if (!d) return null;
  const was = base ? new Date(base) : null;
  const known = was && !Number.isNaN(was.getTime());
  const at = new Date(
    d.y,
    d.m - 1,
    d.d,
    known ? was.getHours() : hour,
    known ? was.getMinutes() : minute,
  );
  return Number.isNaN(at.getTime()) ? null : at.toISOString();
}

/** A sprint being edited, as typed. Days are "YYYY-MM-DD", empty for none. */
export interface SprintForm {
  name: string;
  goal: string;
  start: string;
  end: string;
}

/** Why a sprint cannot be saved as typed. */
export type SprintFormError = 'name' | 'start' | 'end' | 'order' | 'startFuture';

export type SprintEdit = { ok: true; patch: SprintPatch } | { ok: false; error: SprintFormError };

/**
 * What editing a sprint changes, with the server's rules: it needs a name;
 * its end follows its start; one that has begun keeps both dates, and began
 * no later than today. Dates left as they were keep their exact time.
 */
export function sprintEdit(sprint: Sprint, form: SprintForm, now: Date = new Date()): SprintEdit {
  const name = form.name.trim();
  if (!name) return { ok: false, error: 'name' };
  const active = sprint.state === 'active';
  if (active && !form.start) return { ok: false, error: 'start' };
  if (active && !form.end) return { ok: false, error: 'end' };
  if (active && form.start > dayAfter(0, now)) return { ok: false, error: 'startFuture' };

  const patch: SprintPatch = {};
  if (name !== sprint.name) patch.name = name;
  const goal = form.goal.trim();
  if (goal !== (sprint.goal ?? '').trim()) patch.goal = goal || null;

  let start = sprint.start;
  let end = sprint.end;
  if (form.start !== localDay(sprint.start)) {
    start = form.start ? atDay(form.start, sprint.start, 9) : null;
    patch.start = start;
  }
  if (form.end !== localDay(sprint.end)) {
    end = form.end ? atDay(form.end, sprint.end, 17) : null;
    patch.end = end;
  }
  if (start && end && new Date(end).getTime() <= new Date(start).getTime()) {
    return { ok: false, error: 'order' };
  }
  return { ok: true, patch };
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

/** The filter row's search kept in a board's address (?jql=). */
export interface BoardSearch {
  /** What the row shows; empty for a query it can't show. */
  filters: FilterState;
  /** The query to send; undefined for none, or for one the row can't show. */
  jql: string | undefined;
  /** The address holds a query the row can't show: the whole board is shown. */
  refused: boolean;
  /** The query without the quick filter row's fields, whose cards give the row its values. */
  rowJql: string | undefined;
}

export function readBoardSearch(param: string | null): BoardSearch {
  const jql = param?.trim();
  if (!jql) return { filters: emptyFilters(), jql: undefined, refused: false, rowJql: undefined };
  const r = jqlToFilters(jql);
  if (!r.ok) return { filters: emptyFilters(), jql: undefined, refused: true, rowJql: undefined };
  const rest = QUICK_FIELDS.reduce((s, f) => withoutField(s, f), r.state);
  return { filters: r.state, jql, refused: false, rowJql: filtersToJql(rest) || undefined };
}

/**
 * The quick filter row's counts, from a board's cards: assignees, statuses,
 * types and priorities by id, epics by key ("" counts the cards without one).
 */
export function cardFacets(cards: BoardCard[]): Facets {
  const assignee: Record<string, number> = {};
  const type: Record<string, number> = {};
  const status: Record<string, number> = {};
  const priority: Record<string, number> = {};
  const epic: Record<string, number> = {};
  const users: Facets['users'] = {};
  const epics: Facets['epics'] = {};
  const add = (counts: Record<string, number>, key: string) => {
    counts[key] = (counts[key] ?? 0) + 1;
  };
  for (const c of cards) {
    add(assignee, c.assignee ? String(c.assignee.id) : '');
    if (c.assignee) users[c.assignee.id] = c.assignee;
    add(type, c.type.id);
    add(status, c.status.id);
    add(priority, c.priority?.id ?? '');
    add(epic, c.epic?.key ?? '');
    if (c.epic) epics[c.epic.key] = { key: c.epic.key, summary: c.epic.summary };
  }
  return { total: cards.length, facets: { assignee, type, status, priority, epic }, users, epics };
}

/** The options counted on a board's cards: the values it has. */
export function presentOptions<T extends { facets?: string[] | undefined }>(
  options: T[],
  counts: Record<string, number>,
): T[] {
  return options.filter((o) => o.facets?.some((k) => (counts[k] ?? 0) > 0));
}

/** Turns a quick filter on or off; the address form, '' when none is left. */
export function toggleQuick(selected: number[], id: number): string {
  const next = selected.includes(id) ? selected.filter((x) => x !== id) : [...selected, id];
  return next.join(',');
}

/** The board filters of an address's query ("?quick=1&jql=…"), for a link to another view of the board. */
export function boardFilterQuery(search: string): string {
  const from = new URLSearchParams(search);
  const out = new URLSearchParams();
  for (const k of ['quick', 'jql']) {
    const v = from.get(k);
    if (v) out.set(k, v);
  }
  const s = out.toString();
  return s ? `?${s}` : '';
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

/** A card's place on the board: its section, its column and its rank in that cell. */
export interface CardSpot {
  section: number;
  column: number;
  index: number;
}

/**
 * Where a board key moves the focus. counts[section][column] is the number of
 * cards in each cell (0 for a collapsed section). next and previous go down and
 * up the column, on into the same column of the next or previous section; left
 * and right go to the nearest column of the section that has cards, at the same
 * rank or its last card. At the edges the focus stays. From nowhere, any key
 * goes to the board's first card. Null on a board without cards.
 */
export function boardStep(
  counts: readonly (readonly number[])[],
  at: CardSpot | null,
  dir: 'next' | 'previous' | 'left' | 'right',
): CardSpot | null {
  const count = (section: number, column: number) => counts[section]?.[column] ?? 0;
  if (!at) {
    for (let section = 0; section < counts.length; section++)
      for (let column = 0; column < (counts[section]?.length ?? 0); column++)
        if (count(section, column) > 0) return { section, column, index: 0 };
    return null;
  }
  const { section, column, index } = at;
  if (dir === 'next' || dir === 'previous') {
    const step = dir === 'next' ? 1 : -1;
    if (index + step >= 0 && index + step < count(section, column))
      return { section, column, index: index + step };
    for (let l = section + step; l >= 0 && l < counts.length; l += step) {
      const n = count(l, column);
      if (n > 0) return { section: l, column, index: step > 0 ? 0 : n - 1 };
    }
    return at;
  }
  const step = dir === 'right' ? 1 : -1;
  for (let c = column + step; c >= 0 && c < (counts[section]?.length ?? 0); c += step) {
    const n = count(section, c);
    if (n > 0) return { section, column: c, index: Math.min(index, n - 1) };
  }
  return at;
}
