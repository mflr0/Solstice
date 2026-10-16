<script lang="ts">
  import type { VersionRow } from '../../core/admin-types.js';
  import {
    nameProblem,
    versionPayload,
    versionProblems,
    type VersionForm,
  } from '../../core/project-admin.js';
  import { t } from '../../i18n/t.js';
  import { projectAdminApi } from '../../lib/admin-api.js';
  import { errorMessage } from '../../lib/api.js';
  import Modal from '../Modal.svelte';

  /**
   * Creating a version, or changing one: name, description, start and
   * release dates. Releasing has its own dialog, which can move issues.
   */
  interface Props {
    open: boolean;
    projectKey: string;
    /** The version to change; null to create one. */
    version: VersionRow | null;
    /** Every version of the project, to spot a name already taken. */
    versions: VersionRow[];
    onclose: () => void;
    onsaved: (version: VersionRow, created: boolean) => void;
  }
  const { open, projectKey, version, versions, onclose, onsaved }: Props = $props();

  const blank = (): VersionForm => ({ name: '', description: '', startDate: '', releaseDate: '' });

  let form = $state<VersionForm>(blank());
  let tried = $state(false);
  let touched = $state(false);
  let error = $state('');
  let busy = $state(false);

  const nameIssue = $derived(nameProblem(form.name, versions, version?.id ?? null));
  const days = $derived(versionProblems(form));
  const nameShown = $derived(
    (tried || touched) && nameIssue ? t.projectVersions.problems[nameIssue] : '',
  );
  const startShown = $derived(days.start ? t.projectVersions.problems[days.start] : '');
  const releaseShown = $derived(
    days.release
      ? t.projectVersions.problems[days.release]
      : days.dates
        ? t.projectVersions.problems[days.dates]
        : '',
  );

  $effect(() => {
    if (!open) return;
    form = version
      ? {
          name: version.name,
          description: version.description ?? '',
          startDate: version.startDate ?? '',
          releaseDate: version.releaseDate ?? '',
        }
      : blank();
    tried = false;
    touched = false;
    error = '';
  });

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    tried = true;
    error = '';
    if (nameIssue || Object.keys(days).length > 0) return;
    busy = true;
    try {
      const body = versionPayload(form);
      const saved = version
        ? await projectAdminApi.updateVersion(projectKey, version.id, body)
        : await projectAdminApi.createVersion(projectKey, body);
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
  title={version ? t.projectVersions.editTitle(version.name) : t.projectVersions.createTitle}
  onclose={() => !busy && onclose()}
>
  <form id="version-form" class="version-form" onsubmit={submit} novalidate>
    <label class="field">
      <span>{t.projects.name}</span>
      <!-- svelte-ignore a11y_autofocus -->
      <input
        class="input"
        bind:value={form.name}
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
    <div class="dates">
      <label class="field">
        <span>{t.projectVersions.startDate}</span>
        <input class="input" type="date" bind:value={form.startDate} aria-invalid={!!startShown} />
        {#if startShown}<span class="field-error">{startShown}</span>{/if}
      </label>
      <label class="field">
        <span>{t.projectVersions.releaseDate}</span>
        <input
          class="input"
          type="date"
          bind:value={form.releaseDate}
          aria-invalid={!!releaseShown}
        />
        {#if releaseShown}<span class="field-error">{releaseShown}</span>{/if}
      </label>
    </div>
    {#if error}<p class="error-text" role="alert">{error}</p>{/if}
  </form>
  {#snippet footer()}
    <button type="button" class="btn btn-quiet" disabled={busy} onclick={onclose}
      >{t.common.cancel}</button
    >
    <button class="btn btn-primary" type="submit" form="version-form" disabled={busy}
      >{busy ? t.common.saving : version ? t.common.save : t.projectVersions.create}</button
    >
  {/snippet}
</Modal>

<style>
  .version-form {
    display: grid;
    gap: 14px;
  }

  .dates {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 14px;
    align-items: start;
  }

  .field-error {
    font-size: 12.5px;
    color: var(--accent-text);
  }

  .input[aria-invalid='true'] {
    border-color: var(--accent);
  }

  @media (max-width: 600px) {
    .dates {
      grid-template-columns: minmax(0, 1fr);
    }
  }
</style>
