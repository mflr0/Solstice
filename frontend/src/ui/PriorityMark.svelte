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

  const glyph = $derived.by(() => {
    if (!priority) return '';
    const count = session.meta?.priorities.length ?? 0;
    return PRIORITY_GLYPHS[priorityLevel(session.priorityIndex(priority), count)];
  });
</script>

{#if priority}
  <span class="prio" title={priority.name}>
    <span class="glyph" style:color={priority.color || 'var(--ink-soft)'} aria-hidden="true"
      >{glyph}</span
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

  .glyph {
    display: inline-block;
    min-width: 18px;
    font-size: 10.5px;
    letter-spacing: -1px;
    line-height: 1;
  }
</style>
