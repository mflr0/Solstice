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
