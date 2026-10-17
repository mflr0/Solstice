<script lang="ts">
  /**
   * The two links the AGPL asks every user to be offered, signed in or not:
   * the source code, and the licences of what ships with the application.
   * The texts come from the caller, as the setup wizard has its own catalogue.
   */
  interface Props {
    source: string;
    licenses: string;
  }
  const { source, licenses }: Props = $props();
</script>

<p class="source-links">
  <a href="https://github.com/mflr0/Solstice" target="_blank" rel="noopener noreferrer">{source}</a>
  <span aria-hidden="true">·</span>
  <a href="/licenses/README.txt" target="_blank" rel="noopener noreferrer">{licenses}</a>
</p>

<style>
  .source-links {
    margin: 0;
    font-size: 12px;
    color: var(--muted);
  }
</style>
