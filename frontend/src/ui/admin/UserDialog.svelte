<script lang="ts">
  import {
    hasProblems,
    matches,
    newUserForm,
    suggestUsername,
    userProblems,
    type UserForm,
  } from '../../core/admin-forms.js';
  import type { Group, UserRow } from '../../core/admin-types.js';
  import { t } from '../../i18n/t.js';
  import { adminApi } from '../../lib/admin-api.js';
  import { errorMessage } from '../../lib/api.js';
  import { publicSettings } from '../../lib/public-settings.svelte.js';
  import { session } from '../../lib/session.svelte.js';
  import Modal from '../Modal.svelte';

  /**
   * Creating an account, or changing one: name, e-mail, administrator
   * rights, whether it may sign in, and its groups.
   */
  interface Props {
    open: boolean;
    /** The account to change; null to create one. */
    user: UserRow | null;
    /** Every group, or null when the server does not list them. */
    groups: Group[] | null;
    onclose: () => void;
    onsaved: (message: string) => void;
  }
  const { open, user, groups, onclose, onsaved }: Props = $props();

  let form = $state<UserForm>(newUserForm());
  let tried = $state(false);
  let touched = $state<Record<string, boolean>>({});
  let error = $state('');
  let busy = $state(false);
  let groupSearch = $state('');
  /** The user name follows the display name until typed by hand. */
  let usernameTyped = $state(false);

  const isNew = $derived(user === null);
  const isSelf = $derived(user !== null && user.id === session.me?.id);
  const minPassword = $derived(publicSettings.value.passwordMinLength);
  /** Invitations need e-mail links to work on this instance. */
  const canInvite = $derived(isNew && publicSettings.value.emailLinks);
  const inviting = $derived(canInvite && form.invite);
  const problems = $derived(userProblems({ ...form, invite: inviting }, isNew, minPassword));
  const shown = (f: 'username' | 'email' | 'password') => {
    const p = problems[f];
    return (tried || touched[f]) && p ? t.usersAdmin.problems[p](minPassword) : '';
  };
  const visibleGroups = $derived(
    (groups ?? []).filter((g) => form.groups.includes(g.id) || matches(groupSearch, g.name)),
  );

  $effect(() => {
    if (!open) return;
    form = user
      ? {
          username: user.username,
          name: user.name,
          email: user.email,
          password: '',
          admin: user.admin,
          active: user.active,
          groups: (user.groups ?? []).map((g) => g.id),
          invite: false,
        }
      : newUserForm();
    tried = false;
    touched = {};
    error = '';
    groupSearch = '';
    usernameTyped = false;
  });

  function nameTyped() {
    if (isNew && !usernameTyped) form.username = suggestUsername(form.name);
  }

  function toggleGroup(id: number, on: boolean) {
    form.groups = on ? [...form.groups, id] : form.groups.filter((g) => g !== id);
  }

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    tried = true;
    error = '';
    if (hasProblems(problems)) return;
    busy = true;
    try {
      if (!user) {
        await adminApi.createUser({
          username: form.username.trim().toLowerCase(),
          name: form.name.trim(),
          email: form.email.trim(),
          password: inviting ? '' : form.password,
          admin: form.admin,
          ...(inviting ? { invite: true } : {}),
          ...(groups ? { groups: form.groups } : {}),
        });
        const who = form.name.trim() || form.username.trim();
        onsaved(inviting ? t.usersAdmin.invited(who) : t.usersAdmin.created(who));
      } else {
        const patch: Parameters<typeof adminApi.updateUser>[1] = {};
        if (form.name.trim() !== user.name) patch.name = form.name.trim();
        if (form.email.trim() !== user.email) patch.email = form.email.trim();
        if (!isSelf && form.admin !== user.admin) patch.admin = form.admin;
        if (!isSelf && form.active !== user.active) patch.active = form.active;
        if (Object.keys(patch).length) await adminApi.updateUser(user.id, patch);
        const before = (user.groups ?? []).map((g) => g.id).sort();
        if (groups && before.join() !== [...form.groups].sort().join()) {
          await adminApi.setUserGroups(user.id, form.groups);
        }
        onsaved(t.usersAdmin.updated(form.name.trim() || user.username));
      }
      onclose();
    } catch (err) {
      error = errorMessage(err);
    } finally {
      busy = false;
    }
  }
</script>

<Modal
  {open}
  title={user ? t.usersAdmin.editTitle(user.name) : t.usersAdmin.newUser}
  onclose={() => !busy && onclose()}
>
  <form id="user-form" class="user-form" onsubmit={submit} novalidate>
    <div class="form-grid">
      <label class="field">
        <span>{t.usersAdmin.name}</span>
        <!-- svelte-ignore a11y_autofocus -->
        <input
          class="input"
          bind:value={form.name}
          oninput={nameTyped}
          autocomplete="off"
          autofocus
        />
      </label>
      <label class="field">
        <span>{t.usersAdmin.username}</span>
        <input
          class="input mono"
          bind:value={form.username}
          disabled={!isNew}
          autocomplete="off"
          spellcheck="false"
          aria-invalid={!!shown('username')}
          oninput={() => (usernameTyped = true)}
          onblur={() => (touched.username = true)}
        />
        {#if shown('username')}
          <span class="field-error">{shown('username')}</span>
        {:else if isNew}
          <span class="hint">{t.usersAdmin.usernameHint}</span>
        {/if}
      </label>
    </div>
    <label class="field">
      <span>{t.usersAdmin.email}</span>
      <input
        class="input"
        type="email"
        bind:value={form.email}
        autocomplete="off"
        aria-invalid={!!shown('email')}
        onblur={() => (touched.email = true)}
      />
      {#if shown('email')}<span class="field-error">{shown('email')}</span>{/if}
    </label>
    {#if canInvite}
      <div class="choices">
        <label class="check">
          <input class="checkbox" type="checkbox" bind:checked={form.invite} />
          <span>{t.usersAdmin.inviteLabel}</span>
        </label>
        {#if inviting}<p class="form-hint">{t.usersAdmin.inviteHint}</p>{/if}
      </div>
    {/if}
    {#if isNew && !inviting}
      <label class="field">
        <span>{t.usersAdmin.password}</span>
        <input
          class="input"
          type="password"
          bind:value={form.password}
          autocomplete="new-password"
          aria-invalid={!!shown('password')}
          onblur={() => (touched.password = true)}
        />
        {#if shown('password')}
          <span class="field-error">{shown('password')}</span>
        {:else}
          <span class="hint">{t.usersAdmin.passwordHint(minPassword)}</span>
        {/if}
      </label>
    {/if}

    <fieldset class="choices">
      <legend class="field-label">{t.usersAdmin.rights}</legend>
      <label class="check">
        <input class="checkbox" type="checkbox" bind:checked={form.admin} disabled={isSelf} />
        <span>{t.usersAdmin.adminLabel}</span>
      </label>
      {#if !isNew}
        <label class="check">
          <input class="checkbox" type="checkbox" bind:checked={form.active} disabled={isSelf} />
          <span>{t.usersAdmin.activeLabel}</span>
        </label>
      {/if}
      {#if isSelf}<p class="form-hint">{t.usersAdmin.selfHint}</p>{/if}
    </fieldset>

    {#if groups}
      <fieldset class="choices">
        <legend class="field-label">{t.usersAdmin.groups}</legend>
        {#if groups.length === 0}
          <p class="form-hint">{t.usersAdmin.noGroups}</p>
        {:else}
          {#if groups.length > 8}
            <input
              class="input"
              type="search"
              bind:value={groupSearch}
              placeholder={t.usersAdmin.findGroup}
              aria-label={t.usersAdmin.findGroup}
            />
          {/if}
          <div class="group-list">
            {#each visibleGroups as g (g.id)}
              <label class="check">
                <input
                  class="checkbox"
                  type="checkbox"
                  checked={form.groups.includes(g.id)}
                  onchange={(e) => toggleGroup(g.id, e.currentTarget.checked)}
                />
                <span>{g.name}</span>
              </label>
            {/each}
          </div>
        {/if}
      </fieldset>
    {/if}

    {#if error}<p class="error-text" role="alert">{error}</p>{/if}
  </form>
  {#snippet footer()}
    <button type="button" class="btn btn-quiet" disabled={busy} onclick={onclose}
      >{t.common.cancel}</button
    >
    <button class="btn btn-primary" type="submit" form="user-form" disabled={busy}
      >{busy ? t.common.saving : isNew ? t.usersAdmin.create : t.common.save}</button
    >
  {/snippet}
</Modal>

<style>
  .user-form {
    display: grid;
    gap: 14px;
  }

  .choices .form-hint {
    margin: 0;
  }

  .group-list {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
    gap: 6px 14px;
    max-height: 220px;
    overflow: auto;
  }
</style>
