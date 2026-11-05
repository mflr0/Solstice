<script lang="ts">
  import { t } from '../i18n/t.js';

  /** Says a theme was written for an older contract than the server's; nothing otherwise. */
  let { theme, server }: { theme: number; server: number } = $props();
</script>

{#if theme < server}
  <span class="flag">{t.themes.outdated(theme, server)}</span>
{/if}

<style>
  .flag {
    color: var(--accent-text);
    font-size: 0.8125rem;
    overflow-wrap: anywhere;
  }
</style>
