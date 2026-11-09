<script lang="ts">
  import { fileExtension, isImage } from '../../core/marks.js';
  import type { HiddenIssue, IssueDetail } from '../../core/types.js';
  import { formatDateTime, formatSize, t } from '../../i18n/t.js';
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
  /** Nested dragenter/dragleave pairs, so crossing child nodes does not hide the overlay. */
  let depth = 0;
  let section: HTMLElement | undefined = $state();
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

  /** Files from outside, not text, links or a dragged issue. */
  const carriesFiles = (e: DragEvent): boolean => e.dataTransfer?.types.includes('Files') ?? false;

  /** Whether no dialog is open over this view (a background page, the clone dialog). */
  function inFront(): boolean {
    const top = [...document.querySelectorAll('dialog:modal')].at(-1);
    return !top || (!!section && top.contains(section));
  }

  function endDrag() {
    depth = 0;
    dragging = false;
  }

  function enter(e: DragEvent) {
    if (!canAttach || !carriesFiles(e) || !inFront()) return;
    depth++;
    dragging = true;
  }

  function leave(e: DragEvent) {
    if (!carriesFiles(e) || depth === 0) return;
    if (--depth === 0) dragging = false;
  }

  // Taken even without attach, or the browser opens the file in place of the page.
  function over(e: DragEvent) {
    if (!carriesFiles(e) || !inFront()) return;
    e.preventDefault();
    if (e.dataTransfer) e.dataTransfer.dropEffect = canAttach ? 'copy' : 'none';
  }

  function drop(e: DragEvent) {
    if (!carriesFiles(e) || !inFront()) return;
    // An editor already took it and inserted its reference.
    const taken = e.defaultPrevented;
    e.preventDefault();
    endDrag();
    if (!taken) void send([...(e.dataTransfer?.files ?? [])]);
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

<svelte:window ondragenter={enter} ondragleave={leave} ondragover={over} ondrop={drop} />

<section bind:this={section} class="attachments" class:dragging aria-label={t.issue.attachments}>
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
  {#if dragging}<p class="drop"><span>{t.issue.dropToAttach(issue.key)}</span></p>{/if}
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
    font: 600 0.8125rem/1 var(--font-mono);
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
    font-size: 0.78125rem;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .sub {
    font-size: 0.71875rem;
    color: var(--muted);
  }

  .del {
    position: absolute;
    top: 4px;
    right: 4px;
    width: 1.375rem;
    height: 1.375rem;
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
    font-size: 0.75rem;
  }

  /* Over the whole view: files land anywhere on it. */
  .drop {
    position: fixed;
    inset: 0;
    z-index: 65;
    display: grid;
    place-items: center;
    margin: 0;
    outline: 2px dashed var(--accent);
    outline-offset: -12px;
    background: color-mix(in srgb, var(--paper) 70%, transparent);
    font: var(--title-italic) 1rem var(--font-title);
    color: var(--accent-text);
    pointer-events: none;
  }

  .drop span {
    padding: 10px 18px;
    border: 1px solid var(--rule-strong);
    background: var(--paper-raised);
    box-shadow: var(--shadow-pop);
  }
</style>
