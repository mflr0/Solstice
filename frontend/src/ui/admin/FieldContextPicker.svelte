<script lang="ts">
  import { matches } from '../../core/admin-forms.js';
  import type { CatalogItem } from '../../core/admin-types.js';
  import type { FieldForm } from '../../core/catalogs.js';
  import { toggleIn } from '../../core/catalogs.js';
  import type { AdminProjectRow } from '../../core/project-admin.js';
  import { t } from '../../i18n/t.js';

  /**
   * Where a custom field applies: every project and issue type, or only
   * those ticked. Edits the context part of the form it is given.
   */
  interface Props {
    form: FieldForm;
    /** Null while loading, or when the list could not be read. */
    projects: AdminProjectRow[] | null;
    issueTypes: CatalogItem[] | null;
    projectsProblem?: string | undefined;
    typesProblem?: string | undefined;
    /** Makes the radio names unique when two pickers could coexist. */
    prefix?: string | undefined;
  }
  let {
    form = $bindable(),
    projects,
    issueTypes,
    projectsProblem = '',
    typesProblem = '',
    prefix = 'ctx',
  }: Props = $props();

  let search = $state('');
  const visible = $derived(
    (projects ?? []).filter((p) => form.projects.includes(p.key) || matches(search, p.key, p.name)),
  );
</script>

<div class="context">
  <p class="form-hint">{t.fieldsAdmin.contextHint}</p>

  <fieldset class="field choices">
    <legend class="field-label">{t.fieldsAdmin.projectsLabel}</legend>
    <div class="choices inline">
      <label class="check">
        <input
          type="radio"
          name="{prefix}-projects"
          checked={form.allProjects}
          onchange={() => (form.allProjects = true)}
        />
        <span>{t.fieldsAdmin.allProjects}</span>
      </label>
      <label class="check">
        <input
          type="radio"
          name="{prefix}-projects"
          checked={!form.allProjects}
          onchange={() => (form.allProjects = false)}
        />
        <span>{t.fieldsAdmin.onlyProjects}</span>
      </label>
    </div>
    {#if !form.allProjects}
      {#if projects && projects.length === 0}
        <p class="form-hint">{t.fieldsAdmin.noProjects}</p>
      {:else}
        {#if projects && projects.length > 8}
          <input
            class="input"
            type="search"
            bind:value={search}
            placeholder={t.fieldsAdmin.findProject}
            aria-label={t.fieldsAdmin.findProject}
          />
        {/if}
        <div class="pick-list">
          {#each visible as p (p.key)}
            <label class="check">
              <input
                class="checkbox"
                type="checkbox"
                checked={form.projects.includes(p.key)}
                onchange={(e) =>
                  (form.projects = toggleIn(form.projects, p.key, e.currentTarget.checked))}
              />
              <span><strong class="mono">{p.key}</strong> {p.name}</span>
            </label>
          {/each}
        </div>
      {/if}
      {#if projectsProblem}<span class="field-error">{projectsProblem}</span>{/if}
    {/if}
  </fieldset>

  <fieldset class="field choices">
    <legend class="field-label">{t.fieldsAdmin.typesLabel}</legend>
    <div class="choices inline">
      <label class="check">
        <input
          type="radio"
          name="{prefix}-types"
          checked={form.allTypes}
          onchange={() => (form.allTypes = true)}
        />
        <span>{t.fieldsAdmin.allTypes}</span>
      </label>
      <label class="check">
        <input
          type="radio"
          name="{prefix}-types"
          checked={!form.allTypes}
          onchange={() => (form.allTypes = false)}
        />
        <span>{t.fieldsAdmin.onlyTypes}</span>
      </label>
    </div>
    {#if !form.allTypes}
      <div class="pick-list">
        {#each issueTypes ?? [] as it (it.id)}
          <label class="check">
            <input
              class="checkbox"
              type="checkbox"
              checked={form.issueTypes.includes(String(it.id))}
              onchange={(e) =>
                (form.issueTypes = toggleIn(
                  form.issueTypes,
                  String(it.id),
                  e.currentTarget.checked,
                ))}
            />
            <span>{it.name}</span>
          </label>
        {/each}
      </div>
      {#if typesProblem}<span class="field-error">{typesProblem}</span>{/if}
    {/if}
  </fieldset>
</div>

<style>
  .context {
    display: grid;
    gap: 14px;
  }

  .context .form-hint {
    margin: 0;
  }

  .pick-list {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(190px, 1fr));
    gap: 6px 14px;
    max-height: 200px;
    overflow: auto;
  }
</style>
