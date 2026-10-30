<script lang="ts">
  import type { Snippet } from 'svelte';
  import { backdropClick, type Point } from '../core/modal.js';
  import { t } from '../i18n/t.js';

  /**
   * A modal built on <dialog>: the browser traps focus. Escape, the Close
   * button and a click on the backdrop all go through onclose, so a dialog
   * that is busy can refuse.
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

  // Where the press started, when it started on the backdrop: a drag out of
  // the dialog (selecting text) must not close it.
  let down: Point | null = null;

  function onpointerdown(e: PointerEvent): void {
    down = e.target === dialog ? { x: e.clientX, y: e.clientY } : null;
  }

  // Backdrop clicks target the dialog itself; so do clicks on its padding,
  // which the rect test keeps.
  function onclick(e: MouseEvent): void {
    const start = down;
    down = null;
    if (!dialog || e.target !== dialog || !start) return;
    if (backdropClick(dialog.getBoundingClientRect(), start, { x: e.clientX, y: e.clientY }))
      onclose();
  }

  // Escape: let onclose decide instead of the browser closing it outright.
  function oncancel(e: Event): void {
    e.preventDefault();
    onclose();
  }

  $effect(() => {
    if (!dialog) return;
    if (open && !dialog.open) dialog.showModal();
    else if (!open && dialog.open) dialog.close();
  });
</script>

<dialog
  bind:this={dialog}
  class="modal"
  aria-labelledby={titleId}
  {onclose}
  {oncancel}
  {onpointerdown}
  {onclick}
>
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
