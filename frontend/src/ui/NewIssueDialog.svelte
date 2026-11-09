<script lang="ts">
  import { hasProblems } from '../core/admin-forms.js';
  import { emptyDraft, restValue, type CustomDraft } from '../core/custom-fields.js';
  import { jiraDuration } from '../core/dates.js';
  import { readEstimate } from '../core/estimates.js';
  import { projectOfIssueKey } from '../core/marks.js';
  import {
    emptyMemory,
    memoryKey,
    newIssueProblems,
    readMemory,
    recall,
    remember,
    startProject,
    type NewIssueMemory,
  } from '../core/new-issue.js';
  import { paths } from '../core/route.js';
  import type { OfferedField, Project, ProjectDetail, UserRef } from '../core/types.js';
  import { t } from '../i18n/t.js';
  import { projectAdminApi } from '../lib/admin-api.js';
  import { api, ApiError, errorMessage, type IssueFields } from '../lib/api.js';
  import { router } from '../lib/router.svelte.js';
  import { session } from '../lib/session.svelte.js';
  import { dialogs, notify } from '../lib/ui.svelte.js';
  import CustomFieldInput from './CustomFieldInput.svelte';
  import Dropdown from './Dropdown.svelte';
  import LabelEditor from './LabelEditor.svelte';
  import Modal from './Modal.svelte';
  import Lock from './Lock.svelte';
  import MultiPick from './MultiPick.svelte';
  import PriorityMark from './PriorityMark.svelte';
  import TypeMark from './TypeMark.svelte';
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
  // The custom fields the project and type offer, and what was entered in them.
  let offered = $state<OfferedField[]>([]);
  let custom = $state<Record<number, CustomDraft>>({});
  // A server error about one custom field, shown next to it.
  let customError = $state<{ id: number; message: string } | null>(null);
  // The viewer as the chosen project's assignable list returns them; null when they can't be assigned there.
  let me = $state<UserRef | null>(null);
  const labelsId = $props.id();
  let pending = $state(false);
  let error = $state('');
  // Fields show their problems once a submit has been tried.
  let tried = $state(false);

  // Custom fields that take the form's whole width.
  const wideTypes = ['textarea', 'labels', 'radio', 'checkboxes', 'multiuser'];
  // Custom fields whose editor is an input or button a `<label for>` can name; the others carry the name themselves.
  const labelled = ['text', 'url', 'number', 'date', 'datetime', 'select', 'labels'];

  const parent = $derived(dialogs.createDefaults.parent ?? '');
  const types = $derived((detail?.issueTypes ?? []).filter((ty) => ty.subtask === (parent !== '')));
  const canCreate = $derived(detail?.can.create === true);
  const configure = $derived(detail?.can.configure === true);
  const problems = $derived(
    newIssueProblems({ project: projectKey, type: typeId, summary, estimate }, t.units.duration),
  );
  const shown = $derived(tried ? problems : {});
  const problem = $derived(
    error ||
      (customError ? t.create.checkFields : '') ||
      (detail && !canCreate ? t.create.cannotCreate : '') ||
      (hasProblems(shown) ? t.create.checkFields : ''),
  );

  async function createComponent(name: string): Promise<number> {
    const key = projectKey;
    const c = await projectAdminApi.createComponent(key, { name });
    if (detail && key === projectKey) detail.components = [...detail.components, c];
    return c.id;
  }

  async function createVersion(name: string): Promise<number> {
    const key = projectKey;
    const v = await projectAdminApi.createVersion(key, { name });
    if (detail && key === projectKey) detail.versions = [...detail.versions, v];
    return v.id;
  }

  function loadMemory(): NewIssueMemory {
    const id = session.me?.id;
    if (id === undefined) return emptyMemory;
    try {
      return readMemory(localStorage.getItem(memoryKey(id)));
    } catch {
      return emptyMemory;
    }
  }

  // Fills in what the viewer last chose in this project. The remembered assignee
  // is kept only if still assignable there; otherwise the default applies.
  function restore(key: string, d: ProjectDetail) {
    const r = recall(loadMemory(), key, d.components);
    components = r.components;
    assignee = null;
    const who = r.assignee;
    if (!who) return;
    api
      .assignable(key, who.username)
      .then((list) => {
        if (key === projectKey && assignee === null && list.some((u) => u.id === who.id)) {
          assignee = who;
        }
      })
      .catch(() => {});
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
    custom = {};
    customError = null;
    error = '';
    tried = false;
    const prios = session.meta?.priorities ?? [];
    priorityId = prios[Math.floor(prios.length / 2)]?.id ?? '';
    const explicit = dialogs.createDefaults.project ?? (parent ? projectOfIssueKey(parent) : null);
    void api.projects().then((list) => {
      projects = list;
      const key = startProject(
        explicit,
        loadMemory(),
        list.map((p) => p.key),
      );
      // The same project as last time: its detail is loaded, only the fields need restoring.
      if (key === projectKey && detail) restore(key, detail);
      else projectKey = key;
    });
  });

  $effect(() => {
    me = null;
    const key = projectKey;
    const who = session.me;
    if (!dialogs.create || !key || !who) return;
    api
      .assignable(key, who.username)
      .then((list) => {
        if (key === projectKey) me = list.find((u) => u.id === who.id) ?? null;
      })
      .catch(() => {});
  });

  $effect(() => {
    const key = projectKey;
    if (!key) return;
    detail = null;
    void api.project(key).then((d) => {
      if (key !== projectKey) return;
      detail = d;
      restore(key, d);
      fixVersions = [];
      const offered = d.issueTypes.filter((ty) => ty.subtask === (parent !== ''));
      if (!offered.some((ty) => ty.id === typeId)) typeId = offered[0]?.id ?? '';
    });
  });

  // A value of a field the new project or type no longer offers is dropped.
  $effect(() => {
    const key = projectKey;
    const type = typeId;
    if (!dialogs.create || !key || !type) {
      offered = [];
      return;
    }
    api
      .projectFields(key, type)
      .then((list) => {
        if (key !== projectKey || type !== typeId) return;
        offered = list;
        custom = Object.fromEntries(
          list.filter((f) => f.id in custom).map((f) => [f.id, custom[f.id]!]),
        );
        if (customError && !list.some((f) => f.id === customError?.id)) customError = null;
      })
      .catch(() => {
        if (key === projectKey && type === typeId) offered = [];
      });
  });

  function setCustom(id: number, value: CustomDraft) {
    custom[id] = value;
    if (customError?.id === id) customError = null;
  }

  // The custom field a server error names, if the form shows it.
  function customFieldOf(err: unknown): OfferedField | undefined {
    if (!(err instanceof ApiError)) return undefined;
    const data = err.data;
    if (!data || typeof data !== 'object' || !('field' in data) || typeof data.field !== 'string') {
      return undefined;
    }
    const id = data.field.replace(/^customfield_/, '');
    return offered.find((f) => String(f.id) === id);
  }

  function close() {
    dialogs.create = false;
  }

  async function submit(e?: SubmitEvent) {
    e?.preventDefault();
    if (!detail || pending) return;
    tried = true;
    const typed = readEstimate(estimate, t.units.duration);
    if (hasProblems(problems) || !typed.ok) return;
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
    if (typed.seconds !== null) fields.originalEstimate = jiraDuration(typed.seconds);
    if (parent) fields.parent = parent;
    if (level) fields.securityLevel = Number(level);
    const values: Record<string, unknown> = {};
    for (const f of offered) {
      const v = restValue(f, custom[f.id] ?? emptyDraft(f));
      if (v !== undefined) values[String(f.id)] = v;
    }
    if (Object.keys(values).length) fields.customFields = values;
    customError = null;
    try {
      const { key } = await api.createIssue(fields);
      try {
        const id = session.me?.id;
        if (id !== undefined) {
          const m = remember(loadMemory(), projectKey, { assignee, components });
          localStorage.setItem(memoryKey(id), JSON.stringify(m));
        }
      } catch {
        // Not remembered: no harm.
      }
      close();
      notify(t.create.created(key));
      router.navigate(paths.issue(key));
    } catch (err) {
      const f = customFieldOf(err);
      if (f) customError = { id: f.id, message: errorMessage(err) };
      else error = errorMessage(err);
    } finally {
      pending = false;
    }
  }
</script>

{#snippet req()}<span class="req" aria-hidden="true">*</span>{/snippet}

<Modal
  open={dialogs.create}
  title={parent ? t.create.subtaskTitle(parent) : t.create.title}
  onclose={close}
>
  <form id="new-issue" class="form" onsubmit={submit} novalidate>
    <div class="three">
      <div class="field">
        <span class="field-label">{t.create.project}{@render req()}</span>
        <Dropdown
          label={t.create.project}
          value={projectKey}
          options={projects.map((p) => ({ value: p.key, label: `${p.name} — ${p.key}` }))}
          disabled={parent !== ''}
          required
          invalid={!!shown.project}
          onchange={(v) => (projectKey = v)}
        />
        {#if shown.project}<span class="field-error">{t.create.missing.project}</span>{/if}
      </div>
      <div class="field">
        <span class="field-label">{t.create.type}{@render req()}</span>
        <Dropdown
          label={t.create.type}
          value={typeId}
          options={types.map((ty) => ({ value: ty.id, label: session.typeName(ty) }))}
          disabled={!detail}
          required
          invalid={!!shown.type}
          onchange={(v) => (typeId = v)}
        >
          {#snippet icon(id)}
            {@const ty = types.find((x) => x.id === id)}
            {#if ty}<TypeMark type={ty} />{/if}
          {/snippet}
        </Dropdown>
        {#if shown.type}<span class="field-error">{t.create.missing.type}</span>{/if}
      </div>
      <div class="field">
        <span class="field-label">{t.create.priority}</span>
        <Dropdown
          label={t.create.priority}
          value={priorityId}
          options={(session.meta?.priorities ?? []).map((p) => ({
            value: p.id,
            label: session.priorityName(p),
          }))}
          onchange={(v) => (priorityId = v)}
        >
          {#snippet icon(id)}
            <PriorityMark priority={session.meta?.priorities.find((p) => p.id === id) ?? null} />
          {/snippet}
        </Dropdown>
      </div>
    </div>

    <label class="field">
      <span>{t.create.summary}{@render req()}</span>
      <!-- The dialog opens on the summary, the one field always needed. -->
      <!-- svelte-ignore a11y_autofocus -->
      <input
        class="input summary"
        bind:value={summary}
        maxlength="255"
        required
        autofocus
        aria-invalid={!!shown.summary}
        aria-describedby={shown.summary ? 'new-issue-summary-error' : undefined}
      />
      {#if shown.summary}
        <span class="field-error" id="new-issue-summary-error">{t.create.missing.summary}</span>
      {/if}
    </label>

    <div class="field description" class:kept={offered.length > 0}>
      <span class="field-label">{t.create.description}</span>
      <WikiEditor bind:value={description} label={t.create.description} rows={4} />
    </div>

    <div class="three">
      <div class="field assignee">
        <span class="field-label">{t.create.assignee}</span>
        {#if me && assignee?.id !== me.id}
          <button type="button" class="link-btn take" onclick={() => (assignee = me)}
            >{t.common.assignToMe}</button
          >
        {/if}
        <UserPicker
          field
          value={assignee}
          label={t.create.assignee}
          noneLabel={detail?.defaultAssignee === 'lead' || detail?.defaultAssignee === 'reporter'
            ? t.create.defaultAssignee
            : t.common.unassigned}
          search={(q) => api.assignable(projectKey, q)}
          onpick={(u) => (assignee = u)}
        />
      </div>
      <label class="field">
        <span>{t.create.due}</span>
        <input class="input" type="date" bind:value={due} />
      </label>
      <label class="field">
        <span>{t.create.estimate}</span>
        <input
          class="input"
          bind:value={estimate}
          placeholder={t.issue.estimateHint}
          aria-invalid={!!shown.estimate}
          aria-describedby={shown.estimate ? 'new-issue-estimate-error' : undefined}
        />
        {#if shown.estimate}
          <span class="field-error" id="new-issue-estimate-error">{t.issue.estimateInvalid}</span>
        {/if}
      </label>
    </div>

    {#if detail}
      <div class="three">
        {#if detail.components.length || configure}
          <div class="field">
            <span class="field-label">{t.create.components}</span>
            <MultiPick
              field
              options={detail.components.filter((c) => !c.archived)}
              selected={components}
              label={t.create.components}
              oncreate={configure ? createComponent : undefined}
              onchange={(ids) => (components = ids)}
            />
          </div>
        {/if}
        {#if detail.versions.length || configure}
          <div class="field">
            <span class="field-label">{t.create.fixVersions}</span>
            <MultiPick
              field
              options={detail.versions.filter((v) => !v.archived && !v.released)}
              selected={fixVersions}
              label={t.create.fixVersions}
              oncreate={configure ? createVersion : undefined}
              onchange={(ids) => (fixVersions = ids)}
            />
          </div>
        {/if}
        {#if detail.securityLevels.length}
          <div class="field">
            <span class="field-label">{t.create.security}</span>
            <Dropdown
              label={t.create.security}
              value={level}
              options={[
                { value: '', label: t.issue.public },
                ...detail.securityLevels.map((l) => ({ value: String(l.id), label: l.name })),
              ]}
              onchange={(v) => (level = v)}
            >
              {#snippet icon(id)}
                {#if id}<Lock />{/if}
              {/snippet}
            </Dropdown>
          </div>
        {/if}
      </div>
    {/if}

    <div class="field">
      <span class="field-label">{t.create.labels}</span>
      <label class="boxed field-box" for={labelsId}>
        <LabelEditor id={labelsId} {labels} onchange={(l) => (labels = l)} />
      </label>
    </div>

    {#if offered.length}
      <div class="three">
        {#each offered as f (f.id)}
          {@const fid = `${labelsId}-cf-${f.id}`}
          {@const problem = customError?.id === f.id ? customError.message : ''}
          <div class="field" class:wide={wideTypes.includes(f.type)}>
            {#if labelled.includes(f.type)}
              <label class="field-label" for={fid}>{f.name}</label>
            {:else}
              <span class="field-label">{f.name}</span>
            {/if}
            <CustomFieldInput
              field={f}
              id={fid}
              value={custom[f.id] ?? emptyDraft(f)}
              invalid={!!problem}
              describedby={problem ? `${fid}-error` : undefined}
              onchange={(v) => setCustom(f.id, v)}
            />
            {#if problem}<span class="field-error" id="{fid}-error">{problem}</span>{/if}
          </div>
        {/each}
      </div>
    {/if}
  </form>

  {#snippet footer()}
    <!-- In the footer, the reason a ticket can't be created stays in sight. -->
    {#if !problem}<p class="legend" aria-hidden="true">{@render req()}{t.create.required}</p>{/if}
    <p class="status" class:error-text={problem} role="alert">{problem}</p>
    <button type="button" class="btn btn-quiet" onclick={close}>{t.common.cancel}</button>
    <button type="submit" form="new-issue" class="btn btn-primary" disabled={pending || !canCreate}
      >{pending ? t.create.pending : t.create.submit}</button
    >
  {/snippet}
</Modal>

<style>
  .form {
    display: flex;
    flex-direction: column;
    gap: 12px;
    flex: 1 1 auto;
    min-height: 0;
  }

  .form > * {
    flex-shrink: 0;
  }

  .three {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 12px;
    align-items: start;
  }

  /* The body takes the height the head and foot leave, not the shared 72vh,
     so the whole form fits from 720px tall. */
  :global(dialog.modal[open]:has(#new-issue)) {
    display: flex;
    flex-direction: column;
  }

  :global(dialog.modal:has(#new-issue) > .modal-body) {
    display: flex;
    flex-direction: column;
    flex: 1 1 auto;
    min-height: 0;
    max-height: none;
  }

  /* When field errors need the room, the description gives it up, down to
     two lines, so the form still fits without scrolling. */
  .form > .description {
    display: flex;
    flex-direction: column;
    flex-shrink: 1;
    min-height: 0;
  }

  /* With custom fields the form scrolls anyway: the description keeps its rows. */
  .form > .description.kept {
    flex-shrink: 0;
  }

  .description :global(.editor),
  .description :global(.editor-host) {
    display: flex;
    flex-direction: column;
    flex: 1 1 auto;
    min-height: 0;
  }

  .description :global(.editor-content) {
    flex: 1 1 calc(var(--editor-rows) * 1.6em + 20px);
    min-height: calc(2 * 1.6em + 20px);
    max-height: none;
  }

  .status {
    flex: 1;
    margin: 0;
  }

  .status:empty {
    flex: 0;
  }

  .req {
    margin-left: 2px;
    color: var(--accent-text);
  }

  .legend {
    flex: 1;
    margin: 0;
    font-size: 0.75rem;
    color: var(--muted);
  }

  .field-error {
    font-size: 0.75rem;
    color: var(--accent-text);
  }

  .form :global([aria-invalid='true']) {
    border-color: var(--accent);
  }

  /* The link sits on the label's line, at its right. */
  .assignee {
    grid-template-columns: minmax(0, 1fr) auto;
    align-items: baseline;
    column-gap: 8px;
  }

  .assignee > :global(.picker) {
    grid-column: 1 / -1;
  }

  .take {
    font-size: 0.75rem;
    line-height: 1.2;
    white-space: nowrap;
  }

  .summary {
    font: var(--title-weight) 1rem/1.3 var(--font-title);
    padding: 8px 10px;
  }

  .boxed {
    border: 1px solid var(--rule-strong);
    background: var(--paper-raised);
    padding: 6px 8px;
    min-height: 2.125rem;
    display: flex;
    align-items: center;
    cursor: text;
  }

  .wide {
    grid-column: 1 / -1;
  }

  @media (max-width: 640px) {
    .three {
      grid-template-columns: 1fr;
    }
  }
</style>
