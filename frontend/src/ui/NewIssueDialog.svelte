<script lang="ts">
  import { hasProblems } from '../core/admin-forms.js';
  import type { LayoutField } from '../core/admin-types.js';
  import { emptyDraft, restValue, type CustomDraft } from '../core/custom-fields.js';
  import { jiraDuration } from '../core/dates.js';
  import { readEstimate } from '../core/estimates.js';
  import { projectOfIssueKey } from '../core/marks.js';
  import {
    defaultCreateLayout,
    emptyMemory,
    fieldRuns,
    memoryKey,
    newIssueProblems,
    notAttached,
    queueFiles,
    queueFor,
    readMemory,
    recall,
    remember,
    startProject,
    unqueueFile,
    type NewIssueMemory,
  } from '../core/new-issue.js';
  import { paths } from '../core/route.js';
  import type { OfferedField, Project, ProjectDetail, UserRef } from '../core/types.js';
  import { formatSize, t } from '../i18n/t.js';
  import { projectAdminApi } from '../lib/admin-api.js';
  import { api, ApiError, errorMessage, isVisible, type IssueFields } from '../lib/api.js';
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
  let environment = $state('');
  let priorityId = $state('');
  let assignee = $state<UserRef | null>(null);
  let labels = $state<string[]>([]);
  let components = $state<number[]>([]);
  let fixVersions = $state<number[]>([]);
  let affects = $state<number[]>([]);
  let due = $state('');
  let estimate = $state('');
  let level = $state('');
  // The custom fields the project and type offer, and what was entered in them.
  let offered = $state<OfferedField[]>([]);
  let custom = $state<Record<number, CustomDraft>>({});
  // A server error about one custom field, shown next to it.
  let customError = $state<{ id: number; message: string } | null>(null);
  // The create layout of the chosen project and type.
  let layout = $state<readonly LayoutField[]>(defaultCreateLayout);
  // The viewer as the chosen project's assignable list returns them; null when they can't be assigned there.
  let me = $state<UserRef | null>(null);
  const labelsId = $props.id();
  let pending = $state(false);
  let error = $state('');
  // Fields show their problems once a submit has been tried.
  let tried = $state(false);
  // Files to attach once the issue exists.
  let queue = $state<File[]>([]);
  let uploading = $state(false);
  let picker: HTMLInputElement | undefined = $state();
  let form: HTMLFormElement | undefined = $state();
  let dragging = $state(false);
  /** Nested dragenter/dragleave pairs, so crossing child nodes does not hide the overlay. */
  let depth = 0;

  // Fields that take the form's whole width.
  const wideFields = ['description', 'environment', 'labels'];
  const wideTypes = ['textarea', 'labels', 'radio', 'checkboxes', 'multiuser'];
  // Custom fields whose editor is an input or button a `<label for>` can name; the others carry the name themselves.
  const labelled = ['text', 'url', 'number', 'date', 'datetime', 'select', 'labels'];

  const parent = $derived(dialogs.createDefaults.parent ?? '');
  const types = $derived((detail?.issueTypes ?? []).filter((ty) => ty.subtask === (parent !== '')));
  const canCreate = $derived(detail?.can.create === true);
  const configure = $derived(detail?.can.configure === true);
  const canAttach = $derived(detail?.can.attach === true);

  const customId = (id: number) => `customfield_${id}`;
  const offeredField = (id: string) => offered.find((o) => customId(o.id) === id);

  // Whether the form can draw a field: a list needs something to pick or the right to add to it.
  function drawable(f: LayoutField): boolean {
    switch (f.field) {
      case 'reporter':
        // Always the viewer: the form can't set it.
        return false;
      case 'components':
        return !!detail && (detail.components.length > 0 || configure || !!f.required);
      case 'fixVersions':
      case 'versions':
        return !!detail && (detail.versions.length > 0 || configure || !!f.required);
      case 'security':
        return !!detail && detail.securityLevels.length > 0;
      default:
        return !f.field.startsWith('customfield_') || !!offeredField(f.field);
    }
  }

  // Offered custom fields the layout misses come last, as the server places them.
  const placed = $derived<LayoutField[]>([
    ...layout,
    ...offered
      .filter((o) => !layout.some((f) => f.field === customId(o.id)))
      .map((o) => ({ field: customId(o.id), hidden: false })),
  ]);
  const fieldsShown = $derived(placed.filter((f) => !f.hidden && drawable(f)));
  const shows = (id: string) => fieldsShown.some((f) => f.field === id);
  const runs = $derived(
    fieldRuns(fieldsShown, (f) => {
      const o = offeredField(f.field);
      return o ? wideTypes.includes(o.type) : wideFields.includes(f.field);
    }),
  );
  const required = $derived(placed.filter((f) => f.required && !f.hidden).map((f) => f.field));
  const typedEstimate = $derived(shows('timetracking') ? estimate : '');
  const filled = $derived.by(() => {
    const has: Record<string, boolean> = {
      description: !!description.trim(),
      environment: !!environment.trim(),
      priority: !!priorityId,
      // The project's default fills it on the server.
      assignee:
        !!assignee || detail?.defaultAssignee === 'lead' || detail?.defaultAssignee === 'reporter',
      reporter: true,
      components: components.length > 0,
      fixVersions: fixVersions.length > 0,
      versions: affects.length > 0,
      labels: labels.length > 0,
      duedate: !!due,
      timetracking: !!typedEstimate.trim(),
      security: !!level || parent !== '',
    };
    for (const f of offered)
      has[customId(f.id)] = restValue(f, custom[f.id] ?? emptyDraft(f)) !== undefined;
    return Object.keys(has).filter((k) => has[k]);
  });
  const problems = $derived(
    newIssueProblems(
      { project: projectKey, type: typeId, summary, estimate: typedEstimate, filled },
      required,
      t.units.duration,
    ),
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
    environment = '';
    assignee = null;
    labels = [];
    components = [];
    fixVersions = [];
    affects = [];
    due = '';
    estimate = '';
    level = '';
    custom = {};
    customError = null;
    error = '';
    tried = false;
    queue = [];
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
      queue = queueFor(queue, d.can.attach === true);
      restore(key, d);
      fixVersions = [];
      affects = [];
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

  $effect(() => {
    const key = projectKey;
    const type = typeId;
    if (!dialogs.create || !key || !type) {
      layout = defaultCreateLayout;
      return;
    }
    api
      .layout(key, type, 'create')
      .then((l) => {
        if (key === projectKey && type === typeId) layout = l.fields;
      })
      .catch(() => {
        if (key === projectKey && type === typeId) layout = defaultCreateLayout;
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
    queue = [];
  }

  function enqueue(files: File[]) {
    if (canAttach && files.length) queue = queueFiles(queue, files);
  }

  /** Files from outside, not text or links. */
  const carriesFiles = (e: DragEvent): boolean => e.dataTransfer?.types.includes('Files') ?? false;

  /** Whether this dialog is open and no other dialog is over it. */
  function inFront(): boolean {
    const top = [...document.querySelectorAll('dialog:modal')].at(-1);
    return !!form && !!top && top.contains(form);
  }

  function endDrag() {
    depth = 0;
    dragging = false;
  }

  function dragEnter(e: DragEvent) {
    if (!canAttach || !carriesFiles(e) || !inFront()) return;
    depth++;
    dragging = true;
  }

  function dragLeave(e: DragEvent) {
    if (!carriesFiles(e) || depth === 0) return;
    if (--depth === 0) dragging = false;
  }

  // Taken even without attach, or the browser leaves the form for the file.
  function dragOver(e: DragEvent) {
    if (!carriesFiles(e) || !inFront()) return;
    e.preventDefault();
    if (e.dataTransfer) e.dataTransfer.dropEffect = canAttach ? 'copy' : 'none';
  }

  function drop(e: DragEvent) {
    if (!carriesFiles(e) || !inFront()) return;
    // The description editor already queued it and inserted its reference.
    const taken = e.defaultPrevented;
    e.preventDefault();
    endDrag();
    if (!taken) enqueue([...(e.dataTransfer?.files ?? [])]);
  }

  // The issue is kept whatever happens here: what failed is named, to attach it from the issue.
  async function attach(key: string, files: File[]) {
    uploading = true;
    try {
      const d = await api.upload(key, files);
      const failed = isVisible(d) ? notAttached(files, d.attachments) : [];
      if (failed.length) notify(t.create.notAttached(failed.join(', ')), 'error');
    } catch (err) {
      const names = files.map((f) => f.name).join(', ');
      notify(`${errorMessage(err)} ${t.create.notAttached(names)}`, 'error');
    } finally {
      uploading = false;
    }
  }

  async function submit(e?: SubmitEvent) {
    e?.preventDefault();
    if (!detail || pending) return;
    tried = true;
    const typed = readEstimate(typedEstimate, t.units.duration);
    if (hasProblems(problems) || !typed.ok) return;
    pending = true;
    error = '';
    // Only what the form shows: the server fills the hidden fields with their defaults.
    const fields: IssueFields = { project: projectKey, type: typeId, summary: summary.trim() };
    if (shows('description') && description.trim()) fields.description = description;
    if (shows('environment') && environment.trim()) fields.environment = environment;
    if (shows('priority') && priorityId) fields.priority = priorityId;
    if (shows('assignee') && assignee) fields.assignee = assignee.id;
    if (shows('labels')) fields.labels = labels;
    if (shows('components')) fields.components = components;
    if (shows('fixVersions')) fields.fixVersions = fixVersions;
    if (shows('versions')) fields.affectsVersions = affects;
    if (shows('duedate') && due) fields.due = due;
    if (typed.seconds !== null) fields.originalEstimate = jiraDuration(typed.seconds);
    if (parent) fields.parent = parent;
    if (shows('security') && level) fields.securityLevel = Number(level);
    const values: Record<string, unknown> = {};
    for (const f of offered) {
      if (!shows(customId(f.id))) continue;
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
      notify(t.create.created(key));
      if (queue.length) await attach(key, queue);
      close();
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

{#snippet mark(f: LayoutField)}{#if f.required}{@render req()}{/if}{/snippet}

{#snippet missing(f: LayoutField)}
  {#if shown[f.field] === 'required'}
    <span class="field-error" id="{labelsId}-{f.field}-error">{t.create.required}</span>
  {/if}
{/snippet}

{#snippet field(f: LayoutField)}
  {@const invalid = shown[f.field] === 'required'}
  {@const describedby = invalid ? `${labelsId}-${f.field}-error` : undefined}
  {#if f.field === 'description'}
    <div class="field description" class:kept={offered.length > 0}>
      <span class="field-label">{t.create.description}{@render mark(f)}</span>
      <WikiEditor
        bind:value={description}
        label={t.create.description}
        rows={4}
        onfiles={canAttach ? enqueue : undefined}
      />
      {@render missing(f)}
    </div>
  {:else if f.field === 'environment'}
    <div class="field">
      <span class="field-label">{t.issue.environment}{@render mark(f)}</span>
      <WikiEditor
        bind:value={environment}
        label={t.issue.environment}
        rows={2}
        onfiles={canAttach ? enqueue : undefined}
      />
      {@render missing(f)}
    </div>
  {:else if f.field === 'priority'}
    <div class="field">
      <span class="field-label">{t.create.priority}{@render mark(f)}</span>
      <Dropdown
        label={t.create.priority}
        value={priorityId}
        options={(session.meta?.priorities ?? []).map((p) => ({
          value: p.id,
          label: session.priorityName(p),
        }))}
        {invalid}
        onchange={(v) => (priorityId = v)}
      >
        {#snippet icon(id)}
          <PriorityMark priority={session.meta?.priorities.find((p) => p.id === id) ?? null} />
        {/snippet}
      </Dropdown>
      {@render missing(f)}
    </div>
  {:else if f.field === 'assignee'}
    <div class="field assignee">
      <span class="field-label">{t.create.assignee}{@render mark(f)}</span>
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
      {@render missing(f)}
    </div>
  {:else if f.field === 'duedate'}
    <label class="field">
      <span>{t.create.due}{@render mark(f)}</span>
      <input
        class="input"
        type="date"
        bind:value={due}
        aria-invalid={invalid}
        aria-describedby={describedby}
      />
      {@render missing(f)}
    </label>
  {:else if f.field === 'timetracking'}
    <label class="field">
      <span>{t.create.estimate}{@render mark(f)}</span>
      <input
        class="input"
        bind:value={estimate}
        placeholder={t.issue.estimateHint}
        aria-invalid={invalid || !!shown.estimate}
        aria-describedby={shown.estimate ? 'new-issue-estimate-error' : describedby}
      />
      {#if shown.estimate}
        <span class="field-error" id="new-issue-estimate-error">{t.issue.estimateInvalid}</span>
      {:else}
        {@render missing(f)}
      {/if}
    </label>
  {:else if f.field === 'components' && detail}
    <div class="field">
      <span class="field-label">{t.create.components}{@render mark(f)}</span>
      <MultiPick
        field
        options={detail.components.filter((c) => !c.archived)}
        selected={components}
        label={t.create.components}
        oncreate={configure ? createComponent : undefined}
        onchange={(ids) => (components = ids)}
      />
      {@render missing(f)}
    </div>
  {:else if f.field === 'fixVersions' && detail}
    <div class="field">
      <span class="field-label">{t.create.fixVersions}{@render mark(f)}</span>
      <MultiPick
        field
        options={detail.versions.filter((v) => !v.archived && !v.released)}
        selected={fixVersions}
        label={t.create.fixVersions}
        oncreate={configure ? createVersion : undefined}
        onchange={(ids) => (fixVersions = ids)}
      />
      {@render missing(f)}
    </div>
  {:else if f.field === 'versions' && detail}
    <div class="field">
      <span class="field-label">{t.issue.affectsVersions}{@render mark(f)}</span>
      <MultiPick
        field
        options={detail.versions.filter((v) => !v.archived)}
        selected={affects}
        label={t.issue.affectsVersions}
        oncreate={configure ? createVersion : undefined}
        onchange={(ids) => (affects = ids)}
      />
      {@render missing(f)}
    </div>
  {:else if f.field === 'security' && detail}
    <div class="field">
      <span class="field-label">{t.create.security}{@render mark(f)}</span>
      <Dropdown
        label={t.create.security}
        value={level}
        options={[
          { value: '', label: t.issue.public },
          ...detail.securityLevels.map((l) => ({ value: String(l.id), label: l.name })),
        ]}
        {invalid}
        onchange={(v) => (level = v)}
      >
        {#snippet icon(id)}
          {#if id}<Lock />{/if}
        {/snippet}
      </Dropdown>
      {@render missing(f)}
    </div>
  {:else if f.field === 'labels'}
    <div class="field">
      <span class="field-label">{t.create.labels}{@render mark(f)}</span>
      <label class="boxed field-box" class:invalid for={labelsId}>
        <LabelEditor id={labelsId} {labels} onchange={(l) => (labels = l)} />
      </label>
      {@render missing(f)}
    </div>
  {:else}
    {@const o = offeredField(f.field)}
    {#if o}
      {@const fid = `${labelsId}-cf-${o.id}`}
      {@const problem = customError?.id === o.id ? customError.message : ''}
      <div class="field">
        {#if labelled.includes(o.type)}
          <label class="field-label" for={fid}>{o.name}{@render mark(f)}</label>
        {:else}
          <span class="field-label">{o.name}{@render mark(f)}</span>
        {/if}
        <CustomFieldInput
          field={o}
          id={fid}
          value={custom[o.id] ?? emptyDraft(o)}
          invalid={!!problem || invalid}
          describedby={problem ? `${fid}-error` : describedby}
          onchange={(v) => setCustom(o.id, v)}
        />
        {#if problem}
          <span class="field-error" id="{fid}-error">{problem}</span>
        {:else}
          {@render missing(f)}
        {/if}
      </div>
    {/if}
  {/if}
{/snippet}

{#snippet files()}
  {#if canAttach}
    <div class="field files">
      <div class="files-head">
        <span class="field-label">{t.issue.attachments}</span>
        <button type="button" class="btn btn-small" onclick={() => picker?.click()}
          >{t.issue.attach}</button
        >
        <input
          bind:this={picker}
          type="file"
          multiple
          hidden
          onchange={(e) => {
            enqueue([...(e.currentTarget.files ?? [])]);
            e.currentTarget.value = '';
          }}
        />
      </div>
      {#if queue.length}
        <ul class="queue">
          {#each queue as f (f.name)}
            <li>
              <span class="name" title={f.name}>{f.name}</span>
              <span class="size">{formatSize(f.size)}</span>
              <button
                type="button"
                class="del"
                aria-label={`${t.common.remove} ${f.name}`}
                onclick={() => (queue = unqueueFile(queue, f.name))}>×</button
              >
            </li>
          {/each}
        </ul>
      {/if}
    </div>
  {/if}
{/snippet}

<svelte:window
  ondragenter={dragEnter}
  ondragleave={dragLeave}
  ondragover={dragOver}
  ondrop={drop}
/>

<Modal
  open={dialogs.create}
  title={parent ? t.create.subtaskTitle(parent) : t.create.title}
  onclose={close}
>
  <form bind:this={form} id="new-issue" class="form" onsubmit={submit} novalidate>
    <div class="two">
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

    {#each runs as run (run.fields[0]!.field)}
      {#if run.wide}
        {@render field(run.fields[0]!)}
        {#if run.fields[0]!.field === 'description'}{@render files()}{/if}
      {:else}
        <div class="two">
          {#each run.fields as f (f.field)}{@render field(f)}{/each}
        </div>
      {/if}
    {/each}
    {#if !shows('description')}{@render files()}{/if}
    {#if dragging}<p class="drop"><span>{t.create.dropFiles}</span></p>{/if}
  </form>

  {#snippet footer()}
    <!-- In the footer, the reason a ticket can't be created stays in sight. -->
    {#if !problem}<p class="legend" aria-hidden="true">{@render req()}{t.create.required}</p>{/if}
    <p class="status" class:error-text={problem} role="alert">{problem}</p>
    <button type="button" class="btn btn-quiet" onclick={close}>{t.common.cancel}</button>
    <button type="submit" form="new-issue" class="btn btn-primary" disabled={pending || !canCreate}
      >{uploading ? t.issue.uploading : pending ? t.create.pending : t.create.submit}</button
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

  .two {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
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

  .form :global([aria-invalid='true']),
  .boxed.invalid {
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

  /* A long file name is clipped, not widening the form. */
  .files {
    grid-template-columns: minmax(0, 1fr);
  }

  .files-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
  }

  .queue {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
  }

  .queue li {
    display: flex;
    align-items: center;
    gap: 6px;
    min-width: 0;
    max-width: 100%;
    padding: 2px 4px 2px 8px;
    border: 1px solid var(--rule-strong);
    background: var(--paper-raised);
    font-size: 0.78125rem;
  }

  .queue .name {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .queue .size {
    flex-shrink: 0;
    color: var(--muted);
  }

  .del {
    flex-shrink: 0;
    width: 1.375rem;
    height: 1.375rem;
    border: 1px solid transparent;
    background: none;
    cursor: pointer;
  }

  .del:hover,
  .del:focus-visible {
    color: var(--accent-text);
    border-color: var(--accent);
  }

  /* Over the whole view: files land anywhere on the dialog. */
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

  @media (max-width: 640px) {
    .two {
      grid-template-columns: 1fr;
    }
  }
</style>
