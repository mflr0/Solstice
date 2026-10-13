<script lang="ts">
  import { t } from '../i18n/fr.js';
  import { dismiss, notes } from '../lib/ui.svelte.js';

  /** Short notes pinned at the bottom of the screen. */
</script>

<div class="notes" role="status" aria-live="polite">
  {#each notes as n (n.id)}
    <p class="note {n.kind}">
      <span>{n.text}</span>
      <button type="button" class="link-btn" onclick={() => dismiss(n.id)}>{t.common.close}</button>
    </p>
  {/each}
</div>

<style>
  .notes {
    position: fixed;
    left: 20px;
    bottom: 20px;
    z-index: 60;
    display: grid;
    gap: 8px;
    max-width: min(440px, calc(100vw - 40px));
  }

  .note {
    display: flex;
    gap: 14px;
    align-items: baseline;
    justify-content: space-between;
    margin: 0;
    padding: 10px 14px;
    background: var(--paper-raised);
    border: 1px solid var(--rule-strong);
    border-left: 3px solid var(--ink-soft);
    border-radius: 4px;
    box-shadow: var(--shadow-pop);
    font-size: 13.5px;
  }

  .note.error {
    border-left-color: var(--accent);
  }
</style>
