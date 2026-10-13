<script lang="ts">
  import type { Attachment } from '../core/types.js';
  import { t } from '../i18n/fr.js';
  import WikiText from './WikiText.svelte';

  /** A plain textarea for wiki markup, with a preview tab and Ctrl+Enter. */
  interface Props {
    value: string;
    label: string;
    placeholder?: string | undefined;
    rows?: number | undefined;
    attachments?: Attachment[] | undefined;
    autofocus?: boolean | undefined;
    onsubmit?: (() => void) | undefined;
  }
  let {
    value = $bindable(),
    label,
    placeholder = '',
    rows = 6,
    attachments = [],
    autofocus = false,
    onsubmit,
  }: Props = $props();

  let preview = $state(false);
  let area: HTMLTextAreaElement | undefined = $state();
  const id = `wiki-${Math.random().toString(36).slice(2, 8)}`;

  $effect(() => {
    if (autofocus) area?.focus();
  });

  function keydown(e: KeyboardEvent) {
    if (e.key === 'Enter' && (e.ctrlKey || e.metaKey) && onsubmit) {
      e.preventDefault();
      onsubmit();
    }
  }
</script>

<div class="editor">
  <div class="tabs" role="tablist" aria-label={label}>
    <button
      type="button"
      role="tab"
      aria-selected={!preview}
      aria-controls={id}
      class:active={!preview}
      onclick={() => (preview = false)}>{t.common.write}</button
    >
    <button
      type="button"
      role="tab"
      aria-selected={preview}
      aria-controls={id}
      class:active={preview}
      onclick={() => (preview = true)}>{t.common.preview}</button
    >
  </div>
  <div {id} role="tabpanel">
    {#if preview}
      <div class="preview">
        {#if value.trim()}
          <WikiText source={value} {attachments} />
        {:else}
          <p class="empty">{t.common.nothingToPreview}</p>
        {/if}
      </div>
    {:else}
      <textarea
        bind:this={area}
        bind:value
        class="textarea"
        aria-label={label}
        {placeholder}
        {rows}
        onkeydown={keydown}
      ></textarea>
    {/if}
  </div>
  <p class="hint">
    <code>{t.comments.hint}</code>
    {#if onsubmit}<span>{t.common.ctrlEnter}</span>{/if}
  </p>
</div>

<style>
  .tabs {
    display: flex;
    gap: 2px;
    margin-bottom: -1px;
  }

  .tabs button {
    border: 1px solid transparent;
    border-bottom: none;
    background: none;
    padding: 4px 10px;
    font-size: 12px;
    color: var(--muted);
    cursor: pointer;
  }

  .tabs button.active {
    border-color: var(--rule-strong);
    background: var(--paper-raised);
    color: var(--ink);
  }

  .preview {
    border: 1px solid var(--rule-strong);
    background: var(--paper-raised);
    padding: 10px 12px;
    min-height: 90px;
  }

  .hint {
    display: flex;
    justify-content: space-between;
    gap: 12px;
    margin: 4px 0 0;
    font-size: 11.5px;
    color: var(--muted);
  }

  .hint code {
    font-size: 11px;
    white-space: pre-wrap;
  }
</style>
