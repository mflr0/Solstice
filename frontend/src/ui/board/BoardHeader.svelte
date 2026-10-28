<script lang="ts">
  import type { BoardDetail } from '../../core/board-types.js';
  import { paths } from '../../core/route.js';
  import { t } from '../../i18n/t.js';

  /** A board's title block, with the switch between its board, backlog and reports. */
  interface Props {
    detail: BoardDetail;
    view: 'board' | 'backlog' | 'reports' | 'settings';
  }
  const { detail, view }: Props = $props();
  const b = $derived(detail.board);
</script>

<header class="page-head board-head">
  <div class="title">
    <p class="eyebrow">
      <a href={paths.boards()}>{t.boards.title}</a> · {t.boards.kinds[b.kind]}
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
    {#if (b.kind === 'scrum' || b.kanbanBacklog) && view !== 'settings'}
      <nav class="views" aria-label={t.boards.views}>
        <a
          class="btn btn-small"
          class:on={view === 'board'}
          href={paths.board(b.id)}
          aria-current={view === 'board' ? 'page' : undefined}>{t.boards.viewBoard}</a
        >
        <a
          class="btn btn-small"
          class:on={view === 'backlog'}
          href={paths.backlog(b.id)}
          aria-current={view === 'backlog' ? 'page' : undefined}>{t.boards.viewBacklog}</a
        >
        <a
          class="btn btn-small"
          class:on={view === 'reports'}
          href={paths.boardReports(b.id)}
          aria-current={view === 'reports' ? 'page' : undefined}>{t.boards.viewReports}</a
        >
      </nav>
    {/if}
    {#if view === 'settings'}
      <a class="btn" href={paths.board(b.id)}>{t.boardSettings.back}</a>
    {:else if detail.can.manage}
      <a class="btn" href={paths.boardSettings(b.id)}>{t.boards.settings}</a>
    {/if}
  </div>
</header>

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
    gap: 2px;
  }
</style>
