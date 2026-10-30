<script lang="ts">
  import { highlight, highlightPrefix } from '../core/text.js';

  /** A text with what the search matched in `mark.match`. */
  interface Props {
    text: string;
    search: string;
    /** Mark the search as a typed prefix, not as words anywhere. */
    prefix?: boolean | undefined;
  }
  const { text, search, prefix = false }: Props = $props();
  const parts = $derived(prefix ? highlightPrefix(text, search) : highlight(text, search));
</script>

{#each parts as part, i (i)}{#if part.match}<mark class="match">{part.text}</mark
    >{:else}{part.text}{/if}{/each}
