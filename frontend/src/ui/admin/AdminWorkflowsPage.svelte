<script lang="ts">
  import type { GlobalWorkflow } from '../../core/admin-types.js';
  import { adminPath } from '../../core/areas.js';
  import type { StatusRef } from '../../core/types.js';
  import { canDeleteGlobal, globalWorkflowsData, makeDefaultInput } from '../../core/workflow.js';
  import { t } from '../../i18n/t.js';
  import { adminApi } from '../../lib/admin-api.js';
  import { errorMessage } from '../../lib/api.js';
  import { router } from '../../lib/router.svelte.js';
  import { notify } from '../../lib/ui.svelte.js';
  import ConfirmDialog from '../ConfirmDialog.svelte';
  import Dropdown from '../Dropdown.svelte';
  import Modal from '../Modal.svelte';
  import Stamp from '../Stamp.svelte';
  import WorkflowEditor from '../workflow/WorkflowEditor.svelte';

  /**
   * The global workflows (/admin/workflows): the list, default first, then
   * one of them in the editor (/admin/workflows/ID). Projects get copies of
   * them, so nothing here moves an issue.
   */
  interface Props {
    /** The open workflow's id. */
    item: string | null;
    ondirty: (dirty: boolean) => void;
  }
  const { item, ondirty }: Props = $props();

  let list = $state<GlobalWorkflow[] | null>(null);
  let statuses = $state<StatusRef[]>([]);
  let error = $state('');
  let creating = $state(false);
  let newName = $state('');
  let copyFrom = $state('');
  let formError = $state('');
  let busy = $state(false);
  let deleting = $state<GlobalWorkflow | null>(null);

  const data = $derived(list ? globalWorkflowsData(list, statuses) : null);
  const open = $derived(
    item === null ? null : (data?.workflows.find((w) => String(w.id) === item) ?? null),
  );

  async function load() {
    try {
      const [workflows, catalogue] = await Promise.all([adminApi.workflows(), adminApi.statuses()]);
      list = workflows;
      statuses = catalogue.map(({ id, name, category }) => ({ id, name, category }));
    } catch (e) {
      error = errorMessage(e);
    }
  }

  $effect(() => {
    void load();
  });

  function startCreate() {
    newName = '';
    copyFrom = String(list?.find((w) => w.default)?.id ?? '');
    formError = '';
    creating = true;
  }

  async function create(e: SubmitEvent) {
    e.preventDefault();
    busy = true;
    formError = '';
    try {
      const from = Number(copyFrom);
      const { id } = await adminApi.createWorkflow({
        name: newName,
        ...(from ? { copyFrom: from } : {}),
      });
      creating = false;
      notify(t.workflows.created(newName.trim()));
      router.navigate(adminPath('workflows', id));
    } catch (err) {
      formError = errorMessage(err);
    } finally {
      busy = false;
    }
  }

  async function makeDefault(w: GlobalWorkflow) {
    busy = true;
    try {
      list = await adminApi.saveWorkflow(w.id, makeDefaultInput(w));
      notify(t.adminWorkflows.madeDefault(w.name));
    } catch (err) {
      notify(errorMessage(err), 'error');
    } finally {
      busy = false;
    }
  }

  async function remove() {
    const w = deleting;
    if (!w) return;
    await adminApi.deleteWorkflow(w.id);
    deleting = null;
    notify(t.workflows.deleted(w.name));
    await load();
  }
</script>

<header class="page-head">
  <div>
    <p class="eyebrow">{t.adminArea.groups.projects}</p>
    <h1 class="page-title">{t.adminArea.sections.workflows}</h1>
  </div>
  {#if list && item === null}
    <button class="btn btn-primary" onclick={startCreate}>{t.workflows.newWorkflow}</button>
  {/if}
</header>

{#if error}
  <p class="error-text" role="alert">{error}</p>
{:else if !data}
  <p class="muted loading">{t.common.loading}</p>
{:else if item !== null}
  {#if open}
    <WorkflowEditor
      projectKey={null}
      workflow={open}
      {data}
      onsaved={(d) => (list = d.workflows)}
      {ondirty}
      onback={() => router.navigate(adminPath('workflows'))}
    />
  {:else}
    <p class="error-text" role="alert">{t.adminWorkflows.notFound}</p>
    <p><a href={adminPath('workflows')}>← {t.workflows.back}</a></p>
  {/if}
{:else}
  <p class="lead-text">{t.adminWorkflows.intro}</p>
  <div class="scroll">
    <table class="ledger list">
      <thead>
        <tr>
          <th scope="col">{t.workflows.name}</th>
          <th scope="col">{t.workflows.statuses}</th>
          <th scope="col" class="num">{t.workflows.transitions}</th>
          <th scope="col"><span class="sr-only">{t.common.actions}</span></th>
        </tr>
      </thead>
      <tbody>
        {#each data.workflows as w (w.id)}
          <tr>
            <td class="wf">
              <a class="wf-name" href={adminPath('workflows', w.id)}>{w.name}</a>
              {#if w.default}<span class="chip">{t.workflows.defaultBadge}</span>{/if}
            </td>
            <td>
              <span class="flow">
                {#each w.statuses as s (s.id)}<Stamp status={s} />{/each}
              </span>
            </td>
            <td class="num">{w.transitions.length}</td>
            <td class="row-actions">
              <a class="link-btn" href={adminPath('workflows', w.id)}>{t.workflows.edit}</a>
              {#if !w.default}
                <button
                  type="button"
                  class="link-btn"
                  disabled={busy}
                  onclick={() => void makeDefault(w)}>{t.adminWorkflows.makeDefault}</button
                >
              {/if}
              {#if canDeleteGlobal(w, data.workflows)}
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
  <form id="new-global-workflow" class="form" onsubmit={create}>
    <label class="field">
      <span>{t.workflows.newName}</span>
      <!-- svelte-ignore a11y_autofocus -->
      <input class="input" bind:value={newName} required maxlength="255" autofocus />
    </label>
    <div class="field">
      <span>{t.workflows.copyFrom}</span>
      <Dropdown
        label={t.workflows.copyFrom}
        value={copyFrom}
        options={(list ?? []).map((w) => ({
          value: String(w.id),
          label: `${w.name}${w.default ? ` (${t.workflows.defaultBadge})` : ''}`,
        }))}
        onchange={(v) => (copyFrom = v)}
      />
    </div>
    {#if formError}<p class="error-text" role="alert">{formError}</p>{/if}
  </form>
  {#snippet footer()}
    <button type="button" class="btn btn-quiet" onclick={() => (creating = false)}
      >{t.common.cancel}</button
    >
    <button class="btn btn-primary" type="submit" form="new-global-workflow" disabled={busy}
      >{t.workflows.create}</button
    >
  {/snippet}
</Modal>

<ConfirmDialog
  open={deleting !== null}
  title={t.workflows.deleteTitle(deleting?.name ?? '')}
  confirm={t.workflows.delete}
  onclose={() => (deleting = null)}
  onconfirm={remove}
>
  <p>{t.adminWorkflows.deleteKeep}</p>
</ConfirmDialog>

<style>
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
    min-width: 10rem;
  }

  .wf-name {
    font-weight: 600;
    margin-right: 6px;
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
