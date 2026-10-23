<script lang="ts">
  import { onDestroy, untrack } from 'svelte';
  import type {
    ProjectWorkflows,
    StatusRef,
    StrandedDefault,
    StrandedStatus,
    Workflow,
  } from '../../core/types.js';
  import {
    addStatus,
    isDirty,
    removeStatus,
    strandedDefaultOf,
    strandedOf,
    suggestDefaultRemap,
    suggestRemap,
    toDraft,
    toInput,
  } from '../../core/workflow.js';
  import { t } from '../../i18n/t.js';
  import { api, ApiError, errorMessage } from '../../lib/api.js';
  import { notify } from '../../lib/ui.svelte.js';
  import Stamp from '../Stamp.svelte';
  import TypeMark from '../TypeMark.svelte';
  import RemapDialog from './RemapDialog.svelte';
  import StatusList from './StatusList.svelte';
  import TransitionMatrix from './TransitionMatrix.svelte';
  import WorkflowDiagram from './WorkflowDiagram.svelte';

  /**
   * One workflow: its name, start status, the issue types it serves, its
   * statuses and the transitions between them. Nothing is sent until saved;
   * a save that would strand issues asks where they go first.
   */
  interface Props {
    projectKey: string;
    workflow: Workflow;
    data: ProjectWorkflows;
    onsaved: (d: ProjectWorkflows) => void;
    ondirty: (dirty: boolean) => void;
    onback: () => void;
  }
  const { projectKey, workflow, data, onsaved, ondirty, onback }: Props = $props();

  /** The workflow as last saved; the draft starts from it. */
  let base = $state.raw<Workflow>(untrack(() => workflow));
  let draft = $state(untrack(() => toDraft(workflow)));
  const saved = $derived(toDraft(base));
  const readonly = $derived(!data.can.edit);
  const dirty = $derived(!readonly && isDirty(draft, saved, base.default));

  let saving = $state(false);
  let error = $state('');
  let stranded = $state<StrandedStatus[] | null>(null);
  let strandedDefault = $state<StrandedDefault | null>(null);
  let strandedMessage = $state('');
  let remap = $state<Record<string, string>>({});
  let remapDefault = $state<Record<string, string>>({});
  /** Statuses created here, until the catalogue is reloaded. */
  let created = $state<StatusRef[]>([]);

  const catalogue = $derived([
    ...data.statuses,
    ...created.filter((c) => !data.statuses.some((s) => s.id === c.id)),
  ]);
  /** Issue types that follow another workflow than the default. */
  const followers = $derived(
    new Map(
      data.workflows
        .filter((w) => w.id !== base.id && !w.default)
        .flatMap((w) => w.issueTypes.map((ty) => [ty.id, w.name] as const)),
    ),
  );
  const initialStatus = $derived(draft.statuses.find((s) => s.id === draft.initial));

  $effect(() => ondirty(dirty));
  onDestroy(() => ondirty(false));

  // Leaving the page with unsaved changes asks first.
  $effect(() => {
    if (!dirty) return;
    const hold = (e: BeforeUnloadEvent) => e.preventDefault();
    window.addEventListener('beforeunload', hold);
    return () => window.removeEventListener('beforeunload', hold);
  });

  function onadd(s: StatusRef) {
    if (!catalogue.some((c) => c.id === s.id)) created = [...created, s];
    draft = addStatus(draft, s);
  }

  function flipType(id: string, on: boolean) {
    draft.issueTypes = on ? [...draft.issueTypes, id] : draft.issueTypes.filter((x) => x !== id);
  }

  function discard() {
    draft = toDraft(base);
    error = '';
  }

  /** Sends the draft and returns the project's workflows as saved. */
  function send(
    withRemap?: Record<string, string>,
    withRemapDefault?: Record<string, string>,
  ): Promise<ProjectWorkflows> {
    return api.saveWorkflow(base.id, toInput(draft, base.default, withRemap, withRemapDefault));
  }

  /** Adopts the workflows the server saved and tells the panel. */
  function adopt(d: ProjectWorkflows) {
    const updated = d.workflows.find((w) => w.id === base.id);
    stranded = null;
    strandedDefault = null;
    if (updated) {
      base = updated;
      draft = toDraft(updated);
    }
    onsaved(d);
    notify(t.workflows.saved);
  }

  /** Offers the remap a 409 asks for, with its suggested moves; shows the error otherwise. */
  function offerRemap(e: unknown) {
    if (e instanceof ApiError && e.status === 409) {
      const list = strandedOf(e.data);
      const toDefault = strandedDefaultOf(e.data);
      if (list || toDefault) {
        stranded = list ?? [];
        strandedDefault = toDefault;
        strandedMessage = e.message;
        remap = suggestRemap(stranded, draft.statuses, draft.initial);
        remapDefault = toDefault ? suggestDefaultRemap(toDefault) : {};
        return;
      }
    }
    stranded = null;
    strandedDefault = null;
    error = errorMessage(e);
  }

  async function save(
    withRemap?: Record<string, string>,
    withRemapDefault?: Record<string, string>,
  ) {
    saving = true;
    error = '';
    try {
      adopt(await send(withRemap, withRemapDefault));
    } catch (e) {
      offerRemap(e);
    } finally {
      saving = false;
    }
  }
</script>

<div class="editor">
  <p class="crumbs">
    <button type="button" class="link-btn" onclick={onback}>← {t.workflows.back}</button>
  </p>
  <header class="head">
    <h2 class="name">{draft.name.trim() || base.name}</h2>
    {#if base.default}<span class="chip">{t.workflows.defaultBadge}</span>{/if}
  </header>
  {#if readonly}<p class="note">{t.workflows.readOnly}</p>{/if}

  <div class="top">
    <section aria-labelledby="wf-general">
      <h3 class="section-title" id="wf-general">{t.workflows.general}</h3>
      {#if readonly}
        <dl class="facts">
          <div>
            <dt>{t.workflows.initial}</dt>
            <dd>
              {#if initialStatus}<Stamp status={initialStatus} />{/if}
            </dd>
          </div>
          <div>
            <dt>{t.workflows.servedTypes}</dt>
            <dd class="types">
              {#each base.issueTypes as ty (ty.id)}<TypeMark type={ty} withName />{:else}<span
                  class="muted">{t.workflows.noTypes}</span
                >{/each}
            </dd>
          </div>
        </dl>
        {#if base.default}<p class="hint">{t.workflows.isDefault}</p>{/if}
      {:else}
        <div class="fields">
          <label class="field">
            <span>{t.workflows.newName}</span>
            <input
              class="input"
              bind:value={draft.name}
              maxlength="255"
              required
              autocomplete="off"
            />
          </label>
          <label class="field">
            <span>{t.workflows.initial}</span>
            <select class="select" bind:value={draft.initial}>
              {#each draft.statuses as s (s.id)}<option value={s.id}>{s.name}</option>{/each}
            </select>
            <span class="hint">{t.workflows.initialHint}</span>
          </label>
          {#if base.default}
            <div class="field">
              <span class="field-label">{t.workflows.servedTypes}</span>
              <div class="types">
                {#each base.issueTypes as ty (ty.id)}<TypeMark type={ty} withName />{:else}<span
                    class="muted">{t.workflows.noTypes}</span
                  >{/each}
              </div>
              <span class="hint">{t.workflows.isDefault}</span>
            </div>
          {:else}
            <div class="field">
              <label class="check">
                <input type="checkbox" class="checkbox" bind:checked={draft.makeDefault} />
                {t.workflows.makeDefault}
              </label>
              <span class="hint">{t.workflows.makeDefaultHint}</span>
            </div>
            <fieldset class="field served" disabled={draft.makeDefault}>
              <legend class="field-label">{t.workflows.servedTypes}</legend>
              {#each data.issueTypes as ty (ty.id)}
                <label class="check">
                  <input
                    type="checkbox"
                    class="checkbox"
                    checked={draft.makeDefault || draft.issueTypes.includes(ty.id)}
                    onchange={(e) => flipType(ty.id, e.currentTarget.checked)}
                  />
                  <TypeMark type={ty} withName />
                  {#if followers.has(ty.id)}<span class="muted follows"
                      >{t.workflows.follows(followers.get(ty.id) ?? '')}</span
                    >{/if}
                </label>
              {/each}
            </fieldset>
          {/if}
        </div>
      {/if}
    </section>

    <section aria-labelledby="wf-statuses">
      <h3 class="section-title" id="wf-statuses">
        {t.workflows.statuses} <span class="count">{draft.statuses.length}</span>
      </h3>
      <StatusList
        {projectKey}
        statuses={draft.statuses}
        initial={draft.initial}
        usage={base.usage}
        {catalogue}
        {readonly}
        onreorder={(list) => (draft.statuses = list)}
        onremove={(id) => (draft = removeStatus(draft, id))}
        {onadd}
      />
    </section>
  </div>

  <section aria-labelledby="wf-diagram">
    <h3 class="section-title" id="wf-diagram">{t.workflows.diagram}</h3>
    <WorkflowDiagram
      statuses={draft.statuses}
      transitions={draft.transitions}
      initial={draft.initial}
    />
  </section>

  <section aria-labelledby="wf-transitions">
    <h3 class="section-title" id="wf-transitions">
      {t.workflows.transitions}
      <span class="count">{t.workflows.count(draft.transitions.length)}</span>
    </h3>
    {#if !readonly}<p class="hint matrix-hint">{t.workflows.matrixHint}</p>{/if}
    <TransitionMatrix
      statuses={draft.statuses}
      transitions={draft.transitions}
      saved={saved.transitions}
      {readonly}
      onchange={(list) => (draft.transitions = list)}
    />
  </section>

  {#if dirty || error}
    <div class="savebar" role="region" aria-label={t.settings.unsaved}>
      {#if error}
        <p class="error-text" role="alert">{error}</p>
      {:else}
        <span class="state">{t.settings.unsaved}</span>
      {/if}
      <button type="button" class="btn btn-quiet" onclick={discard}>{t.settings.discard}</button>
      <button
        type="button"
        class="btn btn-primary"
        disabled={saving || !dirty}
        onclick={() => void save()}>{saving ? t.common.saving : t.common.save}</button
      >
    </div>
  {/if}
</div>

<RemapDialog
  {stranded}
  {strandedDefault}
  message={strandedMessage}
  statuses={draft.statuses}
  choice={remap}
  choiceDefault={remapDefault}
  busy={saving}
  onchoose={(from, to) => (remap[from] = to)}
  onchooseDefault={(from, to) => (remapDefault[from] = to)}
  onconfirm={() => void save({ ...remap }, { ...remapDefault })}
  onclose={() => {
    stranded = null;
    strandedDefault = null;
  }}
/>

<style>
  .crumbs {
    margin: 18px 0 10px;
    font-size: 13px;
  }

  .head {
    display: flex;
    align-items: baseline;
    flex-wrap: wrap;
    gap: 6px 12px;
  }

  .name {
    font-size: 22px;
    overflow-wrap: anywhere;
  }

  .note {
    margin: 10px 0 0;
    font: var(--title-italic) 14px/1.5 var(--font-title);
    color: var(--ink-soft);
  }

  .top {
    display: grid;
    grid-template-columns: minmax(0, 5fr) minmax(0, 7fr);
    gap: 0 40px;
  }

  @media (max-width: 900px) {
    .top {
      grid-template-columns: minmax(0, 1fr);
    }
  }

  .fields {
    display: grid;
    gap: 14px;
    max-width: 440px;
  }

  .hint {
    font-size: 12px;
    color: var(--muted);
  }

  .matrix-hint {
    max-width: 80ch;
    margin: -2px 0 10px;
  }

  .check {
    display: flex;
    align-items: center;
    gap: 8px;
    min-height: 26px;
    font-size: 13.5px;
  }

  .served {
    border: 0;
    margin: 0;
    padding: 0;
    gap: 2px;
  }

  .served legend {
    padding: 0;
    margin-bottom: 6px;
  }

  .served[disabled] {
    opacity: 0.55;
  }

  .follows {
    font-size: 12px;
  }

  .types {
    display: flex;
    flex-wrap: wrap;
    gap: 6px 14px;
    font-size: 13.5px;
  }

  .facts {
    display: grid;
    gap: 10px;
    margin: 0 0 8px;
  }

  .facts div {
    display: grid;
    gap: 4px;
  }

  .facts dt {
    font: 500 12px/1.2 var(--font-body);
    color: var(--ink-soft);
  }

  .facts dd {
    margin: 0;
  }
</style>
