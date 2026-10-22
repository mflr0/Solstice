/**
 * The columns of the search results: the user picks and orders them among
 * the fields the CSV export knows (GET /api/issues/columns). The key and the
 * summary are always shown, so they are never part of the choice. The choice
 * is kept per browser in localStorage.
 */

/** A column the search can show. */
export interface ColumnChoice {
  id: string;
  name: string;
}

export const STORAGE_KEY = 'solstice.searchColumns';

/** Always shown, first, whatever the choice. */
export const FIXED_COLUMNS: readonly string[] = ['issuekey', 'summary'];

/** The columns the table showed before they could be chosen. */
export const DEFAULT_COLUMNS: readonly string[] = [
  'issuetype',
  'status',
  'priority',
  'assignee',
  'duedate',
  'updated',
];

/** Columns drawn from the issue itself; the others come from the server as text. */
export const NATIVE_COLUMNS: ReadonlySet<string> = new Set([
  'issuetype',
  'status',
  'priority',
  'resolution',
  'assignee',
  'reporter',
  'created',
  'updated',
  'duedate',
]);

/**
 * Reads a stored choice. Anything unreadable gives the default; fixed,
 * repeated and (when known is given) unknown or removed columns are dropped.
 */
export function parseColumns(raw: string | null, known?: readonly string[]): string[] {
  let list: unknown;
  try {
    list = raw === null ? null : JSON.parse(raw);
  } catch {
    list = null;
  }
  const ids = Array.isArray(list) ? list : DEFAULT_COLUMNS;
  const out: string[] = [];
  for (const id of ids) {
    if (typeof id !== 'string' || FIXED_COLUMNS.includes(id) || out.includes(id)) continue;
    if (known && !known.includes(id)) continue;
    out.push(id);
  }
  return out;
}

/** The chosen columns the server must send values for. */
export function serverColumns(chosen: readonly string[]): string[] {
  return chosen.filter((id) => !NATIVE_COLUMNS.has(id));
}

/** The columns of a CSV export of the search, in the table's order (the type is drawn before the key). */
export function exportColumns(chosen: readonly string[]): string[] {
  const type = chosen.includes('issuetype') ? ['issuetype'] : [];
  return [...type, ...FIXED_COLUMNS, ...chosen.filter((id) => id !== 'issuetype')];
}
