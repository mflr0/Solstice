<script lang="ts">
  import type { AdminUser } from '../../core/types.js';
  import { formatDay, t } from '../../i18n/fr.js';
  import { api, errorMessage } from '../../lib/api.js';
  import { session } from '../../lib/session.svelte.js';
  import { notify } from '../../lib/ui.svelte.js';
  import Modal from '../Modal.svelte';
  import NotFoundPage from './NotFoundPage.svelte';

  /** Accounts: who exists, who may sign in, who administers. */
  let users = $state<AdminUser[] | null>(null);
  let q = $state('');
  let error = $state('');
  let creating = $state(false);
  let resetting = $state<AdminUser | null>(null);
  let form = $state({ username: '', name: '', email: '', password: '', admin: false });
  let newPassword = $state('');
  let formError = $state('');
  let timer: ReturnType<typeof setTimeout> | undefined;

  async function load() {
    try {
      users = await api.adminUsers(q.trim());
    } catch (e) {
      error = errorMessage(e);
    }
  }

  $effect(() => {
    document.title = `${t.admin.title} — ${t.appName}`;
    if (session.me?.admin) void load();
  });

  function typed() {
    clearTimeout(timer);
    timer = setTimeout(() => void load(), 250);
  }

  async function patch(u: AdminUser, change: Parameters<typeof api.adminUpdateUser>[1]) {
    try {
      await api.adminUpdateUser(u.id, change);
      notify(t.admin.updated);
      await load();
    } catch (e) {
      notify(errorMessage(e), 'error');
    }
  }

  async function create(e: SubmitEvent) {
    e.preventDefault();
    formError = '';
    try {
      await api.adminCreateUser({ ...form });
      creating = false;
      notify(t.admin.created);
      form = { username: '', name: '', email: '', password: '', admin: false };
      await load();
    } catch (err) {
      formError = errorMessage(err);
    }
  }

  async function reset(e: SubmitEvent) {
    e.preventDefault();
    if (!resetting) return;
    formError = '';
    try {
      await api.adminUpdateUser(resetting.id, { password: newPassword });
      notify(t.admin.updated);
      resetting = null;
      newPassword = '';
      await load();
    } catch (err) {
      formError = errorMessage(err);
    }
  }
</script>

{#if !session.me?.admin}
  <NotFoundPage message={t.admin.forbidden} />
{:else}
  <div class="page">
    <header class="page-head">
      <h1>{t.admin.title}</h1>
      <button class="btn btn-primary" onclick={() => ((creating = true), (formError = ''))}
        >{t.admin.newUser}</button
      >
    </header>

    <input
      class="input filter"
      bind:value={q}
      oninput={typed}
      placeholder={t.common.filter}
      aria-label={t.common.filter}
    />

    {#if error}
      <p class="error-text" role="alert">{error}</p>
    {:else if !users}
      <p class="muted">{t.common.loading}</p>
    {:else}
      <table class="ledger">
        <thead>
          <tr>
            <th>{t.admin.name}</th>
            <th>{t.admin.username}</th>
            <th>{t.admin.email}</th>
            <th>{t.admin.state}</th>
            <th class="num">{t.admin.since}</th>
            <th><span class="sr-only">{t.common.actions}</span></th>
          </tr>
        </thead>
        <tbody>
          {#each users as u (u.id)}
            <tr class:off={!u.active}>
              <td><strong>{u.name}</strong></td>
              <td class="key">{u.username}</td>
              <td>{u.email}</td>
              <td class="tags">
                {#if u.admin}<span class="stamp done">{t.admin.admin}</span>{/if}
                {#if !u.active}<span class="stamp">{t.common.inactive}</span>{/if}
                {#if !u.hasPassword}<span class="muted small">{t.admin.noPassword}</span>{/if}
              </td>
              <td class="num muted">{formatDay(u.created.slice(0, 10))}</td>
              <td class="row-actions">
                <button
                  class="link-btn"
                  onclick={() => ((resetting = u), (newPassword = ''), (formError = ''))}
                  >{t.admin.resetPassword}</button
                >
                {#if u.id !== session.me.id}
                  <button class="link-btn" onclick={() => void patch(u, { active: !u.active })}
                    >{u.active ? t.admin.deactivate : t.admin.activate}</button
                  >
                  <button class="link-btn" onclick={() => void patch(u, { admin: !u.admin })}
                    >{u.admin ? t.admin.removeAdmin : t.admin.makeAdmin}</button
                  >
                {/if}
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    {/if}
  </div>

  <Modal open={creating} title={t.admin.newUser} onclose={() => (creating = false)}>
    <form id="new-user" class="form" onsubmit={create}>
      <label class="field"
        ><span>{t.admin.username}</span><input
          class="input"
          bind:value={form.username}
          required
          autocomplete="off"
        /></label
      >
      <label class="field"
        ><span>{t.admin.name}</span><input class="input" bind:value={form.name} /></label
      >
      <label class="field"
        ><span>{t.admin.email}</span><input
          class="input"
          type="email"
          bind:value={form.email}
        /></label
      >
      <label class="field">
        <span>{t.admin.password}</span>
        <input
          class="input"
          type="password"
          bind:value={form.password}
          minlength="10"
          required
          autocomplete="new-password"
        />
        <span class="hint">{t.admin.passwordHint}</span>
      </label>
      <label class="check"
        ><input type="checkbox" bind:checked={form.admin} /> {t.admin.admin}</label
      >
      {#if formError}<p class="error-text" role="alert">{formError}</p>{/if}
    </form>
    {#snippet footer()}
      <button class="btn btn-quiet" onclick={() => (creating = false)}>{t.common.cancel}</button>
      <button class="btn btn-primary" type="submit" form="new-user">{t.common.save}</button>
    {/snippet}
  </Modal>

  <Modal
    open={resetting !== null}
    title={`${t.admin.resetPassword} — ${resetting?.name ?? ''}`}
    onclose={() => (resetting = null)}
  >
    <form id="reset-password" class="form" onsubmit={reset}>
      <label class="field">
        <span>{t.admin.password}</span>
        <input
          class="input"
          type="password"
          bind:value={newPassword}
          minlength="10"
          required
          autocomplete="new-password"
        />
        <span class="hint">{t.admin.passwordHint}</span>
      </label>
      {#if formError}<p class="error-text" role="alert">{formError}</p>{/if}
    </form>
    {#snippet footer()}
      <button class="btn btn-quiet" onclick={() => (resetting = null)}>{t.common.cancel}</button>
      <button class="btn btn-primary" type="submit" form="reset-password">{t.common.save}</button>
    {/snippet}
  </Modal>
{/if}

<style>
  .filter {
    max-width: 320px;
    margin-bottom: 16px;
  }

  .off td {
    color: var(--muted);
  }

  .tags {
    display: flex;
    gap: 6px;
    align-items: center;
    flex-wrap: wrap;
  }

  .small {
    font-size: 12px;
  }

  .row-actions {
    white-space: nowrap;
    text-align: right;
  }

  .row-actions .link-btn + .link-btn {
    margin-left: 12px;
  }

  .form {
    display: grid;
    gap: 12px;
  }

  .check {
    display: flex;
    gap: 8px;
    align-items: center;
  }
</style>
