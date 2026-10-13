<script lang="ts">
  import { formatDateTime, relative } from '../i18n/fr.js';
  import { clock } from '../lib/ui.svelte.js';

  /** An instant said relatively ("il y a 3 h"); the exact date on hover. */
  interface Props {
    iso: string;
  }
  const { iso }: Props = $props();
  const text = $derived(relative(iso, clock.now));
</script>

<time datetime={iso} title={formatDateTime(iso)}>{text}</time>
