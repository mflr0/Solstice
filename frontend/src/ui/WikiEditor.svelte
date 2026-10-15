<script module lang="ts">
  import type { Component } from 'svelte';

  type RichProps =
    typeof import('./editor/RichEditor.svelte').default extends Component<infer P> ? P : never;

  /** The rich editor, loaded on first use and kept for the session. */
  let loaded: Component<RichProps, object, 'value'> | null = null;
  let loading: Promise<Component<RichProps, object, 'value'>> | null = null;

  function load(): Promise<Component<RichProps, object, 'value'>> {
    loading ??= import('./editor/RichEditor.svelte').then((m) => (loaded = m.default));
    return loading;
  }
</script>

<script lang="ts">
  import type { Attachment } from '../core/types.js';
  import { t } from '../i18n/fr.js';

  /**
   * Text in wiki markup, edited as rich text: what is bound in and out is
   * always the markup. Pages that only show text never load the editor.
   */
  interface Props {
    value: string;
    label: string;
    placeholder?: string | undefined;
    rows?: number | undefined;
    attachments?: Attachment[] | undefined;
    autofocus?: boolean | undefined;
    onsubmit?: (() => void) | undefined;
    /** Escape: give up editing. */
    oncancel?: (() => void) | undefined;
    /** Issue that pasted or dropped files are attached to; none, no upload. */
    uploadTo?: string | undefined;
  }
  let {
    value = $bindable(),
    label,
    placeholder = '',
    rows = 6,
    attachments = [],
    autofocus = false,
    onsubmit,
    oncancel,
    uploadTo,
  }: Props = $props();

  let Rich = $state(loaded);
  let failed = $state(false);

  $effect(() => {
    if (Rich) return;
    load().then(
      (c) => (Rich = c),
      () => (failed = true),
    );
  });

  function keys(e: KeyboardEvent) {
    if (e.key === 'Enter' && (e.ctrlKey || e.metaKey) && onsubmit) {
      e.preventDefault();
      onsubmit();
    }
  }
</script>

{#if Rich}
  <Rich
    bind:value
    {label}
    {placeholder}
    {rows}
    {attachments}
    {autofocus}
    {onsubmit}
    {oncancel}
    {uploadTo}
  />
{:else if failed}
  <!-- The editor could not be fetched (offline…): plain markup still works. -->
  <textarea bind:value class="textarea" aria-label={label} {placeholder} {rows} onkeydown={keys}
  ></textarea>
{:else}
  <div class="editor is-loading" style:min-height={`${rows * 1.6 + 3.2}em`}>
    <p class="muted loading">{t.editor.loading}</p>
  </div>
{/if}
{#if onsubmit || oncancel}
  <p class="hint editor-hint">
    {#if onsubmit}<span>{t.common.ctrlEnter}</span>{/if}
    {#if oncancel}<span>{t.editor.escHint}</span>{/if}
  </p>
{/if}
