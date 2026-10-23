<script lang="ts">
  import { estimateText, readEstimate } from '../core/estimates.js';
  import { projectOfIssueKey } from '../core/marks.js';
  import { paths } from '../core/route.js';
  import type { Project, ProjectDetail, UserRef } from '../core/types.js';
  import { t } from '../i18n/t.js';
  import { api, errorMessage, type IssueFields } from '../lib/api.js';
  import { router } from '../lib/router.svelte.js';
  import { session } from '../lib/session.svelte.js';
  import { dialogs, notify } from '../lib/ui.svelte.js';
  import LabelEditor from './LabelEditor.svelte';
  import Modal from './Modal.svelte';
  import MultiPick from './MultiPick.svelte';
  import UserPicker from './UserPicker.svelte';
  import WikiEditor from './WikiEditor.svelte';

  /** "Nouveau ticket": the form, with fields offered by the chosen project. */

  let projects = $state<Project[]>([]);
  let projectKey = $state('');
  let detail = $state<ProjectDetail | null>(null);
  let typeId = $state('');
  let summary = $state('');
  let description = $state('');
  let priorityId = $state('');
  let assignee = $state<UserRef | null>(null);
  let labels = $state<string[]>([]);
  let components = $state<number[]>([]);
  let fixVersions = $state<number[]>([]);
  let due = $state('');
  let estimate = $state('');
  let level = $state('');
  let pending = $state(false);
  let error = $state('');

  const parent = $derived(dialogs.createDefaults.parent ?? '');
  const types = $derived((detail?.issueTypes ?? []).filter((ty) => ty.subtask === (parent !== '')));
  const canCreate = $derived(detail?.can.create === true);

  function lastProject(): string {
    try {
      return localStorage.getItem('solstice.lastProject') ?? '';
    } catch {
      return '';
    }
  }

  // Each opening starts from a clean form.
  $effect(() => {
    if (!dialogs.create) return;
    summary = '';
    description = '';
    assignee = null;
    labels = [];
    components = [];
    fixVersions = [];
    due = '';
    estimate = '';
    level = '';
    error = '';
    const prios = session.meta?.priorities ?? [];
    priorityId = prios[Math.floor(prios.length / 2)]?.id ?? '';
    const wanted =
      dialogs.createDefaults.project ??
      (parent ? projectOfIssueKey(parent) : null) ??
      lastProject();
    void api.projects().then((list) => {
      projects = list;
      projectKey = list.some((p) => p.key === wanted) ? wanted : (list[0]?.key ?? '');
    });
  });

  $effect(() => {
    const key = projectKey;
    if (!key) return;
    detail = null;
    void api.project(key).then((d) => {
      if (key !== projectKey) return;
      detail = d;
      components = [];
      fixVersions = [];
      const offered = d.issueTypes.filter((ty) => ty.subtask === (parent !== ''));
      if (!offered.some((ty) => ty.id === typeId)) typeId = offered[0]?.id ?? '';
    });
  });

  function close() {
    dialogs.create = false;
  }

  async function submit(e?: SubmitEvent) {
    e?.preventDefault();
    if (!detail || !summary.trim() || pending) return;
    const typed = readEstimate(estimate);
    if (!typed.ok) {
      error = t.issue.estimateInvalid;
      return;
    }
    pending = true;
    error = '';
    const fields: IssueFields = {
      project: projectKey,
      type: typeId,
      summary: summary.trim(),
      labels,
      components,
      fixVersions,
    };
    if (description.trim()) fields.description = description;
    if (priorityId) fields.priority = priorityId;
    if (assignee) fields.assignee = assignee.id;
    if (due) fields.due = due;
    if (typed.seconds !== null) fields.originalEstimate = estimateText(typed.seconds);
    if (parent) fields.parent = parent;
    if (level) fields.securityLevel = Number(level);
    try {
      const { key } = await api.createIssue(fields);
      try {
        localStorage.setItem('solstice.lastProject', projectKey);
      } catch {
        // Not remembered: no harm.
      }
      close();
      notify(t.create.created(key));
      router.navigate(paths.issue(key));
    } catch (err) {
      error = errorMessage(err);
    } finally {
      pending = false;
    }
  }
</script>

<Modal
  open={dialogs.create}
  title={parent ? t.create.subtaskTitle(parent) : t.create.title}
  onclose={close}
>
  <form id="new-issue" class="form" onsubmit={submit}>
    <div class="two">
      <label class="field">
        <span>{t.create.project}</span>
        <select class="select" bind:value={projectKey} disabled={parent !== ''}>
          {#each projects as p (p.key)}
            <option value={p.key}>{p.name} — {p.key}</option>
          {/each}
        </select>
      </label>
      <label class="field">
        <span>{t.create.type}</span>
        <select class="select" bind:value={typeId} disabled={!detail}>
          {#each types as ty (ty.id)}
            <option value={ty.id}>{ty.name}</option>
          {/each}
        </select>
      </label>
    </div>

    {#if detail && !canCreate}
      <p class="error-text">{t.create.cannotCreate}</p>
    {/if}

    <label class="field">
      <span>{t.create.summary}</span>
      <!-- The dialog opens on the summary, the one field always needed. -->
      <!-- svelte-ignore a11y_autofocus -->
      <input class="input summary" bind:value={summary} maxlength="255" required autofocus />
    </label>

    <div class="field">
      <span class="field-label">{t.create.description}</span>
      <WikiEditor bind:value={description} label={t.create.description} rows={7} />
    </div>

    <div class="three">
      <label class="field">
        <span>{t.create.priority}</span>
        <select class="select" bind:value={priorityId}>
          {#each session.meta?.priorities ?? [] as p (p.id)}
            <option value={p.id}>{p.name}</option>
          {/each}
        </select>
      </label>
      <div class="field">
        <span class="field-label">{t.create.assignee}</span>
        <div class="boxed field-box">
          <UserPicker
            value={assignee}
            label={t.create.assignee}
            noneLabel={detail?.defaultAssignee === 'lead'
              ? t.create.defaultAssignee
              : t.common.unassigned}
            search={(q) => api.assignable(projectKey, q)}
            onpick={(u) => (assignee = u)}
          />
        </div>
      </div>
      <label class="field">
        <span>{t.create.due}</span>
        <input class="input" type="date" bind:value={due} />
      </label>
    </div>

    <div class="three">
      <label class="field">
        <span>{t.create.estimate}</span>
        <input class="input" bind:value={estimate} placeholder={t.issue.estimateHint} />
      </label>
    </div>

    {#if detail}
      <div class="three">
        {#if detail.components.length}
          <div class="field">
            <span class="field-label">{t.create.components}</span>
            <div class="boxed field-box">
              <MultiPick
                options={detail.components.filter((c) => !c.archived)}
                selected={components}
                label={t.create.components}
                onchange={(ids) => (components = ids)}
              />
            </div>
          </div>
        {/if}
        {#if detail.versions.length}
          <div class="field">
            <span class="field-label">{t.create.fixVersions}</span>
            <div class="boxed field-box">
              <MultiPick
                options={detail.versions.filter((v) => !v.archived && !v.released)}
                selected={fixVersions}
                label={t.create.fixVersions}
                onchange={(ids) => (fixVersions = ids)}
              />
            </div>
          </div>
        {/if}
        {#if detail.securityLevels.length}
          <label class="field">
            <span>{t.create.security}</span>
            <select class="select" bind:value={level}>
              <option value="">{t.issue.public}</option>
              {#each detail.securityLevels as l (l.id)}
                <option value={String(l.id)}>{l.name}</option>
              {/each}
            </select>
          </label>
        {/if}
      </div>
    {/if}

    <div class="field">
      <span class="field-label">{t.create.labels}</span>
      <div class="boxed field-box"><LabelEditor {labels} onchange={(l) => (labels = l)} /></div>
    </div>

    {#if error}<p class="error-text" role="alert">{error}</p>{/if}
  </form>

  {#snippet footer()}
    <button type="button" class="btn btn-quiet" onclick={close}>{t.common.cancel}</button>
    <button
      type="submit"
      form="new-issue"
      class="btn btn-primary"
      disabled={pending || !canCreate || !summary.trim() || !typeId}
      >{pending ? t.create.pending : t.create.submit}</button
    >
  {/snippet}
</Modal>

<style>
  .form {
    display: grid;
    gap: 14px;
  }

  .two {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 12px;
  }

  .three {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 12px;
  }

  .summary {
    font: var(--title-weight) 16px/1.3 var(--font-title);
    padding: 8px 10px;
  }

  .boxed {
    border: 1px solid var(--rule-strong);
    background: var(--paper-raised);
    padding: 6px 8px;
    min-height: 34px;
    display: flex;
    align-items: center;
  }

  @media (max-width: 640px) {
    .two,
    .three {
      grid-template-columns: 1fr;
    }
  }
</style>
