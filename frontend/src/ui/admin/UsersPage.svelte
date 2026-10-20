<script lang="ts">
  import type { Group, UserRow } from '../../core/admin-types.js';
  import { canResendInvitation } from '../../core/account-links.js';
  import { filterUsers, USER_SHOWS, type UserShow } from '../../core/admin-lists.js';
  import { adminPath } from '../../core/areas.js';
  import { formatNumber } from '../../i18n/format.js';
  import { formatDateTime, relative, t } from '../../i18n/t.js';
  import { adminApi } from '../../lib/admin-api.js';
  import { errorMessage } from '../../lib/api.js';
  import { publicSettings } from '../../lib/public-settings.svelte.js';
  import { router } from '../../lib/router.svelte.js';
  import { session } from '../../lib/session.svelte.js';
  import { notify } from '../../lib/ui.svelte.js';
  import ConfirmDialog from '../ConfirmDialog.svelte';
  import EmptyState from '../EmptyState.svelte';
  import PasswordDialog from './PasswordDialog.svelte';
  import UserDialog from './UserDialog.svelte';

  /** Accounts: who exists, who may sign in, who administers, in which groups. */
  let users = $state<UserRow[] | null>(null);
  let groups = $state<Group[] | null>(null);
  let error = $state('');
  let q = $state(router.param('q') ?? '');
  let show = $state<UserShow>('all');
  let dialog = $state<{ user: UserRow | null } | null>(null);
  let password = $state<UserRow | null>(null);
  let deactivating = $state<UserRow | null>(null);
  let timer: ReturnType<typeof setTimeout> | undefined;
  let seq = 0;

  const list = $derived(filterUsers(users ?? [], show));

  async function load() {
    const mine = ++seq;
    try {
      const found = await adminApi.users(q.trim());
      if (mine === seq) {
        users = found;
        error = '';
      }
    } catch (e) {
      if (mine === seq) error = errorMessage(e);
    }
  }

  $effect(() => {
    void load();
    adminApi
      .groups()
      .then((g) => (groups = g))
      .catch((e: unknown) => notify(errorMessage(e), 'error'));
  });

  // Not in the effect above: load reads q, so that one reruns on every keystroke.
  $effect(() => () => clearTimeout(timer));

  function typed() {
    clearTimeout(timer);
    timer = setTimeout(() => {
      router.setParams({ q: q.trim() || null });
      void load();
    }, 250);
  }

  function clear() {
    q = '';
    show = 'all';
    router.setParams({ q: null });
    void load();
  }

  async function reactivate(u: UserRow) {
    try {
      await adminApi.updateUser(u.id, { active: true });
      notify(t.usersAdmin.reactivated(u.name));
      await load();
    } catch (e) {
      notify(errorMessage(e), 'error');
    }
  }

  async function resendInvitation(u: UserRow) {
    try {
      await adminApi.inviteUser(u.id);
      notify(t.usersAdmin.inviteResent(u.name));
    } catch (e) {
      notify(errorMessage(e), 'error');
    }
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
      {t.usersAdmin.title}
      {#if users}<span class="count muted">{formatNumber(users.length)}</span>{/if}
    </h1>
  </div>
  <button class="btn btn-primary" onclick={() => (dialog = { user: null })}
    >{t.usersAdmin.newUser}</button
  >
</header>
<p class="lead-text">{t.usersAdmin.intro}</p>

<div class="list-tools">
  <input
    class="input search"
    type="search"
    bind:value={q}
    oninput={typed}
    placeholder={t.usersAdmin.search}
    aria-label={t.usersAdmin.search}
  />
  <select class="select" bind:value={show} aria-label={t.usersAdmin.showLabel}>
    {#each USER_SHOWS as s (s)}<option value={s}>{t.usersAdmin.show[s]}</option>{/each}
  </select>
</div>

{#if error}
  <p class="error-text" role="alert">{error}</p>
{:else if !users}
  <p class="muted loading">{t.common.loading}</p>
{:else if list.length === 0}
  <EmptyState kind="search" message={t.adminArea.noMatch}>
    <button class="btn" onclick={clear}>{t.adminArea.clearSearch}</button>
    <button class="btn btn-primary" onclick={() => (dialog = { user: null })}
      >{t.usersAdmin.newUser}</button
    >
  </EmptyState>
{:else}
  <div class="table-scroll">
    <table class="ledger users">
      <thead>
        <tr>
          <th>{t.usersAdmin.name}</th>
          <th class="c-email">{t.usersAdmin.email}</th>
          <th class="c-groups">{t.usersAdmin.groups}</th>
          <th>{t.usersAdmin.state}</th>
          <th class="num c-login">{t.usersAdmin.lastLogin}</th>
          <th><span class="sr-only">{t.common.actions}</span></th>
        </tr>
      </thead>
      <tbody>
        {#each list as u (u.id)}
          <tr class:is-off={!u.active}>
            <td>
              <button class="link-btn name" onclick={() => (dialog = { user: u })}>{u.name}</button>
              <span class="key muted">{u.username}</span>
            </td>
            <td class="c-email">{u.email || '—'}</td>
            <td class="c-groups">
              {#if u.groups?.length}
                <span class="tag-list">
                  {#each u.groups.slice(0, 3) as g (g.id)}<span class="chip">{g.name}</span>{/each}
                  {#if u.groups.length > 3}<span class="muted small"
                      >{t.usersAdmin.more(u.groups.length - 3)}</span
                    >{/if}
                </span>
              {:else}
                <span class="muted">—</span>
              {/if}
            </td>
            <td class="tags">
              <span class="tag-list">
                {#if u.admin}<span class="stamp done">{t.usersAdmin.admin}</span>{/if}
                {#if !u.active}<span class="stamp">{t.usersAdmin.inactive}</span>{/if}
                {#if !u.hasPassword}<span class="muted small">{t.usersAdmin.noPassword}</span>{/if}
              </span>
            </td>
            <td
              class="num muted c-login"
              title={u.lastLogin ? formatDateTime(u.lastLogin) : undefined}
            >
              {u.lastLogin ? relative(u.lastLogin) : t.usersAdmin.never}
            </td>
            <td class="row-actions">
              <button class="link-btn" onclick={() => (dialog = { user: u })}
                >{t.common.edit}</button
              >
              {#if u.id !== session.me?.id}
                <button class="link-btn" onclick={() => (password = u)}
                  >{t.usersAdmin.password}</button
                >
                {#if canResendInvitation(u, publicSettings.value.emailLinks)}
                  <button class="link-btn" onclick={() => void resendInvitation(u)}
                    >{t.usersAdmin.resendInvite}</button
                  >
                {/if}
                {#if u.active}
                  <button class="link-btn" onclick={() => (deactivating = u)}
                    >{t.usersAdmin.deactivate}</button
                  >
                {:else}
                  <button class="link-btn" onclick={() => void reactivate(u)}
                    >{t.usersAdmin.activate}</button
                  >
                {/if}
              {/if}
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
  {#if users.length >= 200}<p class="muted small">{t.usersAdmin.limit}</p>{/if}
  <p class="muted small">
    {t.usersAdmin.groupsHint}
    <a href={adminPath('groups')}>{t.adminArea.sections.groups}</a>
  </p>
{/if}

<UserDialog
  open={dialog !== null}
  user={dialog?.user ?? null}
  {groups}
  onclose={() => (dialog = null)}
  onsaved={saved}
/>

<PasswordDialog user={password} onclose={() => (password = null)} onsaved={saved} />

<ConfirmDialog
  open={deactivating !== null}
  title={t.usersAdmin.deactivateTitle(deactivating?.name ?? '')}
  confirm={t.usersAdmin.deactivate}
  onclose={() => (deactivating = null)}
  onconfirm={async () => {
    if (!deactivating) return;
    await adminApi.updateUser(deactivating.id, { active: false });
    notify(t.usersAdmin.deactivated(deactivating.name));
    void load();
  }}
>
  <p>{t.usersAdmin.deactivateBody}</p>
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

  .tag-list {
    display: flex;
    gap: 6px;
    align-items: center;
    flex-wrap: wrap;
  }

  @media (max-width: 760px) {
    .c-email,
    .c-groups {
      display: none;
    }
  }

  @media (max-width: 600px) {
    .c-login {
      display: none;
    }
  }
</style>
