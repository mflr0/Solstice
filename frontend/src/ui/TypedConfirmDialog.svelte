<script lang="ts">
  import type { Snippet } from 'svelte';
  import { typedMatches } from '../core/admin-forms.js';
  import { t } from '../i18n/t.js';
  import { autofocus } from '../lib/actions.js';
  import { errorMessage } from '../lib/api.js';
  import Modal from './Modal.svelte';

  /**
   * Asks before something that cannot be undone, and wants a phrase typed
   * (a project's key) before the button works. Like ConfirmDialog, the
   * action runs from the dialog, which stays open and shows the server's
   * refusal in place.
   */
  interface Props {
    open: boolean;
    title: string;
    /** What must be typed, exactly. */
    phrase: string;
    /** The field's label ("Type SW to confirm"). */
    label: string;
    /** The button that confirms ("Delete the project"). */
    confirm: string;
    onconfirm: () => Promise<unknown>;
    onclose: () => void;
    children: Snippet;
  }
  const { open, title, phrase, label, confirm, onconfirm, onclose, children }: Props = $props();

  let typed = $state('');
  let busy = $state(false);
  let error = $state('');
  const ready = $derived(typedMatches(typed, phrase));

  $effect(() => {
    if (open) {
      typed = '';
      error = '';
    }
  });

  async function run() {
    if (!ready || busy) return;
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
  <form
    class="typed-body"
    onsubmit={(e) => {
      e.preventDefault();
      void run();
    }}
  >
    <div class="typed-text">{@render children()}</div>
    <label class="field">
      <span>{label}</span>
      <input
        class="input mono"
        bind:value={typed}
        autocomplete="off"
        spellcheck="false"
        autocapitalize="off"
        use:autofocus
      />
    </label>
  </form>
  {#if error}<p class="error-text" role="alert">{error}</p>{/if}
  {#snippet footer()}
    <button type="button" class="btn btn-quiet" disabled={busy} onclick={onclose}
      >{t.common.cancel}</button
    >
    <button
      type="button"
      class="btn btn-danger"
      disabled={busy || !ready}
      onclick={() => void run()}>{busy ? t.adminArea.working : confirm}</button
    >
  {/snippet}
</Modal>

<style>
  .typed-body {
    display: grid;
    gap: 14px;
  }

  .typed-text {
    display: grid;
    gap: 8px;
  }

  .typed-text :global(p) {
    margin: 0;
  }

  .mono {
    font-family: var(--font-mono);
    font-size: 13px;
  }
</style>
