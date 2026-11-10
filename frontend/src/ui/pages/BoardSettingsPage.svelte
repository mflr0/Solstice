<script lang="ts">
  import { tick } from 'svelte';
  import type { BoardDetail, BoardPatch, SectionStrategy } from '../../core/board-types.js';
  import { moveItem } from '../../core/catalogs.js';
  import { searchHref } from '../../core/filters-url.js';
  import { focusAfterRemoval } from '../../core/focus.js';
  import { paths } from '../../core/route.js';
  import type { StatusRef } from '../../core/types.js';
  import { t } from '../../i18n/t.js';
  import { pinnedHeight } from '../../lib/actions.js';
  import { api, errorMessage } from '../../lib/api.js';
  import { router } from '../../lib/router.svelte.js';
  import { session } from '../../lib/session.svelte.js';
  import { touchDrag, type TouchDragHandlers } from '../../lib/touch-drag.js';
  import { notify } from '../../lib/ui.svelte.js';
  import BoardHeader from '../board/BoardHeader.svelte';
  import Dropdown from '../Dropdown.svelte';
  import Modal from '../Modal.svelte';
  import MultiPick from '../MultiPick.svelte';
  import NotFoundPage from './NotFoundPage.svelte';

  /**
   * A board's settings: one save bar sends every changed part in one
   * patch, and the server's verdict (invalid JQL, a status in two
   * columns…) marks the row it names.
   */
  interface Props {
    boardId: number;
  }
  const { boardId }: Props = $props();

  type Part = 'general' | 'columns' | 'quick' | 'sections' | 'estimate';
  type ListPart = 'columns' | 'quick' | 'sections';
  const parts = ['general', 'columns', 'quick', 'sections', 'estimate'] as const;

  interface ColumnDraft {
    id?: number;
    name: string;
    statuses: StatusRef[];
  }
  interface QuickDraft {
    id?: number;
    name: string;
    query: string;
    description: string;
  }
  interface SectionDraft {
    id?: number;
    name: string;
    query: string;
    isDefault: boolean;
  }
  interface Drafts {
    general: { name: string; subQuery: string; doneCutoff: string };
    columns: ColumnDraft[];
    kanbanBacklog: boolean;
    quick: QuickDraft[];
    sections: { strategy: SectionStrategy; list: SectionDraft[] };
    estimate: string;
  }

  let detail = $state<BoardDetail | null>(null);
  let error = $state('');
  let draft = $state<Drafts | null>(null);
  let saveError = $state('');
  /** The parts the failed save sent: only their rows can be the ones it names. */
  let failed = $state<Part[]>([]);
  let saving = $state(false);
  let pool = $state<StatusRef[]>([]);
  let deleting = $state(false);
  let deleteBusy = $state(false);
  let drag = $state<{ part: ListPart; from: number } | null>(null);
  let over = $state<number | null>(null);
  /** What the last move says to a screen reader. */
  let moved = $state('');

  function fromDetail(d: BoardDetail): Drafts {
    return {
      general: {
        name: d.board.name,
        subQuery: d.board.subQuery ?? '',
        doneCutoff: d.board.doneCutoff ?? '',
      },
      columns: d.columns.map((c) => ({
        id: c.id,
        name: c.name,
        statuses: [...c.statuses],
      })),
      kanbanBacklog: d.board.kanbanBacklog,
      quick: d.quickFilters.map((q) => ({
        id: q.id,
        name: q.name,
        query: q.query ?? '',
        description: q.description ?? '',
      })),
      sections: {
        strategy: d.board.sectionBy,
        list: d.sections.map((l) => ({
          id: l.id,
          name: l.name,
          query: l.query ?? '',
          isDefault: !!l.default,
        })),
      },
      estimate: d.board.estimateField ?? '',
    };
  }

  $effect(() => {
    api
      .board(boardId)
      .then((d) => {
        detail = d;
        draft = fromDetail(d);
        document.title = `${t.boardSettings.title} — ${d.board.name} — ${t.appName}`;
        void loadPool(d);
      })
      .catch((e: unknown) => (error = errorMessage(e)));
  });

  /** The statuses the board's projects use, to offer in the columns. */
  async function loadPool(d: BoardDetail) {
    // eslint-disable-next-line svelte/prefer-svelte-reactivity -- a throwaway value, not state
    const seen = new Map<string, StatusRef>();
    try {
      const lists = await Promise.all(d.projects.map((p) => api.boardProjectStatuses(p.key)));
      for (const l of lists)
        for (const w of l.workflows) for (const s of w.statuses) seen.set(s.id, s);
    } catch {
      // Every status then: the columns can still be set up.
      for (const s of session.meta?.statuses ?? []) seen.set(s.id, s);
    }
    pool = [...seen.values()];
  }

  const saved = $derived(detail ? fromDetail(detail) : null);
  const dirty = $derived.by(() => {
    const out: Partial<Record<Part, boolean>> = {};
    if (!draft || !saved) return out;
    for (const part of parts) {
      out[part] = JSON.stringify(draft[part]) !== JSON.stringify(saved[part]);
    }
    out.columns ||= backlogAllowed && draft.kanbanBacklog !== saved.kanbanBacklog;
    return out;
  });
  const changed = $derived(parts.filter((p) => dirty[p]));
  /** Undoing every change hides the bar, and with it the error. */
  const shownError = $derived(changed.length ? saveError : '');

  function errorIn(part: Part): string {
    return failed.includes(part) ? shownError : '';
  }

  /** A Kanban backlog takes the first column: the board keeps the others. */
  const backlogAllowed = $derived(
    !!draft && draft.columns.length >= 2 && (draft.columns[0]?.statuses.length ?? 0) > 0,
  );

  const mapped = $derived(new Set(draft?.columns.flatMap((c) => c.statuses.map((s) => s.id))));
  const unmapped = $derived(byRank(pool.filter((s) => !mapped.has(s.id))));

  /** The admin order of the statuses (`statuses.position`); the pool's order without it. */
  const rank = $derived(new Map((session.meta?.statuses ?? pool).map((s, i) => [s.id, i])));
  /** What a column can pick: the pool, and the statuses columns hold outside it. */
  const known = $derived.by(() => {
    // eslint-disable-next-line svelte/prefer-svelte-reactivity -- a throwaway value, not state
    const all = new Map(pool.map((s) => [s.id, s]));
    for (const c of draft?.columns ?? [])
      for (const s of c.statuses) if (!all.has(s.id)) all.set(s.id, s);
    return byRank([...all.values()]);
  });

  function byRank(list: StatusRef[]): StatusRef[] {
    const at = (s: StatusRef) => rank.get(s.id) ?? rank.size;
    return [...list].sort((a, b) => at(a) - at(b));
  }

  function choices(column: ColumnDraft) {
    return known.map((s) => {
      const other = draft?.columns.find(
        (c) => c !== column && c.statuses.some((x) => x.id === s.id),
      );
      return {
        id: s.id,
        name: s.name,
        note: other ? t.boardSettings.inColumn(other.name) : undefined,
      };
    });
  }

  /**
   * The list the server reads in order: the status on drop first, then the
   * others in status order. A card whose workflow can't reach the first
   * gets the next it can.
   */
  function ordered(drop: string | undefined, list: StatusRef[]): StatusRef[] {
    const first = list.find((s) => s.id === drop);
    const rest = byRank(list.filter((s) => s !== first));
    return first ? [first, ...rest] : rest;
  }

  /** Sets a column's statuses; one another column holds moves here. */
  function pick(column: ColumnDraft, ids: string[]) {
    const chosen = known.filter((s) => ids.includes(s.id));
    for (const c of draft?.columns ?? [])
      if (c !== column && c.statuses.some((s) => ids.includes(s.id)))
        c.statuses = c.statuses.filter((s) => !ids.includes(s.id));
    column.statuses = ordered(column.statuses[0]?.id, chosen);
  }

  function patchFor(part: Part, d: Drafts, kanban: boolean): BoardPatch {
    switch (part) {
      case 'general':
        return {
          name: d.general.name,
          ...(kanban ? { subQuery: d.general.subQuery, doneCutoff: d.general.doneCutoff } : {}),
        };
      case 'columns':
        return {
          ...(kanban ? { kanbanBacklog: backlogAllowed && d.kanbanBacklog } : {}),
          columns: d.columns.map((c) => ({
            ...(c.id ? { id: c.id } : {}),
            name: c.name,
            statuses: c.statuses.map((s) => s.id),
          })),
        };
      case 'quick':
        return {
          quickFilters: d.quick.map((q) => ({
            ...(q.id ? { id: q.id } : {}),
            name: q.name,
            query: q.query,
            ...(q.description.trim() ? { description: q.description } : {}),
          })),
        };
      case 'sections':
        return {
          sectionBy: d.sections.strategy,
          ...(d.sections.strategy === 'custom'
            ? {
                sections: d.sections.list.map((l) => ({
                  ...(l.id ? { id: l.id } : {}),
                  name: l.name,
                  ...(l.query.trim() ? { query: l.query } : {}),
                  default: l.isDefault,
                })),
              }
            : {}),
        };
      case 'estimate':
        return { estimate: d.estimate };
    }
  }

  async function save(e: SubmitEvent) {
    e.preventDefault();
    if (!detail || !draft || saving || !changed.length) return;
    const sent = changed;
    const kanban = detail.board.kind === 'kanban';
    saving = true;
    saveError = '';
    failed = [];
    try {
      const patch = Object.assign(
        {},
        ...sent.map((p) => patchFor(p, draft!, kanban)),
      ) as BoardPatch;
      detail = await api.saveBoard(detail.board.id, patch);
      draft = fromDetail(detail);
      notify(t.boardSettings.saved);
    } catch (err) {
      saveError = errorMessage(err);
      failed = sent;
    } finally {
      saving = false;
    }
  }

  function discard() {
    if (!saved) return;
    draft = saved;
    saveError = '';
    failed = [];
  }

  function listOf(part: ListPart): { name: string }[] {
    if (!draft) return [];
    if (part === 'columns') return draft.columns;
    return part === 'quick' ? draft.quick : draft.sections.list;
  }

  /** Moves a row of the draft; the save bar sends the new order. */
  function reorder(part: ListPart, from: number, to: number): boolean {
    const list = listOf(part);
    if (from === to || to < 0 || to >= list.length) return false;
    list.splice(0, list.length, ...moveItem(list, from, to));
    moved = t.boardSettings.moved(list[to]!.name, to + 1, list.length);
    return true;
  }

  /**
   * Alt+arrows on a grip move its row one place, and focus follows it. By
   * index: an unsaved row's key changes with its place.
   */
  async function gripKey(e: KeyboardEvent, part: ListPart, i: number) {
    if (!e.altKey || e.ctrlKey || e.metaKey || e.shiftKey) return;
    const [back, forward] =
      part === 'columns' ? ['ArrowLeft', 'ArrowRight'] : ['ArrowUp', 'ArrowDown'];
    const by = e.key === back ? -1 : e.key === forward ? 1 : 0;
    if (!by) return;
    // Alt+← would otherwise go back in the history.
    e.preventDefault();
    if (!reorder(part, i, i + by)) return;
    await tick();
    document.getElementById(`${part}-grip-${i + by}`)?.focus();
  }

  const isDragged = (part: ListPart, i: number) => drag?.part === part && drag.from === i;
  const isOver = (part: ListPart, i: number) =>
    drag?.part === part && drag.from !== i && over === i;

  function land(to: number | null) {
    if (drag && to !== null) reorder(drag.part, drag.from, to);
    drag = null;
    over = null;
  }

  /** The row of the dragged row's list under a point, if any. */
  function rowAt(x: number, y: number): number | null {
    const row = document.elementFromPoint(x, y)?.closest<HTMLElement>('[data-part]');
    if (!row || row.dataset['part'] !== drag?.part) return null;
    return Number(row.dataset['index']);
  }

  const touch: TouchDragHandlers = {
    selector: '[data-part]',
    handle: '.grip',
    start(el) {
      drag = { part: el.dataset['part'] as ListPart, from: Number(el.dataset['index']) };
      return true;
    },
    move(x, y) {
      over = rowAt(x, y);
    },
    drop(x, y) {
      land(rowAt(x, y));
    },
    cancel() {
      drag = null;
      over = null;
    },
  };

  /** A row's mouse drop handlers. */
  function dropRow(part: ListPart, i: number) {
    return {
      'data-part': part,
      'data-index': i,
      ondragover: (e: DragEvent) => {
        if (drag?.part !== part) return;
        e.preventDefault();
        over = i;
      },
      ondrop: (e: DragEvent) => {
        e.preventDefault();
        land(i);
      },
    };
  }

  /** Removes a row; focus goes to the delete button of the next row, else the previous, else add. */
  async function removeRow<T>(part: ListPart, list: T[], i: number) {
    const next = focusAfterRemoval(list, list[i]!);
    list.splice(i, 1);
    await tick();
    // The last column can't be deleted: its button is disabled.
    const button =
      next === undefined
        ? null
        : document.querySelector<HTMLElement>(`#${part}-delete-${list.indexOf(next)}:enabled`);
    (button ?? document.getElementById(`${part}-add`))?.focus();
  }

  /** The row an error message names (« name », “name”, „name“…), to mark it. */
  function named(part: Part, name: string): boolean {
    const message = errorIn(part);
    if (!name || !message) return false;
    return [`« ${name} »`, `«${name}»`, `“${name}”`, `„${name}“`].some((q) => message.includes(q));
  }

  async function remove() {
    if (!detail || deleteBusy) return;
    deleteBusy = true;
    try {
      await api.deleteBoard(detail.board.id);
      notify(t.boardSettings.deleted(detail.board.name));
      deleting = false;
      router.navigate(paths.boards());
    } catch (err) {
      notify(errorMessage(err), 'error');
    } finally {
      deleteBusy = false;
    }
  }
</script>

{#if error}
  <NotFoundPage message={error} />
{:else if !detail || !draft}
  <div class="page"><p class="muted loading">{t.common.loading}</p></div>
{:else if !detail.can.manage}
  <div class="page">
    <BoardHeader {detail} view="settings" />
    <p class="empty">{t.boardSettings.forbidden}</p>
  </div>
{:else}
  {@const d = detail}
  {@const f = draft}
  {@const kanban = d.board.kind === 'kanban'}
  <div class="page" data-title={`${t.boardSettings.title} — ${d.board.name}`}>
    <BoardHeader detail={d} view="settings" />
    <p class="eyebrow">{t.boardSettings.title}</p>

    <form onsubmit={save} use:touchDrag={touch}>
      <!-- General -->
      <section class="part" aria-labelledby="part-general">
        <h2 class="section-title" id="part-general">{t.boardSettings.general}</h2>
        <div class="fields">
          <label class="field">
            <span>{t.boardSettings.name}</span>
            <input class="input" bind:value={f.general.name} required maxlength="255" />
          </label>
          <div class="field">
            <span class="field-label">{t.boardSettings.filter}</span>
            <code class="query">{d.board.query}</code>
            <span class="hint"
              >{t.boardSettings.filterHint}
              <a href={searchHref(d.board.query)}>{t.boardSettings.seeIssues}</a></span
            >
          </div>
          {#if kanban}
            <label class="field">
              <span>{t.boardSettings.subQuery}</span>
              <input
                class="input mono"
                bind:value={f.general.subQuery}
                spellcheck="false"
                aria-invalid={errorIn('general').startsWith(t.boardSettings.subQuery) || undefined}
              />
              <span class="hint">{t.boardSettings.subQueryHint}</span>
            </label>
            <label class="field narrow">
              <span>{t.boardSettings.doneCutoff}</span>
              <input class="input mono" bind:value={f.general.doneCutoff} placeholder="2w" />
              <span class="hint">{t.boardSettings.doneCutoffHint}</span>
            </label>
          {/if}
        </div>
      </section>

      <!-- Columns -->
      <section class="part" aria-labelledby="part-columns">
        <h2 class="section-title" id="part-columns">
          {t.boardSettings.columns} <span class="count">{f.columns.length}</span>
        </h2>
        <p class="hint intro">{t.boardSettings.columnsHint}</p>
        {#if kanban}
          <label class="check">
            <input
              class="checkbox"
              type="checkbox"
              checked={f.kanbanBacklog && backlogAllowed}
              disabled={!backlogAllowed}
              onchange={(e) => (f.kanbanBacklog = e.currentTarget.checked)}
            />
            <span>
              {t.boardSettings.kanbanBacklog}
              <span class="hint"
                >{backlogAllowed
                  ? t.boardSettings.kanbanBacklogHint
                  : t.boardSettings.kanbanBacklogNeeds}</span
              >
            </span>
          </label>
        {/if}
        <ol class="columns">
          {#each f.columns as column, i (column.id ?? `new-${i}`)}
            <li
              class="card column-edit"
              class:is-dragged={isDragged('columns', i)}
              class:is-over={isOver('columns', i)}
              {...dropRow('columns', i)}
            >
              <div class="card-head">
                {@render grip('columns', i, column.name)}
                <span class="sr-only">{i + 1}.</span>
                <input
                  id="columns-name-{i}"
                  class="input name"
                  bind:value={column.name}
                  required
                  maxlength="255"
                  aria-label={t.boardSettings.columnName}
                />
                {#if kanban && i === 0 && f.kanbanBacklog && backlogAllowed}
                  <span class="chip">{t.boards.backlog}</span>
                {/if}
              </div>
              <div class="column-body">
                <div class="field">
                  <span class="field-label">{t.boardSettings.statuses}</span>
                  <MultiPick
                    field
                    options={choices(column)}
                    selected={column.statuses.map((s) => s.id)}
                    label={t.boardSettings.statusesOf(column.name)}
                    onchange={(ids) => pick(column, ids)}
                  />
                  {#if !column.statuses.length}
                    <span class="hint">{t.boardSettings.noStatus}</span>
                  {/if}
                </div>
                <div class="field">
                  <span class="field-label">{t.boardSettings.dropStatus}</span>
                  <Dropdown
                    label={t.boardSettings.dropStatusOf(column.name)}
                    value={column.statuses[0]?.id ?? ''}
                    options={column.statuses.map((s) => ({ value: s.id, label: s.name }))}
                    placeholder={t.common.none}
                    disabled={!column.statuses.length}
                    onchange={(v) => (column.statuses = ordered(v, column.statuses))}
                  />
                </div>
                <div class="tools">
                  <button
                    id="columns-delete-{i}"
                    type="button"
                    class="btn btn-small btn-danger"
                    disabled={f.columns.length === 1}
                    onclick={() => void removeRow('columns', f.columns, i)}
                    aria-label={t.boardSettings.remove(column.name)}>{t.common.delete}</button
                  >
                </div>
              </div>
            </li>
          {/each}
          <li class="add-column">
            <button
              id="columns-add"
              type="button"
              class="btn"
              onclick={() => f.columns.push({ name: t.boardSettings.newColumn, statuses: [] })}
              >{t.boardSettings.addColumn}</button
            >
          </li>
        </ol>

        {#if unmapped.length}
          <p class="hint unmapped">
            {t.boardSettings.unmapped(unmapped.map((s) => s.name).join(', '))}
          </p>
        {/if}
      </section>

      <!-- Quick filters -->
      <section class="part" aria-labelledby="part-quick">
        <h2 class="section-title" id="part-quick">
          {t.boardSettings.quickFilters} <span class="count">{f.quick.length}</span>
        </h2>
        <p class="hint intro">{t.boardSettings.quickFiltersHint}</p>
        {#if f.quick.length === 0}<p class="empty">{t.boardSettings.noQuickFilters}</p>{/if}
        <ol class="rows">
          {#each f.quick as q, i (q.id ?? `new-${i}`)}
            <li
              class="row-edit"
              class:invalid={named('quick', q.name)}
              class:is-dragged={isDragged('quick', i)}
              class:is-over={isOver('quick', i)}
              {...dropRow('quick', i)}
            >
              {@render grip('quick', i, q.name)}
              <label class="field">
                <span>{t.boardSettings.name}</span>
                <input class="input" bind:value={q.name} required maxlength="255" />
              </label>
              <label class="field wide">
                <span>{t.boardSettings.query}</span>
                <input
                  class="input mono"
                  bind:value={q.query}
                  spellcheck="false"
                  aria-invalid={named('quick', q.name) || undefined}
                />
              </label>
              <label class="field">
                <span>{t.boardSettings.description}</span>
                <input class="input" bind:value={q.description} maxlength="255" />
              </label>
              {@render remover('quick', f.quick, i, q.name)}
            </li>
          {/each}
        </ol>
        <button
          id="quick-add"
          type="button"
          class="btn"
          onclick={() =>
            f.quick.push({ name: t.boardSettings.newQuickFilter, query: '', description: '' })}
          >{t.boardSettings.addQuickFilter}</button
        >
      </section>

      <!-- Sections -->
      <section class="part" aria-labelledby="part-sections">
        <h2 class="section-title" id="part-sections">{t.boardSettings.sections}</h2>
        <p class="hint intro">{t.boardSettings.sectionsHint}</p>
        <fieldset class="choices">
          <legend class="sr-only">{t.boardSettings.sections}</legend>
          {#each ['none', 'custom', 'assignee', 'epic', 'parent'] as const as s (s)}
            <label class="choice">
              <input type="radio" name="strategy" value={s} bind:group={f.sections.strategy} />
              <span>
                <strong>{t.boardSettings.strategies[s]}</strong>
                <span class="hint">{t.boardSettings.strategyHints[s]}</span>
              </span>
            </label>
          {/each}
        </fieldset>
        {#if f.sections.strategy === 'custom'}
          <h3 class="sub">{t.boardSettings.querySections}</h3>
          <p class="hint">{t.boardSettings.defaultSectionHint}</p>
          {#if f.sections.list.length === 0}<p class="empty">{t.boardSettings.noSections}</p>{/if}
          <ol class="rows">
            {#each f.sections.list as section, i (section.id ?? `new-${i}`)}
              <li
                class="row-edit"
                class:invalid={named('sections', section.name)}
                class:is-dragged={isDragged('sections', i)}
                class:is-over={isOver('sections', i)}
                {...dropRow('sections', i)}
              >
                {@render grip('sections', i, section.name)}
                <label class="field">
                  <span>{t.boardSettings.name}</span>
                  <input class="input" bind:value={section.name} required maxlength="255" />
                </label>
                <label class="field wide">
                  <span>{t.boardSettings.query}</span>
                  <input
                    class="input mono"
                    bind:value={section.query}
                    spellcheck="false"
                    disabled={section.isDefault}
                    aria-invalid={named('sections', section.name) || undefined}
                  />
                </label>
                <label class="check">
                  <input
                    type="radio"
                    name="default-section"
                    checked={section.isDefault}
                    onchange={() => f.sections.list.forEach((l, j) => (l.isDefault = j === i))}
                  />
                  {t.boardSettings.defaultSection}
                </label>
                {@render remover('sections', f.sections.list, i, section.name)}
              </li>
            {/each}
          </ol>
          <button
            id="sections-add"
            type="button"
            class="btn"
            onclick={() =>
              f.sections.list.push({
                name: t.boardSettings.newSection,
                query: '',
                isDefault: false,
              })}>{t.boardSettings.addSection}</button
          >
        {/if}
      </section>
      <p class="sr-only" aria-live="polite">{moved}</p>

      <!-- Estimation -->
      <section class="part" aria-labelledby="part-estimate">
        <h2 class="section-title" id="part-estimate">{t.boardSettings.estimation}</h2>
        <p class="hint intro">{t.boardSettings.estimationHint}</p>
        <fieldset class="choices">
          <legend class="sr-only">{t.boardSettings.estimation}</legend>
          <label class="choice">
            <input type="radio" name="estimate" value="" bind:group={f.estimate} />
            <span><strong>{t.boardSettings.estimateNone}</strong></span>
          </label>
          <label class="choice">
            <input
              type="radio"
              name="estimate"
              value="timeoriginalestimate"
              bind:group={f.estimate}
            />
            <span><strong>{t.boardSettings.estimateTime}</strong></span>
          </label>
          <div class="choice">
            <input
              type="radio"
              name="estimate"
              id="estimate-field"
              checked={f.estimate.startsWith('customfield_')}
              disabled={d.estimateFields.length === 0}
              onchange={() => (f.estimate = d.estimateFields[0]?.id ?? '')}
            />
            <span class="field-choice">
              <label for="estimate-field"><strong>{t.boardSettings.estimateField}</strong></label>
              {#if d.estimateFields.length}
                <Dropdown
                  label={t.boardSettings.estimateField}
                  value={f.estimate.startsWith('customfield_') ? f.estimate : ''}
                  options={d.estimateFields.map((field) => ({
                    value: field.id,
                    label: field.name,
                  }))}
                  placeholder={t.boardSettings.chooseField}
                  onchange={(v) => (f.estimate = v)}
                />
              {:else}
                <span class="hint">{t.boardSettings.noFields}</span>
              {/if}
            </span>
          </div>
        </fieldset>
      </section>

      {#if changed.length}
        <div
          class="savebar"
          role="region"
          aria-label={t.boardSettings.unsaved}
          use:pinnedHeight={'--savebar-height'}
        >
          <span>{t.boardSettings.unsaved}</span>
          <button type="button" class="btn btn-quiet" disabled={saving} onclick={discard}
            >{t.boardSettings.discard}</button
          >
          <button type="submit" class="btn btn-primary" disabled={saving}
            >{saving ? t.common.saving : t.boardSettings.save}</button
          >
          {#if shownError}<p class="error-text" role="alert">{shownError}</p>{/if}
        </div>
      {/if}
    </form>

    <!-- Deleting -->
    <section class="part" aria-labelledby="part-danger">
      <h2 class="section-title" id="part-danger">{t.boardSettings.danger}</h2>
      <div class="bar">
        <p class="hint">{t.boardSettings.dangerHint}</p>
        <span class="grow"></span>
        <button type="button" class="btn btn-danger" onclick={() => (deleting = true)}
          >{t.boardSettings.deleteBoard}</button
        >
      </div>
    </section>
  </div>

  <Modal open={deleting} title={t.boardSettings.deleteTitle} onclose={() => (deleting = false)}>
    <p>{t.boardSettings.deleteConfirm(d.board.name)}</p>
    {#snippet footer()}
      <button type="button" class="btn btn-quiet" onclick={() => (deleting = false)}
        >{t.common.cancel}</button
      >
      <button
        type="button"
        class="btn btn-primary"
        disabled={deleteBusy}
        onclick={() => void remove()}>{t.boardSettings.deleteBoard}</button
      >
    {/snippet}
  </Modal>
{/if}

{#snippet grip(part: ListPart, i: number, name: string)}
  {@const keys = part === 'columns' ? 'Alt+ArrowLeft Alt+ArrowRight' : 'Alt+ArrowUp Alt+ArrowDown'}
  <button
    id="{part}-grip-{i}"
    type="button"
    class="grip"
    draggable="true"
    aria-label={t.boardSettings.reorder(name)}
    aria-keyshortcuts={keys}
    title={part === 'columns' ? t.boardSettings.reorderColumn : t.boardSettings.reorderRow}
    onkeydown={(e) => void gripKey(e, part, i)}
    ondragstart={(e) => {
      drag = { part, from: i };
      e.dataTransfer?.setData('text/plain', name);
      const row = e.currentTarget.closest('li');
      if (row) e.dataTransfer?.setDragImage(row, 12, 12);
    }}
    ondragend={() => ((drag = null), (over = null))}>⋮⋮</button
  >
{/snippet}

{#snippet remover(part: ListPart, list: { name: string }[], i: number, name: string)}
  <div class="tools">
    <button
      id="{part}-delete-{i}"
      type="button"
      class="btn btn-small btn-danger"
      aria-label={t.boardSettings.remove(name)}
      onclick={() => void removeRow(part, list, i)}>{t.common.delete}</button
    >
  </div>
{/snippet}

<style>
  .part {
    margin-bottom: 8px;
  }

  .fields {
    display: grid;
    gap: 14px;
    max-width: 45rem;
  }

  .narrow {
    max-width: 20rem;
  }

  .mono {
    font-family: var(--font-mono);
    font-size: 0.8125rem;
  }

  .query {
    display: block;
    padding: 6px 8px;
    background: var(--paper-sunk);
    border: 1px solid var(--rule);
    font-size: 0.78125rem;
    overflow-wrap: anywhere;
  }

  .hint {
    font-size: 0.78125rem;
    color: var(--muted);
    margin: 0;
  }

  .intro {
    margin: -2px 0 12px;
  }

  .check {
    display: flex;
    gap: 8px;
    align-items: baseline;
    margin: 0 0 14px;
    line-height: 1.4;
  }

  .check .hint {
    display: block;
    white-space: normal;
  }

  .sub {
    margin: 18px 0 6px;
    font: 600 0.8125rem/1.2 var(--font-body);
  }

  .bar {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px 12px;
    margin-top: 16px;
  }

  .savebar .error-text {
    flex-basis: 100%;
    margin: 0;
  }

  /* Columns, side by side like on the board. */
  .columns {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(13.75rem, 1fr));
    gap: 12px;
    margin: 0;
    padding: 0;
    list-style: none;
  }

  .column-edit {
    display: grid;
    align-content: start;
    min-width: 0;
  }

  .column-edit .card-head {
    gap: 6px;
    align-items: center;
    padding: 8px;
  }

  .column-edit .name {
    flex: 1;
    min-width: 0;
    font-weight: 600;
  }

  .column-body {
    display: grid;
    gap: 12px;
    padding: 10px;
  }

  .tools {
    display: flex;
    gap: 4px;
    align-items: center;
  }

  .tools .btn-danger {
    margin-left: auto;
  }

  .add-column {
    display: grid;
    place-items: center;
    min-height: 7.5rem;
    border: 1px dashed var(--rule-strong);
    border-radius: var(--radius-m);
  }

  .unmapped {
    margin-top: 12px;
  }

  /* Quick filters and sections: one row each. */
  .rows {
    display: grid;
    gap: 10px;
    margin: 0 0 12px;
    padding: 0;
    list-style: none;
  }

  .row-edit {
    display: grid;
    grid-template-columns:
      auto minmax(8.75rem, 1fr) minmax(13.75rem, 2fr) minmax(8.75rem, 1fr)
      auto;
    gap: 8px;
    align-items: end;
    padding-bottom: 10px;
    border-bottom: 1px solid var(--rule);
  }

  .grip {
    align-self: center;
    min-width: 1.5rem;
    min-height: 1.5rem;
    padding: 4px;
    border: none;
    border-radius: var(--radius-s);
    background: none;
    color: var(--muted);
    font-size: 0.6875rem;
    letter-spacing: -2px;
    cursor: grab;
    -webkit-user-select: none;
    user-select: none;
  }

  .grip:hover {
    color: var(--ink);
  }

  .is-dragged {
    opacity: 0.45;
  }

  .column-edit.is-over {
    box-shadow: inset 2px 0 0 var(--accent);
  }

  .row-edit.is-over {
    box-shadow: inset 0 2px 0 var(--accent);
  }

  /* The copy of a row that follows the finger while it is dragged by touch. */
  :global(body > .touch-ghost[data-part]) {
    position: fixed;
    top: 0;
    left: 0;
    z-index: 70;
    margin: 0;
    list-style: none;
    pointer-events: none;
    background: var(--paper-raised);
    box-shadow: var(--shadow-pop);
  }

  .row-edit.invalid .mono {
    border-color: var(--accent);
  }

  .check {
    display: flex;
    gap: 6px;
    align-items: center;
    padding-bottom: 6px;
    white-space: nowrap;
    font-size: 0.8125rem;
  }

  .check input,
  .choice input {
    accent-color: var(--accent);
  }

  .choices {
    display: grid;
    gap: 8px;
    margin: 0;
    padding: 0;
    border: none;
  }

  .choice {
    display: flex;
    gap: 10px;
    align-items: baseline;
  }

  .choice > span {
    display: grid;
    gap: 2px;
  }

  .choice > .field-choice {
    display: flex;
    flex-wrap: wrap;
    gap: 8px 12px;
    align-items: baseline;
  }

  .field-choice :global(.select) {
    width: auto;
    min-width: 12.5rem;
  }

  @media (max-width: 760px) {
    .row-edit {
      grid-template-columns: 1fr;
    }

    .row-edit .tools {
      justify-content: flex-start;
    }

    .row-edit .grip {
      justify-self: start;
    }
  }
</style>
