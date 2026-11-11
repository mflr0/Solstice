<script lang="ts">
  import { toggleQuick } from '../../core/board.js';
  import type { QuickFilter } from '../../core/board-types.js';
  import { t } from '../../i18n/t.js';
  import { router } from '../../lib/router.svelte.js';

  /**
   * A board's quick filters as toggle buttons, at the end of the quick filter
   * row; the choice lives in the address (?quick=1,3).
   */
  interface Props {
    filters: QuickFilter[];
    selected: number[];
  }
  const { filters, selected }: Props = $props();

  function set(value: string) {
    router.setParams({ quick: value || null });
  }
</script>

{#if filters.length}
  <span class="divider" aria-hidden="true"></span>
  <div class="quick" role="group" aria-label={t.boards.quickFilters}>
    {#each filters as f (f.id)}
      <button
        type="button"
        class="btn btn-small"
        aria-pressed={selected.includes(f.id)}
        title={f.description || f.query || undefined}
        onclick={() => set(toggleQuick(selected, f.id))}>{f.name}</button
      >
    {/each}
    {#if selected.length}
      <button type="button" class="link-btn" onclick={() => set('')}>{t.boards.showAll}</button>
    {/if}
  </div>
{/if}

<style>
  /* Its buttons wrap with the rest of the row. */
  .quick {
    display: contents;
  }

  .divider {
    align-self: stretch;
    width: 1px;
    margin: 2px 4px;
    background: var(--rule-strong);
  }

  .link-btn {
    margin-left: 6px;
    font-size: 0.78125rem;
  }
</style>
