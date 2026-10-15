/**
 * Release notes, as written on GitHub in Markdown, turned into plain blocks:
 * headings, paragraphs, lists and code. The interface renders them as text
 * (never as HTML), so links become their text and markup marks go.
 */

export type NoteBlock =
  | { kind: 'heading'; text: string }
  | { kind: 'paragraph'; text: string }
  | { kind: 'list'; ordered: boolean; items: string[] }
  | { kind: 'code'; text: string };

/** Inline Markdown to plain text: links keep their text, emphasis its words. */
export function plainInline(s: string): string {
  // Backslash escapes are set aside (as private-use characters), so the
  // marks they protect are not read as markup, then put back as is.
  const PRIVATE = 0xe000;
  return (
    s
      .replace(/\\([\\`*_{}[\]()#+\-.!<>~|])/g, (_, c: string) =>
        String.fromCharCode(PRIVATE + c.charCodeAt(0)),
      )
      // Images, then links: their text.
      .replace(/!\[([^\]]*)\]\([^)]*\)/g, '$1')
      .replace(/\[([^\]]+)\]\([^)]*\)/g, '$1')
      // Autolinks: the address.
      .replace(/<((?:https?|mailto):[^>\s]+)>/g, '$1')
      // HTML tags: gone (their text stays).
      .replace(/<\/?[A-Za-z][^<>]*>/g, '')
      // Code spans, bold, italics, strike-through: the words.
      .replace(/`([^`]+)`/g, '$1')
      .replace(/(\*\*|__)(?=\S)(.+?)(?<=\S)\1/g, '$2')
      .replace(/(^|[^\w*])\*(?=\S)([^*]+?)(?<=\S)\*(?!\w)/g, '$1$2')
      .replace(/(^|[^\w])_(?=\S)([^_]+?)(?<=\S)_(?!\w)/g, '$1$2')
      .replace(/~~(?=\S)(.+?)(?<=\S)~~/g, '$1')
      .replace(/[-]/g, (c) => String.fromCharCode(c.charCodeAt(0) - PRIVATE))
      .replace(/\s+/g, ' ')
      .trim()
  );
}

const bullet = /^\s*[-*+]\s+(.*)$/;
const numbered = /^\s*\d{1,9}[.)]\s+(.*)$/;
const heading = /^\s{0,3}#{1,6}\s+(.*?)\s*#*\s*$/;
const fence = /^\s{0,3}(```|~~~)/;
const rule = /^\s{0,3}([-*_])(\s*\1){2,}\s*$/;

export function parseNotes(markdown: string): NoteBlock[] {
  const blocks: NoteBlock[] = [];
  const lines = markdown.replace(/\r\n?/g, '\n').split('\n');
  let para: string[] = [];
  let list: { ordered: boolean; items: string[] } | null = null;

  const flushParagraph = () => {
    if (para.length) {
      const text = plainInline(para.join(' '));
      if (text) blocks.push({ kind: 'paragraph', text });
      para = [];
    }
  };
  const flush = () => {
    flushParagraph();
    if (list) {
      const items = list.items.map(plainInline).filter((i) => i !== '');
      if (items.length) blocks.push({ kind: 'list', ordered: list.ordered, items });
      list = null;
    }
  };

  for (let i = 0; i < lines.length; i++) {
    const line = lines[i] ?? '';
    const open = fence.exec(line);
    if (open) {
      flush();
      const code: string[] = [];
      for (
        i++;
        i < lines.length && !(lines[i] ?? '').trimStart().startsWith(open[1] ?? '```');
        i++
      ) {
        code.push(lines[i] ?? '');
      }
      if (code.length) blocks.push({ kind: 'code', text: code.join('\n') });
      continue;
    }
    if (line.trim() === '') {
      // A list may go on after a blank line; a paragraph ends.
      flushParagraph();
      continue;
    }
    if (rule.test(line)) {
      flush();
      continue;
    }
    const h = heading.exec(line);
    if (h) {
      flush();
      const text = plainInline(h[1] ?? '');
      if (text) blocks.push({ kind: 'heading', text });
      continue;
    }
    const b = bullet.exec(line);
    const n = b ? null : numbered.exec(line);
    if (b || n) {
      const ordered = !!n;
      if (!list || list.ordered !== ordered) {
        flush();
        list = { ordered, items: [] };
      }
      list.items.push((b ?? n)?.[1] ?? '');
      continue;
    }
    const text = line.replace(/^\s{0,3}>\s?/, '');
    if (list && /^\s/.test(line)) {
      // An indented line continues the last item.
      list.items[list.items.length - 1] += ' ' + text.trim();
      continue;
    }
    if (list) flush();
    para.push(text.trim());
  }
  flush();
  return blocks;
}
