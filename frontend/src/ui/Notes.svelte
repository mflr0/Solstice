<script lang="ts">
  import { t } from '../i18n/t.js';
  import { dismiss, notes } from '../lib/ui.svelte.js';

  /** Short notes pinned at the bottom of the screen. */
</script>

<div class="toasts" role="status" aria-live="polite">
  {#each notes as n (n.id)}
    <p class="toast {n.kind}">
      {#if n.href}<a href={n.href}>{n.text}</a>{:else}<span>{n.text}</span>{/if}
      <button type="button" class="link-btn" onclick={() => dismiss(n.id)}>{t.common.close}</button>
    </p>
  {/each}
</div>
