<script lang="ts">
  import { groupProblems, hasProblems, namedPatch, namedPayload } from '../../core/admin-forms.js';
  import type { Group, GroupDetail } from '../../core/admin-types.js';
  import type { UserRef } from '../../core/types.js';
  import { t } from '../../i18n/t.js';
  import { adminApi } from '../../lib/admin-api.js';
  import { errorMessage } from '../../lib/api.js';
  import { notify } from '../../lib/ui.svelte.js';
  import Modal from '../Modal.svelte';
  import UserPicker from '../UserPicker.svelte';

  /**
   * Creating a group, or changing one: its name and description, and, once it
   * exists, its members (added and removed right away).
   */
  interface Props {
    open: boolean;
    /** The group to change; null to create one. */
    group: Group | null;
    onclose: () => void;
    onsaved: (message: string) => void;
    /** The members changed: the list's counts are out of date. */
    onchanged: () => void;
  }
  const { open, group, onclose, onsaved, onchanged }: Props = $props();

  let name = $state('');
  let description = $state('');
  let tried = $state(false);
  let touched = $state(false);
  let error = $state('');
  let busy = $state(false);
  let detail = $state<GroupDetail | null>(null);
  let seq = 0;

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
    const mine = ++seq;
    if (!group) return;
    adminApi
      .group(group.id)
      .then((d) => {
        if (mine === seq) detail = d;
      })
      .catch((e: unknown) => {
        if (mine === seq) error = errorMessage(e);
      });
  });

  /** People who are not members yet. */
  async function candidates(q: string): Promise<UserRef[]> {
    const members = new Set((detail?.members ?? []).map((m) => m.id));
    return (await adminApi.users(q)).filter((u) => !members.has(u.id));
  }

  async function add(user: UserRef | null) {
    if (!group || !user) return;
    error = '';
    try {
      detail = await adminApi.addMembers(group.id, [user.id]);
      notify(t.adminArea.groupsPage.memberAdded(user.name));
      onchanged();
    } catch (e) {
      error = errorMessage(e);
    }
  }

  async function remove(user: UserRef) {
    if (!group) return;
    error = '';
    try {
      await adminApi.removeMember(group.id, user.id);
      if (detail) detail = { ...detail, members: detail.members.filter((m) => m.id !== user.id) };
      notify(t.adminArea.groupsPage.memberRemoved(user.name));
      onchanged();
    } catch (e) {
      error = errorMessage(e);
    }
  }

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    tried = true;
    error = '';
    if (hasProblems(problems)) return;
    const patch = group ? namedPatch({ name, description }, group, true) : null;
    if (patch && Object.keys(patch).length === 0) {
      onclose();
      return;
    }
    busy = true;
    try {
      const saved = group
        ? await adminApi.updateGroup(group.id, patch ?? {})
        : await adminApi.createGroup(namedPayload({ name, description }));
      onsaved(
        group
          ? t.adminArea.groupsPage.updated(saved.name)
          : t.adminArea.groupsPage.created(saved.name),
      );
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
  title={group ? t.adminArea.groupsPage.editTitle(group.name) : t.adminArea.groupsPage.newGroup}
  onclose={() => !busy && onclose()}
>
  <form id="group-form" class="group-form" onsubmit={submit} novalidate>
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
                        <button type="button" class="link-btn" onclick={() => void remove(m)}
                          >{t.common.remove}</button
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
    {/if}

    {#if error}<p class="error-text" role="alert">{error}</p>{/if}
  </form>
  {#snippet footer()}
    <button type="button" class="btn btn-quiet" disabled={busy} onclick={onclose}
      >{t.common.cancel}</button
    >
    <button class="btn btn-primary" type="submit" form="group-form" disabled={busy}
      >{busy ? t.common.saving : group ? t.common.save : t.adminArea.groupsPage.create}</button
    >
  {/snippet}
</Modal>

<style>
  .group-form {
    display: grid;
    gap: 14px;
  }

  .choices .form-hint {
    margin: 0;
  }

  .short {
    min-height: 0;
    font: 14px/1.4 var(--font-body);
  }

  .member-list {
    max-height: 220px;
    overflow: auto;
  }
</style>
