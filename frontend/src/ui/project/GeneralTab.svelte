<script lang="ts">
  import { untrack } from 'svelte';
  import { hasProblems } from '../../core/admin-forms.js';
  import type { Category } from '../../core/admin-types.js';
  import {
    generalForm,
    generalPatch,
    generalProblems,
    toggleIssueType,
    type GeneralForm,
    type GeneralProblem,
    type ProjectSettings,
  } from '../../core/project-admin.js';
  import { paths } from '../../core/route.js';
  import type { TypeRef, UserRef } from '../../core/types.js';
  import { t } from '../../i18n/t.js';
  import { adminApi, projectAdminApi } from '../../lib/admin-api.js';
  import { api, ApiError, errorMessage } from '../../lib/api.js';
  import { router } from '../../lib/router.svelte.js';
  import { session } from '../../lib/session.svelte.js';
  import { notify } from '../../lib/ui.svelte.js';
  import ConfirmDialog from '../ConfirmDialog.svelte';
  import TypedConfirmDialog from '../TypedConfirmDialog.svelte';
  import UserPicker from '../UserPicker.svelte';

  /**
   * The project's name, description, lead, category, default assignee and
   * issue types. The key is shown but cannot change. Archiving and deleting
   * sit below, for Solstice administrators, as on the Projects page.
   */
  interface Props {
    settings: ProjectSettings;
    /** A saved or archived project: the page keeps the newest copy. */
    onchange: (s: ProjectSettings) => void;
    ondirty: (dirty: boolean) => void;
  }
  const { settings, onchange, ondirty }: Props = $props();

  let form = $state<GeneralForm>(untrack(() => generalForm(settings)));
  let categories = $state<Category[]>([]);
  let tried = $state(false);
  let saving = $state(false);
  let error = $state('');
  /** Types the server refused to drop because issues still use them. */
  let inUse = $state<{ type: TypeRef; issues: number }[]>([]);
  let archiving = $state(false);
  let deleting = $state(false);

  const allTypes = $derived(session.meta?.issueTypes ?? []);
  const problems = $derived(generalProblems(form, allTypes));
  const patch = $derived(generalPatch(form, settings));
  const dirty = $derived(Object.keys(patch).length > 0);
  const isAdmin = $derived(session.me?.admin === true);

  $effect(() => ondirty(dirty));
  $effect(() => () => ondirty(false));

  $effect(() => {
    adminApi
      .categories()
      .then((c) => (categories = c))
      .catch(() => (categories = []));
  });

  function problemText(p: GeneralProblem | undefined): string {
    if (!tried || !p) return '';
    return p === 'leadRequired'
      ? t.projectConfig.general.leadRequired
      : t.projectsAdmin.problems[p];
  }

  function discard() {
    form = generalForm(settings);
    tried = false;
    error = '';
    inUse = [];
  }

  async function save(e: SubmitEvent) {
    e.preventDefault();
    tried = true;
    error = '';
    inUse = [];
    if (hasProblems(problems) || !dirty) return;
    saving = true;
    try {
      const next = await projectAdminApi.update(settings.key, patch);
      form = generalForm(next);
      tried = false;
      onchange(next);
      notify(t.settings.saved);
    } catch (err) {
      error = errorMessage(err);
      const used = err instanceof ApiError ? (err.data as { inUse?: typeof inUse } | null) : null;
      if (used?.inUse) inUse = used.inUse;
    } finally {
      saving = false;
    }
  }

  async function archive(on: boolean) {
    const next = await adminApi.archiveProject(settings.key, on);
    onchange(next);
    notify(
      on ? t.projectsAdmin.archived(settings.name) : t.projectsAdmin.unarchived(settings.name),
    );
  }

  async function remove() {
    await adminApi.deleteProject(settings.key, settings.key);
    notify(t.projectsAdmin.deleted(settings.name));
    router.navigate(paths.projects());
  }
</script>

<form class="settings-form" onsubmit={save} novalidate>
  <p class="lead-text">{t.projectConfig.general.intro}</p>

  <div class="form-grid wide-first">
    <label class="field">
      <span>{t.projects.name}</span>
      <input
        class="input"
        bind:value={form.name}
        autocomplete="off"
        aria-invalid={!!problemText(problems.name)}
      />
      {#if problemText(problems.name)}<span class="field-error">{problemText(problems.name)}</span
        >{/if}
    </label>
    <label class="field">
      <span>{t.projects.key}</span>
      <input class="input mono" value={settings.key} disabled aria-describedby="general-key-hint" />
    </label>
  </div>
  <p class="form-hint" id="general-key-hint">{t.projectConfig.general.keyHint}</p>

  <label class="field">
    <span>{t.projectsAdmin.description}</span>
    <textarea class="textarea" bind:value={form.description} rows="3"></textarea>
  </label>

  <div class="form-grid">
    <div class="field">
      <span>{t.projects.lead}</span>
      <UserPicker
        field
        value={form.lead}
        label={t.projects.lead}
        allowNone={false}
        search={(q) => api.users(q)}
        onpick={(u: UserRef | null) => (form.lead = u)}
      />
      {#if problemText(problems.lead)}<span class="field-error">{problemText(problems.lead)}</span
        >{/if}
    </div>
    <label class="field">
      <span>{t.projectsAdmin.category}</span>
      <select class="select" bind:value={form.category}>
        <option value={null}>{t.projectsAdmin.noCategory}</option>
        {#each categories as c (c.id)}<option value={c.id}>{c.name}</option>{/each}
      </select>
    </label>
  </div>

  <label class="field">
    <span>{t.projectsAdmin.assignee}</span>
    <select class="select" bind:value={form.defaultAssignee}>
      <option value="unassigned">{t.projectsAdmin.assignees.unassigned}</option>
      <option value="lead">{t.projectsAdmin.assignees.lead}</option>
    </select>
  </label>

  {#if allTypes.length > 0}
    <fieldset class="choices">
      <legend class="field-label">{t.projectsAdmin.types}</legend>
      <div class="type-list">
        {#each allTypes as it (it.id)}
          <label class="check">
            <input
              class="checkbox"
              type="checkbox"
              checked={form.issueTypes.includes(it.id)}
              onchange={(e) =>
                (form.issueTypes = toggleIssueType(
                  allTypes,
                  form.issueTypes,
                  it.id,
                  e.currentTarget.checked,
                ))}
            />
            <span>{it.name}</span>
          </label>
        {/each}
      </div>
      {#if problemText(problems.types)}
        <span class="field-error">{problemText(problems.types)}</span>
      {:else}
        <span class="form-hint">{t.projectsAdmin.typesHint}</span>
      {/if}
    </fieldset>
  {/if}

  {#if error}
    <div role="alert">
      <p class="error-text">{error}</p>
      {#if inUse.length > 0}
        <ul class="in-use">
          {#each inUse as u (u.type.id)}
            <li>{u.type.name} · {t.catalogAdmin.issues(u.issues)}</li>
          {/each}
        </ul>
      {/if}
    </div>
  {/if}

  {#if dirty}
    <div class="savebar" role="region" aria-label={t.settings.unsaved}>
      <span>{t.settings.unsaved}</span>
      <button type="button" class="btn btn-quiet" disabled={saving} onclick={discard}
        >{t.settings.discard}</button
      >
      <button class="btn btn-primary" type="submit" disabled={saving}
        >{saving ? t.common.saving : t.common.save}</button
      >
    </div>
  {/if}
</form>

<section class="danger" aria-labelledby="general-danger">
  <h2 class="section-title" id="general-danger">{t.projectConfig.general.archiveSection}</h2>
  <p class="form-hint">{t.projectConfig.general.archiveHint}</p>
  {#if settings.archived}
    <p class="archived-note">{t.projectConfig.general.archivedNote}</p>
  {/if}
  {#if isAdmin}
    <div class="row">
      {#if settings.archived}
        <button
          type="button"
          class="btn"
          onclick={() => void archive(false).catch((e) => notify(errorMessage(e), 'error'))}
          >{t.projectsAdmin.unarchive}</button
        >
      {:else}
        <button type="button" class="btn" onclick={() => (archiving = true)}
          >{t.projectsAdmin.archive}</button
        >
      {/if}
      <button type="button" class="btn btn-danger" onclick={() => (deleting = true)}
        >{t.projectConfig.general.deleteProject}</button
      >
    </div>
  {:else}
    <p class="form-hint">{t.projectConfig.general.adminOnly}</p>
  {/if}
</section>

<ConfirmDialog
  open={archiving}
  title={t.projectsAdmin.archiveTitle(settings.name)}
  confirm={t.projectsAdmin.archive}
  danger={false}
  onclose={() => (archiving = false)}
  onconfirm={() => archive(true)}
>
  <p>{t.projectsAdmin.archiveBody}</p>
</ConfirmDialog>

<TypedConfirmDialog
  open={deleting}
  title={t.projectsAdmin.deleteTitle(settings.name)}
  phrase={settings.key}
  label={t.projectsAdmin.typeToConfirm(settings.key)}
  confirm={t.projectConfig.general.deleteProject}
  onconfirm={remove}
  onclose={() => (deleting = false)}
>
  <p>{t.projectsAdmin.deleteBody(settings.issues)}</p>
</TypedConfirmDialog>

<style>
  .type-list {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(150px, 1fr));
    gap: 6px 14px;
  }

  .in-use {
    margin: 4px 0 0;
    padding-left: 18px;
    font-size: 13px;
  }

  .danger {
    display: grid;
    gap: 10px;
    max-width: 720px;
    margin-top: 32px;
  }

  .danger .section-title {
    margin-bottom: 0;
  }

  .danger .form-hint {
    margin: 0;
  }

  .archived-note {
    margin: 0;
    font-weight: 600;
  }
</style>
