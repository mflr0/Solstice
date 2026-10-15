/**
 * Editing commands and selection queries for the rich editor's toolbar,
 * shortcuts and dialogs.
 */

import { lift, setBlockType, toggleMark, wrapIn } from 'prosemirror-commands';
import type { Mark, MarkType, Node as PMNode, ResolvedPos } from 'prosemirror-model';
import { liftListItem, wrapInList } from 'prosemirror-schema-list';
import { TextSelection, type Command, type EditorState } from 'prosemirror-state';
import { isInTable } from 'prosemirror-tables';
import type { EditorView } from 'prosemirror-view';
import { isSafeUrl, ISSUE_KEY_RE } from '../../core/wiki.js';
import { codeLanguage, schema, withCodeLanguage } from '../../core/wiki-doc.js';

const N = schema.nodes;
const M = schema.marks;

export type MarkName = 'strong' | 'em' | 'underline' | 'strike' | 'code';

export const toggle = (name: MarkName): Command => toggleMark(M[name]);

/** What the toolbar shows as pressed or available. */
export interface Situation {
  marks: Record<MarkName, boolean>;
  /** 0 for a paragraph, 1–6 for a heading, -1 for anything else. */
  heading: number;
  list: 'bullet' | 'ordered' | null;
  quote: boolean;
  code: boolean;
  language: string;
  table: boolean;
  link: boolean;
}

function markActive(state: EditorState, type: MarkType): boolean {
  const { from, $from, to, empty } = state.selection;
  if (empty) return !!type.isInSet(state.storedMarks ?? $from.marks());
  return state.doc.rangeHasMark(from, to, type);
}

function ancestor(state: EditorState, test: (node: PMNode) => boolean): PMNode | null {
  const { $from } = state.selection;
  for (let d = $from.depth; d > 0; d--) {
    const node = $from.node(d);
    if (test(node)) return node;
  }
  return null;
}

export function situation(state: EditorState): Situation {
  const parent = state.selection.$from.parent;
  const list = ancestor(state, (n) => n.type === N.bullet_list || n.type === N.ordered_list);
  return {
    marks: {
      strong: markActive(state, M.strong),
      em: markActive(state, M.em),
      underline: markActive(state, M.underline),
      strike: markActive(state, M.strike),
      code: markActive(state, M.code),
    },
    heading:
      parent.type === N.heading ? Number(parent.attrs.level) : parent.type === N.paragraph ? 0 : -1,
    list: list ? (list.type === N.ordered_list ? 'ordered' : 'bullet') : null,
    quote: ancestor(state, (n) => n.type === N.blockquote) !== null,
    code: parent.type === N.code_block,
    language:
      parent.type === N.code_block ? codeLanguage(parent.attrs.params as string | null) : '',
    table: isInTable(state),
    link: markActive(state, M.link),
  };
}

/** Paragraph (0) or heading level. */
export function setHeading(level: number): Command {
  return level === 0 ? setBlockType(N.paragraph) : setBlockType(N.heading, { level });
}

/** Bullet or numbered list: on, off, or switched to the other kind. */
export function toggleList(kind: 'bullet' | 'ordered'): Command {
  const type = kind === 'bullet' ? N.bullet_list : N.ordered_list;
  return (state, dispatch) => {
    const { $from } = state.selection;
    for (let d = $from.depth; d > 0; d--) {
      const node = $from.node(d);
      if (node.type !== N.bullet_list && node.type !== N.ordered_list) continue;
      if (node.type === type) return liftListItem(N.list_item)(state, dispatch);
      if (dispatch) dispatch(state.tr.setNodeMarkup($from.before(d), type).scrollIntoView());
      return true;
    }
    return wrapInList(type)(state, dispatch);
  };
}

export const toggleQuote: Command = (state, dispatch) =>
  ancestor(state, (n) => n.type === N.blockquote)
    ? lift(state, dispatch)
    : wrapIn(N.blockquote, { form: 'macro', params: null })(state, dispatch);

export const toggleCodeBlock: Command = (state, dispatch) =>
  state.selection.$from.parent.type === N.code_block
    ? setBlockType(N.paragraph)(state, dispatch)
    : setBlockType(N.code_block, { macro: 'code', params: null })(state, dispatch);

export function setLanguage(language: string): Command {
  return (state, dispatch) => {
    const { $from } = state.selection;
    if ($from.parent.type !== N.code_block) return false;
    if (dispatch) {
      const params = withCodeLanguage($from.parent.attrs.params as string | null, language);
      dispatch(state.tr.setNodeMarkup($from.before(), null, { ...$from.parent.attrs, params }));
    }
    return true;
  };
}

/** Where a new block goes: in place of an empty paragraph, else after the current top block. */
function blockSpot(state: EditorState): { from: number; to: number } {
  const { $from } = state.selection;
  if ($from.depth === 0) return { from: $from.pos, to: $from.pos };
  const top = $from.node(1);
  const start = $from.before(1);
  if (top.type === N.paragraph && top.content.size === 0) {
    return { from: start, to: start + top.nodeSize };
  }
  return { from: start + top.nodeSize, to: start + top.nodeSize };
}

export const insertTable: Command = (state, dispatch) => {
  if (isInTable(state)) return false;
  if (dispatch) {
    const cell = (header: boolean): PMNode =>
      (header ? N.table_header : N.table_cell).createAndFill() as PMNode;
    const row = (header: boolean): PMNode =>
      N.table_row.create(null, [cell(header), cell(header), cell(header)]);
    const table = N.table.create(null, [row(true), row(false), row(false)]);
    const { from, to } = blockSpot(state);
    const tr = state.tr.replaceWith(from, to, table);
    // Inside the first cell: table > row > cell > paragraph.
    tr.setSelection(TextSelection.create(tr.doc, from + 4));
    dispatch(tr.scrollIntoView());
  }
  return true;
};

export const insertRule: Command = (state, dispatch) => {
  if (state.selection.$from.parent.type === N.code_block || isInTable(state)) return false;
  if (dispatch) {
    const { from, to } = blockSpot(state);
    const tr = state.tr.replaceWith(from, to, [N.horizontal_rule.create(), N.paragraph.create()]);
    tr.setSelection(TextSelection.create(tr.doc, from + 2));
    dispatch(tr.scrollIntoView());
  }
  return true;
};

/** A hard line break; a plain new line inside code. */
export const lineBreak: Command = (state, dispatch) => {
  if (state.selection.$from.parent.type === N.code_block) {
    if (dispatch) dispatch(state.tr.insertText('\n').scrollIntoView());
    return true;
  }
  if (dispatch) {
    dispatch(
      state.tr.replaceSelectionWith(N.hard_break.create({ form: 'newline' })).scrollIntoView(),
    );
  }
  return true;
};

/* ---------- links ---------- */

/** The extent of a mark around a position, within its text block. */
function markRange($pos: ResolvedPos, mark: Mark): { from: number; to: number } | null {
  const start = $pos.start();
  const pos = $pos.pos;
  let from = -1;
  let to = -1;
  let found: { from: number; to: number } | null = null;
  const close = (): void => {
    if (!found && from >= 0 && from <= pos && pos <= to) found = { from, to };
    from = -1;
  };
  $pos.parent.forEach((child, offset) => {
    if (mark.isInSet(child.marks)) {
      if (from < 0) from = start + offset;
      to = start + offset + child.nodeSize;
    } else {
      close();
    }
  });
  close();
  return found;
}

export interface LinkTarget {
  from: number;
  to: number;
  text: string;
  href: string;
}

/** The link under the selection, or the selected text with no link yet. */
export function linkTarget(state: EditorState): LinkTarget {
  const { from, to, $from } = state.selection;
  const mark =
    M.link.isInSet($from.marks()) ??
    M.link.isInSet($from.nodeAfter?.marks ?? []) ??
    M.link.isInSet($from.nodeBefore?.marks ?? []);
  if (mark) {
    const range = markRange($from, mark);
    if (range && range.from <= from && to <= range.to) {
      return {
        ...range,
        text: state.doc.textBetween(range.from, range.to, ' ', ' '),
        href: mark.attrs.href as string,
      };
    }
  }
  return { from, to, text: state.doc.textBetween(from, to, ' ', ' '), href: '' };
}

/** What was typed in the address field, as a link target; null when it is not one. */
export function normalizeHref(input: string): string | null {
  const v = input.trim();
  if (v === '') return null;
  if (ISSUE_KEY_RE.test(v.toUpperCase()) && !v.includes('/')) return v.toUpperCase();
  if (/^[^\s@/]+@[^\s@/]+\.[^\s@/]+$/.test(v)) return `mailto:${v}`;
  let url = v;
  if (!/^[a-z][a-z0-9+.-]*:/i.test(url) && !url.startsWith('/') && !url.startsWith('#')) {
    if (!/^[^\s/]+\.[^\s/]+/.test(url)) return null;
    url = `https://${url}`;
  }
  return isSafeUrl(url) ? url.replace(/ /g, '%20') : null;
}

/** Sets (or replaces) the link over `target`, with this text. */
export function applyLink(view: EditorView, target: LinkTarget, text: string, href: string): void {
  const { state } = view;
  const label = text.trim() === '' ? href : text;
  let tr = state.tr;
  if (ISSUE_KEY_RE.test(href)) {
    // A link to an issue is markup the editor keeps as written.
    const wiki = label === href ? `[${href}]` : `[${label.replace(/[[\]|]/g, ' ')}|${href}]`;
    tr = tr.replaceWith(target.from, target.to, N.raw_inline.create({ wiki, kind: 'issue' }));
  } else {
    const link = M.link.create({ href, title: null });
    if (label !== target.text || target.from === target.to) {
      const marks = state.doc
        .resolve(target.from)
        .marks()
        .filter((m) => m.type !== M.link);
      tr = tr.replaceWith(target.from, target.to, schema.text(label, link.addToSet(marks)));
      tr.setSelection(TextSelection.create(tr.doc, target.from + label.length));
    } else {
      tr = tr.removeMark(target.from, target.to, M.link).addMark(target.from, target.to, link);
    }
    tr.removeStoredMark(M.link);
  }
  view.dispatch(tr.scrollIntoView());
}

export function removeLink(view: EditorView, target: LinkTarget): void {
  if (target.from === target.to) return;
  view.dispatch(view.state.tr.removeMark(target.from, target.to, M.link));
}

/* ---------- mentions and attachments ---------- */

export function insertMention(view: EditorView, from: number, to: number, username: string): void {
  const mention = N.mention.create({ username });
  const tr = view.state.tr.replaceWith(from, to, [mention, schema.text(' ')]);
  tr.setSelection(TextSelection.create(tr.doc, from + 2));
  view.dispatch(tr.scrollIntoView());
}

/** Inserts wiki references (images, attachment links) at a position, outside code. */
export function insertReferences(view: EditorView, pos: number, refs: PMNode[]): void {
  if (refs.length === 0) return;
  const { state } = view;
  const at = Math.min(pos, state.doc.content.size);
  const $at = state.doc.resolve(at);
  const content: PMNode[] = [];
  refs.forEach((r, i) => {
    if (i > 0) content.push(schema.text(' '));
    content.push(r);
  });
  let tr = state.tr;
  if ($at.parent.inlineContent && $at.parent.type !== N.code_block) {
    tr = tr.insert(at, content);
  } else {
    const after = $at.depth > 0 ? $at.after(1) : at;
    tr = tr.insert(after, N.paragraph.create(null, content));
  }
  view.dispatch(tr.scrollIntoView());
}
