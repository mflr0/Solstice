<script lang="ts">
  import type { StatusRef } from '../core/types.js';
  import Highlight from './Highlight.svelte';

  /** A status, printed like a rubber stamp in its category's ink. */
  interface Props {
    status: StatusRef;
    large?: boolean | undefined;
    /** A search whose matches are marked in the name. */
    search?: string | undefined;
  }
  const { status, large = false, search = '' }: Props = $props();
</script>

<span
  class="stamp {status.category}"
  class:large
  data-category={status.category}
  title={status.name}
  >{#if search}<Highlight text={status.name} {search} />{:else}{status.name}{/if}</span
>
