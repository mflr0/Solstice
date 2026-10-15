<script lang="ts">
  import type { BoardCard } from '../../core/board-types.js';
  import { initials } from '../../core/marks.js';
  import { paths } from '../../core/route.js';
  import { t } from '../../i18n/t.js';
  import Lock from '../Lock.svelte';
  import PriorityMark from '../PriorityMark.svelte';
  import Stamp from '../Stamp.svelte';
  import TypeMark from '../TypeMark.svelte';
  import CardMenu, { type MenuGroup } from './CardMenu.svelte';

  /**
   * One issue on a board (a card) or in a backlog (a row). The summary is
   * the link and covers the whole card; the menu button sits above it.
   */
  interface Props {
    card: BoardCard;
    layout?: 'card' | 'row';
    estimate: '' | 'points' | 'time';
    /** Print the status: when a column holds several. */
    showStatus?: boolean;
    dragging?: boolean;
    /** A drop line above or below the card while another one is dragged here. */
    marker?: 'before' | 'after' | null;
    menu: () => MenuGroup[];
    ondragstart: (e: DragEvent, card: BoardCard) => void;
    ondragend: () => void;
  }
  const {
    card,
    layout = 'card',
    estimate,
    showStatus = false,
    dragging = false,
    marker = null,
    menu,
    ondragstart,
    ondragend,
  }: Props = $props();
</script>

{#snippet who()}
  {#if card.assignee}
    <span
      class="avatar"
      role="img"
      title={card.assignee.name}
      aria-label={`${t.issue.assignee}${t.common.labelSep}${card.assignee.name}`}
      >{initials(card.assignee.name)}</span
    >
  {/if}
{/snippet}

{#snippet sizing()}
  {#if estimate && card.estimate !== null}
    <span class="chip estimate" title={t.boards.estimate}
      ><span class="sr-only">{t.boards.estimate}{t.common.labelSep}</span>{t.boards.estimateValue(
        card.estimate,
        estimate,
      )}</span
    >
  {/if}
{/snippet}

{#snippet epic()}
  {#if card.epic}
    <span
      class="chip epic"
      title={`${t.boards.epic} ${card.epic.key}${t.common.labelSep}${card.epic.summary}`}
      ><span class="sr-only">{t.boards.epic}{t.common.labelSep}</span>{card.epic.summary}</span
    >
  {/if}
{/snippet}

{#snippet parent()}
  {#if card.parent}
    <span class="parent" title={card.parent.summary}
      ><span class="sr-only">{t.boards.subtaskOf(card.parent.key)}</span><span aria-hidden="true"
        >{card.parent.key}</span
      ></span
    >
  {/if}
{/snippet}

<article
  class="board-card"
  class:is-dragging={dragging}
  class:backlog-row={layout === 'row'}
  data-key={card.key}
  role="listitem"
  draggable="true"
  ondragstart={(e) => ondragstart(e, card)}
  {ondragend}
>
  {#if marker === 'before'}<span class="drop-marker before" aria-hidden="true"></span>{/if}
  {#if layout === 'row'}
    <TypeMark type={card.type} />
    <span class="key" aria-hidden="true">{card.key}</span>
    <a class="summary" href={paths.issue(card.key)} draggable="false"
      ><span class="sr-only">{card.key} </span>{card.summary}</a
    >
    {#if card.confidential}<Lock />{/if}
    <span class="tail">
      {@render epic()}
      <Stamp status={card.status} />
      {@render sizing()}
      <PriorityMark priority={card.priority} />
      {@render who()}
    </span>
    <span class="tools"><CardMenu label={t.boards.cardActions(card.key)} groups={menu} /></span>
  {:else}
    <div class="line">
      <TypeMark type={card.type} />
      <span class="key" aria-hidden="true">{card.key}</span>
      {@render parent()}
      {#if card.confidential}<Lock />{/if}
      <span class="tools"><CardMenu label={t.boards.cardActions(card.key)} groups={menu} /></span>
    </div>
    <a class="summary" href={paths.issue(card.key)} draggable="false"
      ><span class="sr-only">{card.key} </span>{card.summary}</a
    >
    {#if card.epic || showStatus || card.assignee || card.priority || (estimate && card.estimate !== null)}
      <div class="line foot">
        {@render epic()}
        {#if showStatus}<Stamp status={card.status} />{/if}
        <span class="push">
          {@render sizing()}
          <PriorityMark priority={card.priority} />
          {@render who()}
        </span>
      </div>
    {/if}
  {/if}
  {#if marker === 'after'}<span class="drop-marker after" aria-hidden="true"></span>{/if}
</article>

<style>
  .board-card {
    position: relative;
    min-width: 0;
  }

  .board-card:has(.summary:focus-visible) {
    outline: 2px solid var(--focus);
    outline-offset: 2px;
  }

  .line {
    display: flex;
    align-items: center;
    gap: 6px;
    min-width: 0;
  }

  .key {
    color: var(--ink-soft);
  }

  .parent {
    font: 11.5px/1 var(--font-mono);
    color: var(--muted);
    white-space: nowrap;
  }

  .parent::before {
    content: '/ ';
  }

  .tools {
    position: relative;
    z-index: 1;
    margin: -4px -6px -4px auto;
  }

  .summary {
    color: inherit;
    text-decoration: none;
    overflow-wrap: anywhere;
    line-height: 1.35;
  }

  .summary:focus-visible {
    outline: none;
  }

  /* The link covers the card: a click anywhere opens the issue. */
  .summary::after {
    content: '';
    position: absolute;
    inset: 0;
  }

  .summary:hover {
    text-decoration: underline;
  }

  .foot {
    flex-wrap: wrap;
  }

  .push {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    margin-left: auto;
  }

  .epic {
    display: inline-block;
    max-width: 100%;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    font-family: var(--font-body);
    color: var(--tone-epic);
    border-color: color-mix(in srgb, var(--tone-epic) 45%, transparent);
  }

  .estimate {
    font-variant-numeric: tabular-nums;
  }

  .avatar {
    display: inline-grid;
    place-items: center;
    width: 22px;
    height: 22px;
    flex: none;
    border: 1px solid var(--rule-strong);
    border-radius: var(--avatar-radius);
    background: var(--paper);
    color: var(--ink-soft);
    font: 500 9.5px/1 var(--font-mono);
  }

  /* Backlog rows: everything on one line. */
  .backlog-row {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .backlog-row .summary {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .backlog-row .tail {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    min-width: 0;
  }

  .backlog-row .epic {
    max-width: 180px;
  }

  .backlog-row .tools {
    margin: -4px -6px -4px 0;
  }

  @media (max-width: 760px) {
    .backlog-row {
      flex-wrap: wrap;
    }

    .backlog-row .summary {
      flex-basis: calc(100% - 80px);
      white-space: normal;
    }

    .backlog-row .tail {
      order: 2;
      flex-wrap: wrap;
      margin-left: 26px;
    }

    .backlog-row .tools {
      margin-left: auto;
    }
  }
</style>
