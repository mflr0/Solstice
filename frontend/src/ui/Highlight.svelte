<script lang="ts">
  import { highlight } from '../core/text.js';

  /** A text with what the search matched in `mark.match`. */
  interface Props {
    text: string;
    search: string;
  }
  const { text, search }: Props = $props();
  const parts = $derived(highlight(text, search));
</script>

{#each parts as part, i (i)}{#if part.match}<mark class="match">{part.text}</mark
    >{:else}{part.text}{/if}{/each}
