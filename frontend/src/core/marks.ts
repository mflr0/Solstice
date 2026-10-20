/**
 * The small typographic marks that stand for priorities, issue types and
 * people. Pure functions, so the rules are tested and the same everywhere.
 */

export type PriorityLevel = 'highest' | 'high' | 'medium' | 'low' | 'lowest';

export const PRIORITY_GLYPHS: Record<PriorityLevel, string> = {
  highest: '▲▲',
  high: '▲',
  medium: '■',
  low: '▼',
  lowest: '▼▼',
};

/**
 * Where a priority sits in the ordered list (most urgent first). Jira's five
 * default priorities map one-to-one; other lists keep the extremes and the
 * exact middle.
 */
export function priorityLevel(index: number, count: number): PriorityLevel {
  if (count <= 1 || index < 0) return 'medium';
  const mid = (count - 1) / 2;
  if (index === mid) return 'medium';
  if (count >= 4 && index === 0) return 'highest';
  if (count >= 4 && index === count - 1) return 'lowest';
  return index < mid ? 'high' : 'low';
}

export type TypeTone = 'bug' | 'story' | 'epic' | 'task' | 'subtask';

/** The family an issue type belongs to, read from its name. */
export function typeTone(name: string, subtask: boolean): TypeTone {
  if (subtask) return 'subtask';
  if (/bug|anomalie|défaut|defaut|incident/i.test(name)) return 'bug';
  if (/epic|épopée|epopee/i.test(name)) return 'epic';
  if (/story|récit|recit|histoire/i.test(name)) return 'story';
  return 'task';
}

/** The letter printed in an issue type's box. */
export function typeLetter(name: string): string {
  const first = /[\p{L}\p{N}]/u.exec(name);
  return first ? first[0].toLocaleUpperCase('fr') : '?';
}

/** "Marie Durand" → "MD"; "admin" → "AD". */
export function initials(name: string): string {
  const words = name.split(/[\s._-]+/).filter((w) => /[\p{L}\p{N}]/u.test(w));
  if (words.length === 0) return '?';
  if (words.length === 1) return (words[0] ?? '').slice(0, 2).toLocaleUpperCase('fr');
  const first = words[0] ?? '';
  const last = words[words.length - 1] ?? '';
  return (first.charAt(0) + last.charAt(0)).toLocaleUpperCase('fr');
}

/** The extension shown on a file's placeholder, e.g. "PDF". */
export function fileExtension(filename: string): string {
  const m = /\.([A-Za-z0-9]{1,5})$/.exec(filename);
  return m?.[1] ? m[1].toUpperCase() : '';
}

const INLINE_IMAGES = new Set(['image/png', 'image/jpeg', 'image/gif', 'image/webp']);

/** Whether the browser may show an attachment as a picture. */
export function isImage(mimeType: string): boolean {
  return INLINE_IMAGES.has(mimeType.split(';')[0]?.trim().toLowerCase() ?? '');
}

/**
 * An issue key as people type it: any case and a project of one character
 * or more ("demo-12", "a-1"), captured as project and number. Lenient on
 * purpose: input is read generously and the server says whether the issue
 * exists. Keys in displayed text use the strict KEY_IN_TEXT (wiki.ts).
 */
export const KEY_TYPED = /^([A-Za-z][A-Za-z0-9_]*)-(\d+)$/;

/** "DEMO-12" → "DEMO"; null when the text is not an issue key. */
export function projectOfKey(key: string): string | null {
  const m = KEY_TYPED.exec(key.trim());
  return m?.[1] ? m[1].toUpperCase() : null;
}
