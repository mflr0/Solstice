<script lang="ts">
  import type { BoardDetail } from '../../core/board-types.js';
  import { paths } from '../../core/route.js';
  import { boardListPath } from '../../core/scope.js';
  import { t } from '../../i18n/t.js';
  import { session } from '../../lib/session.svelte.js';
  import { shownBoard } from '../../lib/ui.svelte.js';
  import { BOARD_VIEW_ICONS } from './board-view-icons.js';
  import ShareBoardDialog from './ShareBoardDialog.svelte';

  /** A board's title block, with the switch between its board, backlog, reports and settings. */
  interface Props {
    detail: BoardDetail;
    view: 'board' | 'backlog' | 'reports' | 'settings';
  }
  const { detail, view }: Props = $props();
  const b = $derived(detail.board);
  const views = $derived([
    { id: 'board', href: paths.board(b.id), label: t.boards.viewBoard },
    ...(b.kind === 'scrum' || b.kanbanBacklog
      ? [{ id: 'backlog', href: paths.backlog(b.id), label: t.boards.viewBacklog }]
      : []),
    { id: 'reports', href: paths.boardReports(b.id), label: t.boards.viewReports },
    ...(detail.can.manage
      ? [{ id: 'settings', href: paths.boardSettings(b.id), label: t.boards.settings }]
      : []),
  ] as { id: Props['view']; href: string; label: string }[]);

  let sharing = $state(false);
  // What was just saved, until the page next loads the board.
  let shares = $derived(detail.shares);

  $effect(() => {
    shownBoard.id = b.id;
    shownBoard.projects = detail.projects;
  });
</script>

<header class="page-head board-head">
  <div class="title">
    <p class="eyebrow">
      <a href={boardListPath(detail.projects, session.scope)}>{t.boards.title}</a> · {t.boards
        .kinds[b.kind]}
    </p>
    <h1 class="page-title">{b.name}</h1>
    {#if detail.projects.length}
      <p class="projects">
        <span class="sr-only">{t.boards.projects}{t.common.labelSep}</span>
        {#each detail.projects as p (p.id)}
          <a class="chip" href={paths.project(p.key)} title={p.name}>{p.key}</a>
        {/each}
      </p>
    {/if}
  </div>
  <div class="row">
    <nav class="views" aria-label={t.boards.views}>
      {#each views as v (v.id)}
        <a
          class="btn btn-icon"
          class:on={view === v.id}
          href={v.href}
          aria-label={v.label}
          title={v.label}
          aria-current={view === v.id ? 'page' : undefined}
        >
          <svg viewBox="0 0 16 16" width="16" height="16" aria-hidden="true">
            {#each BOARD_VIEW_ICONS[v.id] as d, i (i)}<path {d} />{/each}
          </svg>
        </a>
      {/each}
    </nav>
    <button
      type="button"
      class="btn btn-icon"
      aria-label={t.boards.share}
      title={t.boards.share}
      aria-haspopup="dialog"
      onclick={() => (sharing = true)}
    >
      <svg viewBox="0 0 16 16" width="16" height="16" aria-hidden="true">
        {#each BOARD_VIEW_ICONS.share as d, i (i)}<path {d} />{/each}
      </svg>
    </button>
  </div>
</header>

<ShareBoardDialog
  open={sharing}
  {detail}
  {shares}
  onclose={() => (sharing = false)}
  onsaved={(saved) => {
    shares = saved;
    sharing = false;
  }}
/>

<style>
  .board-head {
    flex-wrap: wrap;
  }

  .title {
    min-width: 0;
  }

  .eyebrow a {
    color: inherit;
    text-decoration: none;
  }

  .eyebrow a:hover {
    text-decoration: underline;
  }

  .projects {
    display: flex;
    flex-wrap: wrap;
    gap: 4px;
    margin: 8px 0 0;
  }

  .projects .chip {
    text-decoration: none;
  }

  .projects .chip:hover {
    border-color: var(--ink-soft);
  }

  .views {
    display: flex;
  }

  .views a {
    position: relative;
    border-radius: 0;
  }

  .views a + a {
    margin-left: calc(-1 * var(--frame));
  }

  .views a:first-child {
    border-start-start-radius: var(--radius-s);
    border-end-start-radius: var(--radius-s);
  }

  .views a:last-child {
    border-start-end-radius: var(--radius-s);
    border-end-end-radius: var(--radius-s);
  }

  /* Neighbours overlap by a border: the one that stands out draws its own in full. */
  .views a:hover,
  .views a:focus-visible,
  .views a.on {
    z-index: 1;
  }
</style>
