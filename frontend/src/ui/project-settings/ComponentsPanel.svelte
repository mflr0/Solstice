<script lang="ts">
  import type { ComponentRow } from '../../core/admin-types.js';
  import { otherComponents } from '../../core/project-admin.js';
  import { t } from '../../i18n/t.js';
  import { projectAdminApi } from '../../lib/admin-api.js';
  import { errorMessage } from '../../lib/api.js';
  import { notify } from '../../lib/ui.svelte.js';
  import ConfirmDialog from '../ConfirmDialog.svelte';
  import EmptyState from '../EmptyState.svelte';
  import Who from '../Who.svelte';
  import ComponentDialog from './ComponentDialog.svelte';

  /**
   * The Components tab: the project's components with their lead and how
   * many issues use each, and their creation, change and removal. Deleting a
   * used component asks where its issues go (nowhere is allowed).
   */
  interface Props {
    projectKey: string;
  }
  const { projectKey }: Props = $props();

  let items = $state<ComponentRow[] | null>(null);
  let error = $state('');
  let dialog = $state<{ item: ComponentRow | null } | null>(null);
  let removing = $state<ComponentRow | null>(null);
  let moveTo = $state('');

  const targets = $derived(items && removing ? otherComponents(items, removing.id) : []);

  async function load() {
    try {
      items = await projectAdminApi.components(projectKey);
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
    notify(
      created
        ? t.projectConfig.components.created(c.name)
        : t.projectConfig.components.saved(c.name),
    );
    void load();
  }

  function askRemove(c: ComponentRow) {
    removing = c;
    moveTo = '';
  }

  async function remove() {
    const c = removing;
    if (!c) return;
    const target = targets.find((x) => String(x.id) === moveTo)?.id;
    await projectAdminApi.deleteComponent(projectKey, c.id, target);
    notify(t.projectConfig.components.deleted(c.name));
    await load();
  }
</script>

<h2 class="section-title">
  {t.projectConfig.components.title}
  {#if items}<span class="count">{items.length}</span>{/if}
  {#if items}
    <span class="actions">
      <button type="button" class="btn btn-small" onclick={() => (dialog = { item: null })}
        >{t.projectConfig.components.new}</button
      >
    </span>
  {/if}
</h2>
<p class="lead-text">{t.projectConfig.components.intro}</p>

{#if error}
  <p class="error-text" role="alert">{error}</p>
{:else if !items}
  <p class="muted loading">{t.common.loading}</p>
{:else if items.length === 0}
  <EmptyState
    message={t.projectConfig.components.empty}
    hint={t.projectConfig.components.emptyHint}
  >
    <button type="button" class="btn btn-primary" onclick={() => (dialog = { item: null })}
      >{t.projectConfig.components.new}</button
    >
  </EmptyState>
{:else}
  <div class="table-scroll">
    <table class="ledger">
      <thead>
        <tr>
          <th scope="col">{t.projects.name}</th>
          <th scope="col">{t.projects.lead}</th>
          <th scope="col" class="num">{t.projectsAdmin.issues}</th>
          <th scope="col"><span class="sr-only">{t.common.actions}</span></th>
        </tr>
      </thead>
      <tbody>
        {#each items as c (c.id)}
          <tr class:is-off={c.archived}>
            <td>
              <strong>{c.name}</strong>
              {#if c.archived}<span class="stamp">{t.projects.archived}</span>{/if}
              {#if c.description}<span class="muted small desc">{c.description}</span>{/if}
            </td>
            <td><Who user={c.lead} empty={t.projectConfig.components.noLead} /></td>
            <td class="num">{c.issues}</td>
            <td class="row-actions">
              <button type="button" class="link-btn" onclick={() => (dialog = { item: c })}
                >{t.common.edit}</button
              >
              <button type="button" class="link-btn" onclick={() => askRemove(c)}
                >{t.common.delete}</button
              >
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
{/if}

{#if items}
  <ComponentDialog
    {projectKey}
    open={dialog !== null}
    component={dialog?.item ?? null}
    components={items}
    onclose={() => (dialog = null)}
    onsaved={saved}
  />
{/if}

<ConfirmDialog
  open={removing !== null}
  title={t.projectConfig.components.deleteTitle(removing?.name ?? '')}
  confirm={t.common.delete}
  onclose={() => (removing = null)}
  onconfirm={remove}
>
  {#if removing && removing.issues > 0}
    <p>{t.projectConfig.components.deleteUsed(removing.issues)}</p>
    <label class="field">
      <span>{t.projectConfig.components.moveLabel}</span>
      <select class="select" bind:value={moveTo}>
        <option value="">{t.projectConfig.components.moveNone}</option>
        {#each targets as x (x.id)}<option value={String(x.id)}>{x.name}</option>{/each}
      </select>
    </label>
  {:else}
    <p>{t.projectConfig.components.deleteUnused}</p>
  {/if}
</ConfirmDialog>

<style>
  .desc {
    display: block;
    margin-top: 2px;
    max-width: 60ch;
  }
</style>
