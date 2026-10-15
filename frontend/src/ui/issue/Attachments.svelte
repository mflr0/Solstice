<script lang="ts">
  import { fileExtension, isImage } from '../../core/marks.js';
  import type { HiddenIssue, IssueDetail } from '../../core/types.js';
  import { formatDateTime, formatSize, t } from '../../i18n/fr.js';
  import { api, errorMessage } from '../../lib/api.js';
  import { notify } from '../../lib/ui.svelte.js';

  /** Attached files: pictures as thumbnails, the rest as labelled sheets. */
  interface Props {
    issue: IssueDetail;
    onupdated: (d: IssueDetail | HiddenIssue) => void;
    onreload: () => void;
  }
  const { issue, onupdated, onreload }: Props = $props();

  let dragging = $state(false);
  let uploading = $state(false);
  let input: HTMLInputElement | undefined = $state();
  /** Deleting takes two presses: the first arms the button. */
  let armed = $state<number | null>(null);
  const canAttach = $derived(issue.can.attach === true);

  async function send(files: File[]) {
    if (!files.length || !canAttach) return;
    uploading = true;
    try {
      onupdated(await api.upload(issue.key, files));
    } catch (e) {
      notify(errorMessage(e), 'error');
    } finally {
      uploading = false;
    }
  }

  function drop(e: DragEvent) {
    e.preventDefault();
    dragging = false;
    void send([...(e.dataTransfer?.files ?? [])]);
  }

  async function remove(id: number) {
    if (armed !== id) {
      armed = id;
      return;
    }
    armed = null;
    try {
      await api.deleteAttachment(id);
      onreload();
    } catch (e) {
      notify(errorMessage(e), 'error');
    }
  }
</script>

<section
  class="attachments"
  class:dragging
  aria-label={t.issue.attachments}
  ondragover={(e) => {
    if (!canAttach) return;
    e.preventDefault();
    dragging = true;
  }}
  ondragleave={() => (dragging = false)}
  ondrop={drop}
>
  <h2 class="section-title">
    {t.issue.attachments}
    <span class="count">{issue.attachments.length}</span>
    {#if canAttach}
      <span class="actions">
        <button class="btn btn-small" disabled={uploading} onclick={() => input?.click()}>
          {uploading ? t.issue.uploading : t.issue.attach}
        </button>
        <input
          bind:this={input}
          type="file"
          multiple
          hidden
          onchange={(e) => {
            void send([...(e.currentTarget.files ?? [])]);
            e.currentTarget.value = '';
          }}
        />
      </span>
    {/if}
  </h2>

  {#if issue.attachments.length}
    <ul class="grid">
      {#each issue.attachments as a (a.id)}
        <li class="file" class:missing={a.missing}>
          {#if !a.missing && isImage(a.mimeType)}
            <a class="thumb" href={a.url} target="_blank" rel="noopener">
              <img src={a.url} alt={a.filename} loading="lazy" />
            </a>
          {:else}
            <a
              class="thumb sheet"
              href={a.missing ? undefined : `${a.url}?download=1`}
              aria-label={`${t.issue.download} ${a.filename}`}
            >
              <span class="ext">{fileExtension(a.filename) || '—'}</span>
            </a>
          {/if}
          <div class="meta">
            <span class="name" title={a.filename}>{a.filename}</span>
            <span class="sub" title={formatDateTime(a.created)}>
              {#if a.missing}<span class="error-text">{t.issue.missingFile}</span
                >{:else}{formatSize(a.size)}{/if}
              {#if a.author}· {a.author.name}{/if}
            </span>
          </div>
          {#if a.canDelete}
            <button
              class="del"
              class:armed={armed === a.id}
              aria-label={`${t.common.delete} ${a.filename}`}
              onclick={() => void remove(a.id)}
              onblur={() => (armed = null)}>{armed === a.id ? t.common.confirmDelete : '×'}</button
            >
          {/if}
        </li>
      {/each}
    </ul>
  {/if}
  {#if dragging}<p class="drop">{t.issue.dropHere}</p>{/if}
</section>

<style>
  .attachments {
    position: relative;
  }

  .attachments.dragging {
    outline: 2px dashed var(--accent);
    outline-offset: 6px;
  }

  .grid {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(150px, 1fr));
    gap: 12px;
  }

  .file {
    position: relative;
    border: 1px solid var(--rule-strong);
    background: var(--paper-raised);
  }

  .thumb {
    display: block;
    height: 96px;
    border-bottom: 1px solid var(--rule);
    background: var(--paper-sunk);
    overflow: hidden;
  }

  .thumb img {
    width: 100%;
    height: 100%;
    object-fit: cover;
    display: block;
  }

  .sheet {
    display: grid;
    place-items: center;
    text-decoration: none;
    background: var(--paper-sunk);
  }

  .ext {
    font: 600 13px/1 var(--font-mono);
    letter-spacing: 0.06em;
    padding: 4px 7px;
    border: 1.5px solid var(--ink-soft);
    background: var(--paper-raised);
  }

  .missing .ext {
    border-style: dashed;
    color: var(--muted);
  }

  .meta {
    display: grid;
    gap: 2px;
    padding: 6px 8px 7px;
    min-width: 0;
  }

  .name {
    font-size: 12.5px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .sub {
    font-size: 11.5px;
    color: var(--muted);
  }

  .del {
    position: absolute;
    top: 4px;
    right: 4px;
    width: 22px;
    height: 22px;
    border: 1px solid var(--rule-strong);
    background: var(--paper-raised);
    cursor: pointer;
    opacity: 0;
  }

  .file:hover .del,
  .del:focus-visible {
    opacity: 1;
  }

  .del:hover,
  .del.armed {
    color: var(--accent-text);
    border-color: var(--accent);
  }

  .del.armed {
    width: auto;
    padding: 0 6px;
    opacity: 1;
    font-size: 12px;
  }

  .drop {
    position: absolute;
    inset: 0;
    display: grid;
    place-items: center;
    margin: 0;
    background: color-mix(in srgb, var(--paper) 85%, transparent);
    font: var(--title-italic) 16px var(--font-title);
    color: var(--accent-text);
    pointer-events: none;
  }
</style>
