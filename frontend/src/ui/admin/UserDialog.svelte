<script lang="ts">
  import {
    accessSentence,
    defaultsForm,
    defaultsPayload,
    hasProblems,
    matches,
    newUserForm,
    suggestUsername,
    userProblems,
    type AccessWords,
    type DefaultsForm,
    type UserForm,
  } from '../../core/admin-forms.js';
  import type { Group, PermissionRule, UserAccess, UserRow } from '../../core/admin-types.js';
  import type { AdminProjectRow } from '../../core/project-admin.js';
  import { holderLabels, permissionLabels, t } from '../../i18n/t.js';
  import { adminApi } from '../../lib/admin-api.js';
  import { errorMessage } from '../../lib/api.js';
  import { publicSettings } from '../../lib/public-settings.svelte.js';
  import { session } from '../../lib/session.svelte.js';
  import Highlight from '../Highlight.svelte';
  import Modal from '../Modal.svelte';
  import PermissionStates from './PermissionStates.svelte';
  import PasswordInput from '../PasswordInput.svelte';

  /**
   * Creating an account, or changing one: name, e-mail, administrator
   * rights, whether it may sign in, its groups, its permission overrides,
   * and what it may do in a project.
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
  let overrides = $state<DefaultsForm | null>(null);
  /** The overrides as loaded, as sent. */
  let savedOverrides = '';
  let projects = $state<AdminProjectRow[] | null>(null);
  let project = $state('');
  let access = $state<UserAccess[] | null>(null);
  let accessError = $state('');
  let seq = 0;
  let accessSeq = 0;

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
    overrides = null;
    projects = null;
    project = '';
    access = null;
    accessError = '';
    const mine = ++seq;
    if (!user) return;
    adminApi
      .userPermissions(user.id)
      .then((r) => {
        if (mine !== seq) return;
        overrides = defaultsForm(r);
        savedOverrides = JSON.stringify(defaultsPayload(overrides));
      })
      .catch((e: unknown) => {
        if (mine === seq) error = errorMessage(e);
      });
    adminApi
      .projects()
      .then((ps) => {
        if (mine === seq) projects = ps;
      })
      .catch((e: unknown) => {
        if (mine === seq) accessError = errorMessage(e);
      });
  });

  const accessWords = $derived<AccessWords>({
    ...t.usersAdmin.accessWhy,
    builtin: {
      everyone: holderLabels.everyone,
      lead: holderLabels.lead,
      reporter: holderLabels.reporter,
      assignee: holderLabels.assignee,
    },
  });

  async function loadAccess() {
    access = null;
    accessError = '';
    if (!user || !project) return;
    const mine = ++accessSeq;
    try {
      const a = await adminApi.userAccess(user.id, project);
      if (mine === accessSeq) access = a;
    } catch (e) {
      if (mine === accessSeq) accessError = errorMessage(e);
    }
  }

  /** The overrides to send, or null when they did not change (or did not load). */
  function changedOverrides(): PermissionRule[] | null {
    if (!overrides) return null;
    const rules = defaultsPayload(overrides);
    return JSON.stringify(rules) === savedOverrides ? null : rules;
  }

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
        const before = (user.groups ?? []).map((g) => g.id).sort();
        const rules = changedOverrides();
        let saved = false;
        try {
          if (Object.keys(patch).length) {
            await adminApi.updateUser(user.id, patch);
            saved = true;
          }
          if (groups && before.join() !== [...form.groups].sort().join()) {
            await adminApi.setUserGroups(user.id, form.groups);
            saved = true;
          }
          if (rules) {
            await adminApi.setUserPermissions(user.id, rules);
            savedOverrides = JSON.stringify(rules);
          }
        } catch (err) {
          if (saved) onsaved(t.usersAdmin.updated(form.name.trim() || user.username));
          throw err;
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
        <PasswordInput
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
                <span><Highlight text={g.name} search={groupSearch} /></span>
              </label>
            {/each}
          </div>
        {/if}
      </fieldset>
    {/if}

    {#if user}
      <fieldset class="choices">
        <legend class="field-label">{t.usersAdmin.overrides}</legend>
        <p class="form-hint">{t.usersAdmin.overridesHint}</p>
        {#if overrides}
          <PermissionStates bind:value={overrides} holder={user.name} prefix="user" />
        {:else if !error}
          <p class="muted loading">{t.common.loading}</p>
        {/if}
      </fieldset>

      <section class="access" aria-labelledby="access-title">
        <h3 class="field-label" id="access-title">{t.usersAdmin.access}</h3>
        <p class="form-hint">{t.usersAdmin.accessHint}</p>
        {#if projects}
          <label class="field">
            <span>{t.usersAdmin.accessProject}</span>
            <select class="select" bind:value={project} onchange={() => void loadAccess()}>
              <option value="">{t.usersAdmin.accessPick}</option>
              {#each projects as p (p.id)}
                <option value={p.key}>{p.name} ({p.key})</option>
              {/each}
            </select>
          </label>
        {:else if !accessError}
          <p class="muted loading">{t.common.loading}</p>
        {/if}
        {#if accessError}
          <p class="error-text" role="alert">{accessError}</p>
        {:else if project && !access}
          <p class="muted loading">{t.common.loading}</p>
        {:else if access}
          <ul class="access-list">
            {#each access as a (a.permission)}
              <li>
                <span class="perm">{permissionLabels[a.permission].label}</span>
                <span class="verdict" class:granted={a.granted}
                  ><span aria-hidden="true">{a.granted ? '✓' : '✗'}</span>
                  {a.granted ? t.usersAdmin.granted : t.usersAdmin.notGranted}</span
                >
                <span class="why">{accessSentence(a, accessWords)}</span>
              </li>
            {/each}
          </ul>
        {/if}
      </section>
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

  .choices .form-hint,
  .access .form-hint {
    margin: 0;
  }

  .access {
    display: grid;
    gap: 6px;
  }

  .access h3 {
    margin: 0;
  }

  .access-list {
    display: grid;
    gap: 6px;
    margin: 0;
    padding: 0;
    list-style: none;
  }

  .access-list li {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    gap: 0 12px;
  }

  .perm,
  .why {
    min-width: 0;
    overflow-wrap: anywhere;
  }

  .why {
    grid-column: 1 / -1;
    color: var(--muted);
    font-size: 0.8125rem;
  }

  /* The glyph and word carry the outcome; the color only repeats it. */
  .verdict {
    color: var(--tone-bug);
    font-size: 0.8125rem;
    font-weight: 700;
  }

  .verdict.granted {
    color: var(--stamp-done);
  }

  .group-list {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(11.25rem, 1fr));
    gap: 6px 14px;
    max-height: 13.75rem;
    overflow: auto;
  }
</style>
