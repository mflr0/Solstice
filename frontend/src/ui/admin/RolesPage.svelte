<script lang="ts">
  import { restrictionUsage } from '../../core/admin-forms.js';
  import type { RoleRow } from '../../core/admin-types.js';
  import { formatList, formatNumber } from '../../i18n/format.js';
  import { t } from '../../i18n/t.js';
  import { adminApi } from '../../lib/admin-api.js';
  import { ApiError, errorMessage } from '../../lib/api.js';
  import { notify } from '../../lib/ui.svelte.js';
  import ConfirmDialog from '../ConfirmDialog.svelte';
  import EmptyState from '../EmptyState.svelte';
  import RoleDialog from './RoleDialog.svelte';

  let roles = $state<RoleRow[] | null>(null);
  let error = $state('');
  let dialog = $state<{ role: RoleRow | null } | null>(null);
  let removing = $state<RoleRow | null>(null);
  let blocked = $state<{ comments: number; worklogs: number } | null>(null);

  const restrictedTo = $derived(
    blocked
      ? formatList(
          [
            blocked.comments > 0 ? t.adminArea.restricted.comments(blocked.comments) : '',
            blocked.worklogs > 0 ? t.adminArea.restricted.worklogs(blocked.worklogs) : '',
          ].filter(Boolean),
        )
      : '',
  );

  async function load() {
    try {
      roles = await adminApi.roles();
      error = '';
    } catch (e) {
      error = errorMessage(e);
    }
  }

  $effect(() => {
    void load();
  });

  function remove(r: RoleRow) {
    blocked = null;
    removing = r;
  }

  async function confirmRemove() {
    if (!removing) return;
    try {
      await adminApi.deleteRole(removing.id);
    } catch (e) {
      if (e instanceof ApiError && e.status === 409) blocked = restrictionUsage(e.data);
      throw e;
    }
    notify(t.adminArea.rolesPage.deleted(removing.name));
    void load();
  }

  function saved(message: string) {
    notify(message);
    void load();
  }
</script>

<header class="page-head">
  <div>
    <p class="eyebrow">{t.adminArea.groups.people}</p>
    <h1 class="page-title">
      {t.adminArea.sections.roles}
      {#if roles}<span class="count muted">{formatNumber(roles.length)}</span>{/if}
    </h1>
  </div>
  <button class="btn btn-primary" onclick={() => (dialog = { role: null })}
    >{t.adminArea.rolesPage.newRole}</button
  >
</header>
<p class="lead-text">{t.adminArea.rolesPage.shared}</p>

{#if error}
  <p class="error-text" role="alert">{error}</p>
{:else if !roles}
  <p class="muted loading">{t.common.loading}</p>
{:else if roles.length === 0}
  <EmptyState message={t.adminArea.rolesPage.empty} hint={t.adminArea.rolesPage.emptyHint}>
    <button class="btn btn-primary" onclick={() => (dialog = { role: null })}
      >{t.adminArea.rolesPage.newRole}</button
    >
  </EmptyState>
{:else}
  <div class="table-scroll">
    <table class="ledger roles">
      <thead>
        <tr>
          <th>{t.adminArea.rolesPage.name}</th>
          <th class="c-description">{t.adminArea.rolesPage.description}</th>
          <th><span class="sr-only">{t.common.actions}</span></th>
        </tr>
      </thead>
      <tbody>
        {#each roles as r (r.id)}
          <tr>
            <td>
              <button class="link-btn name" onclick={() => (dialog = { role: r })}>{r.name}</button>
            </td>
            <td class="c-description">{r.description || '—'}</td>
            <td class="row-actions">
              <button class="link-btn" onclick={() => (dialog = { role: r })}
                >{t.common.edit}</button
              >
              <button class="link-btn" onclick={() => remove(r)}>{t.common.delete}</button>
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
{/if}

<RoleDialog
  open={dialog !== null}
  role={dialog?.role ?? null}
  onclose={() => (dialog = null)}
  onsaved={saved}
/>

<ConfirmDialog
  open={removing !== null}
  title={t.adminArea.rolesPage.deleteTitle(removing?.name ?? '')}
  confirm={t.common.delete}
  onclose={() => (removing = null)}
  onconfirm={confirmRemove}
>
  <p>{t.adminArea.rolesPage.deleteBody}</p>
  {#if blocked}<p>{t.adminArea.rolesPage.inUse(restrictedTo)}</p>{/if}
</ConfirmDialog>

<style>
  .count {
    margin-left: 6px;
    font: 500 14px/1 var(--font-mono);
  }

  .name {
    display: block;
    font-weight: 600;
    color: var(--ink);
    text-decoration: none;
    text-align: left;
  }

  .name:hover {
    text-decoration: underline;
  }

  @media (max-width: 760px) {
    .c-description {
      display: none;
    }
  }
</style>
