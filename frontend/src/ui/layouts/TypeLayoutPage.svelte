<script lang="ts">
  import { tick } from 'svelte';
  import type {
    CatalogItem,
    FieldRow,
    IssueLayout,
    LayoutField,
    LayoutForm,
  } from '../../core/admin-types.js';
  import { paths } from '../../core/route.js';
  import { t } from '../../i18n/t.js';
  import { tabs } from '../../lib/actions.js';
  import { adminApi } from '../../lib/admin-api.js';
  import { errorMessage } from '../../lib/api.js';
  import { router } from '../../lib/router.svelte.js';
  import { notify } from '../../lib/ui.svelte.js';
  import ConfirmDialog from '../ConfirmDialog.svelte';
  import Modal from '../Modal.svelte';
  import TypeMark from '../TypeMark.svelte';
  import { systemFields, type LayoutChoice } from './layout-rows.js';
  import LayoutEditor from './LayoutEditor.svelte';

  /**
   * An issue type's layout (Admin › Issue types › Layout): its create form
   * and its issue view, one tab each, edited by LayoutEditor. « Reset to
   * default » deletes the tab's template.
   */
  interface Props {
    typeId: string;
    /** Tells the admin area whether there are unsaved changes (it asks before leaving). */
    ondirty: (dirty: boolean) => void;
  }
  const { typeId, ondirty }: Props = $props();

  const FORMS: LayoutForm[] = ['create', 'view'];

  let type = $state<CatalogItem | null>(null);
  let custom = $state<FieldRow[]>([]);
  let layouts = $state<Partial<Record<LayoutForm, IssueLayout>>>({});
  let loadError = $state('');
  let dirty = $state(false);
  let leaving = $state<(() => void) | null>(null);
  let resetting = $state(false);

  const tab = $derived<LayoutForm>(router.param('form') === 'view' ? 'view' : 'create');
  const layout = $derived(layouts[tab]);
  const fields = $derived(choices(tab));

  $effect(() => {
    ondirty(dirty);
    return () => ondirty(false);
  });

  // Reloading or closing the tab with unsaved changes asks first.
  $effect(() => {
    if (!dirty) return;
    const hold = (e: BeforeUnloadEvent) => e.preventDefault();
    window.addEventListener('beforeunload', hold);
    return () => window.removeEventListener('beforeunload', hold);
  });

  $effect(() => {
    const id = typeId;
    Promise.all([
      adminApi.catalog('issue-types'),
      adminApi.fields(),
      adminApi.typeLayout(id, 'create'),
      adminApi.typeLayout(id, 'view'),
    ])
      .then(([types, fields, create, view]) => {
        const found = types.find((x) => String(x.id) === id);
        if (!found) {
          loadError = t.layouts.notFound;
          return;
        }
        type = found;
        custom = fields;
        layouts = { create, view };
        document.title = `${t.layouts.title(found.name)} — ${t.adminArea.title} — ${t.appName}`;
      })
      .catch((e: unknown) => (loadError = errorMessage(e)));
  });

  /** The fields a form can place: the system ones, then the custom fields whose context can include the type. */
  function choices(form: LayoutForm): LayoutChoice[] {
    const system = systemFields(form).map((id) => ({
      id,
      name: t.layouts.fields[id as keyof typeof t.layouts.fields],
    }));
    const offered = custom
      .filter(
        (f) =>
          f.type !== null &&
          (f.context.issueTypes.length === 0 || f.context.issueTypes.some((x) => x.id === typeId)),
      )
      .map((f) => ({
        id: `customfield_${f.id}`,
        name: f.name,
        note:
          f.context.projects.length > 0
            ? t.layouts.someProjects(f.context.projects.length)
            : undefined,
      }));
    return [...system, ...offered];
  }

  function guard(go: () => void) {
    if (dirty) leaving = go;
    else go();
  }

  function showTab(next: LayoutForm) {
    if (next === tab) return;
    guard(() => router.setParams({ form: next === 'create' ? null : next }));
  }

  async function save(form: LayoutForm, fields: LayoutField[]) {
    layouts[form] = await adminApi.saveTypeLayout(typeId, form, fields);
    notify(t.layouts.saved);
  }

  async function reset() {
    const form = tab;
    await adminApi.resetTypeLayout(typeId, form);
    layouts[form] = await adminApi.typeLayout(typeId, form);
    resetting = false;
    notify(t.layouts.resetDone);
    await tick();
    document.getElementById(`layout-tab-${form}`)?.focus();
  }
</script>

<header class="page-head">
  <div>
    <p class="eyebrow">
      <a href={paths.admin('issueTypes')}>← {t.adminArea.sections.issueTypes}</a>
    </p>
    <h1 class="page-title">
      {#if type}
        <span aria-hidden="true"
          ><TypeMark
            type={{
              id: String(type.id),
              name: type.name,
              subtask: !!type.subtask,
              ...(type.color ? { color: type.color } : {}),
              ...(type.fill ? { fill: type.fill } : {}),
              ...(type.icon ? { icon: type.icon } : {}),
            }}
            asStored
          /></span
        >
        {t.layouts.title(type.name)}
      {:else}
        {t.layouts.action}
      {/if}
    </h1>
  </div>
</header>
<p class="lead-text">{t.layouts.intro}</p>

{#if loadError}
  <p class="error-text" role="alert">{loadError}</p>
  <p><a class="btn" href={paths.admin('issueTypes')}>{t.layouts.backToList}</a></p>
{:else if !type || !layout}
  <p class="muted loading">{t.common.loading}</p>
{:else}
  <div class="tab-bar">
    <div
      class="tabs"
      role="tablist"
      aria-label={t.layouts.tabsLabel}
      use:tabs={(el) => showTab(el.dataset['tab'] as LayoutForm)}
    >
      {#each FORMS as f (f)}
        <button
          type="button"
          class="tab"
          role="tab"
          id="layout-tab-{f}"
          data-tab={f}
          aria-controls="layout-panel-{f}"
          aria-selected={tab === f}
          onclick={() => showTab(f)}>{t.layouts.tabs[f]}</button
        >
      {/each}
    </div>
    {#if layout.source !== 'default'}
      <span class="reset">
        <span class="stamp">{t.layouts.customised}</span>
        <button type="button" class="btn btn-small" onclick={() => (resetting = true)}
          >{t.layouts.reset}</button
        >
      </span>
    {/if}
  </div>
  <div class="panel" role="tabpanel" id="layout-panel-{tab}" aria-labelledby="layout-tab-{tab}">
    <p class="muted source">{tab === 'create' ? t.layouts.createIntro : t.layouts.viewIntro}</p>
    {#key tab}
      <LayoutEditor
        form={tab}
        {fields}
        template={layout.fields}
        onsave={(fields) => save(tab, fields)}
        ondirty={(d) => (dirty = d)}
      />
    {/key}
  </div>
{/if}

<ConfirmDialog
  open={resetting}
  title={t.layouts.resetTitle(t.layouts.tabs[tab])}
  confirm={t.layouts.reset}
  onclose={() => (resetting = false)}
  onconfirm={reset}
>
  <p>{t.layouts.resetBody}</p>
</ConfirmDialog>

<Modal open={leaving !== null} title={t.workflows.leaveTitle} onclose={() => (leaving = null)}>
  <p>{t.layouts.leaveBody}</p>
  {#snippet footer()}
    <button type="button" class="btn btn-quiet" onclick={() => (leaving = null)}
      >{t.workflows.stay}</button
    >
    <button
      type="button"
      class="btn btn-danger"
      onclick={() => {
        const go = leaving;
        leaving = null;
        dirty = false;
        go?.();
      }}>{t.workflows.leave}</button
    >
  {/snippet}
</Modal>

<style>
  .page-title {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px;
  }

  .tab-bar {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    justify-content: space-between;
    gap: 6px 12px;
    border-bottom: 1px solid var(--rule-strong);
  }

  .tab-bar .tabs {
    border-bottom: none;
  }

  .reset {
    display: inline-flex;
    align-items: center;
    gap: 10px;
    padding-bottom: 4px;
  }

  .panel {
    padding-top: 14px;
  }

  .source {
    margin: 0 0 6px;
    max-width: 70ch;
  }
</style>
