<script lang="ts">
  import type { Snippet } from 'svelte';
  import { STORAGE_KEYS } from '../../core/appearance.js';
  import type { BoardDetail, BoardSummary } from '../../core/board-types.js';
  import { paths } from '../../core/route.js';
  import { boardListPath, boardProject, boardRail } from '../../core/scope.js';
  import { t } from '../../i18n/t.js';
  import { api } from '../../lib/api.js';
  import { session } from '../../lib/session.svelte.js';

  /**
   * A board page's content, with a rail on its left to the other boards: the
   * project's, then the other projects', folded. Hidden on a narrow screen.
   */
  interface Props {
    detail: BoardDetail;
    children: Snippet;
  }
  const { detail, children }: Props = $props();

  // Unknown while it loads or when it fails: the rail lists no board.
  let boards = $state<BoardSummary[]>([]);
  $effect(() => {
    api
      .boards()
      .then((b) => (boards = b))
      .catch(() => {});
  });

  const key = $derived(boardProject(detail.projects, session.scope));
  const rail = $derived(boardRail(boards, key));
  const projectName = $derived(detail.projects.find((p) => p.key === key)?.name);

  let collapsed = $state(readCollapsed());

  function readCollapsed(): boolean {
    try {
      return localStorage.getItem(STORAGE_KEYS.boardRail) === 'collapsed';
    } catch {
      return false;
    }
  }

  function toggle() {
    collapsed = !collapsed;
    try {
      localStorage.setItem(STORAGE_KEYS.boardRail, collapsed ? 'collapsed' : 'open');
    } catch {
      // Storage blocked: the choice lasts for this page only.
    }
  }
</script>

<div class="board-frame" class:collapsed>
  <nav class="board-rail" aria-label={t.boards.rail}>
    <button
      type="button"
      class="btn btn-icon btn-quiet rail-toggle"
      aria-expanded={!collapsed}
      aria-label={collapsed ? t.boards.railExpand : t.boards.railCollapse}
      title={collapsed ? t.boards.railExpand : t.boards.railCollapse}
      onclick={toggle}
    >
      <svg viewBox="0 0 16 16" width="16" height="16" aria-hidden="true">
        <path d={collapsed ? 'M6 3l5 5-5 5' : 'M10 3L5 8l5 5'} />
      </svg>
    </button>
    {#if !collapsed}
      {#if key}
        <p class="rail-title eyebrow" title={projectName}>{key}</p>
        <ul>
          {#each rail.current as b (b.id)}
            <li>
              <a
                class="rail-link"
                href={paths.board(b.id)}
                aria-current={b.id === detail.board.id ? 'page' : undefined}>{b.name}</a
              >
            </li>
          {/each}
        </ul>
      {/if}
      <ul class="rail-actions">
        <li>
          <a class="rail-link" href={boardListPath(detail.projects, session.scope)}
            >{t.boards.allBoards}</a
          >
        </li>
        <li>
          <a class="rail-link" href={paths.newBoard(key ?? undefined)}
            ><span class="glyph" aria-hidden="true">+</span>{t.boards.newBoard}</a
          >
        </li>
      </ul>
      {#if rail.others.length}
        <details class="rail-others">
          <summary class="rail-link">{t.boards.otherProjects}</summary>
          {#each rail.others as p (p.key)}
            <details class="rail-project">
              <summary class="rail-link" title={p.name}
                >{p.key} <span class="count">{t.boards.count(p.boards.length)}</span></summary
              >
              <ul>
                {#each p.boards as b (b.id)}
                  <li><a class="rail-link" href={paths.board(b.id)}>{b.name}</a></li>
                {/each}
              </ul>
            </details>
          {/each}
        </details>
      {/if}
    {/if}
  </nav>
  <div class="board-main">{@render children()}</div>
</div>

<style>
  .board-frame {
    display: grid;
    grid-template-columns: 12.5rem minmax(0, 1fr);
    gap: 28px;
    align-items: start;
  }

  .board-frame.collapsed {
    grid-template-columns: 2rem minmax(0, 1fr);
    gap: 16px;
  }

  .board-rail {
    min-width: 0;
    font-size: 0.84375rem;
  }

  .rail-toggle {
    margin-bottom: 8px;
  }

  .rail-title {
    margin: 0 0 4px;
    padding: 0 10px;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  ul {
    list-style: none;
    margin: 0;
    padding: 0;
  }

  .rail-actions,
  .rail-others {
    margin-top: 14px;
  }

  .rail-link {
    display: block;
    padding: 5px 10px;
    border-left: 2px solid transparent;
    border-radius: 0 var(--radius-s) var(--radius-s) 0;
    color: var(--ink-soft);
    text-decoration: none;
    line-height: 1.3;
    overflow-wrap: anywhere;
  }

  .rail-link:hover {
    color: var(--ink);
    background: var(--accent-wash);
  }

  .rail-link[aria-current='page'] {
    color: var(--ink);
    font-weight: 600;
    border-left-color: var(--accent);
    background: var(--accent-wash);
  }

  summary.rail-link {
    cursor: pointer;
    list-style: none;
  }

  summary.rail-link::-webkit-details-marker {
    display: none;
  }

  summary.rail-link::before {
    content: '▸';
    display: inline-block;
    width: 1em;
    color: var(--muted);
  }

  details[open] > summary.rail-link::before {
    content: '▾';
  }

  .glyph {
    display: inline-block;
    width: 1em;
  }

  .rail-project {
    margin-left: 10px;
  }

  .rail-project ul {
    margin-left: 10px;
  }

  .count {
    color: var(--muted);
    font-size: 0.75rem;
  }

  .board-main {
    min-width: 0;
  }

  @media (max-width: 900px) {
    .board-frame,
    .board-frame.collapsed {
      display: block;
    }

    .board-rail {
      display: none;
    }
  }
</style>
