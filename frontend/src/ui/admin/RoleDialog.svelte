<script lang="ts">
  import { hasProblems, namedPatch, namedPayload, roleProblems } from '../../core/admin-forms.js';
  import type { RoleRow } from '../../core/admin-types.js';
  import { t } from '../../i18n/t.js';
  import { adminApi } from '../../lib/admin-api.js';
  import { errorMessage } from '../../lib/api.js';
  import Modal from '../Modal.svelte';

  /** Creating a project role, or changing one: its name and description. */
  interface Props {
    open: boolean;
    /** The role to change; null to create one. */
    role: RoleRow | null;
    onclose: () => void;
    onsaved: (message: string) => void;
  }
  const { open, role, onclose, onsaved }: Props = $props();

  let name = $state('');
  let description = $state('');
  let tried = $state(false);
  let touched = $state(false);
  let error = $state('');
  let busy = $state(false);

  const problems = $derived(roleProblems({ name, description }));
  const shown = $derived(
    (tried || touched) && problems.name ? t.adminArea.rolesPage.problems[problems.name] : '',
  );

  $effect(() => {
    if (!open) return;
    name = role?.name ?? '';
    description = role?.description ?? '';
    tried = false;
    touched = false;
    error = '';
  });

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    tried = true;
    error = '';
    if (hasProblems(problems)) return;
    const patch = role ? namedPatch({ name, description }, role) : null;
    if (patch && Object.keys(patch).length === 0) {
      onclose();
      return;
    }
    busy = true;
    try {
      const saved = role
        ? await adminApi.updateRole(role.id, patch ?? {})
        : await adminApi.createRole(namedPayload({ name, description }));
      onsaved(
        role
          ? t.adminArea.rolesPage.updated(saved.name)
          : t.adminArea.rolesPage.created(saved.name),
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
  title={role ? t.adminArea.rolesPage.editTitle(role.name) : t.adminArea.rolesPage.newRole}
  onclose={() => !busy && onclose()}
>
  <form id="role-form" class="role-form" onsubmit={submit} novalidate>
    <label class="field">
      <span>{t.adminArea.rolesPage.name}</span>
      <!-- svelte-ignore a11y_autofocus -->
      <input
        class="input"
        bind:value={name}
        autocomplete="off"
        autofocus
        aria-invalid={!!shown}
        onblur={() => (touched = true)}
      />
      {#if shown}<span class="field-error">{shown}</span>{/if}
    </label>
    <label class="field">
      <span>{t.adminArea.rolesPage.description}</span>
      <textarea class="textarea short" bind:value={description} rows="2"></textarea>
    </label>
    {#if error}<p class="error-text" role="alert">{error}</p>{/if}
  </form>
  {#snippet footer()}
    <button type="button" class="btn btn-quiet" disabled={busy} onclick={onclose}
      >{t.common.cancel}</button
    >
    <button class="btn btn-primary" type="submit" form="role-form" disabled={busy}
      >{busy ? t.common.saving : role ? t.common.save : t.adminArea.rolesPage.create}</button
    >
  {/snippet}
</Modal>

<style>
  .role-form {
    display: grid;
    gap: 14px;
  }

  .short {
    min-height: 0;
    font: 14px/1.4 var(--font-body);
  }
</style>
