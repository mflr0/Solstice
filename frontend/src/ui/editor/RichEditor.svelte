<script lang="ts">
  import 'prosemirror-view/style/prosemirror.css';
  import { untrack } from 'svelte';
  import { Fragment, Slice } from 'prosemirror-model';
  import { EditorState, type Command } from 'prosemirror-state';
  import { EditorView } from 'prosemirror-view';
  import { redo, undo } from 'prosemirror-history';
  import {
    addColumnAfter,
    addRowAfter,
    deleteColumn,
    deleteRow,
    deleteTable,
    toggleHeaderRow,
  } from 'prosemirror-tables';
  import type { Attachment, UserRef } from '../../core/types.js';
  import { IMAGE_EXT_RE, wikiReferences } from '../../core/wiki.js';
  import {
    parseWiki,
    parseWikiBlocks,
    schema,
    serializeWiki,
    type ParsedWiki,
  } from '../../core/wiki-doc.js';
  import { t } from '../../i18n/t.js';
  import { api, errorMessage, isVisible } from '../../lib/api.js';
  import { refs } from '../../lib/refs.svelte.js';
  import { notify } from '../../lib/ui.svelte.js';
  import {
    applyLink,
    insertMention,
    insertReferences,
    insertRule,
    insertTable,
    linkTarget,
    normalizeHref,
    removeLink,
    setHeading,
    setLanguage,
    situation,
    toggle,
    toggleCodeBlock,
    toggleList,
    toggleQuote,
    type LinkTarget,
    type MarkName,
    type Situation,
  } from './commands.js';
  import { ICONS } from './icons.js';
  import {
    mentionKey,
    nodeViews,
    plugins,
    refreshKey,
    type Live,
    type MentionQuery,
  } from './plugins.js';

  /**
   * The rich text editor: edits a document, reads and writes wiki markup.
   * Loaded on demand by WikiEditor.
   */
  interface Props {
    value: string;
    label: string;
    placeholder?: string | undefined;
    rows?: number | undefined;
    attachments?: Attachment[] | undefined;
    autofocus?: boolean | undefined;
    onsubmit?: (() => void) | undefined;
    oncancel?: (() => void) | undefined;
    /** Issue that pasted or dropped files are attached to; none, no upload. */
    uploadTo?: string | undefined;
  }
  let {
    value = $bindable(),
    label,
    placeholder = '',
    rows = 6,
    attachments = [],
    autofocus = false,
    onsubmit,
    oncancel,
    uploadTo,
  }: Props = $props();

  const LANGUAGES = [
    'bash', 'c', 'cpp', 'csharp', 'css', 'diff', 'go', 'html', 'java', 'javascript', 'json',
    'kotlin', 'php', 'python', 'ruby', 'rust', 'sql', 'typescript', 'xml', 'yaml',
  ]; // prettier-ignore

  let host: HTMLDivElement | undefined = $state();
  let frame: HTMLDivElement | undefined = $state();
  let toolbar: HTMLDivElement | undefined = $state();
  let sourceArea: HTMLTextAreaElement | undefined = $state();
  let view: EditorView | null = null;
  let parsed: ParsedWiki = parseWiki('');
  /** The markup the document was last written to, to tell outside changes apart. */
  let written = '';

  let source = $state(false);
  let at = $state<Situation | null>(null);

  let mention = $state<MentionQuery | null>(null);
  let people = $state<UserRef[]>([]);
  let picked = $state(0);
  let searching = $state(false);
  let searchFor: string | null = null;
  let searchTimer: ReturnType<typeof setTimeout> | undefined;

  let linkAt = $state<LinkTarget | null>(null);
  let linkText = $state('');
  let linkUrl = $state('');
  let linkError = $state('');

  let uploading = $state<string[]>([]);
  /** Files attached from here, known before the page reloads the issue. */
  let added = $state<Attachment[]>([]);
  /** Where uploads in progress will insert their references, kept in step with edits. */
  let pending: { pos: number }[] = [];

  const id = `editor-${Math.random().toString(36).slice(2, 8)}`;
  const files = $derived(new Map([...attachments, ...added].map((a) => [a.filename, a.url])));
  const resolver = $derived(refs.resolver(files));
  const live = new Set<Live>();

  function newState(doc: ParsedWiki['doc']): EditorState {
    return EditorState.create({
      doc,
      plugins: plugins({
        handlers: {
          link: openLink,
          submit: onsubmit ? () => onsubmit?.() : null,
          cancel: oncancel ? () => oncancel?.() : null,
        },
        onMentionKey: mentionKey_,
        placeholder: () => placeholder,
        issueExists: (key) => resolver.issueExists(key),
      }),
    });
  }

  function look(state: EditorState): void {
    at = situation(state);
    const q = mentionKey.getState(state)?.query ?? null;
    mention = q;
    if (q && q.query !== searchFor) search(q.query);
    if (!q) searchFor = null;
  }

  let refsTimer: ReturnType<typeof setTimeout> | undefined;
  function askRefs(): void {
    clearTimeout(refsTimer);
    refsTimer = setTimeout(() => refs.ensure(wikiReferences(value)), 300);
  }

  function reset(markup: string): void {
    parsed = parseWiki(markup);
    written = markup;
    view?.updateState(newState(parsed.doc));
    if (view) look(view.state);
    askRefs();
  }

  // The editor itself: created once, destroyed with the component.
  $effect(() => {
    if (host) return untrack(() => mount(host as HTMLDivElement));
  });

  function mount(into: HTMLDivElement): () => void {
    const markup = value;
    parsed = parseWiki(markup);
    written = markup;
    const v = new EditorView(into, {
      state: newState(parsed.doc),
      nodeViews: nodeViews(() => resolver, live, t.editor.raw),
      attributes: {
        class: 'editor-content wiki',
        role: 'textbox',
        'aria-multiline': 'true',
        'aria-label': label,
        id,
        style: `--editor-rows: ${rows}`,
      },
      dispatchTransaction(tr) {
        const state = v.state.apply(tr);
        v.updateState(state);
        for (const p of pending) p.pos = tr.mapping.map(p.pos);
        look(state);
        if (tr.docChanged) {
          written = serializeWiki(state.doc, parsed);
          value = written;
          askRefs();
        }
      },
      handlePaste(_view, event) {
        const list = [...(event.clipboardData?.files ?? [])];
        if (list.length === 0 || !uploadTo) return false;
        void upload(list, v.state.selection.from);
        return true;
      },
      handleDrop(_view, event, _slice, moved) {
        const list = [...(event.dataTransfer?.files ?? [])];
        if (moved || list.length === 0) return false;
        event.preventDefault();
        if (uploadTo) {
          const pos = v.posAtCoords({ left: event.clientX, top: event.clientY })?.pos;
          void upload(list, pos ?? v.state.selection.from);
        }
        return true;
      },
      clipboardTextParser(text, $context, plain) {
        if ($context.parent.type.spec.code) {
          return new Slice(Fragment.from(text ? schema.text(text) : []), 0, 0);
        }
        // Plain paste (Shift held): the text as typed, line by line.
        const blocks = plain
          ? Fragment.fromArray(
              text.split(/\r?\n\s*\r?\n/).map((para) =>
                schema.nodes.paragraph.create(
                  null,
                  para
                    .split(/\r?\n/)
                    .flatMap((line, i) => [
                      ...(i > 0 ? [schema.nodes.hard_break.create()] : []),
                      ...(line ? [schema.text(line)] : []),
                    ]),
                ),
              ),
            )
          : parseWikiBlocks(text);
        return Slice.maxOpen(blocks);
      },
      handleDOMEvents: {
        focus: () => {
          closeLink();
          return false;
        },
        // Links inside the text are not followed while editing.
        click: (_view, event) => {
          if ((event.target as Element | null)?.closest('a')) event.preventDefault();
          return false;
        },
        dblclick: (_view, event) => {
          if (!(event.target as Element | null)?.closest('.editor-raw')) return false;
          event.preventDefault();
          showSource(true);
          return true;
        },
      },
    });
    view = v;
    look(v.state);
    askRefs();
    if (autofocus) v.focus();
    return () => {
      clearTimeout(searchTimer);
      clearTimeout(refsTimer);
      v.destroy();
      view = null;
    };
  }

  // The value changed from outside (comment sent, source edited): start over from it.
  $effect(() => {
    const markup = value;
    untrack(() => {
      if (view && markup !== written) reset(markup);
    });
  });

  // Names, issues and attachments became known: redraw what shows them.
  $effect(() => {
    void JSON.stringify(refs.users);
    void JSON.stringify(refs.issues);
    void files;
    untrack(() => {
      for (const l of live) l.redraw();
      if (view) view.dispatch(view.state.tr.setMeta(refreshKey, true));
    });
  });

  /* ---------- toolbar keyboard: one tab stop, arrows move ---------- */

  let current = 0;

  function controls(): HTMLElement[] {
    if (!toolbar) return [];
    return [...toolbar.querySelectorAll<HTMLElement>('button, select, input')].filter(
      (el) => !(el as HTMLButtonElement).disabled,
    );
  }

  function rove(): void {
    const list = controls();
    current = Math.max(0, Math.min(current, list.length - 1));
    list.forEach((el, i) => (el.tabIndex = i === current ? 0 : -1));
  }

  $effect(() => {
    void at;
    void source;
    queueMicrotask(rove);
  });

  function toolbarKeys(e: KeyboardEvent): void {
    const target = e.target as HTMLElement;
    if (target.tagName === 'INPUT') return;
    const list = controls();
    const at = list.indexOf(target);
    if (at < 0) return;
    const next =
      e.key === 'ArrowRight'
        ? (at + 1) % list.length
        : e.key === 'ArrowLeft'
          ? (at - 1 + list.length) % list.length
          : e.key === 'Home'
            ? 0
            : e.key === 'End'
              ? list.length - 1
              : -1;
    if (next < 0) return;
    e.preventDefault();
    current = next;
    rove();
    list[next]?.focus();
  }

  function toolbarFocus(e: FocusEvent): void {
    const at = controls().indexOf(e.target as HTMLElement);
    if (at >= 0) {
      current = at;
      rove();
    }
  }

  function run(cmd: Command): void {
    if (!view) return;
    cmd(view.state, view.dispatch, view);
    view.focus();
  }

  // `at` first: it is what makes the toolbar re-check after each change.
  const can = (cmd: Command): boolean => (at && view ? cmd(view.state) : false);

  function showSource(on: boolean): void {
    source = on;
    closeLink();
    if (on) queueMicrotask(() => sourceArea?.focus());
    else queueMicrotask(() => view?.focus());
  }

  function sourceKeys(e: KeyboardEvent): void {
    if (e.key === 'Enter' && (e.ctrlKey || e.metaKey) && onsubmit) {
      e.preventDefault();
      onsubmit();
    } else if (e.key === 'Escape' && oncancel) {
      e.preventDefault();
      oncancel();
    }
  }

  function grow(node: HTMLTextAreaElement): void {
    node.style.height = 'auto';
    node.style.height = `${node.scrollHeight + 2}px`;
  }

  /* ---------- floating panels ---------- */

  function placeAt(pos: number): string {
    if (!view || !frame) return '';
    try {
      const c = view.coordsAtPos(pos);
      const box = frame.getBoundingClientRect();
      const left = Math.max(0, Math.min(c.left - box.left, box.width - 260));
      return `left: ${left}px; top: ${c.bottom - box.top + 6}px`;
    } catch {
      return '';
    }
  }

  function search(query: string): void {
    searchFor = query;
    clearTimeout(searchTimer);
    searching = true;
    searchTimer = setTimeout(() => {
      api
        .users(query)
        .then((list) => {
          if (searchFor !== query) return;
          people = [...list].sort((a, b) => Number(b.active) - Number(a.active)).slice(0, 8);
          picked = 0;
        })
        .catch(() => {
          people = [];
        })
        .finally(() => {
          if (searchFor === query) searching = false;
        });
    }, 120);
  }

  function pick(user: UserRef | undefined): void {
    if (!view || !mention || !user) return;
    insertMention(view, mention.from, mention.to, user.username);
    view.focus();
  }

  function mentionKey_(e: KeyboardEvent): boolean {
    if (!view) return false;
    if (e.key === 'Escape') {
      view.dispatch(view.state.tr.setMeta(mentionKey, 'dismiss'));
      return true;
    }
    if (people.length === 0) return false;
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      const step = e.key === 'ArrowDown' ? 1 : -1;
      picked = (picked + step + people.length) % people.length;
      return true;
    }
    if (e.key === 'Enter' || e.key === 'Tab') {
      pick(people[picked]);
      return true;
    }
    return false;
  }

  function openLink(): void {
    if (!view) return;
    const target = linkTarget(view.state);
    linkAt = target;
    linkText = target.text;
    linkUrl = target.href;
    linkError = '';
  }

  function closeLink(): void {
    linkAt = null;
  }

  function submitLink(e: SubmitEvent): void {
    e.preventDefault();
    if (!view || !linkAt) return;
    const href = normalizeHref(linkUrl);
    if (!href) {
      linkError = t.editor.badUrl;
      return;
    }
    applyLink(view, linkAt, linkText, href);
    closeLink();
    view.focus();
  }

  function unlink(): void {
    if (!view || !linkAt) return;
    removeLink(view, linkAt);
    closeLink();
    view.focus();
  }

  function linkKeys(e: KeyboardEvent): void {
    if (e.key === 'Escape') {
      e.preventDefault();
      e.stopPropagation();
      closeLink();
      view?.focus();
    }
  }

  /* ---------- attachments ---------- */

  /** Screenshots all arrive as "image.png": give them a dated name. */
  function named(file: File): File {
    if (file.name && !/^image\.\w+$/i.test(file.name)) return file;
    const ext = (/\/(\w+)$/.exec(file.type)?.[1] ?? 'png').replace('jpeg', 'jpg');
    const d = new Date();
    const pad = (n: number): string => String(n).padStart(2, '0');
    const stamp =
      `${d.getFullYear()}${pad(d.getMonth() + 1)}${pad(d.getDate())}` +
      `-${pad(d.getHours())}${pad(d.getMinutes())}${pad(d.getSeconds())}`;
    return new File([file], `capture-${stamp}.${ext}`, { type: file.type });
  }

  async function upload(list: File[], pos: number): Promise<void> {
    if (!uploadTo) return;
    const sent = list.map(named);
    const names = sent.map((f) => f.name).join(', ');
    const spot = { pos };
    pending = [...pending, spot];
    uploading = [...uploading, names];
    try {
      const d = await api.upload(uploadTo, sent);
      if (!isVisible(d)) return;
      added = d.attachments;
      const known = new Set(d.attachments.map((a) => a.filename));
      const refsToInsert = sent
        .map((f) => f.name)
        .filter((name) => known.has(name) && !/[[\]|]/.test(name))
        .map((name) =>
          IMAGE_EXT_RE.test(name) && !name.includes('!')
            ? schema.nodes.raw_inline.create({ wiki: `!${name}!`, kind: 'image' })
            : schema.nodes.raw_inline.create({ wiki: `[^${name}]`, kind: 'file' }),
        );
      if (view) insertReferences(view, spot.pos, refsToInsert);
    } catch (e) {
      notify(errorMessage(e), 'error');
    } finally {
      pending = pending.filter((p) => p !== spot);
      uploading = uploading.filter((n) => n !== names);
    }
  }

  const MARKS: { name: MarkName; label: string; glyph: string; keys: string }[] = [
    { name: 'strong', label: t.editor.bold, glyph: 'B', keys: 'Ctrl+B' },
    { name: 'em', label: t.editor.italic, glyph: 'I', keys: 'Ctrl+I' },
    { name: 'underline', label: t.editor.underline, glyph: 'U', keys: 'Ctrl+U' },
    { name: 'strike', label: t.editor.strike, glyph: 'S', keys: 'Ctrl+Maj+X' },
  ];
</script>

{#snippet icon(name: string)}
  <svg viewBox="0 0 16 16" width="16" height="16" aria-hidden="true">
    {#each ICONS[name] ?? [] as d, i (i)}<path {d} />{/each}
  </svg>
{/snippet}

{#snippet button(
  name: string,
  title: string,
  cmd: Command,
  pressed: boolean | null = null,
  keys = '',
)}
  <button
    type="button"
    class="editor-button"
    aria-label={title}
    aria-pressed={pressed === null ? undefined : pressed}
    title={keys ? `${title} (${keys})` : title}
    disabled={source || !can(cmd)}
    onmousedown={(e) => e.preventDefault()}
    onclick={() => run(cmd)}>{@render icon(name)}</button
  >
{/snippet}

<div class="editor" class:is-source={source} bind:this={frame}>
  <div
    bind:this={toolbar}
    class="editor-toolbar"
    role="toolbar"
    tabindex="-1"
    aria-label={t.editor.toolbar}
    aria-controls={id}
    onkeydown={toolbarKeys}
    onfocusin={toolbarFocus}
  >
    <div class="editor-group">
      <select
        class="select editor-style"
        aria-label={t.editor.style}
        disabled={source || !at || at.heading < 0}
        value={at?.heading ?? 0}
        onchange={(e) => run(setHeading(Number(e.currentTarget.value)))}
      >
        <option value={-1} hidden>—</option>
        <option value={0}>{t.editor.paragraph}</option>
        {#each [1, 2, 3, 4, 5, 6] as level (level)}
          <option value={level}>{t.editor.heading(level)}</option>
        {/each}
      </select>
    </div>

    <div class="editor-group">
      {#each MARKS as m (m.name)}
        <button
          type="button"
          class="editor-button editor-mark"
          data-mark={m.name}
          aria-label={m.label}
          aria-pressed={at?.marks[m.name] ?? false}
          title={`${m.label} (${m.keys})`}
          disabled={source || !can(toggle(m.name))}
          onmousedown={(e) => e.preventDefault()}
          onclick={() => run(toggle(m.name))}><span aria-hidden="true">{m.glyph}</span></button
        >
      {/each}
      {@render button('code', t.editor.code, toggle('code'), at?.marks.code ?? false, 'Ctrl+E')}
      <button
        type="button"
        class="editor-button"
        aria-label={t.editor.link}
        aria-pressed={at?.link ?? false}
        aria-haspopup="dialog"
        title={`${t.editor.link} (Ctrl+K)`}
        disabled={source || !at || at.code}
        onmousedown={(e) => e.preventDefault()}
        onclick={openLink}>{@render icon('link')}</button
      >
    </div>

    <div class="editor-group">
      {@render button(
        'bulletList',
        t.editor.bulletList,
        toggleList('bullet'),
        at?.list === 'bullet',
        'Ctrl+Maj+8',
      )}
      {@render button(
        'orderedList',
        t.editor.orderedList,
        toggleList('ordered'),
        at?.list === 'ordered',
        'Ctrl+Maj+7',
      )}
      {@render button('quote', t.editor.quote, toggleQuote, at?.quote ?? false, 'Ctrl+Maj+9')}
      {@render button(
        'codeBlock',
        t.editor.codeBlock,
        toggleCodeBlock,
        at?.code ?? false,
        'Ctrl+Alt+C',
      )}
      {@render button('table', t.editor.table, insertTable)}
      {@render button('rule', t.editor.rule, insertRule)}
      <button
        type="button"
        class="editor-button editor-at"
        aria-label={t.editor.mention}
        title={t.editor.mention}
        disabled={source || !at || at.code}
        onmousedown={(e) => e.preventDefault()}
        onclick={() =>
          run((state, dispatch) => {
            const before = state.selection.$from.nodeBefore?.text?.slice(-1) ?? '';
            dispatch?.(state.tr.insertText(before && !/\s/.test(before) ? ' @' : '@'));
            return true;
          })}><span aria-hidden="true">@</span></button
      >
    </div>

    <div class="editor-group editor-end">
      {@render button('undo', t.editor.undo, undo, null, 'Ctrl+Z')}
      {@render button('redo', t.editor.redo, redo, null, 'Ctrl+Maj+Z')}
      <button
        type="button"
        class="editor-button editor-source-toggle"
        aria-pressed={source}
        title={t.editor.sourceLabel}
        onclick={() => showSource(!source)}>{t.editor.source}</button
      >
    </div>

    {#if at?.table && !source}
      <div class="editor-group editor-context" role="group" aria-label={t.editor.tableTools}>
        <span class="editor-context-label" aria-hidden="true">{t.editor.tableTools}</span>
        {@render button('rowAfter', t.editor.rowAfter, addRowAfter)}
        {@render button('columnAfter', t.editor.columnAfter, addColumnAfter)}
        {@render button('deleteRow', t.editor.deleteRow, deleteRow)}
        {@render button('deleteColumn', t.editor.deleteColumn, deleteColumn)}
        {@render button('headerRow', t.editor.headerRow, toggleHeaderRow)}
        {@render button('deleteTable', t.editor.deleteTable, deleteTable)}
      </div>
    {/if}
    {#if at?.code && !source}
      <div class="editor-group editor-context">
        <label class="editor-context-label" for={`${id}-lang`}>{t.editor.language}</label>
        <input
          id={`${id}-lang`}
          class="input editor-language"
          list={`${id}-langs`}
          value={at.language}
          placeholder={t.editor.noLanguage}
          autocomplete="off"
          spellcheck="false"
          onchange={(e) => run(setLanguage(e.currentTarget.value))}
          onkeydown={(e) => {
            if (e.key === 'Enter') {
              e.preventDefault();
              run(setLanguage(e.currentTarget.value));
            }
          }}
        />
        <datalist id={`${id}-langs`}>
          {#each LANGUAGES as lang (lang)}<option value={lang}></option>{/each}
        </datalist>
      </div>
    {/if}
  </div>

  <div bind:this={host} class="editor-host" hidden={source}></div>

  {#if source}
    <textarea
      bind:this={sourceArea}
      bind:value
      class="textarea editor-source"
      aria-label={`${label} — ${t.editor.sourceLabel}`}
      spellcheck="false"
      {placeholder}
      {rows}
      onkeydown={sourceKeys}
      oninput={(e) => grow(e.currentTarget)}
      use:grow
    ></textarea>
  {/if}

  {#if mention && !source}
    <div
      class="popover editor-menu"
      role="listbox"
      aria-label={t.editor.people}
      style={placeAt(mention.from)}
    >
      {#each people as u, i (u.id)}
        <button
          type="button"
          class="item"
          class:active={i === picked}
          role="option"
          aria-selected={i === picked}
          onmousedown={(e) => e.preventDefault()}
          onclick={() => pick(u)}
        >
          <span class="name">{u.name}</span>
          <span class="muted">{u.username}{u.active ? '' : ` · ${t.common.inactive}`}</span>
        </button>
      {:else}
        <p class="empty editor-menu-empty">
          {searching ? t.editor.mentionSearching : t.editor.mentionEmpty}
        </p>
      {/each}
    </div>
  {/if}

  {#if linkAt && !source}
    <div
      class="popover editor-link"
      role="dialog"
      aria-label={t.editor.link}
      tabindex="-1"
      style={placeAt(linkAt.from)}
      onkeydown={linkKeys}
    >
      <form onsubmit={submitLink}>
        <label class="field">
          <span>{t.editor.linkUrl}</span>
          <!-- svelte-ignore a11y_autofocus -->
          <input
            class="input"
            bind:value={linkUrl}
            autofocus
            autocomplete="off"
            spellcheck="false"
            placeholder="https://…"
          />
        </label>
        <label class="field">
          <span>{t.editor.linkText}</span>
          <input class="input" bind:value={linkText} autocomplete="off" />
        </label>
        {#if linkError}<p class="error-text" role="alert">{linkError}</p>{/if}
        <div class="editor-link-actions">
          {#if linkAt.href}
            <button type="button" class="btn btn-small btn-quiet" onclick={unlink}
              >{t.editor.linkRemove}</button
            >
          {/if}
          <button
            type="button"
            class="btn btn-small btn-quiet"
            onclick={() => (closeLink(), view?.focus())}>{t.common.cancel}</button
          >
          <button type="submit" class="btn btn-small btn-primary">{t.editor.linkApply}</button>
        </div>
      </form>
    </div>
  {/if}

  {#if uploading.length > 0}
    <p class="editor-status muted" role="status">{t.editor.uploading(uploading.join(', '))}</p>
  {/if}
</div>
