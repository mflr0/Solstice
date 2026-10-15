<script lang="ts">
  import type { Snippet } from 'svelte';
  import Illustration from './Illustration.svelte';

  /**
   * An empty list that says what to do next: a line on what would be here,
   * then the action that fills it (a button or a link).
   */
  interface Props {
    message: string;
    hint?: string | undefined;
    kind?: 'empty' | 'search' | undefined;
    children?: Snippet | undefined;
  }
  const { message, hint, kind = 'empty', children }: Props = $props();
</script>

<div class="empty-state">
  <Illustration {kind} />
  <p class="empty">{message}</p>
  {#if hint}<p class="muted hint">{hint}</p>{/if}
  {#if children}<div class="row">{@render children()}</div>{/if}
</div>

<style>
  .empty-state {
    display: grid;
    justify-items: start;
    gap: 6px;
    padding: 18px 0 8px;
  }

  .empty {
    margin: 0;
    padding: 0;
  }

  .hint {
    margin: 0 0 6px;
    max-width: 62ch;
    font-size: 13px;
  }
</style>
