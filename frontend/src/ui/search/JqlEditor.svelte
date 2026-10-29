<script lang="ts">
  import { tick, untrack } from 'svelte';
  import {
    charIndex,
    completionAt,
    FIELDS,
    functionsFor,
    highlight,
    JqlError,
    operatorsFor,
    parse,
    quote,
    quoteField,
    SORT_FIELDS,
    type Completion,
  } from '../../core/jql.js';
  import { CATEGORY_JQL } from '../../core/filters.js';
  import { STATUS_CATEGORIES } from '../../core/types.js';
  import { t } from '../../i18n/t.js';
  import { catalog, fold, kindOf, listFieldOf, loadOptions } from './sources.svelte.js';

  /**
   * The query as JQL, coloured as it is typed, with suggestions for fields,
   * operators, functions and values (the same lists as the filters). The
   * server's error, if any, shows under it, the faulty token underlined.
   */
  interface Props {
    /** The query in force. */
    value: string;
    /** What is being typed; follows value when the query changes. */
    draft: string;
    /** The server's message for the query in force. */
    error: string | null;
    /** Where the server stopped reading it (characters), when it says. */
    errorPos?: number | null | undefined;
    onsubmit: (jql: string) => void;
    /** A project the search is limited to, for value suggestions. */
    project?: string | undefined;
    autofocus?: boolean | undefined;
  }
  let {
    value,
    draft = $bindable(),
    error,
    errorPos = null,
    onsubmit,
    project,
    autofocus = false,
  }: Props = $props();

  interface Item {
    label: string;
    insert: string;
    detail: string;
    /** Characters to step back after inserting (inside membersOf("…")). */
    back?: number;
    /** Where the replaced text starts, when not at the current word. */
    from?: number;
  }

  let textarea: HTMLTextAreaElement | undefined = $state();
  let field: HTMLDivElement | undefined = $state();
  let pre: HTMLPreElement | undefined = $state();
  let items = $state<Item[]>([]);
  let open = $state(false);
  let active = $state(0);
  let ctx: Completion | null = null;
  let at = $state({ left: 0, top: 0 });
  let seq = 0;
  const uid = $props.id();
  const id = `jql-${uid}`;

  $effect(() => {
    const v = value;
    untrack(() => (draft = v));
  });

  $effect(() => {
    if (autofocus && textarea) {
      textarea.focus();
      textarea.setSelectionRange(textarea.value.length, textarea.value.length);
    }
  });

  const styled = $derived(highlight(draft));

  /** Where the error points: the server's position, else the client parser's. */
  const errorAt = $derived.by(() => {
    if (!error) return -1;
    if (errorPos !== null) return charIndex(value, errorPos);
    try {
      parse(value);
      return -1;
    } catch (e) {
      return e instanceof JqlError ? e.pos : -1;
    }
  });

  const segments = $derived.by(() => {
    const out: { text: string; cls: string }[] = [];
    const mark = draft === value ? errorAt : -1;
    // The token the error points at (the next one, when it points between two).
    const badIndex = mark < 0 ? -1 : styled.findIndex((tk) => tk.end > mark);
    let pos = 0;
    for (const [i, tk] of styled.entries()) {
      if (tk.start > pos) out.push({ text: draft.slice(pos, tk.start), cls: '' });
      const bad = i === badIndex;
      // The word still being typed at the end is not flagged yet.
      const typing = tk.role === 'error' && i === styled.length - 1 && tk.end === draft.length;
      out.push({ text: tk.raw, cls: `jql-token-${bad ? 'error' : typing ? 'value' : tk.role}` });
      pos = tk.end;
    }
    if (pos < draft.length) out.push({ text: draft.slice(pos), cls: '' });
    // An error at the very end (a value missing): a mark after the text.
    if (mark >= 0 && badIndex < 0) out.push({ text: ' ', cls: 'jql-token-error' });
    return out;
  });

  const startsWith = (s: string, prefix: string) => fold(s).startsWith(fold(prefix));

  async function itemsFor(c: Completion): Promise<Item[]> {
    // A value of several words being typed: its values come first.
    if (c.phrase) {
      const { prefix, from, inList } = c.phrase;
      const values = await valueItems({ ...c, kind: 'value', prefix, from, inList, keywords: [] });
      const rest = await itemsFor({ ...c, phrase: null });
      return [...values.map((v) => ({ ...v, from })), ...rest];
    }
    const p = c.prefix;
    const keywords = (kws: string[]) =>
      kws
        .filter((k) => startsWith(k, p))
        .map((k) => ({ label: k, insert: k === '(' ? '(' : `${k} `, detail: t.search.keyword }));
    switch (c.kind) {
      case 'field': {
        const system = FIELDS.filter(
          (f) => startsWith(f.name, p) || (f.aliases ?? []).some((a) => startsWith(a, p)),
        );
        const custom = (await catalog.loadFields()).filter(
          (f) => startsWith(f.name, p) && !FIELDS.some((sys) => fold(sys.name) === fold(f.name)),
        );
        return [
          ...system.map((f) => ({
            label: f.name,
            insert: `${quoteField(f.name)} `,
            detail: t.search.jqlFields[f.name] ?? '',
          })),
          ...custom.map((f) => ({
            label: f.name,
            insert: `${f.jql} `,
            detail: t.search.customField,
          })),
          ...keywords(c.keywords),
        ];
      }
      case 'operator':
        await catalog.loadFields();
        return operatorsFor(c.field, kindOf(c.field))
          .filter((op) => startsWith(op, p))
          .map((op) => ({
            label: op,
            insert: op.endsWith(' in') || op === 'in' ? `${op} (` : `${op} `,
            detail: t.search.operator,
          }));
      case 'keyword':
        return keywords(c.keywords);
      case 'sort':
        return SORT_FIELDS.filter((f) => startsWith(f, p)).map((f) => ({
          label: f,
          insert: `${f} `,
          detail: t.search.jqlFields[f] ?? '',
        }));
      case 'value':
        return valueItems(c);
      default:
        return [];
    }
  }

  async function valueItems(c: Completion): Promise<Item[]> {
    await catalog.loadFields();
    const p = c.prefix;
    const end = c.inList ? '' : ' ';
    const out: Item[] = [];
    if (c.keywords.includes('(') && !p) out.push({ label: '(', insert: '(', detail: '' });
    const kind = kindOf(c.field);
    if (kind === 'category') {
      for (const v of STATUS_CATEGORIES.map((cat) => CATEGORY_JQL[cat])) {
        if (startsWith(v, p)) out.push({ label: v, insert: quote(v) + end, detail: '' });
      }
    } else if (kind === 'date') {
      for (const v of ['-1d', '-7d', '-30d', '-1w']) {
        if (startsWith(v, p)) out.push({ label: v, insert: v + end, detail: '' });
      }
    }
    const list = listFieldOf(c.field);
    if (list) {
      try {
        const opts = await loadOptions(list, p, project ? [quote(project)] : []);
        for (const o of opts.slice(0, 12)) {
          out.push({ label: o.label, insert: o.value + end, detail: o.hint ?? '' });
        }
      } catch {
        // Suggestions are a help: without them the editor still works.
      }
    }
    for (const fn of functionsFor(c.field, kind)) {
      if (!startsWith(fn, p)) continue;
      const inside = fn.endsWith('("")');
      out.push({
        label: fn,
        insert: fn + (inside ? '' : end),
        detail: t.search.function,
        back: inside ? 2 : 0,
      });
    }
    if (kind !== 'text' && startsWith('EMPTY', p) && c.keywords.includes('EMPTY')) {
      out.push({ label: 'EMPTY', insert: `EMPTY${end}`, detail: t.search.keyword });
    }
    return out;
  }

  /** The caret's place under the text, for the suggestion list. */
  function caretPoint(offset: number): { left: number; top: number } {
    if (!pre || !field) return { left: 0, top: 0 };
    const box = field.getBoundingClientRect();
    const walker = document.createTreeWalker(pre, NodeFilter.SHOW_TEXT);
    let left = offset;
    for (let n = walker.nextNode(); n; n = walker.nextNode()) {
      const len = n.textContent?.length ?? 0;
      if (left <= len) {
        const r = document.createRange();
        r.setStart(n, left);
        r.setEnd(n, left);
        const rect = r.getBoundingClientRect();
        if (rect.bottom === 0 && rect.left === 0) break;
        const x = Math.max(0, Math.min(rect.left - box.left - 8, box.width - 280));
        return { left: x, top: rect.bottom - box.top + 4 };
      }
      left -= len;
    }
    return { left: 0, top: box.height + 4 };
  }

  async function suggest(force = false) {
    if (!textarea) return;
    const caret = textarea.selectionStart;
    if (textarea.selectionEnd !== caret) {
      open = false;
      return;
    }
    const c = completionAt(draft, caret);
    ctx = c;
    const mine = ++seq;
    const list = await itemsFor(c);
    if (mine !== seq) return;
    items = list.slice(0, 14);
    active = 0;
    const only = items.length === 1 && items[0]?.insert.trim() === c.prefix;
    open = items.length > 0 && (force || !only);
    if (!open) return;
    // Measure once the coloured copy shows the text just typed.
    await tick();
    if (mine === seq) at = caretPoint(caret);
  }

  async function accept(item: Item) {
    const c = ctx;
    if (!c || !textarea) return;
    const before = draft.slice(0, item.from ?? c.from);
    const after = draft.slice(c.to);
    let ins = item.insert;
    if (ins.endsWith(' ') && after.startsWith(' ')) ins = ins.slice(0, -1);
    draft = before + ins + after;
    const caret = before.length + ins.length - (item.back ?? 0);
    open = false;
    await tick();
    textarea.focus();
    textarea.setSelectionRange(caret, caret);
    void suggest();
  }

  function keydown(e: KeyboardEvent) {
    if (open && items.length) {
      if (e.key === 'ArrowDown') {
        e.preventDefault();
        active = (active + 1) % items.length;
        return;
      }
      if (e.key === 'ArrowUp') {
        e.preventDefault();
        active = (active - 1 + items.length) % items.length;
        return;
      }
      if (e.key === 'Enter' || e.key === 'Tab') {
        const item = items[active];
        if (item) {
          e.preventDefault();
          void accept(item);
        }
        return;
      }
      if (e.key === 'Escape') {
        e.preventDefault();
        e.stopPropagation();
        open = false;
        return;
      }
    }
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      open = false;
      onsubmit(draft);
    } else if (e.key === ' ' && e.ctrlKey) {
      e.preventDefault();
      void suggest(true);
    }
  }

  $effect(() => {
    if (!open) return;
    document.getElementById(`${id}-opt-${active}`)?.scrollIntoView({ block: 'nearest' });
  });
</script>

<div class="jql-editor" class:has-error={!!error}>
  <div class="jql-field" bind:this={field}>
    <pre class="jql-highlight" aria-hidden="true" bind:this={pre}>{#each segments as s, i (i)}<span
          class={s.cls}>{s.text}</span
        >{/each}&#10;</pre>
    <textarea
      bind:this={textarea}
      bind:value={draft}
      class="jql-input"
      rows="1"
      spellcheck="false"
      autocomplete="off"
      autocapitalize="off"
      aria-label={t.search.jqlLabel}
      aria-describedby="{id}-hint{error ? ` ${id}-error` : ''}"
      aria-invalid={error ? 'true' : undefined}
      role="combobox"
      aria-expanded={open}
      aria-controls="{id}-list"
      aria-autocomplete="list"
      aria-activedescendant={open ? `${id}-opt-${active}` : undefined}
      oninput={() => void suggest()}
      onkeydown={keydown}
      onclick={() => open && void suggest()}
      onblur={() => (open = false)}
    ></textarea>
    {#if open}
      <ul
        class="popover jql-suggestions"
        id="{id}-list"
        role="listbox"
        aria-label={t.search.suggestions}
        style:left="{at.left}px"
        style:top="{at.top}px"
      >
        {#each items as it, i (i)}
          <!-- Chosen with the keyboard from the editor, or with a pointer. -->
          <li
            id="{id}-opt-{i}"
            class="item"
            class:active={i === active}
            role="option"
            aria-selected={i === active}
            onpointerdown={(e) => {
              e.preventDefault();
              void accept(it);
            }}
            onmouseenter={() => (active = i)}
          >
            <span class="label">{it.label}</span>
            {#if it.detail}<span class="detail">{it.detail}</span>{/if}
          </li>
        {/each}
      </ul>
    {/if}
  </div>
  <div class="jql-foot">
    <span class="hint" id="{id}-hint">{t.search.jqlHint}</span>
    <button
      type="button"
      class="btn btn-primary btn-small"
      onclick={() => {
        open = false;
        onsubmit(draft);
      }}>{t.search.run}</button
    >
  </div>
  {#if error}
    <p class="error-text jql-error" id="{id}-error" role="alert">{error}</p>
  {/if}
</div>

<style>
  .jql-field {
    position: relative;
    display: grid;
  }

  .jql-highlight,
  .jql-input {
    grid-area: 1 / 1;
    margin: 0;
    min-height: 3.875rem;
    padding: 8px 10px;
    border: var(--frame) solid transparent;
    border-radius: var(--radius-xs);
    font: 0.84375rem/1.6 var(--font-mono);
    letter-spacing: 0;
    white-space: pre-wrap;
    overflow-wrap: anywhere;
    tab-size: 2;
  }

  .jql-highlight {
    border-color: var(--rule-strong);
    background-color: var(--paper-raised);
    color: var(--ink);
    pointer-events: none;
  }

  .jql-input {
    resize: none;
    overflow: hidden;
    background: transparent;
    color: transparent;
    caret-color: var(--ink);
  }

  .jql-input::selection {
    background: color-mix(in srgb, var(--accent) 22%, transparent);
    color: transparent;
  }

  .jql-input:focus {
    outline: none;
    border-color: var(--accent);
    box-shadow: 0 0 0 1px var(--accent);
  }

  .has-error .jql-highlight {
    border-color: var(--accent);
  }

  .jql-token-field {
    color: var(--stamp-doing);
  }

  .jql-token-operator,
  .jql-token-punct {
    color: var(--muted);
  }

  .jql-token-keyword {
    color: var(--tone-epic);
    font-weight: 600;
  }

  .jql-token-value {
    color: var(--ink);
  }

  .jql-token-string {
    color: var(--stamp-done);
  }

  .jql-token-function {
    color: var(--tone-task);
    font-style: italic;
  }

  .jql-token-error {
    color: var(--accent-text);
    text-decoration: underline wavy var(--accent);
    text-underline-offset: 3px;
  }

  .jql-suggestions {
    width: 17.5rem;
    max-height: 16.25rem;
    list-style: none;
    margin: 0;
  }

  .jql-suggestions .label {
    font: 0.8125rem var(--font-mono);
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .jql-suggestions .detail {
    margin-left: auto;
    padding-left: 10px;
    font-size: 0.71875rem;
    color: var(--muted);
    white-space: nowrap;
  }

  .jql-foot {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    margin-top: 6px;
  }

  .jql-foot .hint {
    font-size: 0.75rem;
    color: var(--muted);
  }

  .jql-error {
    margin: 8px 0 0;
    padding: 6px 10px;
    border-left: 3px solid var(--accent);
    background: var(--accent-wash);
    font: 0.8125rem/1.5 var(--font-body);
  }

  @media (max-width: 600px) {
    .jql-foot .hint {
      display: none;
    }

    .jql-foot {
      justify-content: flex-end;
    }
  }
</style>
