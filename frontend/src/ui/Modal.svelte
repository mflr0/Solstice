<script lang="ts">
  import type { Snippet } from 'svelte';
  import { t } from '../i18n/t.js';

  /**
   * A modal built on <dialog>: the browser traps focus and closes it on
   * Escape. A click outside does not close it, so a half-typed form is not
   * lost by accident.
   */
  interface Props {
    open: boolean;
    title: string;
    onclose: () => void;
    children: Snippet;
    footer?: Snippet | undefined;
  }
  const { open, title, onclose, children, footer }: Props = $props();

  let dialog: HTMLDialogElement | undefined = $state();
  const uid = $props.id();
  const titleId = `dlg-${uid}`;

  $effect(() => {
    if (!dialog) return;
    if (open && !dialog.open) dialog.showModal();
    else if (!open && dialog.open) dialog.close();
  });
</script>

<dialog bind:this={dialog} class="modal" aria-labelledby={titleId} {onclose}>
  {#if open}
    <header class="modal-head">
      <h2 class="modal-title" id={titleId}>{title}</h2>
      <button type="button" class="btn btn-quiet btn-small modal-close" onclick={onclose}
        >{t.common.close} <kbd>{t.common.escKey}</kbd></button
      >
    </header>
    <div class="modal-body">{@render children()}</div>
    {#if footer}<footer class="modal-foot">{@render footer()}</footer>{/if}
  {/if}
</dialog>
