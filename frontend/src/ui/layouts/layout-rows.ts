import type { LayoutField, LayoutForm, LayoutZone } from '../../core/admin-types.js';
import { moveItem } from '../../core/catalogs.js';

/** A field an editor can place: its id and how the editor names it. */
export interface LayoutChoice {
  id: string;
  name: string;
  /** Said under the name, e.g. that the field only exists in some projects. */
  note?: string | undefined;
}

/** Fields every create form shows first, in this order (backend/internal/layout). */
export const FIXED_FIELDS = ['project', 'issuetype', 'summary'] as const;

const SYSTEM_FIELDS = [
  'description',
  'environment',
  'priority',
  'assignee',
  'reporter',
  'components',
  'fixVersions',
  'versions',
  'labels',
  'duedate',
  'timetracking',
  'security',
] as const;

const VIEW_ONLY = ['created', 'updated', 'resolutiondate', 'epic', 'sprint'] as const;

const MAIN_BY_DEFAULT = ['description', 'environment'];

/** The system fields a form places, as the server accepts them. */
export function systemFields(form: LayoutForm): string[] {
  return form === 'view' ? [...SYSTEM_FIELDS, ...VIEW_ONLY] : [...SYSTEM_FIELDS];
}

export const isCustom = (id: string) => id.startsWith('customfield_');

/**
 * The rows an editor starts from: the template's fields it knows, then the
 * custom fields the template doesn't place (shown, side zone in the view),
 * then the system fields it misses (hidden). Same rules as the server's
 * resolution, so the editor shows what the forms will.
 */
export function layoutRows(
  form: LayoutForm,
  template: readonly LayoutField[],
  choices: readonly LayoutChoice[],
): LayoutField[] {
  const known = new Set(choices.map((c) => c.id));
  const out: LayoutField[] = [];
  for (const f of template) {
    if (!known.has(f.field) || out.some((r) => r.field === f.field)) continue;
    out.push(normalise(form, f));
  }
  const placed = new Set(out.map((r) => r.field));
  const missing = choices.filter((c) => !placed.has(c.id));
  for (const c of missing.filter((c) => isCustom(c.id))) {
    out.push(normalise(form, { field: c.id, hidden: false, zone: 'side' }));
  }
  for (const c of missing.filter((c) => !isCustom(c.id))) {
    const zone: LayoutZone = MAIN_BY_DEFAULT.includes(c.id) ? 'main' : 'side';
    out.push(normalise(form, { field: c.id, hidden: true, zone }));
  }
  return out;
}

/** A field as the form's template holds it: a zone in the view only, required in the create form only. */
function normalise(form: LayoutForm, f: LayoutField): LayoutField {
  if (form === 'view') return { field: f.field, zone: f.zone ?? 'side', hidden: f.hidden };
  return { field: f.field, hidden: f.hidden, required: !f.hidden && !!f.required };
}

/** The rows of a zone (the view) or all of them (the create form). */
export function zoneRows(rows: readonly LayoutField[], zone?: LayoutZone): LayoutField[] {
  return zone ? rows.filter((r) => r.zone === zone) : [...rows];
}

/** Moves a row one step among the rows of its zone. */
export function stepRow(rows: readonly LayoutField[], field: string, by: -1 | 1): LayoutField[] {
  const row = rows.find((r) => r.field === field);
  if (!row) return [...rows];
  const peers = zoneRows(rows, row.zone);
  const next = peers[peers.indexOf(row) + by];
  if (!next) return [...rows];
  return moveItem(rows, rows.indexOf(row), rows.indexOf(next));
}

/** Puts a row at the end of another zone. */
export function setZone(
  rows: readonly LayoutField[],
  field: string,
  zone: LayoutZone,
): LayoutField[] {
  const row = rows.find((r) => r.field === field);
  if (!row || row.zone === zone) return [...rows];
  return [...rows.filter((r) => r !== row), { ...row, zone }];
}

/** Where a dragged row lands: on another row (it takes its place), or at the end of a zone. */
export interface DropTarget {
  field?: string | undefined;
  zone?: LayoutZone | undefined;
}

/** Drops a row on a target, as the mouse and the finger do. */
export function dropRow(
  rows: readonly LayoutField[],
  field: string,
  target: DropTarget,
): LayoutField[] {
  const row = rows.find((r) => r.field === field);
  if (!row || target.field === field) return [...rows];
  const on = rows.find((r) => r.field === target.field);
  const zone = on?.zone ?? target.zone ?? row.zone;
  if (!on) return zone && zone !== row.zone ? setZone(rows, field, zone) : [...rows];
  if (on.zone === row.zone) return moveItem(rows, rows.indexOf(row), rows.indexOf(on));
  const rest = rows.filter((r) => r !== row);
  rest.splice(rest.indexOf(on), 0, zone ? { ...row, zone } : row);
  return rest;
}

/** Shows or hides a row; a hidden field can't be required. */
export function setShown(rows: readonly LayoutField[], field: string, shown: boolean) {
  return rows.map((r) =>
    r.field !== field
      ? r
      : {
          ...r,
          hidden: !shown,
          ...(r.required !== undefined && !shown ? { required: false } : {}),
        },
  );
}

/** Makes a row required or not (the create form). */
export function setRequired(rows: readonly LayoutField[], field: string, required: boolean) {
  return rows.map((r) => (r.field === field && !r.hidden ? { ...r, required } : r));
}
