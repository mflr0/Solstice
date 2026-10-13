<script module lang="ts">
  /** Focuses an input and selects its text, for in-place editing. */
  function focusSelect(node: HTMLInputElement) {
    queueMicrotask(() => {
      node.focus();
      node.select();
    });
  }
</script>

<script lang="ts">
  import { untrack } from 'svelte';
  import { paths } from '../../core/route.js';
  import type { ChangeEvent, HiddenIssue, IssueDetail } from '../../core/types.js';
  import { t } from '../../i18n/fr.js';
  import { outside } from '../../lib/actions.js';
  import { api, ApiError, errorMessage, isVisible, type IssueFields } from '../../lib/api.js';
  import { live } from '../../lib/live.svelte.js';
  import { router } from '../../lib/router.svelte.js';
  import { session } from '../../lib/session.svelte.js';
  import { notify, openCreate } from '../../lib/ui.svelte.js';
  import PriorityMark from '../PriorityMark.svelte';
  import RelTime from '../RelTime.svelte';
  import Stamp from '../Stamp.svelte';
  import TypeMark from '../TypeMark.svelte';
  import Who from '../Who.svelte';
  import WikiEditor from '../WikiEditor.svelte';
  import WikiText from '../WikiText.svelte';
  import NotFoundPage from '../pages/NotFoundPage.svelte';
  import Attachments from './Attachments.svelte';
  import Comments from './Comments.svelte';
  import Fiche from './Fiche.svelte';
  import History from './History.svelte';
  import Links from './Links.svelte';
  import StatusControl from './StatusControl.svelte';
  import Worklogs from './Worklogs.svelte';

  /** An issue: its text and activity on the left, its record card on the right. */
  interface Props {
    issueKey: string;
  }
  const { issueKey }: Props = $props();

  let issue = $state<IssueDetail | null>(null);
  let missing = $state('');
  let tab = $state<'comments' | 'history' | 'worklogs'>('comments');
  let tick = $state(0);
  let note = $state('');

  let editingSummary = $state(false);
  let summaryDraft = $state('');
  let editingDescription = $state(false);
  let descriptionDraft = $state('');
  let savingDescription = $state(false);
  let menuOpen = $state(false);
  let deleteArmed = $state(false);

  async function load(key: string) {
    try {
      const d = await api.issue(key);
      if (key !== issueKey) return;
      issue = d;
      missing = '';
      document.title = `${d.key} ${d.summary} — ${t.appName}`;
      // An old key (renamed project, moved issue): show the current one.
      if (d.key !== key) {
        notify(t.issue.moved(key, d.key));
        router.navigate(paths.issue(d.key), { replace: true });
      }
    } catch (e) {
      if (key !== issueKey) return;
      if (e instanceof ApiError && e.status === 404) missing = e.message;
      else notify(errorMessage(e), 'error');
    }
  }

  $effect(() => {
    const key = issueKey;
    untrack(() => {
      // After a redirect from an old key the issue is already here.
      if (issue && issue.key === key) return;
      issue = null;
      missing = '';
      note = '';
      editingSummary = false;
      editingDescription = false;
      tab = 'comments';
      void load(key);
    });
  });

  // Someone else changed this issue: reload it and say who.
  $effect(() =>
    live.onChange((e: ChangeEvent) => {
      if (!issue || e.key !== issue.key) return;
      if (e.what === 'deleted') {
        missing = t.issue.deleted(e.key);
        issue = null;
        return;
      }
      void load(issue.key).then(() => {
        tick++;
        if (e.by !== session.me?.id) void whoChanged(e);
      });
    }),
  );

  /** The event only carries an id; the latest activity tells the name. */
  async function whoChanged(e: ChangeEvent) {
    let name = '';
    try {
      if (e.what === 'comment') {
        const list = await api.comments(e.key);
        name =
          [...list].reverse().find((c) => c.updatedBy?.id === e.by || c.author?.id === e.by)?.author
            ?.name ?? '';
      } else {
        const h = await api.history(e.key);
        name = [...h].reverse().find((x) => x.author?.id === e.by)?.author?.name ?? '';
      }
    } catch {
      // Keep the anonymous wording.
    }
    note = t.issue.updatedBy(name || t.issue.someone);
  }

  function accept(d: IssueDetail | HiddenIssue) {
    if (isVisible(d)) {
      issue = d;
      tick++;
    } else {
      notify(t.issue.hidden(d.key));
      router.navigate(paths.project(issue?.project.key ?? ''));
    }
  }

  async function update(fields: IssueFields): Promise<void> {
    if (!issue) return;
    try {
      accept(await api.updateIssue(issue.key, fields));
    } catch (e) {
      notify(errorMessage(e), 'error');
      throw e;
    }
  }

  async function saveSummary() {
    if (!issue) return;
    const v = summaryDraft.trim();
    editingSummary = false;
    if (v && v !== issue.summary) await update({ summary: v }).catch(() => {});
  }

  async function saveDescription() {
    if (!issue) return;
    savingDescription = true;
    try {
      await update({ description: descriptionDraft.trim() ? descriptionDraft : null });
      editingDescription = false;
    } catch {
      // The note already says why.
    } finally {
      savingDescription = false;
    }
  }

  async function toggle(kind: 'watch' | 'vote') {
    if (!issue) return;
    try {
      const counter =
        kind === 'watch'
          ? await api.watch(issue.key, !issue.watchers.me)
          : await api.vote(issue.key, !issue.votes.me);
      if (kind === 'watch') issue.watchers = counter;
      else issue.votes = counter;
    } catch (e) {
      notify(errorMessage(e), 'error');
    }
  }

  async function remove() {
    if (!issue) return;
    if (!deleteArmed) {
      deleteArmed = true;
      return;
    }
    const { key, project } = issue;
    try {
      await api.deleteIssue(key);
      notify(t.issue.deleted(key));
      router.navigate(paths.project(project.key));
    } catch (e) {
      notify(errorMessage(e), 'error');
    }
  }
</script>

{#if missing}
  <NotFoundPage message={missing} />
{:else if !issue}
  <div class="page"><p class="muted">{t.common.loading}</p></div>
{:else}
  <article class="page issue">
    <nav class="crumbs" aria-label={t.common.breadcrumbs}>
      <a href={paths.project(issue.project.key)}>{issue.project.name}</a>
      {#if issue.parent}
        <span aria-hidden="true">/</span>
        <a class="key" href={paths.issue(issue.parent.key)} title={issue.parent.summary}
          >{issue.parent.key}</a
        >
      {/if}
      <span aria-hidden="true">/</span>
      <span class="key current">{issue.key}</span>
    </nav>

    <header class="head">
      {#if editingSummary}
        <input
          class="summary-input"
          bind:value={summaryDraft}
          aria-label={t.issue.editSummary}
          maxlength="255"
          onkeydown={(e) => {
            if (e.key === 'Enter') void saveSummary();
            if (e.key === 'Escape') editingSummary = false;
          }}
          onblur={() => void saveSummary()}
          use:focusSelect
        />
      {:else if issue.can.edit}
        <h1>
          <button
            class="summary-btn"
            title={t.issue.editSummary}
            onclick={() => {
              summaryDraft = issue?.summary ?? '';
              editingSummary = true;
            }}>{issue.summary}</button
          >
        </h1>
      {:else}
        <h1>{issue.summary}</h1>
      {/if}

      <div class="meta">
        <StatusControl {issue} onchange={update} />
        {#if issue.resolution}<span class="resolution">{issue.resolution.name}</span>{/if}
        <span class="dot" aria-hidden="true">·</span>
        <TypeMark type={issue.type} withName />
        {#if issue.priority}
          <span class="dot" aria-hidden="true">·</span>
          <PriorityMark priority={issue.priority} withName />
        {/if}
        <span class="dot" aria-hidden="true">·</span>
        <span class="created muted">
          {t.issue.created.toLowerCase()}
          <RelTime iso={issue.created} />
          {#if issue.creator}{t.issue.by} <Who user={issue.creator} initialsBox={false} />{/if}
        </span>

        <span class="actions">
          <button
            class="btn btn-small"
            class:on={issue.watchers.me}
            aria-pressed={issue.watchers.me}
            onclick={() => void toggle('watch')}
          >
            {issue.watchers.me ? t.issue.watching : t.issue.watch}
            <span class="n">{issue.watchers.count}</span>
          </button>
          <button
            class="btn btn-small"
            class:on={issue.votes.me}
            aria-pressed={issue.votes.me}
            onclick={() => void toggle('vote')}
          >
            {issue.votes.me ? t.issue.voted : t.issue.vote}
            <span class="n">{issue.votes.count}</span>
          </button>
          {#if issue.can.delete}
            <span class="more" use:outside={() => ((menuOpen = false), (deleteArmed = false))}>
              <button
                class="btn btn-small btn-quiet"
                aria-haspopup="menu"
                aria-expanded={menuOpen}
                aria-label={t.issue.menu}
                onclick={() => (menuOpen = !menuOpen)}>⋯</button
              >
              {#if menuOpen}
                <div class="popover" role="menu">
                  <button class="item danger" role="menuitem" onclick={() => void remove()}>
                    {deleteArmed ? t.issue.confirmDelete(issue.key) : t.issue.delete}
                  </button>
                </div>
              {/if}
            </span>
          {/if}
        </span>
      </div>
      {#if note}<p class="note" role="status">{note}</p>{/if}
    </header>

    <div class="layout">
      <div class="sheet">
        <section aria-label={t.issue.description}>
          <h2 class="section-title">
            {t.issue.description}
            {#if issue.can.edit && !editingDescription}
              <span class="actions">
                <button
                  class="btn btn-small"
                  onclick={() => {
                    descriptionDraft = issue?.description ?? '';
                    editingDescription = true;
                  }}>{issue.description ? t.common.edit : t.issue.addDescription}</button
                >
              </span>
            {/if}
          </h2>
          {#if editingDescription}
            <WikiEditor
              bind:value={descriptionDraft}
              label={t.issue.description}
              rows={12}
              attachments={issue.attachments}
              autofocus
              onsubmit={() => void saveDescription()}
            />
            <div class="row end">
              <button class="btn btn-quiet" onclick={() => (editingDescription = false)}
                >{t.common.cancel}</button
              >
              <button
                class="btn btn-primary"
                disabled={savingDescription}
                onclick={() => void saveDescription()}
                >{savingDescription ? t.common.saving : t.common.save}</button
              >
            </div>
          {:else if issue.description}
            <WikiText source={issue.description} attachments={issue.attachments} />
          {:else}
            <p class="empty">{t.issue.noDescription}</p>
          {/if}
        </section>

        {#if issue.environment}
          <section aria-label={t.issue.environment}>
            <h2 class="section-title">{t.issue.environment}</h2>
            <WikiText source={issue.environment} attachments={issue.attachments} />
          </section>
        {/if}

        {#if issue.subtasks.length || (!issue.type.subtask && issue.can.create)}
          <section aria-label={t.issue.subtasks}>
            <h2 class="section-title">
              {t.issue.subtasks}
              <span class="count">{issue.subtasks.length}</span>
              {#if !issue.type.subtask && issue.can.create}
                <span class="actions">
                  <button
                    class="btn btn-small"
                    onclick={() => openCreate({ parent: issue?.key ?? '' })}
                    >{t.issue.addSubtask}</button
                  >
                </span>
              {/if}
            </h2>
            {#if issue.subtasks.length}
              <ul class="subtasks">
                {#each issue.subtasks as s (s.key)}
                  <li>
                    <TypeMark type={s.type} />
                    <a class="key" href={paths.issue(s.key)}>{s.key}</a>
                    <a class="summary" href={paths.issue(s.key)}>{s.summary}</a>
                    <Stamp status={s.status} />
                    <span class="who"><Who user={s.assignee} /></span>
                  </li>
                {/each}
              </ul>
            {/if}
          </section>
        {/if}

        <Links {issue} onupdated={accept} onreload={() => void load(issue?.key ?? issueKey)} />
        <Attachments
          {issue}
          onupdated={accept}
          onreload={() => void load(issue?.key ?? issueKey)}
        />

        <section class="activity" aria-label={t.issue.activity}>
          <div class="tabs" role="tablist" aria-label={t.issue.activity}>
            <button
              role="tab"
              aria-selected={tab === 'comments'}
              class:active={tab === 'comments'}
              onclick={() => (tab = 'comments')}
              >{t.issue.tabs.comments} <span class="n">{issue.commentCount}</span></button
            >
            <button
              role="tab"
              aria-selected={tab === 'history'}
              class:active={tab === 'history'}
              onclick={() => (tab = 'history')}>{t.issue.tabs.history}</button
            >
            <button
              role="tab"
              aria-selected={tab === 'worklogs'}
              class:active={tab === 'worklogs'}
              onclick={() => (tab = 'worklogs')}>{t.issue.tabs.worklogs}</button
            >
          </div>
          <div role="tabpanel" class="panel">
            {#if tab === 'comments'}
              <Comments {issue} {tick} />
            {:else if tab === 'history'}
              <History issueKey={issue.key} {tick} />
            {:else}
              <Worklogs {issue} {tick} onlogged={() => void load(issue?.key ?? issueKey)} />
            {/if}
          </div>
        </section>
      </div>

      <Fiche {issue} onchange={update} />
    </div>
  </article>
{/if}

<style>
  .crumbs {
    display: flex;
    gap: 8px;
    align-items: baseline;
    font-size: 13px;
    color: var(--muted);
    margin-bottom: 10px;
  }

  .crumbs a {
    color: var(--ink-soft);
  }

  .crumbs .current {
    color: var(--ink);
  }

  .head {
    padding-bottom: 16px;
    margin-bottom: 22px;
    border-bottom: 1px solid var(--rule-strong);
  }

  h1 {
    font-size: 29px;
    line-height: 1.2;
    max-width: 62ch;
  }

  .summary-btn {
    all: unset;
    cursor: text;
    border-bottom: 1px dashed transparent;
  }

  .summary-btn:hover {
    border-bottom-color: var(--rule-strong);
  }

  .summary-btn:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 3px;
  }

  .summary-input {
    width: 100%;
    font: 600 29px/1.2 var(--serif);
    border: none;
    border-bottom: 2px solid var(--accent);
    background: transparent;
    padding: 0 0 2px;
  }

  .summary-input:focus {
    outline: none;
  }

  .meta {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px 10px;
    margin-top: 14px;
    font-size: 13.5px;
  }

  .resolution {
    font: italic 14px var(--serif);
    color: var(--ink-soft);
  }

  .dot {
    color: var(--rule-strong);
  }

  .created {
    display: inline-flex;
    gap: 5px;
    align-items: center;
    font-size: 13px;
  }

  .actions {
    margin-left: auto;
    display: flex;
    gap: 6px;
    align-items: center;
  }

  .n {
    font: 11.5px var(--mono);
    color: var(--muted);
  }

  .more {
    position: relative;
  }

  .more .popover {
    right: 0;
    top: calc(100% + 4px);
    min-width: 240px;
  }

  .danger {
    color: var(--accent);
    white-space: normal;
  }

  .note {
    margin: 12px 0 0;
    font: italic 13.5px var(--serif);
    color: var(--stamp-doing);
  }

  .layout {
    display: grid;
    grid-template-columns: minmax(0, 1fr) 340px;
    gap: 40px;
    align-items: start;
  }

  .sheet {
    min-width: 0;
  }

  .sheet > section:first-child :global(.section-title) {
    margin-top: 4px;
  }

  .end {
    justify-content: flex-end;
    margin-top: 8px;
  }

  .subtasks {
    list-style: none;
    margin: 0;
    padding: 0;
  }

  .subtasks li {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 5px 0;
    border-bottom: 1px solid var(--rule);
  }

  .subtasks .summary {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    text-decoration: none;
  }

  .subtasks .summary:hover {
    text-decoration: underline;
  }

  .subtasks .who {
    width: 150px;
    font-size: 13px;
  }

  .activity {
    margin-top: 34px;
  }

  .tabs {
    display: flex;
    gap: 0;
    border-bottom: 1px solid var(--rule-strong);
  }

  .tabs button {
    border: none;
    border-bottom: 2px solid transparent;
    background: none;
    padding: 8px 14px 7px;
    margin-bottom: -1px;
    font: 500 13px var(--sans);
    color: var(--ink-soft);
    cursor: pointer;
  }

  .tabs button.active {
    color: var(--ink);
    border-bottom-color: var(--accent);
  }

  .panel {
    padding-top: 6px;
  }

  @media (max-width: 980px) {
    .layout {
      grid-template-columns: minmax(0, 1fr);
    }
  }
</style>
