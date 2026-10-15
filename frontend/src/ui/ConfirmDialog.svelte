<script lang="ts">
  import type { Snippet } from 'svelte';
  import { t } from '../i18n/fr.js';
  import { errorMessage } from '../lib/api.js';
  import Modal from './Modal.svelte';

  /**
   * Asks before something that cannot be undone. The action runs from the
   * dialog: it stays open while it works and shows the server's refusal in
   * place, so nothing is lost on a mistake.
   */
  interface Props {
    open: boolean;
    title: string;
    /** The button that confirms ("Supprimer", "Révoquer"…). */
    confirm: string;
    /** Destructive actions draw the button in the danger style. */
    danger?: boolean | undefined;
    onconfirm: () => Promise<unknown>;
    onclose: () => void;
    children: Snippet;
  }
  const { open, title, confirm, danger = true, onconfirm, onclose, children }: Props = $props();

  let busy = $state(false);
  let error = $state('');

  $effect(() => {
    if (open) error = '';
  });

  async function run() {
    busy = true;
    error = '';
    try {
      await onconfirm();
      onclose();
    } catch (e) {
      error = errorMessage(e);
    } finally {
      busy = false;
    }
  }
</script>

<Modal {open} {title} onclose={() => !busy && onclose()}>
  <div class="confirm-body">{@render children()}</div>
  {#if error}<p class="error-text" role="alert">{error}</p>{/if}
  {#snippet footer()}
    <button type="button" class="btn btn-quiet" disabled={busy} onclick={onclose}
      >{t.common.cancel}</button
    >
    <button
      type="button"
      class="btn {danger ? 'btn-danger' : 'btn-primary'}"
      disabled={busy}
      onclick={() => void run()}>{busy ? t.adminArea.working : confirm}</button
    >
  {/snippet}
</Modal>

<style>
  .confirm-body {
    display: grid;
    gap: 8px;
  }

  .confirm-body :global(p) {
    margin: 0;
  }
</style>
