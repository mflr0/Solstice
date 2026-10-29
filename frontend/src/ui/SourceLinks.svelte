<script lang="ts">
  import { LICENSES_PATH, SOURCE_URL } from '../lib/links.js';

  /**
   * The two links the AGPL asks every user to be offered, signed in or not.
   * The texts come from the caller, as the setup wizard has its own catalogue.
   */
  interface Props {
    source: string;
    licenses: string;
  }
  const { source, licenses }: Props = $props();
</script>

<p class="source-links">
  <a href={SOURCE_URL} target="_blank" rel="noopener noreferrer">{source}</a>
  <span aria-hidden="true">·</span>
  <a href={LICENSES_PATH} target="_blank" rel="noopener noreferrer">{licenses}</a>
</p>

<style>
  .source-links {
    margin: 0;
    font-size: 0.75rem;
    color: var(--muted);
  }
</style>
