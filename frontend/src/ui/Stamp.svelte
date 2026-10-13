<script lang="ts">
  import type { StatusRef } from '../core/types.js';

  /** A status, printed like a rubber stamp in its category's ink. */
  interface Props {
    status: StatusRef;
    large?: boolean | undefined;
  }
  const { status, large = false }: Props = $props();
</script>

<span class="stamp {status.category}" class:large title={status.name}>{status.name}</span>
