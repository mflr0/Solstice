<script lang="ts">
  import { tick } from 'svelte';
  import {
    applyMove,
    dropIndex,
    groupLanes,
    neighbours,
    parseQuick,
    staysInPlace,
    stepMove,
    wipState,
    type Lane,
  } from '../../core/board.js';
  import type {
    BoardCard as Card,
    BoardColumn,
    BoardDetail,
    CardMove,
    Sprint,
  } from '../../core/board-types.js';
  import { paths } from '../../core/route.js';
  import type { IssueBrief } from '../../core/types.js';
  import { t } from '../../i18n/fr.js';
  import { api, errorMessage } from '../../lib/api.js';
  import { debounce, live } from '../../lib/live.svelte.js';
  import { router } from '../../lib/router.svelte.js';
  import { notify } from '../../lib/ui.svelte.js';
  import BoardCard from '../board/BoardCard.svelte';
  import BoardHeader from '../board/BoardHeader.svelte';
  import type { MenuGroup } from '../board/CardMenu.svelte';
  import CompleteSprintDialog from '../board/CompleteSprintDialog.svelte';
  import QuickFilters from '../board/QuickFilters.svelte';
  import SprintHead from '../board/SprintHead.svelte';
  import Illustration from '../Illustration.svelte';
  import TypeMark from '../TypeMark.svelte';
  import Stamp from '../Stamp.svelte';
  import NotFoundPage from './NotFoundPage.svelte';

  /**
   * A board: its columns side by side, optionally cut into swimlanes.
   * Cards are dragged between columns (a workflow transition) and within
   * one (their rank); each card's menu does the same from the keyboard.
   */
  interface Props {
    boardId: number;
  }
  const { boardId }: Props = $props();

  let detail = $state<BoardDetail | null>(null);
  let error = $state('');
  let cards = $state<Card[] | null>(null);
  let cardsError = $state('');
  let sprint = $state<Sprint | null>(null);
  let truncated = $state(false);
  let announcement = $state('');
  let collapsed = $state<Record<string, boolean>>({});
  let completing = $state<Sprint | null>(null);

  const quick = $derived(parseQuick(router.param('quick')));
  const quickParam = $derived(quick.join(','));

  function loadDetail(id: number) {
    api
      .board(id)
      .then((d) => {
        if (id !== boardId) return;
        detail = d;
        document.title = `${d.board.name} — ${t.appName}`;
      })
      .catch((e: unknown) => (error = errorMessage(e)));
  }

  $effect(() => {
    const id = boardId;
    detail = null;
    error = '';
    cards = null;
    collapsed = {};
    loadDetail(id);
  });

  // Moves in flight: a reload meanwhile would show the card back where it was.
  let pending = 0;
  let seq = 0;

  async function loadCards(id: number, q: string) {
    if (pending > 0) return;
    const mine = ++seq;
    try {
      const res = await api.boardCards(id, { quick: q || undefined });
      if (mine !== seq || pending > 0) return;
      cards = res.cards;
      sprint = res.sprint;
      truncated = res.truncated;
      cardsError = '';
    } catch (e) {
      if (mine === seq) cardsError = errorMessage(e);
    }
  }

  function reload() {
    void loadCards(boardId, quickParam);
  }

  $effect(() => {
    void loadCards(boardId, quickParam);
  });

  $effect(() => live.onChange(debounce(reload, 400)));

  const lanes = $derived(
    detail && cards ? groupLanes(cards, detail.board.swimlanes, detail.swimlanes) : [],
  );
  const banded = $derived(lanes.length > 0 && lanes[0]?.kind !== 'all');
  const counts = $derived.by(() => {
    const out: Record<number, number> = {};
    for (const lane of lanes) for (const c of lane.cards) out[c.column] = (out[c.column] ?? 0) + 1;
    return out;
  });
  const shownCount = $derived(lanes.reduce((n, l) => n + l.cards.length, 0));
  const futureSprints = $derived(detail?.sprints.filter((s) => s.state === 'future') ?? []);

  function laneName(lane: Lane): string {
    switch (lane.kind) {
      case 'custom':
        return lane.name;
      case 'assignee':
        return lane.user?.name ?? t.boards.unassigned;
      case 'epic':
        return lane.issue ? `${lane.issue.key} ${lane.issue.summary}` : t.boards.noEpic;
      case 'parent':
        return `${lane.issue.key} ${lane.issue.summary}`;
      default:
        return t.boards.otherIssues;
    }
  }

  /** The issue an epic or parent lane stands for. */
  function laneIssue(lane: Lane): IssueBrief | null {
    return lane.kind === 'epic' || lane.kind === 'parent' ? lane.issue : null;
  }

  function cell(lane: Lane, column: BoardColumn): Card[] {
    return lane.cards.filter((c) => c.column === column.id);
  }

  // Rows of the board grid: the column heads, then for each lane its band
  // (when lanes are shown) and its cells.
  const rows = $derived(1 + (banded ? 2 : 1) * Math.max(lanes.length, 1));
  const bandRow = (k: number) => 2 + 2 * k;
  const cellRow = (k: number) => (banded ? 3 + 2 * k : 2 + k);

  // Moving cards ---------------------------------------------------------------

  async function move(m: CardMove, message: string, refocus = false) {
    if (!cards) return;
    const before = cards;
    cards = applyMove(cards, m);
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

  function menuFor(card: Card, keys: string[]): () => MenuGroup[] {
    return () => {
      const at = keys.indexOf(card.key);
      return [
        {
          label: t.boards.moveTo,
          items: (detail?.columns ?? []).map((c) => ({
            label: c.name,
            disabled: c.id === card.column,
            run: () =>
              void move({ issue: card.key, column: c.id }, t.boards.moved(card.key, c.name), true),
          })),
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

  // Dragging -------------------------------------------------------------------

  let drag = $state<{ key: string; lane: string; column: number } | null>(null);
  let over = $state<{ lane: string; column: number; index: number } | null>(null);

  function startDrag(e: DragEvent, card: Card, lane: Lane) {
    if (!e.dataTransfer) return;
    e.dataTransfer.effectAllowed = 'move';
    e.dataTransfer.setData('text/plain', card.key);
    drag = { key: card.key, lane: lane.id, column: card.column };
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

  function dragOver(e: DragEvent, lane: Lane, column: BoardColumn) {
    // Cards stay in their lane: lanes are views (an assignee, an epic), not places.
    if (!drag || drag.lane !== lane.id) return;
    e.preventDefault();
    if (e.dataTransfer) e.dataTransfer.dropEffect = 'move';
    const index = indexAt(e.currentTarget as HTMLElement, e.clientY);
    if (over?.lane !== lane.id || over.column !== column.id || over.index !== index) {
      over = { lane: lane.id, column: column.id, index };
    }
  }

  function dragLeave(e: DragEvent, lane: Lane, column: BoardColumn) {
    const el = e.currentTarget as HTMLElement;
    if (el.contains(e.relatedTarget as Node | null)) return;
    if (over?.lane === lane.id && over.column === column.id) over = null;
  }

  function drop(e: DragEvent, lane: Lane, column: BoardColumn) {
    if (!drag || drag.lane !== lane.id) return;
    e.preventDefault();
    const { key, column: from } = drag;
    const index = indexAt(e.currentTarget as HTMLElement, e.clientY);
    const keys = cell(lane, column).map((c) => c.key);
    endDrag();
    if (from === column.id && staysInPlace(keys, key, index)) return;
    const m: CardMove = { issue: key, ...neighbours(keys, key, index) };
    if (from !== column.id) m.column = column.id;
    void move(m, from !== column.id ? t.boards.moved(key, column.name) : t.boards.ranked(key));
  }

  /** The drop line: on the card the dragged one would land above, or below the last. */
  function markerOf(lane: Lane, column: BoardColumn, list: Card[], card: Card) {
    if (!over || over.lane !== lane.id || over.column !== column.id) return null;
    const rest = list.filter((c) => c.key !== drag?.key);
    if (rest[over.index]?.key === card.key) return 'before';
    if (over.index >= rest.length && rest[rest.length - 1]?.key === card.key) return 'after';
    return null;
  }
</script>

{#if error}
  <NotFoundPage message={error} />
{:else if !detail}
  <div class="page"><p class="muted loading">{t.common.loading}</p></div>
{:else}
  {@const d = detail}
  <div class="page board-page" data-title={d.board.name}>
    <BoardHeader detail={d} view="board" />

    {#if d.board.kind === 'scrum' && sprint}
      <SprintHead title={sprint.name} {sprint} meta={t.boards.cards(shownCount)}>
        {#snippet actions()}
          {#if d.can.manage}
            <button class="btn" onclick={() => (completing = sprint)}
              >{t.boards.completeSprint}</button
            >
          {/if}
        {/snippet}
      </SprintHead>
    {/if}

    <QuickFilters filters={d.quickFilters} selected={quick} />

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
        <p class="empty">{quick.length ? t.boards.emptyFiltered : t.boards.emptyBoard}</p>
      {/if}
      <div
        class="board"
        style:grid-template-columns="repeat({d.columns.length}, minmax(min(250px, 78vw), 1fr))"
        style:grid-template-rows="repeat({rows}, auto)"
        style:row-gap="0"
      >
        {#each d.columns as column, i (column.id)}
          {@const count = counts[column.id] ?? 0}
          {@const wip = wipState(count, column.min, column.max)}
          <section
            class="board-column"
            class:is-over={over?.column === column.id}
            class:over-limit={wip === 'over'}
            class:under-limit={wip === 'under'}
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
              {#if column.min !== null || column.max !== null}
                <span class="wip-limit"
                  >{t.boards.wip(column.min, column.max)}{#if wip !== 'ok'}<span class="sr-only"
                      >, {wip === 'over' ? t.boards.wipOver : t.boards.wipUnder}</span
                    >{/if}</span
                >
              {/if}
            </header>
            {#each lanes as lane, k (lane.id)}
              {#if banded}<div class="band-gap" style:grid-row={bandRow(k)}></div>{/if}
              {#if !collapsed[lane.id]}
                {@const list = cell(lane, column)}
                {@const keys = list.map((c) => c.key)}
                <div
                  class="board-cards"
                  style:grid-row={cellRow(k)}
                  role="list"
                  aria-label={banded ? `${column.name} — ${laneName(lane)}` : column.name}
                  ondragover={(e) => dragOver(e, lane, column)}
                  ondragleave={(e) => dragLeave(e, lane, column)}
                  ondrop={(e) => drop(e, lane, column)}
                >
                  {#each list as card (card.key)}
                    <BoardCard
                      {card}
                      estimate={d.board.estimate}
                      showStatus={column.statuses.length > 1}
                      dragging={drag?.key === card.key}
                      marker={markerOf(lane, column, list, card)}
                      menu={menuFor(card, keys)}
                      ondragstart={(e, c) => startDrag(e, c, lane)}
                      ondragend={endDrag}
                    />
                  {/each}
                  {#if list.length === 0 && over?.lane === lane.id && over.column === column.id}
                    <span class="drop-marker" aria-hidden="true"></span>
                  {/if}
                </div>
              {/if}
            {/each}
          </section>
        {/each}
        {#if banded}
          {#each lanes as lane, k (lane.id)}
            {@const issue = laneIssue(lane)}
            <div class="board-lane" style:grid-column="1 / -1" style:grid-row={bandRow(k)}>
              <button
                type="button"
                class="board-lane-toggle"
                aria-expanded={!collapsed[lane.id]}
                onclick={() => (collapsed[lane.id] = !collapsed[lane.id])}
              >
                <span class="chevron" aria-hidden="true"></span>
                {#if issue}
                  <TypeMark type={issue.type} />
                  <span class="key">{issue.key}</span>
                  <span class="board-lane-title">{issue.summary}</span>
                {:else}
                  <span class="board-lane-title">{laneName(lane)}</span>
                {/if}
                <span class="board-lane-count">{t.boards.cards(lane.cards.length)}</span>
              </button>
              {#if lane.kind === 'parent' && issue}<Stamp status={issue.status} />{/if}
            </div>
          {/each}
        {/if}
      </div>
    {/if}
    <p class="sr-only" aria-live="polite">{announcement}</p>
  </div>

  <CompleteSprintDialog
    boardId={d.board.id}
    sprint={completing}
    future={futureSprints}
    onclose={() => (completing = null)}
    oncompleted={() => {
      completing = null;
      loadDetail(boardId);
      reload();
    }}
  />
{/if}

<style>
  .board-page {
    max-width: 1640px;
  }

  .notice {
    margin: 0 0 12px;
    color: var(--ink-soft);
    font-size: 13px;
  }

  .board {
    align-items: stretch;
  }

  .board-column-title {
    margin: 0;
  }

  .board-cards {
    position: relative;
    align-content: start;
    min-height: 56px;
  }

  /* Lane bands cross the columns; their label stays in view when scrolled sideways. */
  .board-lane {
    position: relative;
    z-index: 1;
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .board-lane-toggle {
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

  .board-lane-toggle .board-lane-title {
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
    .chevron {
      transition: none;
    }
  }
</style>
