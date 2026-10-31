<script lang="ts">
  import { highlight, highlightPrefix, highlightText } from '../core/text.js';

  /** A text with what the search matched in `mark.match`. */
  interface Props {
    text: string;
    search: string;
    /** Mark the search as a typed prefix, not as words anywhere. */
    prefix?: boolean | undefined;
    /** Mark the search by the server's rule for `text ~`. */
    fulltext?: boolean | undefined;
  }
  const { text, search, prefix = false, fulltext = false }: Props = $props();
  const parts = $derived(
    fulltext
      ? highlightText(text, search)
      : prefix
        ? highlightPrefix(text, search)
        : highlight(text, search),
  );
</script>

{#each parts as part, i (i)}{#if part.match}<mark class="match">{part.text}</mark
    >{:else}{part.text}{/if}{/each}
