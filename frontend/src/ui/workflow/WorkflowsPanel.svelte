<script lang="ts">
  import { paths } from '../../core/route.js';
  import type { ProjectWorkflows, StrandedDefault, Workflow } from '../../core/types.js';
  import { strandedDefaultOf, suggestRemap } from '../../core/workflow.js';
  import { t } from '../../i18n/fr.js';
  import { api, ApiError, errorMessage } from '../../lib/api.js';
  import { router } from '../../lib/router.svelte.js';
  import { notify } from '../../lib/ui.svelte.js';
  import Modal from '../Modal.svelte';
  import Stamp from '../Stamp.svelte';
  import TypeMark from '../TypeMark.svelte';
  import RemapDialog from './RemapDialog.svelte';
  import WorkflowEditor from './WorkflowEditor.svelte';

  /**
   * The project's workflows: the list (default first, with the issue types
   * each one serves), then one of them in the editor. The open workflow is
   * in the address (?tab=workflows&workflow=ID).
   */
  interface Props {
    projectKey: string;
    /** Runs `go` now, or once the user agrees to drop unsaved changes. */
    guard: (go: () => void) => void;
    ondirty: (dirty: boolean) => void;
  }
  const { projectKey, guard, ondirty }: Props = $props();

  let data = $state<ProjectWorkflows | null>(null);
  let error = $state('');
  let creating = $state(false);
  let newName = $state('');
  let copyFrom = $state('');
  let formError = $state('');
  let busy = $state(false);
  let deleting = $state<Workflow | null>(null);
  /** Deleting needs issues moved first: the workflow, and where they sit. */
  let stuck = $state<{ workflow: Workflow; issues: StrandedDefault; message: string } | null>(null);
  let stuckChoice = $state<Record<string, string>>({});

  const openId = $derived(Number(router.param('workflow')) || null);
  const open = $derived(data?.workflows.find((w) => w.id === openId) ?? null);
  const fallback = $derived(data?.workflows.find((w) => w.default) ?? null);

  function href(id: number): string {
    return `${paths.projectSettings(projectKey)}?tab=workflows&workflow=${id}`;
  }

  async function load() {
    try {
      data = await api.workflows(projectKey);
    } catch (e) {
      error = errorMessage(e);
    }
  }

  $effect(() => {
    void projectKey;
    void load();
  });

  function startCreate() {
    newName = '';
    copyFrom = String(fallback?.id ?? '');
    formError = '';
    creating = true;
  }

  async function create(e: SubmitEvent) {
    e.preventDefault();
    busy = true;
    formError = '';
    try {
      const from = Number(copyFrom);
      const { id } = await api.createWorkflow(projectKey, {
        name: newName,
        ...(from ? { copyFrom: from } : {}),
      });
      creating = false;
      notify(t.workflows.created(newName.trim()));
      await load();
      router.setParams({ workflow: String(id) }, { replace: false });
    } catch (err) {
      formError = errorMessage(err);
    } finally {
      busy = false;
    }
  }

  async function remove(w = deleting, remapDefault?: Record<string, string>) {
    if (!w) return;
    busy = true;
    try {
      await api.deleteWorkflow(w.id, remapDefault);
      deleting = null;
      stuck = null;
      notify(t.workflows.deleted(w.name));
      await load();
    } catch (err) {
      const issues =
        err instanceof ApiError && err.status === 409 ? strandedDefaultOf(err.data) : null;
      if (issues && err instanceof Error) {
        deleting = null;
        stuck = { workflow: w, issues, message: err.message };
        stuckChoice = suggestRemap(
          issues.stranded,
          issues.workflow.statuses,
          issues.workflow.statuses[0]?.id ?? '',
        );
      } else {
        notify(errorMessage(err), 'error');
      }
    } finally {
      busy = false;
    }
  }
</script>

{#if error}
  <p class="error-text" role="alert">{error}</p>
{:else if !data}
  <p class="muted loading">{t.common.loading}</p>
{:else if open}
  {#key open.id}
    <WorkflowEditor
      {projectKey}
      workflow={open}
      {data}
      onsaved={(d) => (data = d)}
      {ondirty}
      onback={() => guard(() => router.setParams({ workflow: null }, { replace: false }))}
    />
  {/key}
{:else}
  <h2 class="section-title">
    {t.workflows.tab} <span class="count">{data.workflows.length}</span>
    {#if data.can.edit}
      <span class="actions">
        <button type="button" class="btn btn-small" onclick={startCreate}
          >{t.workflows.newWorkflow}</button
        >
      </span>
    {/if}
  </h2>
  <p class="lead">{t.workflows.intro}</p>
  {#if !data.can.edit}<p class="lead note">{t.workflows.readOnly}</p>{/if}

  <div class="scroll">
    <table class="ledger list">
      <thead>
        <tr>
          <th scope="col">{t.workflows.name}</th>
          <th scope="col">{t.workflows.types}</th>
          <th scope="col">{t.workflows.statuses}</th>
          <th scope="col" class="num">{t.workflows.transitions}</th>
          <th scope="col"><span class="sr-only">{t.common.actions}</span></th>
        </tr>
      </thead>
      <tbody>
        {#each data.workflows as w (w.id)}
          <tr>
            <td class="wf">
              <a class="wf-name" href={href(w.id)}>{w.name}</a>
              {#if w.default}<span class="chip">{t.workflows.defaultBadge}</span>{/if}
            </td>
            <td>
              <span class="types">
                {#each w.issueTypes as ty (ty.id)}<TypeMark type={ty} withName />{:else}<span
                    class="muted">{t.workflows.noTypes}</span
                  >{/each}
              </span>
            </td>
            <td>
              <span class="flow">
                {#each w.statuses as s (s.id)}<Stamp status={s} />{/each}
              </span>
            </td>
            <td class="num">{w.transitions.length}</td>
            <td class="row-actions">
              <a class="link-btn" href={href(w.id)}
                >{data.can.edit ? t.workflows.edit : t.workflows.view}</a
              >
              {#if data.can.edit && !w.default}
                <button type="button" class="link-btn" onclick={() => (deleting = w)}
                  >{t.workflows.delete}</button
                >
              {/if}
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
{/if}

<Modal open={creating} title={t.workflows.newWorkflow} onclose={() => (creating = false)}>
  <form id="new-workflow" class="form" onsubmit={create}>
    <label class="field">
      <span>{t.workflows.newName}</span>
      <!-- svelte-ignore a11y_autofocus -->
      <input class="input" bind:value={newName} required maxlength="255" autofocus />
    </label>
    <label class="field">
      <span>{t.workflows.copyFrom}</span>
      <select class="select" bind:value={copyFrom}>
        {#each data?.workflows ?? [] as w (w.id)}
          <option value={String(w.id)}
            >{w.name}{w.default ? ` (${t.workflows.defaultBadge})` : ''}</option
          >
        {/each}
      </select>
    </label>
    {#if formError}<p class="error-text" role="alert">{formError}</p>{/if}
  </form>
  {#snippet footer()}
    <button type="button" class="btn btn-quiet" onclick={() => (creating = false)}
      >{t.common.cancel}</button
    >
    <button class="btn btn-primary" type="submit" form="new-workflow" disabled={busy}
      >{t.workflows.create}</button
    >
  {/snippet}
</Modal>

<Modal
  open={deleting !== null}
  title={t.workflows.deleteTitle(deleting?.name ?? '')}
  onclose={() => (deleting = null)}
>
  {#if deleting && deleting.issueTypes.length > 0}
    <p class="lead">
      {t.workflows.deleteTypes(
        deleting.issueTypes.map((ty) => ty.name),
        fallback?.name ?? '',
      )}
    </p>
  {/if}
  <p class="lead">{t.workflows.deleteKeep}</p>
  {#snippet footer()}
    <button type="button" class="btn btn-quiet" onclick={() => (deleting = null)}
      >{t.common.cancel}</button
    >
    <button type="button" class="btn btn-danger" disabled={busy} onclick={() => void remove()}
      >{t.workflows.delete}</button
    >
  {/snippet}
</Modal>

<RemapDialog
  stranded={null}
  strandedDefault={stuck?.issues ?? null}
  message={stuck?.message ?? ''}
  statuses={[]}
  choice={{}}
  choiceDefault={stuckChoice}
  {busy}
  confirmLabel={t.workflows.remapDeleteConfirm}
  onchoose={() => {}}
  onchooseDefault={(from, to) => (stuckChoice[from] = to)}
  onconfirm={() => void remove(stuck?.workflow ?? null, { ...stuckChoice })}
  onclose={() => (stuck = null)}
/>

<style>
  .lead {
    max-width: 80ch;
    margin: 0 0 14px;
    font-size: 13.5px;
    color: var(--ink-soft);
  }

  .note {
    font: var(--title-italic) 14px/1.5 var(--font-title);
  }

  /* Positioned, so hidden labels (.sr-only) stay inside the scrolling box. */
  .scroll {
    position: relative;
    overflow-x: auto;
  }

  .list td {
    vertical-align: top;
  }

  .list tbody tr {
    cursor: default;
  }

  .wf {
    min-width: 160px;
  }

  .wf-name {
    font-weight: 600;
    margin-right: 6px;
  }

  .types {
    display: flex;
    flex-wrap: wrap;
    gap: 4px 12px;
  }

  .flow {
    display: flex;
    flex-wrap: wrap;
    gap: 4px;
  }

  .row-actions {
    white-space: nowrap;
    text-align: right;
  }

  .row-actions > * + * {
    margin-left: 12px;
  }

  .form {
    display: grid;
    gap: 12px;
  }
</style>
