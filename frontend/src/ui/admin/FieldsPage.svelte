<script lang="ts">
  import { matches } from '../../core/admin-forms.js';
  import type { CatalogItem, FieldRow, FieldTypeInfo } from '../../core/admin-types.js';
  import { optionsEditable } from '../../core/catalogs.js';
  import type { AdminProjectRow } from '../../core/project-admin.js';
  import { formatNumber } from '../../i18n/format.js';
  import { t } from '../../i18n/t.js';
  import { adminApi } from '../../lib/admin-api.js';
  import { errorMessage } from '../../lib/api.js';
  import { notify } from '../../lib/ui.svelte.js';
  import ConfirmDialog from '../ConfirmDialog.svelte';
  import EmptyState from '../EmptyState.svelte';
  import FieldDialog from './FieldDialog.svelte';
  import FieldOptionsDialog from './FieldOptionsDialog.svelte';

  /**
   * The custom fields: what each is, where it applies (every project and
   * issue type, or those its context lists), how many issues hold a value,
   * and their creation, change, options and removal. A field holding values
   * is deleted only once its name is typed.
   */

  /** Projects and issue types shown in a context cell before "+n". */
  const SHOWN = 3;

  let items = $state<FieldRow[] | null>(null);
  let error = $state('');
  let q = $state('');
  let dialog = $state<{ item: FieldRow | null } | null>(null);
  let optionsOf = $state<FieldRow | null>(null);
  let removing = $state<FieldRow | null>(null);
  let typed = $state('');
  let types = $state<FieldTypeInfo[] | null>(null);
  let projects = $state<AdminProjectRow[] | null>(null);
  let issueTypes = $state<CatalogItem[] | null>(null);

  const list = $derived(
    (items ?? []).filter((f) =>
      matches(q, f.name, f.description ?? '', f.type ? t.fieldsAdmin.types[f.type] : ''),
    ),
  );

  async function load() {
    try {
      items = await adminApi.fields();
      error = '';
    } catch (e) {
      error = errorMessage(e);
    }
  }

  $effect(() => {
    void load();
    // What the dialogs offer: a list that cannot be read leaves its picker empty.
    adminApi
      .fieldTypes()
      .then((x) => (types = x))
      .catch(() => {});
    adminApi
      .projects()
      .then((x) => (projects = x))
      .catch(() => {});
    adminApi
      .catalog('issue-types')
      .then((x) => (issueTypes = x))
      .catch(() => {});
  });

  function saved(name: string, created: boolean) {
    notify(created ? t.fieldsAdmin.created(name) : t.catalogAdmin.saved(name));
    void load();
  }

  function askRemove(f: FieldRow) {
    removing = f;
    typed = '';
  }

  async function remove() {
    const f = removing;
    if (!f) return;
    if (f.issues > 0 && typed.trim() !== f.name) throw new Error(t.fieldsAdmin.nameMismatch);
    await adminApi.deleteField(f.id, f.issues > 0 ? typed.trim() : undefined);
    notify(t.catalogAdmin.deleted(f.name));
    await load();
  }

  /** "KEY1, KEY2, KEY3 +2". */
  function some(names: string[]): string {
    const head = names.slice(0, SHOWN).join(', ');
    return names.length > SHOWN ? `${head} ${t.fieldsAdmin.more(names.length - SHOWN)}` : head;
  }

  const contextParts = (f: FieldRow) => [
    ...(f.context.projects.length > 0
      ? [t.fieldsAdmin.projectsOnly(some(f.context.projects.map((p) => p.key)))]
      : []),
    ...(f.context.issueTypes.length > 0
      ? [t.fieldsAdmin.typesOnly(some(f.context.issueTypes.map((x) => x.name)))]
      : []),
  ];
</script>

<header class="page-head">
  <div>
    <p class="eyebrow">{t.adminArea.groups.issues}</p>
    <h1 class="page-title">
      {t.adminArea.sections.fields}
      {#if items}<span class="count muted">{formatNumber(items.length)}</span>{/if}
    </h1>
  </div>
  <button class="btn btn-primary" onclick={() => (dialog = { item: null })}
    >{t.fieldsAdmin.newItem}</button
  >
</header>
<p class="lead-text">{t.fieldsAdmin.intro}</p>

{#if error}
  <p class="error-text" role="alert">{error}</p>
{:else if !items}
  <p class="muted loading">{t.common.loading}</p>
{:else if items.length === 0}
  <EmptyState message={t.fieldsAdmin.empty} hint={t.fieldsAdmin.emptyHint}>
    <button class="btn btn-primary" onclick={() => (dialog = { item: null })}
      >{t.fieldsAdmin.newItem}</button
    >
  </EmptyState>
{:else}
  <div class="list-tools">
    <input
      class="input search"
      type="search"
      bind:value={q}
      placeholder={t.fieldsAdmin.search}
      aria-label={t.fieldsAdmin.search}
    />
  </div>

  {#if list.length === 0}
    <EmptyState kind="search" message={t.adminArea.noMatch}>
      <button class="btn" onclick={() => (q = '')}>{t.adminArea.clearSearch}</button>
    </EmptyState>
  {:else}
    <div class="table-scroll">
      <table class="ledger catalog">
        <thead>
          <tr>
            <th>{t.catalogAdmin.name}</th>
            <th class="c-type">{t.fieldsAdmin.type}</th>
            <th class="c-context">{t.fieldsAdmin.appliesTo}</th>
            <th class="num c-usage">{t.fieldsAdmin.inUse}</th>
            <th><span class="sr-only">{t.common.actions}</span></th>
          </tr>
        </thead>
        <tbody>
          {#each list as f (f.id)}
            <tr>
              <td>
                <strong>{f.name}</strong>
                {#if f.description}<span class="muted small desc">{f.description}</span>{/if}
              </td>
              <td class="c-type">
                {f.type ? t.fieldsAdmin.types[f.type] : t.fieldsAdmin.otherType}
              </td>
              <td class="c-context">
                {#each contextParts(f) as part (part)}
                  <span class="part">{part}</span>
                {:else}
                  <span class="muted">{t.fieldsAdmin.everywhere}</span>
                {/each}
              </td>
              <td class="num c-usage" class:muted={f.issues === 0}>
                {f.issues === 0 ? t.catalogAdmin.unused : t.catalogAdmin.issues(f.issues)}
              </td>
              <td class="row-actions">
                <button class="link-btn" onclick={() => (dialog = { item: f })}
                  >{t.common.edit}</button
                >
                {#if optionsEditable(f)}
                  <button class="link-btn" onclick={() => (optionsOf = f)}
                    >{t.fieldsAdmin.optionsAction}</button
                  >
                {/if}
                <button class="link-btn" onclick={() => askRemove(f)}>{t.common.delete}</button>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
{/if}

{#if items}
  <FieldDialog
    open={dialog !== null}
    field={dialog?.item ?? null}
    fields={items}
    {types}
    {projects}
    {issueTypes}
    onclose={() => (dialog = null)}
    onsaved={saved}
  />
  <FieldOptionsDialog
    open={optionsOf !== null}
    field={optionsOf}
    onclose={() => (optionsOf = null)}
    onchanged={() => void load()}
  />
{/if}

<ConfirmDialog
  open={removing !== null}
  title={t.fieldsAdmin.deleteTitle(removing?.name ?? '')}
  confirm={t.common.delete}
  onclose={() => (removing = null)}
  onconfirm={remove}
>
  {#if removing && removing.issues > 0}
    <p>{t.fieldsAdmin.deleteUsed(removing.issues)}</p>
    <label class="field">
      <span>{t.fieldsAdmin.typeToConfirm(removing.name)}</span>
      <input class="input" bind:value={typed} autocomplete="off" spellcheck="false" />
    </label>
  {:else}
    <p>{t.fieldsAdmin.deleteUnused}</p>
  {/if}
</ConfirmDialog>

<style>
  .count {
    margin-left: 6px;
    font: 500 14px/1 var(--font-mono);
  }

  .desc {
    display: block;
    margin-top: 2px;
    max-width: 60ch;
  }

  .part {
    display: block;
  }

  .c-usage,
  .c-type {
    white-space: nowrap;
  }

  @media (max-width: 760px) {
    .c-type,
    .c-usage {
      display: none;
    }
  }
</style>
