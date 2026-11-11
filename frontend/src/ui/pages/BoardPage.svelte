<script lang="ts">
  import { tick } from 'svelte';
  import {
    applyMove,
    boardStep,
    cardFacets,
    dropIndex,
    groupSections,
    neighbours,
    parseQuick,
    readBoardSearch,
    staysInPlace,
    stepMove,
    type CardSpot,
    type Section,
  } from '../../core/board.js';
  import type {
    BoardCard as Card,
    BoardColumn,
    BoardDetail,
    CardMove,
    Sprint,
  } from '../../core/board-types.js';
  import { filtersToJql, QUICK_FIELDS, type FilterState } from '../../core/filters.js';
  import { paths } from '../../core/route.js';
  import { shortcutFor } from '../../core/shortcuts.js';
  import type { IssueBrief } from '../../core/types.js';
  import { t } from '../../i18n/t.js';
  import { hiddenSides } from '../../lib/actions.js';
  import { touchDrag, type TouchDragHandlers } from '../../lib/touch-drag.js';
  import { api, errorMessage } from '../../lib/api.js';
  import { debounce, live } from '../../lib/live.svelte.js';
  import { router } from '../../lib/router.svelte.js';
  import { appearance, ignoreShortcut, mac, notify, openEdit } from '../../lib/ui.svelte.js';
  import BoardCard from '../board/BoardCard.svelte';
  import BoardHeader from '../board/BoardHeader.svelte';
  import BoardRail from '../board/BoardRail.svelte';
  import type { MenuGroup } from '../board/CardMenu.svelte';
  import CompleteSprintDialog from '../board/CompleteSprintDialog.svelte';
  import EditSprintDialog from '../board/EditSprintDialog.svelte';
  import { issueGroup } from '../board/issue-menu.js';
  import BoardQuickFilters from '../board/QuickFilters.svelte';
  import CloneDialog from '../issue/CloneDialog.svelte';
  import FilterBar from '../search/FilterBar.svelte';
  import QuickFilters from '../search/QuickFilters.svelte';
  import SprintHead from '../board/SprintHead.svelte';
  import Illustration from '../Illustration.svelte';
  import TypeMark from '../TypeMark.svelte';
  import Stamp from '../Stamp.svelte';
  import NotFoundPage from './NotFoundPage.svelte';

  /**
   * A board: its columns side by side, optionally cut into sections.
   * Cards are dragged between columns (a workflow transition) and within
   * one (their rank), with the mouse or by holding a card by touch; each
   * card's menu does the same from the keyboard.
   */
  interface Props {
    boardId: number;
  }
  const { boardId }: Props = $props();

  let detail = $state<BoardDetail | null>(null);
  let error = $state('');
  let cards = $state<Card[] | null>(null);
  let cardsError = $state('');
  let duplicating = $state<string | null>(null);
  let sprint = $state<Sprint | null>(null);
  let truncated = $state(false);
  let announcement = $state('');
  let collapsed = $state<Record<string, boolean>>({});
  let completing = $state<Sprint | null>(null);
  let editing = $state<Sprint | null>(null);
  // Columns out of sight on either side of the board, each shown by a shade.
  let moreLeft = $state(false);
  let moreRight = $state(false);

  const quick = $derived(parseQuick(router.param('quick')));
  const quickParam = $derived(quick.join(','));
  const search = $derived(readBoardSearch(router.param('jql')));
  const filtered = $derived(quick.length > 0 || search.jql !== undefined);

  // One history entry per interaction with the filter row, as on the search page.
  let lastBurst: string | null = null;
  function applyFilters(next: FilterState, burst: string) {
    const replace = burst === lastBurst;
    lastBurst = burst;
    router.setParams({ jql: filtersToJql(next) }, { replace });
  }

  function loadDetail() {
    api
      .board(boardId)
      .then((d) => {
        detail = d;
        document.title = `${d.board.name} — ${t.appName}`;
      })
      .catch((e: unknown) => (error = errorMessage(e)));
  }

  $effect(() => {
    loadDetail();
  });

  // Moves in flight: a reload meanwhile would show the card back where it was.
  let pending = 0;
  let seq = 0;

  async function loadCards(id: number, q: string, jql: string | undefined) {
    if (pending > 0) return;
    const mine = ++seq;
    try {
      const res = await api.boardCards(id, { quick: q || undefined, jql });
      if (mine !== seq || pending > 0) return;
      cards = res.cards;
      sprint = res.sprint;
      truncated = res.truncated;
      cardsError = '';
    } catch (e) {
      if (mine === seq) cardsError = errorMessage(e);
    }
  }

  /** The cards without the quick filter row's fields, when it has some: they give the row its values. */
  let rowCards = $state<Card[] | null>(null);
  let rowSeq = 0;

  async function loadRowCards(id: number, q: string, jql: string | undefined) {
    const mine = ++rowSeq;
    try {
      const res = await api.boardCards(id, { quick: q || undefined, jql });
      if (mine === rowSeq) rowCards = res.cards;
    } catch {
      // The row shows oneself only.
    }
  }

  function reload() {
    void loadCards(boardId, quickParam, search.jql);
    if (search.rowJql !== search.jql) void loadRowCards(boardId, quickParam, search.rowJql);
  }

  $effect(() => {
    void loadCards(boardId, quickParam, search.jql);
  });

  $effect(() => {
    if (search.rowJql !== search.jql) void loadRowCards(boardId, quickParam, search.rowJql);
  });

  $effect(() => live.onChange(debounce(reload, 400)));

  /** A Kanban backlog's column: its issues are on the backlog page, not here. */
  const backlogColumn = $derived(detail?.board.kanbanBacklog ? detail.columns[0] : undefined);
  const columns = $derived(detail ? detail.columns.filter((c) => c.id !== backlogColumn?.id) : []);

  const rowFacets = $derived.by(() => {
    const source = search.rowJql === search.jql ? cards : rowCards;
    return source ? cardFacets(source.filter((c) => c.column !== backlogColumn?.id)) : null;
  });

  const sections = $derived(
    detail && cards ? groupSections(cards, detail.board.sectionBy, detail.sections) : [],
  );
  const banded = $derived(sections.length > 0 && sections[0]?.kind !== 'all');
  const counts = $derived.by(() => {
    const out: Record<number, number> = {};
    for (const section of sections)
      for (const c of section.cards) out[c.column] = (out[c.column] ?? 0) + 1;
    return out;
  });
  const shownCount = $derived(sections.reduce((n, l) => n + l.cards.length, 0));
  const futureSprints = $derived(detail?.sprints.filter((s) => s.state === 'future') ?? []);

  function sectionName(section: Section): string {
    switch (section.kind) {
      case 'custom':
        return section.name;
      case 'assignee':
        return section.user?.name ?? t.boards.unassigned;
      case 'epic':
        return section.issue ? `${section.issue.key} ${section.issue.summary}` : t.boards.noEpic;
      case 'parent':
        return `${section.issue.key} ${section.issue.summary}`;
      default:
        return t.boards.otherIssues;
    }
  }

  /** The issue an epic or parent section stands for. */
  function sectionIssue(section: Section): IssueBrief | null {
    return section.kind === 'epic' || section.kind === 'parent' ? section.issue : null;
  }

  function cell(section: Section, column: BoardColumn): Card[] {
    return section.cards.filter((c) => c.column === column.id);
  }

  // Rows of the board grid: the column heads, then for each section its band
  // (when sections are shown) and its cells.
  const rows = $derived(1 + (banded ? 2 : 1) * Math.max(sections.length, 1));
  const bandRow = (k: number) => 2 + 2 * k;
  const cellRow = (k: number) => (banded ? 3 + 2 * k : 2 + k);

  // Moving cards ---------------------------------------------------------------

  async function move(m: CardMove, message: string, refocus = false) {
    if (!cards) return;
    const before = cards;
    cards = applyMove(cards, m).filter((c) => c.column !== backlogColumn?.id);
    pending++;
    if (refocus) {
      await tick();
      document
        .querySelector<HTMLElement>(`.board-card[data-key="${m.issue}"] .card-menu-toggle`)
        ?.focus();
    }
    try {
      await api.moveCard(boardId, m);
      announcement = message;
    } catch (e) {
      cards = before;
      notify(errorMessage(e), 'error');
      announcement = errorMessage(e);
    } finally {
      pending--;
      if (pending === 0) reload();
    }
  }

  function watched(key: string, on: boolean) {
    if (cards) cards = cards.map((c) => (c.key === key ? { ...c, watching: on } : c));
  }

  function menuFor(card: Card, keys: string[]): () => MenuGroup[] {
    return () => {
      const at = keys.indexOf(card.key);
      return [
        issueGroup(
          card,
          (key) => openEdit(key, reload),
          (key) => (duplicating = key),
          watched,
        ),
        {
          label: t.boards.moveTo,
          items: [
            ...columns.map((c) => ({
              label: c.name,
              disabled: c.id === card.column,
              run: () =>
                void move(
                  { issue: card.key, column: c.id },
                  t.boards.moved(card.key, c.name),
                  true,
                ),
            })),
            ...(backlogColumn
              ? [
                  {
                    label: t.boards.backlog,
                    run: () =>
                      backlogColumn &&
                      void move(
                        { issue: card.key, column: backlogColumn.id },
                        t.boards.moved(card.key, t.boards.backlog),
                      ),
                  },
                ]
              : []),
          ],
        },
        {
          label: t.boards.order,
          items: [
            {
              label: t.boards.moveUp,
              disabled: at <= 0,
              run: () => step(card.key, keys, -1),
            },
            {
              label: t.boards.moveDown,
              disabled: at < 0 || at >= keys.length - 1,
              run: () => step(card.key, keys, 1),
            },
          ],
        },
      ];
    };
  }

  function step(key: string, keys: string[], dir: -1 | 1) {
    const m = stepMove(keys, key, dir);
    if (m) void move(m, dir < 0 ? t.boards.movedUp(key) : t.boards.movedDown(key), true);
  }

  // Keys -----------------------------------------------------------------------

  function cellCards(section: Section, column: BoardColumn): HTMLElement[] {
    return [
      ...document.querySelectorAll<HTMLElement>(
        `.board-cards[data-section="${section.id}"][data-column="${column.id}"] > .board-card`,
      ),
    ];
  }

  /** The results keys on the board: the focus moves from card to card. */
  function onKey(e: KeyboardEvent) {
    if (ignoreShortcut(e)) return;
    const active = document.activeElement;
    const card = active?.closest<HTMLElement>('.board .board-card') ?? null;
    // An open card menu takes the keys, even before its first item has the focus.
    if (active?.closest('.card-menu') || card?.querySelector('[aria-expanded="true"]')) return;
    if (!card && active && active !== document.body) return;
    let action = shortcutFor(e, 'results', appearance.shortcuts, appearance.bindings, mac);
    // Down and Up are not bindings: they move between cards like the other arrows.
    if (!action && !e.altKey && !e.ctrlKey && !e.metaKey && !e.shiftKey)
      action = e.key === 'ArrowDown' ? 'next' : e.key === 'ArrowUp' ? 'previous' : null;
    // Off a card, the arrows keep scrolling the page.
    if (!action || (!card && e.key.startsWith('Arrow'))) return;
    if (action === 'open') {
      // Enter on the summary link or the menu button keeps its own meaning.
      if (!card?.dataset.key || (e.key === 'Enter' && active !== card)) return;
      e.preventDefault();
      router.navigate(paths.issue(card.dataset.key));
      return;
    }
    if (action !== 'next' && action !== 'previous' && action !== 'left' && action !== 'right')
      return;
    const counts = sections.map((l) => columns.map((c) => cellCards(l, c).length));
    let at: CardSpot | null = null;
    if (card) {
      const box = card.parentElement;
      const section = sections.findIndex((l) => l.id === box?.dataset.section);
      const column = columns.findIndex((c) => String(c.id) === box?.dataset.column);
      const index =
        section < 0 || column < 0
          ? -1
          : cellCards(sections[section]!, columns[column]!).indexOf(card);
      if (index >= 0) at = { section, column, index };
    }
    const to = boardStep(counts, at, action);
    if (!to) return;
    e.preventDefault();
    const section = sections[to.section];
    const column = columns[to.column];
    if (section && column) cellCards(section, column)[to.index]?.focus();
  }

  // Dragging -------------------------------------------------------------------

  let drag = $state<{ key: string; section: string; column: number } | null>(null);
  let over = $state<{ section: string; column: number; index: number } | null>(null);

  function startDrag(e: DragEvent, card: Card, section: Section) {
    if (!e.dataTransfer) return;
    e.dataTransfer.effectAllowed = 'move';
    e.dataTransfer.setData('text/plain', card.key);
    drag = { key: card.key, section: section.id, column: card.column };
  }

  function endDrag() {
    drag = null;
    over = null;
  }

  /** Where the pointer would drop the dragged card among a cell's cards. */
  function indexAt(el: HTMLElement, y: number): number {
    const middles = [...el.querySelectorAll<HTMLElement>('.board-card')]
      .filter((c) => c.dataset.key !== drag?.key)
      .map((c) => {
        const r = c.getBoundingClientRect();
        return r.top + r.height / 2;
      });
    return dropIndex(middles, y);
  }

  function dragOver(e: DragEvent, section: Section, column: BoardColumn) {
    // Cards stay in their section: sections are views (an assignee, an epic), not places.
    if (!drag || drag.section !== section.id) return;
    e.preventDefault();
    if (e.dataTransfer) e.dataTransfer.dropEffect = 'move';
    hover(e.currentTarget as HTMLElement, section, column, e.clientY);
  }

  function hover(el: HTMLElement, section: Section, column: BoardColumn, y: number) {
    const index = indexAt(el, y);
    if (over?.section !== section.id || over.column !== column.id || over.index !== index) {
      over = { section: section.id, column: column.id, index };
    }
  }

  function dragLeave(e: DragEvent, section: Section, column: BoardColumn) {
    const el = e.currentTarget as HTMLElement;
    if (el.contains(e.relatedTarget as Node | null)) return;
    if (over?.section === section.id && over.column === column.id) over = null;
  }

  function drop(e: DragEvent, section: Section, column: BoardColumn) {
    if (!drag || drag.section !== section.id) return;
    e.preventDefault();
    land(e.currentTarget as HTMLElement, section, column, e.clientY);
  }

  function land(el: HTMLElement, section: Section, column: BoardColumn, y: number) {
    if (!drag) return;
    const { key, column: from } = drag;
    const index = indexAt(el, y);
    const keys = cell(section, column).map((c) => c.key);
    endDrag();
    if (from === column.id && staysInPlace(keys, key, index)) return;
    const m: CardMove = { issue: key, ...neighbours(keys, key, index) };
    if (from !== column.id) m.column = column.id;
    void move(m, from !== column.id ? t.boards.moved(key, column.name) : t.boards.ranked(key));
  }

  /** The cell of the dragged card's section under a finger, with its section and column. */
  function cellAt(x: number, y: number) {
    const el = document.elementFromPoint(x, y)?.closest<HTMLElement>('.board-cards');
    const section = sections.find((l) => l.id === el?.dataset.section);
    const column = detail?.columns.find((c) => String(c.id) === el?.dataset.column);
    if (!el || !section || !column || section.id !== drag?.section) return null;
    return { el, section, column };
  }

  const touch: TouchDragHandlers = {
    start(el) {
      const card = cards?.find((c) => c.key === el.dataset.key);
      const section = el.closest<HTMLElement>('.board-cards')?.dataset.section;
      if (!card || !section) return false;
      drag = { key: card.key, section, column: card.column };
      return true;
    },
    move(x, y) {
      const at = cellAt(x, y);
      if (at) hover(at.el, at.section, at.column, y);
      else over = null;
    },
    drop(x, y) {
      const at = cellAt(x, y);
      if (at) land(at.el, at.section, at.column, y);
      else endDrag();
    },
    cancel: endDrag,
  };

  /** The drop line: on the card the dragged one would land above, or below the last. */
  function markerOf(section: Section, column: BoardColumn, list: Card[], card: Card) {
    if (!over || over.section !== section.id || over.column !== column.id) return null;
    const rest = list.filter((c) => c.key !== drag?.key);
    if (rest[over.index]?.key === card.key) return 'before';
    if (over.index >= rest.length && rest[rest.length - 1]?.key === card.key) return 'after';
    return null;
  }
</script>

<svelte:window onkeydown={onKey} />

{#if error}
  <NotFoundPage message={error} />
{:else if !detail}
  <div class="page"><p class="muted loading">{t.common.loading}</p></div>
{:else}
  {@const d = detail}
  <div class="page board-page" data-title={d.board.name}>
    <BoardHeader detail={d} view="board" />
    <BoardRail detail={d}>
      {#if d.board.kind === 'scrum' && sprint}
        <SprintHead title={sprint.name} {sprint} meta={t.boards.cards(shownCount)}>
          {#snippet actions()}
            {#if d.can.manage}
              <button class="btn btn-quiet" onclick={() => (editing = sprint)}
                >{t.boards.editSprint}</button
              >
              <button class="btn" onclick={() => (completing = sprint)}
                >{t.boards.completeSprint}</button
              >
            {/if}
          {/snippet}
        </SprintHead>
      {/if}

      <div class="board-search">
        <FilterBar
          filters={search.filters}
          onchange={applyFilters}
          valueCounts={false}
          rowFields={QUICK_FIELDS}
        />
        <QuickFilters filters={search.filters} onchange={applyFilters} cards={rowFacets}>
          <BoardQuickFilters filters={d.quickFilters} selected={quick} />
        </QuickFilters>
      </div>
      {#if search.refused}<p class="notice" role="status">{t.search.tooComplex}</p>{/if}

      {#if cardsError}
        <p class="error-text" role="alert">{cardsError}</p>
      {:else if !cards}
        <p class="muted loading">{t.common.loading}</p>
      {:else if d.board.kind === 'scrum' && !sprint}
        <Illustration kind="empty" />
        <p class="empty">{t.boards.noActiveSprint} {t.boards.noActiveSprintHint}</p>
        <p><a class="btn" href={paths.backlog(d.board.id)}>{t.boards.openBacklog}</a></p>
      {:else}
        {#if truncated}<p class="notice" role="status">{t.boards.truncated}</p>{/if}
        {#if cards.length === 0}
          <p class="empty">{filtered ? t.boards.emptyFiltered : t.boards.emptyBoard}</p>
        {/if}
        <div
          class="board"
          class:more-left={moreLeft}
          class:more-right={moreRight}
          use:hiddenSides={(left, right) => {
            moreLeft = left;
            moreRight = right;
          }}
          use:touchDrag={touch}
          style:grid-template-columns="repeat({columns.length}, minmax(min(250px, 78vw), 1fr))"
          style:grid-template-rows="repeat({rows}, auto)"
          style:row-gap="0"
        >
          <!-- Before the columns, so the toggles don't come after every card in the tab order. -->
          {#if banded}
            {#each sections as section, k (section.id)}
              {@const issue = sectionIssue(section)}
              <div class="board-section" style:grid-column="1 / -1" style:grid-row={bandRow(k)}>
                <button
                  type="button"
                  class="board-section-toggle"
                  aria-expanded={!collapsed[section.id]}
                  onclick={() => (collapsed[section.id] = !collapsed[section.id])}
                >
                  <span class="chevron" aria-hidden="true"></span>
                  {#if issue}
                    <TypeMark type={issue.type} />
                    <span class="key">{issue.key}</span>
                    <span class="board-section-title">{issue.summary}</span>
                  {:else}
                    <span class="board-section-title">{sectionName(section)}</span>
                  {/if}
                  <span class="board-section-count">{t.boards.cards(section.cards.length)}</span>
                </button>
                {#if section.kind === 'parent' && issue}<Stamp status={issue.status} />{/if}
              </div>
            {/each}
          {/if}
          {#each columns as column, i (column.id)}
            {@const count = counts[column.id] ?? 0}
            <section
              class="board-column"
              class:is-over={over?.column === column.id}
              aria-labelledby="col-{column.id}"
              style:grid-column={i + 1}
              style:grid-row="1 / span {rows}"
              style:display="grid"
              style:grid-template-rows="subgrid"
            >
              <header class="board-column-head" style:grid-row="1">
                <h2 class="board-column-title" id="col-{column.id}">{column.name}</h2>
                <span class="board-column-count"
                  ><span aria-hidden="true">{count}</span><span class="sr-only"
                    >{t.boards.cards(count)}</span
                  ></span
                >
              </header>
              {#each sections as section, k (section.id)}
                {#if banded}<div class="band-gap" style:grid-row={bandRow(k)}></div>{/if}
                {#if !collapsed[section.id]}
                  {@const list = cell(section, column)}
                  {@const keys = list.map((c) => c.key)}
                  <div
                    class="board-cards"
                    style:grid-row={cellRow(k)}
                    role="list"
                    data-section={section.id}
                    data-column={column.id}
                    aria-label={banded ? `${column.name} — ${sectionName(section)}` : column.name}
                    ondragover={(e) => dragOver(e, section, column)}
                    ondragleave={(e) => dragLeave(e, section, column)}
                    ondrop={(e) => drop(e, section, column)}
                  >
                    {#each list as card (card.key)}
                      <BoardCard
                        {card}
                        estimate={d.board.estimate}
                        showStatus={column.statuses.length > 1}
                        dragging={drag?.key === card.key}
                        marker={markerOf(section, column, list, card)}
                        menu={menuFor(card, keys)}
                        ondragstart={(e, c) => startDrag(e, c, section)}
                        ondragend={endDrag}
                      />
                    {/each}
                    {#if list.length === 0 && over?.section === section.id && over.column === column.id}
                      <span class="drop-marker" aria-hidden="true"></span>
                    {/if}
                  </div>
                {/if}
              {/each}
            </section>
          {/each}
        </div>
      {/if}
      <p class="sr-only" aria-live="polite">{announcement}</p>
    </BoardRail>
  </div>

  <CloneDialog
    open={duplicating !== null}
    issueKey={duplicating ?? ''}
    onclose={() => (duplicating = null)}
    onopen={reload}
  />
  <EditSprintDialog
    sprint={editing}
    onclose={() => (editing = null)}
    onsaved={() => {
      editing = null;
      loadDetail();
      reload();
    }}
  />
  <CompleteSprintDialog
    boardId={d.board.id}
    sprint={completing}
    future={futureSprints}
    onclose={() => (completing = null)}
    oncompleted={() => {
      completing = null;
      loadDetail();
      reload();
    }}
  />
{/if}

<style>
  .board-page {
    max-width: 102.5rem;
  }

  .board-search {
    margin-bottom: 10px;
  }

  .notice {
    margin: 0 0 12px;
    color: var(--ink-soft);
    font-size: 0.8125rem;
  }

  .board {
    align-items: stretch;
  }

  /*
   * A shade on each edge past which columns are out of sight: grid items over
   * the whole grid, stuck to the board's edges while it scrolls.
   */
  .board::before,
  .board::after {
    --shade: color-mix(in srgb, var(--ink) 16%, transparent);
    content: '';
    position: sticky;
    z-index: 2;
    grid-row: 1 / -1;
    grid-column: 1 / -1;
    width: 28px;
    pointer-events: none;
    opacity: 0;
    transition: opacity 0.15s ease;
  }

  .board::before {
    left: 0;
    justify-self: start;
    background: linear-gradient(to right, var(--shade), transparent);
  }

  .board::after {
    right: 0;
    justify-self: end;
    background: linear-gradient(to left, var(--shade), transparent);
  }

  .more-left::before,
  .more-right::after {
    opacity: 1;
  }

  .board-column-title {
    margin: 0;
  }

  .board-cards {
    position: relative;
    align-content: start;
    min-height: 56px;
  }

  /* Section bands cross the columns; their label stays in view when scrolled sideways. */
  .board-section {
    position: relative;
    z-index: 1;
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .board-section-toggle {
    position: sticky;
    left: 0;
    display: inline-flex;
    align-items: center;
    gap: 8px;
    max-width: min(100%, 90vw);
    min-width: 0;
    border: none;
    background: none;
    padding: 6px 4px;
    color: inherit;
    font: inherit;
    text-align: left;
    cursor: pointer;
  }

  .board-section-toggle .board-section-title {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .chevron {
    width: 0;
    height: 0;
    flex: none;
    border-left: 4.5px solid transparent;
    border-right: 4.5px solid transparent;
    border-top: 6px solid currentColor;
    transition: transform 0.12s ease;
  }

  [aria-expanded='false'] .chevron {
    transform: rotate(-90deg);
  }

  @media (prefers-reduced-motion: reduce) {
    .chevron,
    .board::before,
    .board::after {
      transition: none;
    }
  }
</style>
