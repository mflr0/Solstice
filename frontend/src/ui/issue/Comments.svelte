<script lang="ts">
  import { tick as settled } from 'svelte';
  import { focusAfterRemoval } from '../../core/focus.js';
  import type { Comment, IssueDetail } from '../../core/types.js';
  import { t } from '../../i18n/t.js';
  import { refocus } from '../../lib/actions.js';
  import { api, errorMessage } from '../../lib/api.js';
  import { notify } from '../../lib/ui.svelte.js';
  import RelTime from '../RelTime.svelte';
  import Who from '../Who.svelte';
  import WikiEditor from '../WikiEditor.svelte';
  import WikiText from '../WikiText.svelte';

  /** The conversation, oldest first, with the writing box at the end. */
  interface Props {
    issue: IssueDetail;
    /** Bumped by the page when the issue changed elsewhere. */
    tick: number;
  }
  const { issue, tick }: Props = $props();
  const uid = $props.id();

  let comments = $state<Comment[] | null>(null);
  let error = $state('');
  let draft = $state('');
  let sending = $state(false);
  let editing = $state<number | null>(null);
  let editDraft = $state('');
  let armed = $state<number | null>(null);
  let compose = $state<HTMLElement>();
  let thread = $state<HTMLElement>();

  async function load(key: string) {
    try {
      const list = await api.comments(key);
      if (key === issue.key) {
        comments = list;
        error = '';
      }
    } catch (e) {
      error = errorMessage(e);
    }
  }

  $effect(() => {
    void tick;
    void load(issue.key);
  });

  async function send() {
    if (!draft.trim() || sending) return;
    sending = true;
    try {
      const c = await api.addComment(issue.key, draft);
      comments = [...(comments ?? []), c];
      draft = '';
    } catch (e) {
      notify(errorMessage(e), 'error');
    } finally {
      sending = false;
    }
  }

  async function stopEditing(id: number) {
    editing = null;
    await settled();
    refocus(document.getElementById(`comment-edit-${id}-${uid}`));
  }

  /** The writing box, or the tab for someone who may edit a comment but not write one. */
  function addControl() {
    return (
      compose?.querySelector<HTMLElement>('[contenteditable="true"], textarea') ??
      thread
        ?.closest('.issue-activity')
        ?.querySelector<HTMLElement>('[role="tab"][aria-selected="true"]')
    );
  }

  async function saveEdit(id: number) {
    if (!editDraft.trim()) return;
    try {
      const c = await api.editComment(id, editDraft);
      comments = (comments ?? []).map((x) => (x.id === id ? c : x));
      await stopEditing(id);
    } catch (e) {
      notify(errorMessage(e), 'error');
    }
  }

  async function remove(id: number) {
    if (armed !== id) {
      armed = id;
      return;
    }
    armed = null;
    try {
      await api.deleteComment(id);
      const list = comments ?? [];
      const next = focusAfterRemoval(
        list.filter((x) => x.canEdit && x.id !== editing).map((x) => x.id),
        id,
      );
      comments = list.filter((x) => x.id !== id);
      await settled();
      refocus(document.getElementById(`comment-delete-${next}-${uid}`) ?? addControl());
    } catch (e) {
      notify(errorMessage(e), 'error');
    }
  }
</script>

{#if error}
  <p class="error-text" role="alert">{error}</p>
{:else if !comments}
  <p class="muted loading">{t.common.loading}</p>
{:else}
  {#if comments.length === 0}
    <p class="empty">{t.comments.empty}</p>
  {/if}
  <ol class="thread" bind:this={thread}>
    {#each comments as c (c.id)}
      <li class="comment list-row" id={`comment-${c.id}`}>
        <header>
          <Who user={c.author} empty="—" />
          <span class="when muted">
            <RelTime iso={c.created} />
            {#if c.updated !== c.created}
              <span class="edited" title={c.updatedBy?.name ?? ''}>· {t.comments.edited}</span>
            {/if}
          </span>
          {#if c.visibility}
            <span class="restricted">{t.comments.visibleTo(c.visibility.group ?? '')}</span>
          {/if}
          {#if c.canEdit && editing !== c.id}
            <span class="tools">
              <button
                id={`comment-edit-${c.id}-${uid}`}
                class="link-btn"
                onclick={() => {
                  editing = c.id;
                  editDraft = c.body;
                }}>{t.common.edit}</button
              >
              <button
                id={`comment-delete-${c.id}-${uid}`}
                class="link-btn"
                class:armed={armed === c.id}
                onclick={() => void remove(c.id)}
                onblur={() => (armed = null)}
                >{armed === c.id ? t.common.confirmDelete : t.common.delete}</button
              >
            </span>
          {/if}
        </header>
        {#if editing === c.id}
          <WikiEditor
            bind:value={editDraft}
            label={t.common.edit}
            attachments={issue.attachments}
            autofocus
            onsubmit={() => void saveEdit(c.id)}
            oncancel={() => void stopEditing(c.id)}
            uploadTo={issue.can.attach ? issue.key : undefined}
          />
          <div class="row end">
            <button class="btn btn-quiet btn-small" onclick={() => void stopEditing(c.id)}
              >{t.common.cancel}</button
            >
            <button class="btn btn-primary btn-small" onclick={() => void saveEdit(c.id)}
              >{t.common.save}</button
            >
          </div>
        {:else}
          <WikiText source={c.body} attachments={issue.attachments} />
        {/if}
      </li>
    {/each}
  </ol>

  {#if issue.can.comment}
    <div class="compose" bind:this={compose}>
      <WikiEditor
        bind:value={draft}
        label={t.comments.placeholder}
        placeholder={t.comments.placeholder}
        rows={4}
        attachments={issue.attachments}
        onsubmit={() => void send()}
        uploadTo={issue.can.attach ? issue.key : undefined}
      />
      <div class="row end">
        <button
          class="btn btn-primary"
          disabled={sending || !draft.trim()}
          onclick={() => void send()}>{t.comments.send}</button
        >
      </div>
    </div>
  {/if}
{/if}

<style>
  .thread {
    list-style: none;
    margin: 0;
    padding: 0;
  }

  .comment {
    padding: 12px 0 6px;
    border-bottom: 1px solid var(--rule);
  }

  header {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 6px 12px;
    margin-bottom: 6px;
    font-size: 0.8125rem;
  }

  header :global(.who) {
    font-weight: 600;
  }

  .when {
    font-size: 0.78125rem;
  }

  .edited {
    margin-left: 3px;
  }

  .restricted {
    font: 0.71875rem var(--font-mono);
    color: var(--accent-text);
    border: 1px solid var(--margin-line);
    padding: 0 5px;
  }

  .tools {
    margin-left: auto;
    display: flex;
    gap: 12px;
    font-size: 0.78125rem;
  }

  .tools .armed {
    color: var(--accent-text);
  }

  .comment :global(.wiki) {
    padding-left: 27px;
  }

  .compose {
    margin-top: 16px;
  }

  .end {
    justify-content: flex-end;
    margin-top: 8px;
  }
</style>
