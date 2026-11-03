<script lang="ts">
  import {
    defaultsForm,
    defaultsPayload,
    groupProblems,
    hasProblems,
    namedPatch,
    namedPayload,
    type DefaultsForm,
  } from '../../core/admin-forms.js';
  import type { BuiltinGroup, Group, GroupDetail, PermissionRule } from '../../core/admin-types.js';
  import { paths } from '../../core/route.js';
  import type { UserRef } from '../../core/types.js';
  import { holderLabels, t } from '../../i18n/t.js';
  import { adminApi } from '../../lib/admin-api.js';
  import { errorMessage } from '../../lib/api.js';
  import { notify } from '../../lib/ui.svelte.js';
  import Modal from '../Modal.svelte';
  import PermissionStates from './PermissionStates.svelte';
  import UserPicker from '../UserPicker.svelte';

  interface Props {
    open: boolean;
    group: Group | null;
    /** A built-in group: only its default permissions are shown. */
    builtin?: BuiltinGroup | null;
    onclose: () => void;
    onsaved: (message: string) => void;
    onchanged: () => void;
  }
  const { open, group, builtin = null, onclose, onsaved, onchanged }: Props = $props();

  let name = $state('');
  let description = $state('');
  let tried = $state(false);
  let touched = $state(false);
  let error = $state('');
  let busy = $state(false);
  let changing = $state(false);
  let detail = $state<GroupDetail | null>(null);
  let defaults = $state<DefaultsForm | null>(null);
  /** The defaults as loaded, as sent. */
  let savedDefaults = '';
  let seq = 0;

  const holderName = $derived(builtin ? holderLabels[builtin] : (group?.name ?? ''));

  const problems = $derived(groupProblems({ name, description }));
  const shownName = $derived(
    (tried || touched) && problems.name ? t.adminArea.groupsPage.problems[problems.name] : '',
  );
  const shownDescription = $derived(
    problems.description ? t.adminArea.groupsPage.problems[problems.description] : '',
  );

  $effect(() => {
    if (!open) return;
    name = group?.name ?? '';
    description = group?.description ?? '';
    tried = false;
    touched = false;
    error = '';
    detail = null;
    defaults = null;
    const mine = ++seq;
    const failed = (e: unknown) => {
      if (mine === seq) error = errorMessage(e);
    };
    if (group)
      adminApi
        .group(group.id)
        .then((d) => {
          if (mine === seq) detail = d;
        })
        .catch(failed);
    const rules = builtin
      ? adminApi.builtinPermissions(builtin)
      : group
        ? adminApi.groupPermissions(group.id)
        : null;
    rules
      ?.then((r) => {
        if (mine !== seq) return;
        defaults = defaultsForm(r);
        savedDefaults = JSON.stringify(defaultsPayload(defaults));
      })
      .catch(failed);
  });

  /** The defaults to send, or null when they did not change (or did not load). */
  function changedDefaults(): PermissionRule[] | null {
    if (!defaults) return null;
    const rules = defaultsPayload(defaults);
    return JSON.stringify(rules) === savedDefaults ? null : rules;
  }

  async function candidates(q: string): Promise<UserRef[]> {
    const members = new Set((detail?.members ?? []).map((m) => m.id));
    return (await adminApi.users(q)).filter((u) => !members.has(u.id));
  }

  async function add(user: UserRef | null) {
    if (!group || !user || changing) return;
    error = '';
    changing = true;
    try {
      detail = await adminApi.addMembers(group.id, [user.id]);
      notify(t.adminArea.groupsPage.memberAdded(user.name));
      onchanged();
    } catch (e) {
      error = errorMessage(e);
    } finally {
      changing = false;
    }
  }

  async function remove(user: UserRef) {
    if (!group || changing) return;
    error = '';
    changing = true;
    try {
      await adminApi.removeMember(group.id, user.id);
      if (detail) detail = { ...detail, members: detail.members.filter((m) => m.id !== user.id) };
      notify(t.adminArea.groupsPage.memberRemoved(user.name));
      onchanged();
    } catch (e) {
      error = errorMessage(e);
    } finally {
      changing = false;
    }
  }

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    tried = true;
    error = '';
    const rules = changedDefaults();
    if (builtin) {
      if (!rules) {
        onclose();
        return;
      }
      busy = true;
      try {
        await adminApi.setBuiltinPermissions(builtin, rules);
        onsaved(t.adminArea.groupsPage.updated(holderName));
        onclose();
      } catch (err) {
        error = errorMessage(err);
      } finally {
        busy = false;
      }
      return;
    }
    if (hasProblems(problems)) return;
    const patch = group ? namedPatch({ name, description }, group, true) : null;
    if (patch && Object.keys(patch).length === 0 && !rules) {
      onclose();
      return;
    }
    busy = true;
    let saved: Group | null = null;
    try {
      if (!group) saved = await adminApi.createGroup(namedPayload({ name, description }));
      else if (patch && Object.keys(patch).length > 0)
        saved = await adminApi.updateGroup(group.id, patch);
      if (group && rules) {
        await adminApi.setGroupPermissions(group.id, rules);
        savedDefaults = JSON.stringify(rules);
      }
      const shown = saved?.name ?? group?.name ?? '';
      onsaved(
        group ? t.adminArea.groupsPage.updated(shown) : t.adminArea.groupsPage.created(shown),
      );
      onclose();
    } catch (err) {
      error = errorMessage(err);
      // The name was saved, the defaults were not.
      if (saved) onchanged();
    } finally {
      busy = false;
    }
  }
</script>

<Modal
  {open}
  title={builtin || group
    ? t.adminArea.groupsPage.editTitle(holderName)
    : t.adminArea.groupsPage.newGroup}
  onclose={() => !busy && onclose()}
>
  <form id="group-form" class="group-form" onsubmit={submit} novalidate>
    {#if builtin}
      <p class="form-hint">{t.adminArea.groupsPage.builtinHint}</p>
    {:else}
      <label class="field">
        <span>{t.adminArea.groupsPage.name}</span>
        <!-- svelte-ignore a11y_autofocus -->
        <input
          class="input"
          bind:value={name}
          autocomplete="off"
          autofocus
          aria-invalid={!!shownName}
          onblur={() => (touched = true)}
        />
        <span class="form-hint">{t.adminArea.groupsPage.nameHint}</span>
        {#if shownName}<span class="field-error">{shownName}</span>{/if}
      </label>
      <label class="field">
        <span>{t.adminArea.groupsPage.description}</span>
        <textarea
          class="textarea short"
          bind:value={description}
          rows="2"
          aria-invalid={!!shownDescription}
        ></textarea>
        {#if shownDescription}<span class="field-error">{shownDescription}</span>{/if}
      </label>
    {/if}

    {#if builtin || group}
      <fieldset class="choices">
        <legend class="field-label">{t.adminArea.groupsPage.defaults}</legend>
        <p class="form-hint">{t.adminArea.groupsPage.defaultsHint}</p>
        {#if !defaults}
          {#if !error}<p class="muted loading">{t.common.loading}</p>{/if}
        {:else}
          <PermissionStates bind:value={defaults} holder={holderName} prefix="group" />
        {/if}
      </fieldset>
    {/if}

    {#if group}
      <fieldset class="choices">
        <legend class="field-label">{t.adminArea.groupsPage.members}</legend>
        {#if !detail}
          {#if !error}<p class="muted loading">{t.common.loading}</p>{/if}
        {:else}
          <UserPicker
            value={null}
            label={t.adminArea.groupsPage.addMember}
            noneLabel={t.adminArea.groupsPage.addMember}
            search={candidates}
            onpick={(u) => void add(u)}
            allowNone={false}
            disabled={changing}
            field
          />
          <p class="form-hint">{t.adminArea.groupsPage.membersHint}</p>
          {#if detail.members.length === 0}
            <p class="muted">{t.adminArea.groupsPage.noMembers}</p>
          {:else}
            <div class="table-scroll member-list">
              <table class="ledger">
                <tbody>
                  {#each detail.members as m (m.id)}
                    <tr class:is-off={!m.active}>
                      <td>
                        {m.name}
                        <span class="key muted">{m.username}</span>
                      </td>
                      <td class="row-actions">
                        <button
                          type="button"
                          class="link-btn"
                          disabled={changing}
                          aria-label={t.adminArea.groupsPage.removeMember(m.name)}
                          onclick={() => void remove(m)}>{t.common.remove}</button
                        >
                      </td>
                    </tr>
                  {/each}
                </tbody>
              </table>
            </div>
          {/if}
        {/if}
      </fieldset>

      {#if detail}
        <section class="where-block">
          <h3 class="field-label">{t.adminArea.groupsPage.projectMembers}</h3>
          <p class="form-hint">{t.adminArea.groupsPage.projectMembersHint}</p>
          {#if detail.projects.length === 0}
            <p class="muted">{t.adminArea.groupsPage.noProjectMembers}</p>
          {:else}
            <ul class="where">
              {#each detail.projects as p (p.project.id)}
                <li>
                  <a href="{paths.projectSettings(p.project.key)}?tab=rights"
                    >{p.project.name} <span class="key muted">{p.project.key}</span></a
                  >
                  {#if p.users.length > 0}
                    <p>
                      <span class="muted">{t.adminArea.groupsPage.projectPeople}</span>
                      {p.users.map((u) => u.name).join(', ')}
                    </p>
                  {/if}
                  {#if p.groups.length > 0}
                    <p>
                      <span class="muted">{t.adminArea.groupsPage.projectGroups}</span>
                      {p.groups.map((g) => g.name).join(', ')}
                    </p>
                  {/if}
                </li>
              {/each}
            </ul>
          {/if}
        </section>

        <section class="where-block">
          <h3 class="field-label">{t.adminArea.groupsPage.memberOf}</h3>
          {#if detail.memberOf.length === 0}
            <p class="muted">{t.adminArea.groupsPage.noMemberOf}</p>
          {:else}
            <ul class="where">
              {#each detail.memberOf as m (`${m.project.id}-${m.group.id}`)}
                <li>
                  {m.group.name}
                  <span class="muted">·</span>
                  <a href="{paths.projectSettings(m.project.key)}?tab=rights"
                    >{m.project.name} <span class="key muted">{m.project.key}</span></a
                  >
                </li>
              {/each}
            </ul>
          {/if}
        </section>
      {/if}
    {/if}

    {#if error}<p class="error-text" role="alert">{error}</p>{/if}
  </form>
  {#snippet footer()}
    <button type="button" class="btn btn-quiet" disabled={busy} onclick={onclose}
      >{t.common.cancel}</button
    >
    <button class="btn btn-primary" type="submit" form="group-form" disabled={busy}
      >{busy
        ? t.common.saving
        : group || builtin
          ? t.common.save
          : t.adminArea.groupsPage.create}</button
    >
  {/snippet}
</Modal>

<style>
  .group-form {
    display: grid;
    gap: 14px;
  }

  .field .form-hint,
  .choices .form-hint,
  .where-block .form-hint {
    margin: 0;
  }

  .short {
    min-height: 0;
    font: 0.875rem/1.4 var(--font-body);
  }

  .where-block {
    display: grid;
    gap: 6px;
  }

  .where-block h3 {
    margin: 0;
  }

  .where {
    display: grid;
    gap: 8px;
    margin: 0;
    padding: 0;
    list-style: none;
    max-height: 13.75rem;
    overflow: auto;
    overflow-wrap: anywhere;
  }

  .where p {
    margin: 2px 0 0;
  }

  .member-list {
    max-height: 13.75rem;
    overflow: auto;
  }
</style>
