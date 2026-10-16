<script lang="ts">
  import type { ComponentRow } from '../../core/admin-types.js';
  import { otherComponents } from '../../core/project-admin.js';
  import { formatNumber } from '../../i18n/format.js';
  import { t } from '../../i18n/t.js';
  import { projectAdminApi } from '../../lib/admin-api.js';
  import { errorMessage } from '../../lib/api.js';
  import { notify } from '../../lib/ui.svelte.js';
  import ConfirmDialog from '../ConfirmDialog.svelte';
  import EmptyState from '../EmptyState.svelte';
  import Who from '../Who.svelte';
  import ComponentDialog from './ComponentDialog.svelte';

  /**
   * The project's components: create, change, archive and delete. A deleted
   * component's issues can move to another component first.
   */
  interface Props {
    projectKey: string;
  }
  const { projectKey }: Props = $props();

  let components = $state<ComponentRow[] | null>(null);
  let error = $state('');
  let dialog = $state<{ component: ComponentRow | null } | null>(null);
  let removing = $state<ComponentRow | null>(null);
  let moveTo = $state('');
  let busyId = $state<number | null>(null);

  const targets = $derived(removing && components ? otherComponents(components, removing.id) : []);

  async function load() {
    try {
      components = await projectAdminApi.components(projectKey);
      error = '';
    } catch (e) {
      error = errorMessage(e);
    }
  }

  $effect(() => {
    void projectKey;
    void load();
  });

  function saved(c: ComponentRow, created: boolean) {
    notify(created ? t.projectComponents.created(c.name) : t.projectComponents.saved(c.name));
    void load();
  }

  async function archive(c: ComponentRow, on: boolean) {
    busyId = c.id;
    try {
      await projectAdminApi.updateComponent(projectKey, c.id, { archived: on });
      notify(on ? t.projectComponents.archived(c.name) : t.projectComponents.unarchived(c.name));
      await load();
    } catch (e) {
      notify(errorMessage(e), 'error');
    } finally {
      busyId = null;
    }
  }

  function askRemove(c: ComponentRow) {
    removing = c;
    moveTo = '';
  }

  async function remove() {
    const c = removing;
    if (!c) return;
    const target = c.issues > 0 ? targets.find((x) => String(x.id) === moveTo)?.id : undefined;
    await projectAdminApi.deleteComponent(projectKey, c.id, target);
    notify(t.projectComponents.deleted(c.name));
    await load();
  }
</script>

<div class="head">
  <p class="lead-text">{t.projectComponents.intro}</p>
  {#if components && components.length > 0}
    <button class="btn btn-primary" onclick={() => (dialog = { component: null })}
      >{t.projectComponents.newComponent}</button
    >
  {/if}
</div>

{#if error}
  <p class="error-text" role="alert">{error}</p>
{:else if !components}
  <p class="muted loading">{t.common.loading}</p>
{:else if components.length === 0}
  <EmptyState message={t.projectComponents.empty} hint={t.projectComponents.emptyHint}>
    <button class="btn btn-primary" onclick={() => (dialog = { component: null })}
      >{t.projectComponents.newComponent}</button
    >
  </EmptyState>
{:else}
  <div class="table-scroll">
    <table class="ledger">
      <thead>
        <tr>
          <th>{t.projects.name}</th>
          <th>{t.projects.lead}</th>
          <th>{t.projectComponents.assignee}</th>
          <th class="num">{t.projectsAdmin.issues}</th>
          <th><span class="sr-only">{t.common.actions}</span></th>
        </tr>
      </thead>
      <tbody>
        {#each components as c (c.id)}
          <tr class:is-off={c.archived}>
            <td>
              <strong>{c.name}</strong>
              {#if c.archived}<span class="stamp">{t.projects.archived}</span>{/if}
              {#if c.description}<span class="muted small desc">{c.description}</span>{/if}
            </td>
            <td><Who user={c.lead} empty={t.common.none} /></td>
            <td>{t.projectComponents.assigneeTypes[c.assigneeType]}</td>
            <td class="num">{formatNumber(c.issues)}</td>
            <td class="row-actions">
              <button class="link-btn" onclick={() => (dialog = { component: c })}
                >{t.common.edit}</button
              >
              <button
                class="link-btn"
                disabled={busyId === c.id}
                onclick={() => void archive(c, !c.archived)}
                >{c.archived ? t.projectsAdmin.unarchive : t.projectsAdmin.archive}</button
              >
              <button class="link-btn" onclick={() => askRemove(c)}>{t.common.delete}</button>
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
  <p class="form-hint note">{t.projectComponents.archivedHint}</p>
{/if}

<ComponentDialog
  open={dialog !== null}
  {projectKey}
  component={dialog?.component ?? null}
  components={components ?? []}
  onclose={() => (dialog = null)}
  onsaved={saved}
/>

<ConfirmDialog
  open={removing !== null}
  title={t.projectComponents.deleteTitle(removing?.name ?? '')}
  confirm={t.common.delete}
  onclose={() => (removing = null)}
  onconfirm={remove}
>
  {#if removing && removing.issues > 0}
    <p>{t.projectComponents.deleteUsed(removing.issues)}</p>
    <label class="field">
      <span>{t.projectComponents.moveLabel}</span>
      <select class="select" bind:value={moveTo}>
        <option value="">{t.projectComponents.moveNone}</option>
        {#each targets as x (x.id)}<option value={String(x.id)}>{x.name}</option>{/each}
      </select>
    </label>
  {:else}
    <p>{t.projectComponents.deleteUnused}</p>
  {/if}
</ConfirmDialog>

<style>
  .head {
    display: flex;
    flex-wrap: wrap;
    align-items: flex-start;
    justify-content: space-between;
    gap: 10px 16px;
  }

  .head .lead-text {
    flex: 1 1 320px;
    margin-bottom: 14px;
  }

  .desc {
    display: block;
    margin-top: 2px;
    max-width: 60ch;
  }

  .note {
    margin: 10px 0 0;
  }

  .stamp {
    margin-left: 6px;
  }
</style>
