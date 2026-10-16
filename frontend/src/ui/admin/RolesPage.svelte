<script lang="ts">
  import type { RoleRow } from '../../core/admin-types.js';
  import {
    filterRoles,
    namedPayload,
    restrictedUsage,
    roleInUse,
    roleNameProblem,
  } from '../../core/people-admin.js';
  import { formatNumber } from '../../i18n/format.js';
  import { t } from '../../i18n/t.js';
  import { adminApi } from '../../lib/admin-api.js';
  import { ApiError, errorMessage } from '../../lib/api.js';
  import { router } from '../../lib/router.svelte.js';
  import { notify } from '../../lib/ui.svelte.js';
  import ConfirmDialog from '../ConfirmDialog.svelte';
  import EmptyState from '../EmptyState.svelte';
  import TypedConfirmDialog from '../TypedConfirmDialog.svelte';
  import NamedDialog from './NamedDialog.svelte';

  /**
   * The project roles, shared by every project (who holds one is set in each
   * project's settings): find, create, rename and delete them. A role held
   * in projects or named by grants wants its name typed before it goes;
   * comments or work logs restricted to it forbid the deletion (the server
   * says so, with the counts). Roles have no default members in the model.
   */
  let roles = $state<RoleRow[] | null>(null);
  let error = $state('');
  let q = $state(router.param('q') ?? '');
  let dialog = $state<{ role: RoleRow | null } | null>(null);
  let deleting = $state<RoleRow | null>(null);
  let timer: ReturnType<typeof setTimeout> | undefined;

  const list = $derived(filterRoles(roles ?? [], q));
  const inUse = $derived(deleting !== null && roleInUse(deleting));

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

  function typed() {
    clearTimeout(timer);
    timer = setTimeout(() => router.setParams({ q: q.trim() || null }), 250);
  }

  function clear() {
    q = '';
    router.setParams({ q: null });
  }

  const problem = (name: string) => {
    const p = roleNameProblem(name, roles ?? [], dialog?.role?.id ?? null);
    return p ? t.rolesAdmin.problems[p] : '';
  };

  async function save(input: { name: string; description: string }) {
    const role = dialog?.role ?? null;
    const body = namedPayload(input.name, input.description);
    const saved = role ? await adminApi.updateRole(role.id, body) : await adminApi.createRole(body);
    notify(role ? t.rolesAdmin.saved(saved.name) : t.rolesAdmin.created(saved.name));
    await load();
  }

  /** What the role is used for, told before it goes ("held in 2 projects and named by 3 grants"). */
  function usedBy(r: RoleRow): string {
    const parts: string[] = [];
    if (r.projects > 0) parts.push(t.rolesAdmin.inProjects(r.projects));
    if (r.grants > 0) parts.push(t.rolesAdmin.inGrants(r.grants));
    return parts.join(', ');
  }

  async function remove() {
    const role = deleting;
    if (!role) return;
    try {
      await adminApi.deleteRole(role.id);
    } catch (e) {
      // Comments or work logs are still restricted to the role: say how many.
      const left = e instanceof ApiError && e.status === 409 ? restrictedUsage(e.data) : null;
      if (left)
        throw new Error(t.rolesAdmin.restricted(left.comments, left.worklogs), { cause: e });
      throw e;
    }
    notify(t.rolesAdmin.deleted(role.name));
    await load();
  }
</script>

<header class="page-head">
  <div>
    <p class="eyebrow">{t.adminArea.groups.people}</p>
    <h1 class="page-title">
      {t.rolesAdmin.title}
      {#if roles}<span class="count muted">{formatNumber(roles.length)}</span>{/if}
    </h1>
  </div>
  <button class="btn btn-primary" onclick={() => (dialog = { role: null })}
    >{t.rolesAdmin.newRole}</button
  >
</header>
<p class="lead-text">{t.rolesAdmin.intro}</p>

{#if error}
  <p class="error-text" role="alert">{error}</p>
{:else if !roles}
  <p class="muted loading">{t.common.loading}</p>
{:else if roles.length === 0}
  <EmptyState message={t.rolesAdmin.empty} hint={t.rolesAdmin.emptyHint}>
    <button class="btn btn-primary" onclick={() => (dialog = { role: null })}
      >{t.rolesAdmin.newRole}</button
    >
  </EmptyState>
{:else}
  <div class="list-tools">
    <input
      class="input search"
      type="search"
      bind:value={q}
      oninput={typed}
      placeholder={t.rolesAdmin.search}
      aria-label={t.rolesAdmin.search}
    />
  </div>

  {#if list.length === 0}
    <EmptyState kind="search" message={t.adminArea.noMatch}>
      <button class="btn" onclick={clear}>{t.adminArea.clearSearch}</button>
    </EmptyState>
  {:else}
    <div class="table-scroll">
      <table class="ledger roles">
        <thead>
          <tr>
            <th>{t.rolesAdmin.name}</th>
            <th class="c-desc">{t.rolesAdmin.description}</th>
            <th class="num" title={t.rolesAdmin.projectsHint}>{t.rolesAdmin.projects}</th>
            <th class="num c-grants" title={t.rolesAdmin.grantsHint}>{t.rolesAdmin.grants}</th>
            <th><span class="sr-only">{t.common.actions}</span></th>
          </tr>
        </thead>
        <tbody>
          {#each list as r (r.id)}
            <tr>
              <td><strong>{r.name}</strong></td>
              <td class="c-desc muted">{r.description ?? '—'}</td>
              <td class="num" class:muted={r.projects === 0}>{formatNumber(r.projects)}</td>
              <td class="num c-grants" class:muted={r.grants === 0}>{formatNumber(r.grants)}</td>
              <td class="row-actions">
                <button class="link-btn" onclick={() => (dialog = { role: r })}
                  >{t.common.edit}</button
                >
                <button class="link-btn" onclick={() => (deleting = r)}>{t.common.delete}</button>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
  <p class="muted small">{t.rolesAdmin.holdersHint}</p>
{/if}

<NamedDialog
  open={dialog !== null}
  title={dialog?.role ? t.rolesAdmin.editTitle(dialog.role.name) : t.rolesAdmin.createTitle}
  name={dialog?.role?.name ?? ''}
  description={dialog?.role?.description ?? ''}
  {problem}
  nameLabel={t.rolesAdmin.name}
  descriptionLabel={t.rolesAdmin.description}
  submit={dialog?.role ? t.common.save : t.rolesAdmin.create}
  onsave={save}
  onclose={() => (dialog = null)}
/>

<TypedConfirmDialog
  open={deleting !== null && inUse}
  title={t.rolesAdmin.deleteTitle(deleting?.name ?? '')}
  phrase={deleting?.name ?? ''}
  label={t.rolesAdmin.typeToConfirm(deleting?.name ?? '')}
  confirm={t.common.delete}
  onclose={() => (deleting = null)}
  onconfirm={remove}
>
  <p>{t.rolesAdmin.deleteUsed(deleting ? usedBy(deleting) : '')}</p>
</TypedConfirmDialog>

<ConfirmDialog
  open={deleting !== null && !inUse}
  title={t.rolesAdmin.deleteTitle(deleting?.name ?? '')}
  confirm={t.common.delete}
  onclose={() => (deleting = null)}
  onconfirm={remove}
>
  <p>{t.rolesAdmin.deleteUnused}</p>
</ConfirmDialog>

<style>
  .count {
    margin-left: 6px;
    font: 500 14px/1 var(--font-mono);
  }

  .c-desc {
    max-width: 50ch;
  }

  @media (max-width: 760px) {
    .c-desc {
      display: none;
    }
  }

  @media (max-width: 600px) {
    .c-grants {
      display: none;
    }
  }
</style>
