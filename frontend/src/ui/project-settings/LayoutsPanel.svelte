<script lang="ts">
  import { onDestroy, tick } from 'svelte';
  import type { IssueLayout, LayoutField, LayoutForm } from '../../core/admin-types.js';
  import type { ProjectSettings } from '../../core/project-admin.js';
  import type { OfferedField } from '../../core/types.js';
  import { t } from '../../i18n/t.js';
  import { tabs } from '../../lib/actions.js';
  import { projectAdminApi } from '../../lib/admin-api.js';
  import { api, errorMessage } from '../../lib/api.js';
  import { notify } from '../../lib/ui.svelte.js';
  import ConfirmDialog from '../ConfirmDialog.svelte';
  import Dropdown from '../Dropdown.svelte';
  import { systemFields, type LayoutChoice } from '../layouts/layout-rows.js';
  import LayoutEditor from '../layouts/LayoutEditor.svelte';

  /**
   * The Issue layouts tab: for each of the project's types, whether its
   * forms follow the type's layout or this project's own. « Customise »
   * starts from what the form inherits; « Reset » deletes the override.
   */
  interface Props {
    projectKey: string;
    /** Runs a switch, after asking when there are unsaved changes. */
    guard: (go: () => void) => void;
    /** Tells the page whether there are unsaved changes (it asks before leaving). */
    ondirty: (dirty: boolean) => void;
  }
  const { projectKey, guard, ondirty }: Props = $props();

  const FORMS: LayoutForm[] = ['create', 'view'];

  let types = $state<ProjectSettings['issueTypes'] | null>(null);
  let typeId = $state('');
  let form = $state<LayoutForm>('create');
  let offered = $state<OfferedField[]>([]);
  let layouts = $state<Partial<Record<LayoutForm, IssueLayout>>>({});
  /** The forms « Customise » opened that have no override yet. */
  let editing = $state<Partial<Record<LayoutForm, boolean>>>({});
  let loadError = $state('');
  let dirty = $state(false);
  let resetting = $state(false);

  const layout = $derived(layouts[form]);
  const custom = $derived(layout?.source === 'project');
  const fields = $derived(choices(form));

  $effect(() => {
    ondirty(dirty);
  });
  onDestroy(() => ondirty(false));

  // Reloading or closing the tab with unsaved changes asks first.
  $effect(() => {
    if (!dirty) return;
    const hold = (e: BeforeUnloadEvent) => e.preventDefault();
    window.addEventListener('beforeunload', hold);
    return () => window.removeEventListener('beforeunload', hold);
  });

  $effect(() => {
    // The settings, not the project: a global administrator may not browse it.
    projectAdminApi
      .settings(projectKey)
      .then((d) => {
        types = d.issueTypes;
        typeId = d.issueTypes[0]?.id ?? '';
      })
      .catch((e: unknown) => (loadError = errorMessage(e)));
  });

  $effect(() => {
    const key = projectKey;
    const id = typeId;
    if (!id) return;
    layouts = {};
    editing = {};
    Promise.all([
      api.projectFields(key, id),
      projectAdminApi.layout(key, id, 'create'),
      projectAdminApi.layout(key, id, 'view'),
    ])
      .then(([f, create, view]) => {
        if (id !== typeId) return;
        offered = f;
        layouts = { create, view };
      })
      .catch((e: unknown) => (loadError = errorMessage(e)));
  });

  /** The fields a form can place: the system ones, then the custom fields the project offers for the type. */
  function choices(f: LayoutForm): LayoutChoice[] {
    const system = systemFields(f).map((id) => ({
      id,
      name: t.layouts.fields[id as keyof typeof t.layouts.fields],
    }));
    return [...system, ...offered.map((o) => ({ id: `customfield_${o.id}`, name: o.name }))];
  }

  function pickType(next: string) {
    if (next === typeId) return;
    guard(() => {
      dirty = false;
      typeId = next;
    });
  }

  function showForm(next: LayoutForm) {
    if (next === form) return;
    guard(() => {
      dirty = false;
      form = next;
    });
  }

  function cancel() {
    const f = form;
    guard(() => {
      dirty = false;
      editing[f] = false;
    });
  }

  async function save(f: LayoutForm, rows: LayoutField[]) {
    layouts[f] = await projectAdminApi.saveLayout(projectKey, typeId, f, rows);
    notify(t.layouts.saved);
  }

  async function reset() {
    const f = form;
    await projectAdminApi.resetLayout(projectKey, typeId, f);
    layouts[f] = await projectAdminApi.layout(projectKey, typeId, f);
    editing[f] = false;
    dirty = false;
    resetting = false;
    notify(t.projectConfig.layouts.resetDone);
    await tick();
    document.getElementById(`plo-tab-${f}`)?.focus();
  }
</script>

{#if loadError}
  <p class="error-text" role="alert">{loadError}</p>
{:else if !types}
  <p class="muted loading">{t.common.loading}</p>
{:else}
  <h2 class="section-title">{t.projectConfig.layouts.title}</h2>
  <p class="lead-text">{t.projectConfig.layouts.intro}</p>

  {#if types.length === 0}
    <p class="muted">{t.projectConfig.layouts.noTypes}</p>
  {:else}
    <div class="field type">
      <span>{t.projectConfig.layouts.type}</span>
      <Dropdown
        label={t.projectConfig.layouts.type}
        value={typeId}
        options={types.map((x) => ({ value: x.id, label: x.name }))}
        onchange={pickType}
      />
    </div>

    <div
      class="tabs"
      role="tablist"
      aria-label={t.layouts.tabsLabel}
      use:tabs={(el) => showForm(el.dataset['tab'] as LayoutForm)}
    >
      {#each FORMS as f (f)}
        <button
          type="button"
          class="tab"
          role="tab"
          id="plo-tab-{f}"
          data-tab={f}
          aria-controls="plo-panel-{f}"
          aria-selected={form === f}
          onclick={() => showForm(f)}>{t.layouts.tabs[f]}</button
        >
      {/each}
    </div>
    <div class="form-panel" role="tabpanel" id="plo-panel-{form}" aria-labelledby="plo-tab-{form}">
      {#if !layout}
        <p class="muted loading">{t.common.loading}</p>
      {:else}
        <div class="status">
          {#if custom}
            <span class="stamp">{t.projectConfig.layouts.customised}</span>
            <button type="button" class="btn btn-small" onclick={() => (resetting = true)}
              >{t.projectConfig.layouts.reset}</button
            >
          {:else}
            <span>
              {t.projectConfig.layouts.follows}
              <span class="muted small">{t.projectConfig.layouts.followsHint}</span>
            </span>
            {#if editing[form]}
              <button type="button" class="btn btn-small btn-quiet" onclick={cancel}
                >{t.common.cancel}</button
              >
            {:else}
              <button type="button" class="btn btn-small" onclick={() => (editing[form] = true)}
                >{t.projectConfig.layouts.customise}</button
              >
            {/if}
          {/if}
        </div>
        {#if custom || editing[form]}
          <p class="muted source">
            {form === 'create' ? t.layouts.createIntro : t.layouts.viewIntro}
          </p>
          {#key `${typeId}:${form}`}
            <LayoutEditor
              {form}
              {fields}
              template={layout.fields}
              onsave={(rows) => save(form, rows)}
              ondirty={(d) => (dirty = d)}
            />
          {/key}
        {/if}
      {/if}
    </div>
  {/if}
{/if}

<ConfirmDialog
  open={resetting}
  title={t.projectConfig.layouts.resetTitle(t.layouts.tabs[form])}
  confirm={t.projectConfig.layouts.reset}
  onclose={() => (resetting = false)}
  onconfirm={reset}
>
  <p>{t.projectConfig.layouts.resetBody}</p>
</ConfirmDialog>

<style>
  .type {
    max-width: 20rem;
    margin-bottom: 14px;
  }

  .form-panel {
    padding-top: 14px;
  }

  .status {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 6px 12px;
    margin-bottom: 10px;
  }

  .status .small {
    display: block;
  }

  .source {
    margin: 0 0 6px;
    max-width: 70ch;
  }
</style>
