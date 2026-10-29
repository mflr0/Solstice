<script lang="ts">
  import { securityLevelProblem } from '../../core/admin-forms.js';
  import { t } from '../../i18n/t.js';
  import { securityApi, type SecurityLevel } from '../../lib/admin-api.js';
  import { errorMessage } from '../../lib/api.js';
  import Modal from '../Modal.svelte';

  /** Creating a security level of a project, or changing one: its name and description. */
  interface Props {
    open: boolean;
    project: string;
    /** The level to change; null to create one. */
    level: SecurityLevel | null;
    /** Every level of the project, to spot a name already taken. */
    levels: SecurityLevel[];
    onclose: () => void;
    onsaved: (level: SecurityLevel, created: boolean) => void;
  }
  const { open, project, level, levels, onclose, onsaved }: Props = $props();

  let name = $state('');
  let description = $state('');
  let tried = $state(false);
  let touched = $state(false);
  let error = $state('');
  let busy = $state(false);

  const problem = $derived(securityLevelProblem(name, levels, level?.id ?? null));
  const shown = $derived((tried || touched) && problem ? t.securityAdmin.problems[problem] : '');

  $effect(() => {
    if (!open) return;
    name = level?.name ?? '';
    description = level?.description ?? '';
    tried = false;
    touched = false;
    error = '';
  });

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    tried = true;
    error = '';
    if (problem) return;
    const input = { name: name.trim(), description: description.trim() || null };
    busy = true;
    try {
      const saved = level
        ? await securityApi.update(project, level.id, input)
        : await securityApi.create(project, input);
      onsaved(saved, level === null);
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
  title={level ? t.securityAdmin.editTitle(level.name) : t.securityAdmin.newTitle}
  onclose={() => !busy && onclose()}
>
  <form id="security-level-form" class="level-form" onsubmit={submit} novalidate>
    <label class="field">
      <span>{t.securityAdmin.name}</span>
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
      <span>{t.securityAdmin.description}</span>
      <textarea class="textarea short" bind:value={description} rows="2"></textarea>
    </label>
    {#if error}<p class="error-text" role="alert">{error}</p>{/if}
  </form>
  {#snippet footer()}
    <button type="button" class="btn btn-quiet" disabled={busy} onclick={onclose}
      >{t.common.cancel}</button
    >
    <button class="btn btn-primary" type="submit" form="security-level-form" disabled={busy}
      >{busy ? t.common.saving : t.common.save}</button
    >
  {/snippet}
</Modal>

<style>
  .level-form {
    display: grid;
    gap: 14px;
  }

  .short {
    min-height: 0;
    font: 0.875rem/1.4 var(--font-body);
  }
</style>
