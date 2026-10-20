/**
 * ProseMirror plumbing for the rich editor: key bindings, typing shortcuts,
 * the "@" suggestion state, placeholder and issue-key decorations, and the
 * views of nodes the editor shows but does not edit (mentions, raw markup).
 */

import { baseKeymap, chainCommands, toggleMark } from 'prosemirror-commands';
import { dropCursor } from 'prosemirror-dropcursor';
import { gapCursor } from 'prosemirror-gapcursor';
import { history, redo, undo } from 'prosemirror-history';
import {
  InputRule,
  inputRules,
  textblockTypeInputRule,
  undoInputRule,
  wrappingInputRule,
} from 'prosemirror-inputrules';
import { keymap } from 'prosemirror-keymap';
import type { MarkType, Node as PMNode } from 'prosemirror-model';
import { liftListItem, sinkListItem, splitListItem } from 'prosemirror-schema-list';
import {
  Plugin,
  PluginKey,
  TextSelection,
  type Command,
  type EditorState,
} from 'prosemirror-state';
import { goToNextCell, tableEditing } from 'prosemirror-tables';
import { Decoration, DecorationSet, type EditorView, type NodeView } from 'prosemirror-view';
import { KEY_IN_TEXT, renderWiki, type WikiResolver } from '../../core/wiki.js';
import { schema } from '../../core/wiki-doc.js';
import {
  insertRule,
  lineBreak,
  setHeading,
  toggle,
  toggleCodeBlock,
  toggleList,
  toggleQuote,
} from './commands.js';

const N = schema.nodes;
const M = schema.marks;

/* ---------- "@" suggestions ---------- */

export interface MentionQuery {
  from: number;
  to: number;
  query: string;
}

interface MentionState {
  query: MentionQuery | null;
  /** Escape closes the list until the "@" is gone. */
  dismissedAt: number;
}

export const mentionKey = new PluginKey<MentionState>('mention');

const MENTION_RE = /(?:^|[\s(«"'’])@([\p{L}\p{N}._-]{0,40})$/u;

function findMention(state: EditorState): MentionQuery | null {
  const { $from, empty } = state.selection;
  if (!empty || $from.parent.type === N.code_block || !$from.parent.inlineContent) return null;
  const before = $from.parent.textBetween(
    Math.max(0, $from.parentOffset - 60),
    $from.parentOffset,
    undefined,
    '￼',
  );
  const m = MENTION_RE.exec(before);
  if (!m) return null;
  const query = m[1] ?? '';
  return { from: $from.pos - query.length - 1, to: $from.pos, query };
}

function mentionPlugin(onKey: (event: KeyboardEvent) => boolean): Plugin<MentionState> {
  return new Plugin<MentionState>({
    key: mentionKey,
    state: {
      init: () => ({ query: null, dismissedAt: -1 }),
      apply(tr, prev, _old, state) {
        const query = findMention(state);
        if (!query) return { query: null, dismissedAt: -1 };
        let dismissedAt = prev.dismissedAt >= 0 ? tr.mapping.map(prev.dismissedAt) : -1;
        if (tr.getMeta(mentionKey) === 'dismiss') dismissedAt = query.from;
        return { query: query.from === dismissedAt ? null : query, dismissedAt };
      },
    },
    props: {
      handleKeyDown(view, event) {
        return mentionKey.getState(view.state)?.query ? onKey(event) : false;
      },
    },
  });
}

/* ---------- decorations ---------- */

const KEY_RE = new RegExp(KEY_IN_TEXT, 'g');
const WORD_RE = /[\p{L}\p{N}]/u;

/** Issue keys that exist look like links, as in displayed text. */
function issueKeyDecorations(doc: PMNode, exists: (key: string) => boolean): DecorationSet {
  const found: Decoration[] = [];
  doc.descendants((node, pos, parent) => {
    if (!node.isText || parent?.type === N.code_block || M.link.isInSet(node.marks)) return;
    if (M.code.isInSet(node.marks)) return;
    const text = node.text ?? '';
    for (const m of text.matchAll(KEY_RE)) {
      const prev = text.charAt(m.index - 1);
      const next = text.charAt(m.index + m[0].length);
      if (WORD_RE.test(prev) || prev === '-' || prev === '_' || WORD_RE.test(next)) continue;
      if (!exists(m[0])) continue;
      const from = pos + m.index;
      found.push(
        Decoration.inline(from, from + m[0].length, { nodeName: 'a', class: 'issue-key' }),
      );
    }
  });
  return DecorationSet.create(doc, found);
}

export const refreshKey = new PluginKey('refresh');

function decorationsPlugin(placeholder: () => string, exists: (key: string) => boolean): Plugin {
  return new Plugin<DecorationSet>({
    key: refreshKey,
    state: {
      init: (_config, state) => issueKeyDecorations(state.doc, exists),
      apply(tr, set, _old, state) {
        if (tr.docChanged || tr.getMeta(refreshKey)) return issueKeyDecorations(state.doc, exists);
        return set;
      },
    },
    props: {
      decorations(state) {
        const keys = refreshKey.getState(state) as DecorationSet | undefined;
        const first = state.doc.firstChild;
        const empty =
          state.doc.childCount === 1 && first?.type === N.paragraph && first.content.size === 0;
        const text = placeholder();
        if (!empty || text === '' || !first) return keys;
        const hint = Decoration.node(0, first.nodeSize, {
          class: 'is-empty',
          'data-placeholder': text,
        });
        return (keys ?? DecorationSet.empty).add(state.doc, [hint]);
      },
    },
  });
}

/* ---------- typing shortcuts ---------- */

/** "*word*" typed → bold word, and the like. */
function markRule(re: RegExp, type: MarkType, open: number, close: number): InputRule {
  return new InputRule(re, (state, match, start, end) => {
    const inner = match[1];
    if (!inner) return null;
    const openFrom = start + (match[0].length - inner.length - open - close);
    const innerFrom = openFrom + open;
    const innerTo = innerFrom + inner.length;
    const tr = state.tr.delete(innerTo, end).delete(openFrom, innerFrom);
    tr.addMark(openFrom, openFrom + inner.length, type.create());
    tr.removeStoredMark(type);
    return tr;
  });
}

function typingRules(): Plugin {
  return inputRules({
    rules: [
      wrappingInputRule(/^\s*([*-])\s$/, N.bullet_list, { marker: '*' }),
      wrappingInputRule(/^\s*(?:#|1[.)])\s$/, N.ordered_list),
      wrappingInputRule(/^\s*>\s$/, N.blockquote, { form: 'macro', params: null }),
      wrappingInputRule(/^bq\.\s$/, N.blockquote, { form: 'macro', params: null }),
      textblockTypeInputRule(/^h([1-6])\.\s$/, N.heading, (m) => ({ level: Number(m[1]) })),
      textblockTypeInputRule(/^```$/, N.code_block, { macro: 'code', params: null }),
      new InputRule(/^----$/, (state, _m, start) => {
        const $start = state.doc.resolve(start);
        if ($start.parent.type !== N.paragraph || $start.depth !== 1) return null;
        const tr = state.tr.replaceWith($start.before(), $start.after(), [
          N.horizontal_rule.create(),
          N.paragraph.create(),
        ]);
        return tr.setSelection(TextSelection.create(tr.doc, $start.before() + 2));
      }),
      markRule(/(?:^|[\s(«"'’])\*([^*\s](?:[^*]*[^*\s])?)\*$/u, M.strong, 1, 1),
      markRule(/(?:^|[\s(«"'’])_([^_\s](?:[^_]*[^_\s])?)_$/u, M.em, 1, 1),
      markRule(/\{\{([^{}]+)\}\}$/, M.code, 2, 2),
    ],
  });
}

/* ---------- keys ---------- */

export interface KeyHandlers {
  link: () => void;
  submit: (() => void) | null;
  cancel: (() => void) | null;
}

function keys(handlers: KeyHandlers): Plugin {
  const call =
    (fn: (() => void) | null): Command =>
    () => {
      if (!fn) return false;
      fn();
      return true;
    };
  const bindings: Record<string, Command> = {
    'Mod-z': undo,
    'Shift-Mod-z': redo,
    'Mod-y': redo,
    'Mod-b': toggle('strong'),
    'Mod-i': toggle('em'),
    'Mod-u': toggle('underline'),
    'Mod-Shift-x': toggle('strike'),
    'Mod-e': toggleMark(M.code),
    'Mod-k': () => {
      handlers.link();
      return true;
    },
    'Mod-Shift-7': toggleList('ordered'),
    'Mod-Shift-8': toggleList('bullet'),
    'Mod-Shift-9': toggleQuote,
    'Mod-Alt-c': toggleCodeBlock,
    'Mod-Alt-0': setHeading(0),
    'Shift-Enter': lineBreak,
    'Mod-Enter': call(handlers.submit),
    Escape: call(handlers.cancel),
    Enter: splitListItem(N.list_item),
    Tab: chainCommands(goToNextCell(1), sinkListItem(N.list_item)),
    'Shift-Tab': chainCommands(goToNextCell(-1), liftListItem(N.list_item)),
    Backspace: undoInputRule,
    'Mod-Shift-Enter': insertRule,
  };
  for (let level = 1; level <= 6; level++) bindings[`Mod-Alt-${level}`] = setHeading(level);
  return keymap(bindings);
}

/** Every plugin of the editor, in priority order. */
export function plugins(options: {
  handlers: KeyHandlers;
  onMentionKey: (event: KeyboardEvent) => boolean;
  placeholder: () => string;
  issueExists: (key: string) => boolean;
}): Plugin[] {
  return [
    mentionPlugin(options.onMentionKey),
    typingRules(),
    keys(options.handlers),
    keymap(baseKeymap),
    history(),
    dropCursor({ class: 'editor-drop' }),
    gapCursor(),
    tableEditing(),
    decorationsPlugin(options.placeholder, options.issueExists),
  ];
}

/* ---------- node views ---------- */

/** Something that redraws when names, issues or attachments become known. */
export interface Live {
  redraw(): void;
}

/** "<p>x</p>" → "x": raw inline markup is rendered alone, as a paragraph. */
function inlineHtml(html: string): string {
  const m = /^<p>([\s\S]*)<\/p>$/.exec(html);
  return m && !m[1]?.includes('<p>') ? (m[1] ?? '') : html;
}

class MentionView implements NodeView, Live {
  dom: HTMLElement;
  constructor(
    private node: PMNode,
    private resolver: () => WikiResolver,
    private live: Set<Live>,
  ) {
    this.dom = document.createElement('a');
    this.dom.className = 'mention editor-mention';
    this.dom.contentEditable = 'false';
    live.add(this);
    this.redraw();
  }
  redraw(): void {
    const username = this.node.attrs.username as string;
    this.dom.textContent = this.resolver().user(username) ?? `@${username}`;
    this.dom.title = `@${username}`;
  }
  update(node: PMNode): boolean {
    if (node.type !== this.node.type) return false;
    this.node = node;
    this.redraw();
    return true;
  }
  ignoreMutation(): boolean {
    return true;
  }
  destroy(): void {
    this.live.delete(this);
  }
}

class RawView implements NodeView, Live {
  dom: HTMLElement;
  constructor(
    private node: PMNode,
    private resolver: () => WikiResolver,
    private live: Set<Live>,
    private title: string,
  ) {
    this.dom = document.createElement(node.isInline ? 'span' : 'div');
    this.dom.className = 'editor-raw';
    this.dom.contentEditable = 'false';
    this.dom.dataset.kind = node.attrs.kind as string;
    live.add(this);
    this.redraw();
  }
  redraw(): void {
    const wiki = this.node.attrs.wiki as string;
    const html = renderWiki(wiki, this.resolver());
    const shown = this.node.isInline ? inlineHtml(html) : html;
    if (shown.trim() === '') {
      // Nothing visible once rendered ({anchor}, {toc}…): show the markup.
      const code = document.createElement('code');
      code.textContent = wiki;
      this.dom.replaceChildren(code);
    } else {
      // renderWiki escapes all text and only emits its own tags.
      this.dom.innerHTML = shown;
    }
    this.dom.title = `${wiki}\n${this.title}`;
  }
  update(node: PMNode): boolean {
    if (node.type !== this.node.type) return false;
    const changed = node.attrs.wiki !== this.node.attrs.wiki;
    this.node = node;
    if (changed) this.redraw();
    return true;
  }
  ignoreMutation(): boolean {
    return true;
  }
  stopEvent(event: Event): boolean {
    return event.type === 'dblclick';
  }
  destroy(): void {
    this.live.delete(this);
  }
}

export function nodeViews(
  resolver: () => WikiResolver,
  live: Set<Live>,
  rawTitle: string,
): Record<string, (node: PMNode, view: EditorView) => NodeView> {
  return {
    mention: (node) => new MentionView(node, resolver, live),
    raw_inline: (node) => new RawView(node, resolver, live, rawTitle),
    raw_block: (node) => new RawView(node, resolver, live, rawTitle),
  };
}
