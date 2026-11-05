/** Where focus goes when the control that holds it leaves the page. */

/**
 * The row whose control takes focus once `removed` is gone from `rows`, the
 * rows that have that control, in display order: the next one, else the
 * previous one, else none (the section's add control takes it).
 */
export function focusAfterRemoval<T>(rows: readonly T[], removed: T): T | undefined {
  const at = rows.indexOf(removed);
  if (at < 0) return undefined;
  return at + 1 < rows.length ? rows[at + 1] : rows[at - 1];
}

/**
 * The ids to try, in turn, for focus once `removed` is gone from `rows`, in
 * display order: the same action on the rows after it, nearest first, then on
 * those before it, nearest first, then `fallback`. A list, as a row may not
 * have the action.
 */
export function focusOrderAfterRemoval<T>(
  rows: readonly T[],
  removed: T,
  idOf: (row: T) => string,
  fallback: string,
): string[] {
  const id = idOf(removed);
  const at = rows.findIndex((row) => idOf(row) === id);
  const others = at < 0 ? [] : [...rows.slice(at + 1), ...rows.slice(0, at).reverse()];
  return [...others.map(idOf), fallback];
}
