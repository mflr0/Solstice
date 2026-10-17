<script lang="ts">
  import { hasProblems } from '../../core/admin-forms.js';
  import type { Category } from '../../core/admin-types.js';
  import {
    generalChanged,
    generalForm,
    generalPatch,
    generalProblems,
    toggleIssueType,
    typesInUse,
    type GeneralForm,
    type ProjectSettings,
  } from '../../core/project-admin.js';
  import type { UserRef } from '../../core/types.js';
  import { formatList } from '../../i18n/format.js';
  import { t } from '../../i18n/t.js';
  import { adminApi, projectAdminApi } from '../../lib/admin-api.js';
  import { api, ApiError, errorMessage } from '../../lib/api.js';
  import { session } from '../../lib/session.svelte.js';
  import { notify } from '../../lib/ui.svelte.js';
  import UserPicker from '../UserPicker.svelte';

  /**
   * The General tab: name, description, lead, category, who new issues go
   * to and the issue types the project offers. Changes wait for « Save »;
   * only the fields that changed are sent.
   */
  interface Props {
    projectKey: string;
    /** Tells the page whether there are unsaved changes (it asks before leaving). */
    ondirty: (dirty: boolean) => void;
  }
  const { projectKey, ondirty }: Props = $props();

  let settings = $state<ProjectSettings | null>(null);
  let form = $state<GeneralForm | null>(null);
  let categories = $state<Category[]>([]);
  let loadError = $state('');
  let error = $state('');
  let tried = $state(false);
  let touched = $state(false);
  let saving = $state(false);

  const allTypes = $derived(session.meta?.issueTypes ?? settings?.issueTypes ?? []);
  const problems = $derived(form ? generalProblems(form, allTypes) : {});
  const nameShown = $derived(
    (tried || touched) && problems.name ? t.projectsAdmin.problems[problems.name] : '',
  );
  const typesShown = $derived(
    tried && problems.types ? t.projectsAdmin.problems[problems.types] : '',
  );
  const dirty = $derived(settings && form ? generalChanged(settings, form) : false);

  $effect(() => {
    ondirty(dirty);
  });

  $effect(() => {
    const key = projectKey;
    projectAdminApi
      .settings(key)
      .then((s) => {
        settings = s;
        form = generalForm(s);
      })
      .catch((e: unknown) => (loadError = errorMessage(e)));
    // The choice of category comes from the server; the form works without it.
    adminApi
      .categories()
      .then((c) => (categories = c))
      .catch(() => (categories = []));
  });

  function discard() {
    if (!settings) return;
    form = generalForm(settings);
    tried = false;
    touched = false;
    error = '';
  }

  async function save() {
    if (!settings || !form) return;
    tried = true;
    error = '';
    if (hasProblems(problems)) return;
    saving = true;
    try {
      const next = await projectAdminApi.update(projectKey, generalPatch(settings, form));
      settings = next;
      form = generalForm(next);
      tried = false;
      touched = false;
      notify(t.settings.saved);
    } catch (e) {
      const used = e instanceof ApiError && e.status === 409 ? typesInUse(e.data) : [];
      error = used.length
        ? t.projectConfig.general.typesInUse(
            formatList(used.map((u) => t.projectConfig.general.typeUsage(u.name, u.issues))),
          )
        : errorMessage(e);
    } finally {
      saving = false;
    }
  }
</script>

{#if loadError}
  <p class="error-text" role="alert">{loadError}</p>
{:else if !settings || !form}
  <p class="muted loading">{t.common.loading}</p>
{:else}
  <h2 class="section-title">{t.projectConfig.general.title}</h2>
  <p class="lead-text">{t.projectConfig.general.intro}</p>

  <form
    class="settings-form"
    onsubmit={(e) => {
      e.preventDefault();
      void save();
    }}
    novalidate
  >
    <label class="field">
      <span>{t.projects.name}</span>
      <input
        class="input"
        bind:value={form.name}
        autocomplete="off"
        aria-invalid={!!nameShown}
        onblur={() => (touched = true)}
      />
      {#if nameShown}<span class="field-error">{nameShown}</span>{/if}
    </label>

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
          onpick={(u: UserRef | null) => form && (form.lead = u)}
        />
        <span class="hint">{t.projectConfig.general.leadHint}</span>
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
                onchange={(e) => {
                  if (form)
                    form.issueTypes = toggleIssueType(
                      form.issueTypes,
                      it.id,
                      e.currentTarget.checked,
                    );
                }}
              />
              <span>{it.name}</span>
            </label>
          {/each}
        </div>
        {#if typesShown}
          <span class="field-error">{typesShown}</span>
        {:else}
          <span class="hint">{t.projectsAdmin.typesHint}</span>
        {/if}
      </fieldset>
    {/if}

    {#if error}<p class="error-text" role="alert">{error}</p>{/if}
  </form>

  {#if dirty}
    <div class="savebar" role="region" aria-label={t.settings.unsaved}>
      <span>{t.settings.unsaved}</span>
      <button type="button" class="btn btn-quiet" disabled={saving} onclick={discard}
        >{t.settings.discard}</button
      >
      <button type="button" class="btn btn-primary" disabled={saving} onclick={() => void save()}
        >{saving ? t.common.saving : t.common.save}</button
      >
    </div>
  {/if}
{/if}

<style>
  .type-list {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(150px, 1fr));
    gap: 6px 14px;
  }

  .hint {
    font-size: 12.5px;
    color: var(--muted);
  }
</style>
