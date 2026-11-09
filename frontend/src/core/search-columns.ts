/**
 * The columns of the search results: the user picks and orders them among
 * the fields the CSV export knows (GET /api/issues/columns). The key and the
 * summary are always shown, so they are never part of the choice. An open
 * saved filter can carry its own choice; otherwise it is kept per browser in
 * localStorage.
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
  'resolutiondate',
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

/**
 * The columns a view shows: its own (a saved filter's) when it has some,
 * else the stored choice. Both are cleaned like a stored choice.
 */
export function viewColumns(
  own: readonly string[] | null | undefined,
  stored: string | null,
  known?: readonly string[],
): string[] {
  return parseColumns(own ? JSON.stringify(own) : stored, known);
}

/** The stored choice, null when none or when storage is blocked. */
export function readStoredColumns(): string | null {
  try {
    return localStorage.getItem(STORAGE_KEY);
  } catch {
    return null;
  }
}

/** Keeps the choice for this browser; in private browsing it lasts for the page only. */
export function storeColumns(next: readonly string[]): void {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(next));
  } catch {
    // Storage blocked: nothing to keep it in.
  }
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
