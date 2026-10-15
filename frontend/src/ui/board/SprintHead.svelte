<script lang="ts">
  import type { Snippet } from 'svelte';
  import { daysLeft } from '../../core/board.js';
  import type { Sprint } from '../../core/board-types.js';
  import { t } from '../../i18n/fr.js';
  import { clock } from '../../lib/ui.svelte.js';

  /**
   * The heading of a sprint (its name, dates, time left, goal) or of the
   * backlog; buttons go on the right.
   */
  interface Props {
    title: string;
    sprint?: Sprint | null | undefined;
    /** Issue count, estimate total… */
    meta?: string | undefined;
    id?: string | undefined;
    actions?: Snippet | undefined;
  }
  const { title, sprint = null, meta, id, actions }: Props = $props();

  const dates = $derived.by(() => {
    if (!sprint) return '';
    if (sprint.state !== 'active' || !sprint.start || !sprint.end) {
      return t.boards.sprintStates[sprint.state];
    }
    const range = t.boards.dates(t.boards.day(sprint.start), t.boards.day(sprint.end));
    return `${range} · ${t.boards.daysLeft(daysLeft(sprint.end, clock.now))}`;
  });
</script>

<header class="sprint-head">
  <h2 class="sprint-name" {id}>{title}</h2>
  {#if dates}<span class="sprint-dates">{dates}</span>{/if}
  {#if meta}<span class="sprint-meta">{meta}</span>{/if}
  {#if actions}<div class="sprint-actions">{@render actions()}</div>{/if}
  {#if sprint?.goal}
    <p class="sprint-goal">
      <span class="sr-only">{t.boards.goal}{t.common.labelSep}</span>{sprint.goal}
    </p>
  {/if}
</header>
