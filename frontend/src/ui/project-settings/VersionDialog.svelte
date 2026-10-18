<script lang="ts">
  import type { VersionRow } from '../../core/admin-types.js';
  import {
    versionForm,
    versionPayload,
    versionProblems,
    type VersionForm,
  } from '../../core/project-admin.js';
  import { hasProblems } from '../../core/admin-forms.js';
  import { t } from '../../i18n/t.js';
  import { projectAdminApi } from '../../lib/admin-api.js';
  import { errorMessage } from '../../lib/api.js';
  import Modal from '../Modal.svelte';

  /** Creating a version, or changing one: name, description, start and release dates. */
  interface Props {
    projectKey: string;
    open: boolean;
    /** The version to change; null to create one. */
    version: VersionRow | null;
    /** Every version of the project, to spot a name already taken. */
    versions: VersionRow[];
    onclose: () => void;
    onsaved: (version: VersionRow, created: boolean) => void;
  }
  const { projectKey, open, version, versions, onclose, onsaved }: Props = $props();

  let form = $state<VersionForm>(versionForm(null));
  let tried = $state(false);
  let touched = $state(false);
  let error = $state('');
  let busy = $state(false);

  const problems = $derived(versionProblems(form, versions, version?.id ?? null));
  const nameShown = $derived(
    (tried || touched) && problems.name ? t.projectConfig.problems[problems.name] : '',
  );
  const datesShown = $derived(problems.dates ? t.projectConfig.problems[problems.dates] : '');
  const formId = 'version-form';

  $effect(() => {
    if (!open) return;
    form = versionForm(version);
    tried = false;
    touched = false;
    error = '';
  });

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    tried = true;
    error = '';
    if (hasProblems(problems)) return;
    const payload = versionPayload(form, version);
    if (version && Object.keys(payload).length === 0) {
      onclose();
      return;
    }
    busy = true;
    try {
      const saved = version
        ? await projectAdminApi.updateVersion(projectKey, version.id, payload)
        : await projectAdminApi.createVersion(projectKey, payload);
      onsaved(saved, version === null);
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
  title={version
    ? t.projectConfig.versions.editTitle(version.name)
    : t.projectConfig.versions.newTitle}
  onclose={() => !busy && onclose()}
>
  <form id={formId} class="version-form" onsubmit={submit} novalidate>
    <label class="field">
      <span>{t.projects.name}</span>
      <!-- svelte-ignore a11y_autofocus -->
      <input
        class="input"
        bind:value={form.name}
        placeholder={t.projectConfig.versions.namePlaceholder}
        autocomplete="off"
        autofocus
        aria-invalid={!!nameShown}
        onblur={() => (touched = true)}
      />
      {#if nameShown}<span class="field-error">{nameShown}</span>{/if}
    </label>

    <label class="field">
      <span>{t.projectsAdmin.description}</span>
      <textarea class="textarea" bind:value={form.description} rows="2"></textarea>
    </label>

    <div class="form-grid">
      <label class="field">
        <span>{t.projectConfig.versions.startDate}</span>
        <input class="input" type="date" bind:value={form.startDate} aria-invalid={!!datesShown} />
      </label>
      <label class="field">
        <span>{t.projectConfig.versions.releaseDate}</span>
        <input
          class="input"
          type="date"
          bind:value={form.releaseDate}
          aria-invalid={!!datesShown}
        />
      </label>
    </div>
    {#if datesShown}<span class="field-error">{datesShown}</span>{/if}

    {#if error}<p class="error-text" role="alert">{error}</p>{/if}
  </form>
  {#snippet footer()}
    <button type="button" class="btn btn-quiet" disabled={busy} onclick={onclose}
      >{t.common.cancel}</button
    >
    <button class="btn btn-primary" type="submit" form={formId} disabled={busy}
      >{busy ? t.common.saving : version ? t.common.save : t.catalogAdmin.create}</button
    >
  {/snippet}
</Modal>

<style>
  .version-form {
    display: grid;
    gap: 14px;
  }
</style>
