<script lang="ts">
  import { PRIORITY_GLYPHS, priorityLevel } from '../core/marks.js';
  import type { PriorityRef } from '../core/types.js';
  import { session } from '../lib/session.svelte.js';

  /** A priority as stacked triangles, tinted with the priority's colour. */
  interface Props {
    priority: PriorityRef | null;
    withName?: boolean | undefined;
  }
  const { priority, withName = false }: Props = $props();

  const index = $derived(priority ? session.priorityIndex(priority) : -1);
  const level = $derived(priorityLevel(index, session.meta?.priorities.length ?? 0));
</script>

{#if priority}
  <span class="prio priority-mark" title={priority.name}>
    <span
      class="glyph"
      data-level={index < 0 ? 'other' : level}
      style:--priority-color={priority.color || null}
      aria-hidden="true">{PRIORITY_GLYPHS[level]}</span
    >
    {#if withName}<span>{priority.name}</span>{:else}<span class="sr-only">{priority.name}</span
      >{/if}
  </span>
{/if}

<style>
  .prio {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    white-space: nowrap;
  }

  /* The server's colour comes in --priority-color; skins may override either. */
  .glyph {
    color: var(--priority-color, var(--ink-soft));
    display: inline-block;
    min-width: 18px;
    font-size: 10.5px;
    letter-spacing: -1px;
    line-height: 1;
  }
</style>
