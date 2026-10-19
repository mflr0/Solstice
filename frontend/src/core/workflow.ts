/**
 * Editing a workflow, without the interface: the draft being edited, the
 * transition matrix (rows "from", columns "to", one row for "from any
 * status"), change detection and the body sent to the server.
 */

import {
  STATUS_CATEGORIES,
  type StatusCategory,
  type StatusRef,
  type StrandedDefault,
  type StrandedStatus,
  type Workflow,
  type WorkflowInput,
} from './types.js';

/** A transition being edited. `key` tells rows apart; id is null until saved. */
export interface DraftTransition {
  key: string;
  id: number | null;
  name: string;
  /** null: from any status. */
  from: string | null;
  to: string;
}

export interface Draft {
  name: string;
  initial: string;
  statuses: StatusRef[];
  transitions: DraftTransition[];
  /** Ids of the issue types following the workflow (not used by the default one). */
  issueTypes: string[];
  /** Make it the project's default workflow. */
  makeDefault: boolean;
}

export function toDraft(w: Workflow): Draft {
  return {
    name: w.name,
    initial: w.initial.id,
    statuses: [...w.statuses],
    transitions: w.transitions.map((t) => ({
      key: `id:${t.id}`,
      id: t.id,
      name: t.name,
      from: t.from?.id ?? null,
      to: t.to.id,
    })),
    issueTypes: w.default ? [] : w.issueTypes.map((t) => t.id),
    makeDefault: false,
  };
}

/** A new transition is named after the status it leads to. */
export function defaultName(to: StatusRef): string {
  return to.name;
}

/** The name saved for a transition: its own, or its target's when left blank. */
export function effectiveName(t: DraftTransition, statuses: StatusRef[]): string {
  const name = t.name.trim();
  if (name) return name;
  const to = statuses.find((s) => s.id === t.to);
  return to ? defaultName(to) : t.to;
}

/** The transitions of one matrix cell (usually zero or one). */
export function cell(list: DraftTransition[], from: string | null, to: string): DraftTransition[] {
  return list.filter((t) => t.from === from && t.to === to);
}

export interface MatrixCell {
  to: StatusRef;
  transitions: DraftTransition[];
  /** A status to itself: no transition possible. */
  self: boolean;
}

export interface MatrixRow {
  /** null: the "from any status" row. */
  from: StatusRef | null;
  cells: MatrixCell[];
}

/** The transitions as a matrix: a row per status, then the "from any status" row. */
export function matrix(statuses: StatusRef[], list: DraftTransition[]): MatrixRow[] {
  return [...statuses, null].map((from) => ({
    from,
    cells: statuses.map((to) => ({
      to,
      transitions: cell(list, from?.id ?? null, to.id),
      self: from?.id === to.id,
    })),
  }));
}

/**
 * Checks or unchecks a matrix cell. Unchecking removes the cell's
 * transitions; checking brings back the ones the saved workflow had there
 * (same id, same name, same place in the list), or adds one named after
 * the target.
 */
export function toggleCell(
  list: DraftTransition[],
  from: string | null,
  to: StatusRef,
  saved: DraftTransition[],
): DraftTransition[] {
  if (from === to.id) return list;
  if (cell(list, from, to.id).length > 0) {
    return list.filter((t) => !(t.from === from && t.to === to.id));
  }
  const restored = cell(saved, from, to.id);
  if (restored.length === 0) {
    const key = `new:${from ?? '*'}>${to.id}`;
    return [...list, { key, id: null, name: defaultName(to), from, to: to.id }];
  }
  // Put them back where they were, so saving keeps the order of the buttons.
  const rank = (t: DraftTransition) => {
    const i = saved.findIndex((s) => s.key === t.key);
    return i < 0 ? Infinity : i;
  };
  const out = [...list];
  for (const t of restored) {
    const at = out.findIndex((x) => rank(x) > rank(t));
    out.splice(at < 0 ? out.length : at, 0, { ...t });
  }
  return out;
}

/** Renames one transition. */
export function rename(list: DraftTransition[], key: string, name: string): DraftTransition[] {
  return list.map((t) => (t.key === key ? { ...t, name } : t));
}

/** Adds a status at the end (once). */
export function addStatus(d: Draft, s: StatusRef): Draft {
  if (d.statuses.some((x) => x.id === s.id)) return d;
  return { ...d, statuses: [...d.statuses, s] };
}

/**
 * Removes a status, with the transitions to and from it. The start status
 * moves to the first one left.
 */
export function removeStatus(d: Draft, id: string): Draft {
  const statuses = d.statuses.filter((s) => s.id !== id);
  return {
    ...d,
    statuses,
    transitions: d.transitions.filter((t) => t.to !== id && t.from !== id),
    initial: d.initial === id ? (statuses[0]?.id ?? '') : d.initial,
  };
}

/** Moves the status at index `from` to index `to`. */
export function moveStatus(list: StatusRef[], from: number, to: number): StatusRef[] {
  if (from === to || from < 0 || from >= list.length || to < 0 || to >= list.length) return list;
  const out = [...list];
  const [item] = out.splice(from, 1);
  if (item) out.splice(to, 0, item);
  return out;
}

/**
 * What the draft would save, as a comparable string. Transition order is
 * left out: the editor does not show it, and checking a cell back restores
 * it anyway.
 */
export function signature(d: Draft, isDefault: boolean): string {
  const transitions = d.transitions
    .map((t) => [t.id ?? '', t.from ?? '*', t.to, effectiveName(t, d.statuses)].join('\u0001'))
    .sort();
  return JSON.stringify({
    name: d.name.trim(),
    initial: d.initial,
    statuses: d.statuses.map((s) => s.id),
    transitions,
    issueTypes: isDefault || d.makeDefault ? [] : [...d.issueTypes].sort(),
    makeDefault: !isDefault && d.makeDefault,
  });
}

export function isDirty(d: Draft, saved: Draft, isDefault: boolean): boolean {
  return signature(d, isDefault) !== signature(saved, isDefault);
}

/** The body of PUT /api/workflows/{id}. */
export function toInput(
  d: Draft,
  isDefault: boolean,
  remap?: Record<string, string>,
  remapDefault?: Record<string, string>,
): WorkflowInput {
  const input: WorkflowInput = {
    name: d.name.trim(),
    initial: d.initial,
    statuses: d.statuses.map((s) => s.id),
    transitions: d.transitions.map((t) => ({
      ...(t.id !== null ? { id: t.id } : {}),
      name: effectiveName(t, d.statuses),
      from: t.from,
      to: t.to,
    })),
  };
  if (!isDefault && d.makeDefault) input.default = true;
  else if (!isDefault) input.issueTypes = [...d.issueTypes];
  if (remap && Object.keys(remap).length > 0) input.remap = { ...remap };
  if (remapDefault && Object.keys(remapDefault).length > 0)
    input.remapDefault = { ...remapDefault };
  return input;
}

/**
 * Where stranded issues go unless the user says otherwise: the first status
 * of the same category, else the start status.
 */
export function suggestRemap(
  stranded: StrandedStatus[],
  statuses: StatusRef[],
  initial: string,
): Record<string, string> {
  const out: Record<string, string> = {};
  for (const s of stranded) {
    const same = statuses.find((x) => x.category === s.status.category);
    const target = same?.id ?? initial;
    if (target) out[s.status.id] = target;
  }
  return out;
}

/** Reads the stranded statuses out of a 409 body. */
export function strandedOf(data: unknown): StrandedStatus[] | null {
  if (!data || typeof data !== 'object' || !('stranded' in data)) return null;
  const list = data.stranded;
  return Array.isArray(list) ? (list as StrandedStatus[]) : null;
}

/**
 * Reads, out of a 409 body, the issues of types going back to the default
 * workflow that sit in statuses it lacks (null when there are none).
 */
export function strandedDefaultOf(data: unknown): StrandedDefault | null {
  if (!data || typeof data !== 'object') return null;
  const d = data as { strandedDefault?: unknown; defaultWorkflow?: unknown };
  if (!Array.isArray(d.strandedDefault) || d.strandedDefault.length === 0) return null;
  if (!d.defaultWorkflow || typeof d.defaultWorkflow !== 'object') return null;
  return {
    stranded: d.strandedDefault as StrandedStatus[],
    workflow: d.defaultWorkflow as StrandedDefault['workflow'],
  };
}

/** Statuses grouped by category, each group in the workflow's order. */
export function byCategory(
  statuses: StatusRef[],
): { category: StatusCategory; statuses: StatusRef[] }[] {
  return STATUS_CATEGORIES.map((category) => ({
    category,
    statuses: statuses.filter((s) => s.category === category),
  })).filter((g) => g.statuses.length > 0);
}

/** Catalogue statuses matching a search, those already in the workflow left out. */
export function searchStatuses(catalogue: StatusRef[], taken: StatusRef[], q: string): StatusRef[] {
  const needle = fold(q.trim());
  return catalogue.filter(
    (s) => !taken.some((x) => x.id === s.id) && (!needle || fold(s.name).includes(needle)),
  );
}

/** The catalogue's status of that name, case aside (the server matches the same way). */
export function statusNamed(catalogue: StatusRef[], name: string): StatusRef | undefined {
  const n = name.trim().toLowerCase();
  return catalogue.find((s) => s.name.toLowerCase() === n);
}

function fold(s: string): string {
  // Only Latin accents go: other scripts' marks are letters (Devanagari vowel signs).
  return s
    .normalize('NFD')
    .replace(/[\u0300-\u036f]/g, '')
    .toLowerCase();
}
