<script lang="ts">
  import { tick } from 'svelte';
  import {
    applyMove,
    dropIndex,
    neighbours,
    parseQuick,
    staysInPlace,
    stepMove,
    totalEstimate,
  } from '../../core/board.js';
  import type { BoardCard as Card, BoardDetail, CardMove, Sprint } from '../../core/board-types.js';
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
  import StartSprintDialog from '../board/StartSprintDialog.svelte';
  import NotFoundPage from './NotFoundPage.svelte';

  /**
   * A Scrum board's backlog: its open sprints (the active one first), then
   * the issues not planned yet. Issues are dragged (or moved from their
   * menu) between sections and up and down.
   */
  interface Props {
    boardId: number;
  }
  const { boardId }: Props = $props();

  let detail = $state<BoardDetail | null>(null);
  let error = $state('');
  let sprints = $state<Sprint[]>([]);
  /** Every issue in board order; `sprint` tells its section (none: the backlog). */
  let cards = $state<Card[] | null>(null);
  let truncated = $state(false);
  let listError = $state('');
  let announcement = $state('');
  let starting = $state<Sprint | null>(null);
  let completing = $state<Sprint | null>(null);
  let creatingSprint = $state(false);

  const quick = $derived(parseQuick(router.param('quick')));
  const quickParam = $derived(quick.join(','));

  function loadDetail(id: number) {
    api
      .board(id)
      .then((d) => {
        if (id !== boardId) return;
        detail = d;
        document.title = `${t.boards.backlog} — ${d.board.name} — ${t.appName}`;
      })
      .catch((e: unknown) => (error = errorMessage(e)));
  }

  $effect(() => {
    const id = boardId;
    detail = null;
    error = '';
    cards = null;
    loadDetail(id);
  });

  let pending = 0;
  let seq = 0;

  async function loadList(id: number, q: string) {
    if (pending > 0) return;
    const mine = ++seq;
    try {
      const res = await api.backlog(id, q || undefined);
      if (mine !== seq || pending > 0) return;
      sprints = res.sprints.map((s) => s.sprint);
      cards = [
        ...res.sprints.flatMap((s) => s.cards.map((c) => ({ ...c, sprint: s.sprint.id }))),
        ...res.backlog,
      ];
      truncated = res.truncated;
      listError = '';
    } catch (e) {
      if (mine === seq) listError = errorMessage(e);
    }
  }

  function reload() {
    void loadList(boardId, quickParam);
  }

  $effect(() => {
    void loadList(boardId, quickParam);
  });

  $effect(() => live.onChange(debounce(reload, 400)));

  interface Section {
    /** The sprint's id; 0 for the backlog. */
    id: number;
    sprint: Sprint | null;
    cards: Card[];
  }

  const sections = $derived.by((): Section[] => {
    const all = cards ?? [];
    return [
      ...sprints.map((s) => ({ id: s.id, sprint: s, cards: all.filter((c) => c.sprint === s.id) })),
      { id: 0, sprint: null, cards: all.filter((c) => c.sprint === undefined) },
    ];
  });
  const active = $derived(sprints.find((s) => s.state === 'active') ?? null);
  const future = $derived(sprints.filter((s) => s.state === 'future'));

  function sectionName(id: number): string {
    return id === 0 ? t.boards.backlog : (sprints.find((s) => s.id === id)?.name ?? '');
  }

  function meta(list: Card[]): string {
    const total = totalEstimate(list);
    const n = t.boards.sectionCount(list.length);
    if (total === null || !detail?.board.estimate) return n;
    const sum =
      detail.board.estimate === 'time'
        ? t.boards.estimateValue(total, 'time')
        : t.boards.points(total);
    return `${n} · ${sum}`;
  }

  // Moving issues --------------------------------------------------------------

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

  function menuFor(card: Card, section: Section): () => MenuGroup[] {
    return () => {
      const keys = section.cards.map((c) => c.key);
      const at = keys.indexOf(card.key);
      const order = (label: string, m: CardMove | null, message: string) => ({
        label,
        disabled: m === null,
        run: () => m && void move(m, message, true),
      });
      return [
        {
          label: t.boards.moveTo,
          items: sections.map((s) => ({
            label: s.sprint ? s.sprint.name : t.boards.backlog,
            disabled: s.id === section.id,
            run: () => {
              // To the bottom of a sprint, to the top of the backlog.
              const first = s.cards[0]?.key;
              const last = s.cards[s.cards.length - 1]?.key;
              const place =
                s.id === 0 ? (first ? { before: first } : {}) : last ? { after: last } : {};
              void move(
                { issue: card.key, sprint: s.id, ...place },
                t.boards.moved(card.key, sectionName(s.id)),
                true,
              );
            },
          })),
        },
        {
          label: t.boards.order,
          items: [
            order(t.boards.moveUp, stepMove(keys, card.key, -1), t.boards.movedUp(card.key)),
            order(t.boards.moveDown, stepMove(keys, card.key, 1), t.boards.movedDown(card.key)),
            order(
              t.boards.toTop,
              at > 0 ? { issue: card.key, ...neighbours(keys, card.key, 0) } : null,
              t.boards.ranked(card.key),
            ),
            order(
              t.boards.toBottom,
              at >= 0 && at < keys.length - 1
                ? { issue: card.key, ...neighbours(keys, card.key, keys.length) }
                : null,
              t.boards.ranked(card.key),
            ),
          ],
        },
      ];
    };
  }

  // Dragging -------------------------------------------------------------------

  let drag = $state<{ key: string; section: number } | null>(null);
  let over = $state<{ section: number; index: number } | null>(null);

  function startDrag(e: DragEvent, card: Card) {
    if (!e.dataTransfer) return;
    e.dataTransfer.effectAllowed = 'move';
    e.dataTransfer.setData('text/plain', card.key);
    drag = { key: card.key, section: card.sprint ?? 0 };
  }

  function endDrag() {
    drag = null;
    over = null;
  }

  function indexAt(el: HTMLElement, y: number): number {
    const middles = [...el.querySelectorAll<HTMLElement>('.board-card')]
      .filter((c) => c.dataset.key !== drag?.key)
      .map((c) => {
        const r = c.getBoundingClientRect();
        return r.top + r.height / 2;
      });
    return dropIndex(middles, y);
  }

  function dragOver(e: DragEvent, section: Section) {
    if (!drag) return;
    e.preventDefault();
    if (e.dataTransfer) e.dataTransfer.dropEffect = 'move';
    const index = indexAt(e.currentTarget as HTMLElement, e.clientY);
    if (over?.section !== section.id || over.index !== index) over = { section: section.id, index };
  }

  function dragLeave(e: DragEvent, section: Section) {
    if ((e.currentTarget as HTMLElement).contains(e.relatedTarget as Node | null)) return;
    if (over?.section === section.id) over = null;
  }

  function drop(e: DragEvent, section: Section) {
    if (!drag) return;
    e.preventDefault();
    const { key, section: from } = drag;
    const index = indexAt(e.currentTarget as HTMLElement, e.clientY);
    const keys = section.cards.map((c) => c.key);
    endDrag();
    if (from === section.id && staysInPlace(keys, key, index)) return;
    const m: CardMove = { issue: key, ...neighbours(keys, key, index) };
    if (from !== section.id) m.sprint = section.id;
    void move(
      m,
      from !== section.id ? t.boards.moved(key, sectionName(section.id)) : t.boards.ranked(key),
    );
  }

  function markerOf(section: Section, card: Card) {
    if (!over || over.section !== section.id) return null;
    const rest = section.cards.filter((c) => c.key !== drag?.key);
    if (rest[over.index]?.key === card.key) return 'before';
    if (over.index >= rest.length && rest[rest.length - 1]?.key === card.key) return 'after';
    return null;
  }

  async function createSprint() {
    if (creatingSprint) return;
    creatingSprint = true;
    try {
      await api.createSprint(boardId);
      notify(t.boards.sprintCreated);
      reload();
      loadDetail(boardId);
    } catch (e) {
      notify(errorMessage(e), 'error');
    } finally {
      creatingSprint = false;
    }
  }

  function afterSprintChange() {
    starting = null;
    completing = null;
    reload();
    loadDetail(boardId);
  }
</script>

{#if error}
  <NotFoundPage message={error} />
{:else if !detail}
  <div class="page"><p class="muted loading">{t.common.loading}</p></div>
{:else}
  {@const d = detail}
  <div class="page backlog-page" data-title={`${t.boards.backlog} — ${d.board.name}`}>
    <BoardHeader detail={d} view="backlog" />
    <QuickFilters filters={d.quickFilters} selected={quick} />

    {#if listError}
      <p class="error-text" role="alert">{listError}</p>
    {:else if !cards}
      <p class="muted loading">{t.common.loading}</p>
    {:else}
      {#if truncated}<p class="notice" role="status">{t.boards.truncated}</p>{/if}
      {#each sections as section (section.id)}
        {@const s = section.sprint}
        <section class="backlog-section" aria-labelledby="section-{section.id}">
          <SprintHead
            id="section-{section.id}"
            title={s ? s.name : t.boards.backlog}
            sprint={s}
            meta={meta(section.cards)}
          >
            {#snippet actions()}
              {#if d.can.manage && s?.state === 'future'}
                <button
                  class="btn btn-small"
                  disabled={active !== null}
                  title={active ? t.boards.anotherActive : undefined}
                  onclick={() => (starting = s)}>{t.boards.startSprint}</button
                >
              {:else if d.can.manage && s?.state === 'active'}
                <button class="btn btn-small" onclick={() => (completing = s)}
                  >{t.boards.completeSprint}</button
                >
              {:else if d.can.manage && !s}
                <button class="btn btn-small" disabled={creatingSprint} onclick={createSprint}
                  >{t.boards.createSprint}</button
                >
              {/if}
            {/snippet}
          </SprintHead>
          <div class="board-column backlog-list" class:is-over={over?.section === section.id}>
            <div
              class="board-cards"
              role={section.cards.length ? 'list' : undefined}
              aria-labelledby="section-{section.id}"
              ondragover={(e) => dragOver(e, section)}
              ondragleave={(e) => dragLeave(e, section)}
              ondrop={(e) => drop(e, section)}
            >
              {#each section.cards as card (card.key)}
                <BoardCard
                  {card}
                  layout="row"
                  estimate={d.board.estimate}
                  dragging={drag?.key === card.key}
                  marker={markerOf(section, card)}
                  menu={menuFor(card, section)}
                  ondragstart={startDrag}
                  ondragend={endDrag}
                />
              {:else}
                <p class="drop-empty">
                  {s ? t.boards.dropHere : t.boards.backlogEmpty}
                  {#if over?.section === section.id}<span class="drop-marker" aria-hidden="true"
                    ></span>{/if}
                </p>
              {/each}
            </div>
          </div>
        </section>
      {/each}
    {/if}
    <p class="sr-only" aria-live="polite">{announcement}</p>
  </div>

  <StartSprintDialog
    sprint={starting}
    onclose={() => (starting = null)}
    onstarted={afterSprintChange}
  />
  <CompleteSprintDialog
    boardId={d.board.id}
    sprint={completing}
    {future}
    onclose={() => (completing = null)}
    oncompleted={afterSprintChange}
  />
{/if}

<style>
  /* As wide as the board, so switching views keeps the header in place. */
  .backlog-page {
    max-width: 1640px;
  }

  .notice {
    margin: 0 0 12px;
    color: var(--ink-soft);
    font-size: 13px;
  }

  .backlog-section {
    margin-bottom: 26px;
  }

  .backlog-list {
    min-height: 0;
  }

  .board-cards {
    position: relative;
  }

  .drop-empty {
    position: relative;
    margin: 0;
    padding: 10px 4px;
    color: var(--muted);
    font-size: 13px;
    text-align: center;
  }
</style>
