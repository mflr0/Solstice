<script lang="ts">
  import type { Category } from '../../core/admin-types.js';
  import {
    filterProjects,
    parseCategoryFilter,
    parseShow,
    PROJECT_SHOWS,
    type AdminProjectRow,
    type CategoryFilter,
    type ProjectShow,
  } from '../../core/project-admin.js';
  import { paths } from '../../core/route.js';
  import { formatNumber } from '../../i18n/format.js';
  import { t } from '../../i18n/t.js';
  import { adminApi } from '../../lib/admin-api.js';
  import { errorMessage } from '../../lib/api.js';
  import { router } from '../../lib/router.svelte.js';
  import { notify } from '../../lib/ui.svelte.js';
  import ConfirmDialog from '../ConfirmDialog.svelte';
  import Dropdown from '../Dropdown.svelte';
  import EmptyState from '../EmptyState.svelte';
  import TypedConfirmDialog from '../TypedConfirmDialog.svelte';
  import Who from '../Who.svelte';
  import CategoryDialog from './CategoryDialog.svelte';
  import ProjectDialog from './ProjectDialog.svelte';
  import RenameKeyDialog from './RenameKeyDialog.svelte';

  /**
   * Every project, archived ones included: find, create, archive and delete
   * them, with a link to each one's settings, and the categories that group
   * them. Deleting wants the project's key typed; so does changing the key.
   */
  type AdminProject = AdminProjectRow;

  let projects = $state<AdminProject[] | null>(null);
  let categories = $state<Category[] | null>(null);
  let error = $state('');
  let q = $state(router.param('q') ?? '');
  let show = $state<ProjectShow>(parseShow(router.param('show')));
  let category = $state<CategoryFilter>(parseCategoryFilter(router.param('category')));
  let creating = $state(false);
  let archiving = $state<AdminProject | null>(null);
  let deleting = $state<AdminProject | null>(null);
  let renaming = $state<AdminProject | null>(null);
  let categoryDialog = $state<{ category: Category | null } | null>(null);
  let removingCategory = $state<Category | null>(null);
  let timer: ReturnType<typeof setTimeout> | undefined;

  const list = $derived(filterProjects(projects ?? [], q, show, category));
  const filtered = $derived(q.trim() !== '' || show !== 'all' || category !== 'all');

  async function load() {
    try {
      projects = await adminApi.projects();
      error = '';
    } catch (e) {
      error = errorMessage(e);
    }
  }

  async function loadCategories() {
    try {
      categories = await adminApi.categories();
    } catch (e) {
      notify(errorMessage(e), 'error');
      categories = categories ?? [];
    }
  }

  $effect(() => {
    void load();
    void loadCategories();
  });

  $effect(() => () => clearTimeout(timer));

  /** /admin/projects?new=1 (the overview's link) opens the creation dialog. */
  $effect(() => {
    if (router.param('new') === '1') creating = true;
  });

  /** The address keeps the search and filters, so a reload or a link finds them again. */
  function keepFilters() {
    router.setParams({
      q: q.trim() || null,
      show: show === 'all' ? null : show,
      category: category === 'all' ? null : String(category),
    });
  }

  function typed() {
    clearTimeout(timer);
    timer = setTimeout(keepFilters, 250);
  }

  function clear() {
    q = '';
    show = 'all';
    category = 'all';
    keepFilters();
  }

  function categoryChosen(value: string) {
    category = parseCategoryFilter(value);
    keepFilters();
  }

  function closeCreate() {
    creating = false;
    if (router.param('new')) router.setParams({ new: null });
  }

  function created(p: { key: string }, name: string) {
    notify(t.projectsAdmin.created(name));
    router.navigate(paths.project(p.key));
  }

  async function unarchive(p: AdminProject) {
    try {
      await adminApi.archiveProject(p.key, false);
      notify(t.projectsAdmin.unarchived(p.name));
      await load();
    } catch (e) {
      notify(errorMessage(e), 'error');
    }
  }

  async function archive() {
    const p = archiving;
    if (!p) return;
    await adminApi.archiveProject(p.key, true);
    notify(t.projectsAdmin.archived(p.name));
    await load();
  }

  function renamed(oldKey: string, newKey: string, name: string) {
    notify(t.projectsAdmin.keyChanged(name, oldKey, newKey));
    void load();
  }

  async function remove() {
    const p = deleting;
    if (!p) return;
    await adminApi.deleteProject(p.key, p.key);
    notify(t.projectsAdmin.deleted(p.name));
    await Promise.all([load(), loadCategories()]);
  }

  function categorySaved(c: Category, isNew: boolean) {
    notify(isNew ? t.projectsAdmin.categoryCreated(c.name) : t.projectsAdmin.categorySaved(c.name));
    void Promise.all([loadCategories(), load()]);
  }

  async function removeCategory() {
    const c = removingCategory;
    if (!c) return;
    await adminApi.deleteCategory(c.id);
    notify(t.projectsAdmin.categoryDeleted(c.name));
    if (category === c.id) {
      category = 'all';
      keepFilters();
    }
    await Promise.all([loadCategories(), load()]);
  }
</script>

<header class="page-head">
  <div>
    <p class="eyebrow">{t.adminArea.groups.projects}</p>
    <h1 class="page-title">
      {t.projectsAdmin.title}
      {#if projects}<span class="count muted">{formatNumber(projects.length)}</span>{/if}
    </h1>
  </div>
  <button class="btn btn-primary" onclick={() => (creating = true)}
    >{t.projectsAdmin.newProject}</button
  >
</header>
<p class="lead-text">{t.projectsAdmin.intro}</p>

{#if error}
  <p class="error-text" role="alert">{error}</p>
{:else if !projects}
  <p class="muted loading">{t.common.loading}</p>
{:else if projects.length === 0}
  <EmptyState message={t.projectsAdmin.empty} hint={t.projectsAdmin.emptyHint}>
    <button class="btn btn-primary" onclick={() => (creating = true)}
      >{t.projectsAdmin.newProject}</button
    >
  </EmptyState>
{:else}
  <div class="list-tools">
    <input
      class="input search"
      type="search"
      bind:value={q}
      oninput={typed}
      placeholder={t.projectsAdmin.search}
      aria-label={t.projectsAdmin.search}
    />
    <Dropdown
      label={t.projectsAdmin.showLabel}
      value={show}
      options={PROJECT_SHOWS.map((s) => ({ value: s, label: t.projectsAdmin.show[s] }))}
      onchange={(v) => {
        show = v as (typeof PROJECT_SHOWS)[number];
        keepFilters();
      }}
    />
    <Dropdown
      label={t.projectsAdmin.categoryFilter}
      value={String(category)}
      options={[
        { value: 'all', label: t.projectsAdmin.allCategories },
        { value: 'none', label: t.projectsAdmin.noCategory },
        ...(categories ?? []).map((c) => ({ value: String(c.id), label: c.name })),
      ]}
      onchange={categoryChosen}
    />
  </div>

  {#if list.length === 0}
    <EmptyState kind="search" message={t.adminArea.noMatch}>
      {#if filtered}<button class="btn" onclick={clear}>{t.adminArea.clearSearch}</button>{/if}
    </EmptyState>
  {:else}
    <div class="table-scroll">
      <table class="ledger projects">
        <thead>
          <tr>
            <th class="c-key">{t.projects.key}</th>
            <th>{t.projects.name}</th>
            <th class="c-lead">{t.projects.lead}</th>
            <th class="c-category">{t.projectsAdmin.category}</th>
            <th class="num c-issues">{t.projectsAdmin.issues}</th>
            <th><span class="sr-only">{t.common.actions}</span></th>
          </tr>
        </thead>
        <tbody>
          {#each list as p (p.id)}
            <tr class:is-off={p.archived}>
              <td class="c-key"><a class="key" href={paths.project(p.key)}>{p.key}</a></td>
              <td>
                <a class="name" href={paths.project(p.key)}>{p.name}</a>
                {#if p.archived}<span class="stamp">{t.projects.archived}</span>{/if}
                <span class="muted kind">{t.projects.kinds[p.kind] ?? p.kind}</span>
              </td>
              <td class="c-lead"><Who user={p.lead} empty="—" /></td>
              <td class="c-category">
                {#if p.category}{p.category.name}{:else}<span class="muted">—</span>{/if}
              </td>
              <td class="num c-issues" class:muted={p.issues === 0}>{formatNumber(p.issues)}</td>
              <td class="row-actions">
                <a class="link-btn" href={paths.projectSettings(p.key)}>{t.projects.settings}</a>
                {#if p.archived}
                  <button class="link-btn" onclick={() => void unarchive(p)}
                    >{t.projectsAdmin.unarchive}</button
                  >
                {:else}
                  <button class="link-btn" onclick={() => (archiving = p)}
                    >{t.projectsAdmin.archive}</button
                  >
                {/if}
                <button class="link-btn" onclick={() => (renaming = p)}
                  >{t.projectsAdmin.changeKey}</button
                >
                <button class="link-btn" onclick={() => (deleting = p)}>{t.common.delete}</button>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
{/if}

{#if categories}
  <section class="categories" aria-labelledby="categories-title">
    <div class="section-head">
      <h2 class="section-title" id="categories-title">{t.projectsAdmin.categoriesTitle}</h2>
      <button class="btn btn-small" onclick={() => (categoryDialog = { category: null })}
        >{t.projectsAdmin.newCategory}</button
      >
    </div>
    <p class="lead-text">{t.projectsAdmin.categoriesIntro}</p>
    {#if categories.length === 0}
      <p class="muted">{t.projectsAdmin.categoriesEmpty}</p>
    {:else}
      <div class="table-scroll">
        <table class="ledger">
          <thead>
            <tr>
              <th>{t.projectsAdmin.categoryName}</th>
              <th class="c-desc">{t.projectsAdmin.categoryDescription}</th>
              <th class="num">{t.projectsAdmin.title}</th>
              <th><span class="sr-only">{t.common.actions}</span></th>
            </tr>
          </thead>
          <tbody>
            {#each categories as c (c.id)}
              <tr>
                <td><strong>{c.name}</strong></td>
                <td class="c-desc muted">{c.description ?? '—'}</td>
                <td class="num" class:muted={c.projects === 0}>{formatNumber(c.projects)}</td>
                <td class="row-actions">
                  <button class="link-btn" onclick={() => (categoryDialog = { category: c })}
                    >{t.common.edit}</button
                  >
                  <button class="link-btn" onclick={() => (removingCategory = c)}
                    >{t.common.delete}</button
                  >
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}
  </section>
{/if}

<ProjectDialog open={creating} onclose={closeCreate} oncreated={created} />

<CategoryDialog
  open={categoryDialog !== null}
  category={categoryDialog?.category ?? null}
  categories={categories ?? []}
  onclose={() => (categoryDialog = null)}
  onsaved={categorySaved}
/>

<RenameKeyDialog
  open={renaming !== null}
  project={renaming}
  onrenamed={renamed}
  onclose={() => (renaming = null)}
/>

<ConfirmDialog
  open={archiving !== null}
  title={t.projectsAdmin.archiveTitle(archiving?.name ?? '')}
  confirm={t.projectsAdmin.archive}
  danger={false}
  onclose={() => (archiving = null)}
  onconfirm={archive}
>
  <p>{t.projectsAdmin.archiveBody}</p>
</ConfirmDialog>

<TypedConfirmDialog
  open={deleting !== null}
  title={t.projectsAdmin.deleteTitle(deleting?.name ?? '')}
  phrase={deleting?.key ?? ''}
  label={t.projectsAdmin.typeToConfirm(deleting?.key ?? '')}
  confirm={t.common.delete}
  onclose={() => (deleting = null)}
  onconfirm={remove}
>
  <p>{t.projectsAdmin.deleteBody(deleting?.issues ?? 0)}</p>
</TypedConfirmDialog>

<ConfirmDialog
  open={removingCategory !== null}
  title={t.projectsAdmin.deleteCategoryTitle(removingCategory?.name ?? '')}
  confirm={t.common.delete}
  onclose={() => (removingCategory = null)}
  onconfirm={removeCategory}
>
  <p>{t.projectsAdmin.deleteCategoryBody(removingCategory?.projects ?? 0)}</p>
</ConfirmDialog>

<style>
  .count {
    margin-left: 6px;
    font: 500 0.875rem/1 var(--font-mono);
  }

  .c-key {
    width: 5.625rem;
  }

  .name {
    font: var(--title-weight) 0.96875rem/1.3 var(--font-title);
    text-decoration: none;
  }

  .name:hover {
    text-decoration: underline;
  }

  .kind {
    margin-left: 8px;
    font-size: 0.75rem;
  }

  .stamp {
    margin-left: 8px;
  }

  .c-issues {
    white-space: nowrap;
  }

  .categories {
    margin-top: 36px;
  }

  .section-head {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: 12px;
    margin-bottom: 8px;
  }

  .section-head .section-title {
    margin: 0;
  }

  .c-desc {
    max-width: 50ch;
  }

  @media (max-width: 760px) {
    .c-lead,
    .c-category,
    .c-desc {
      display: none;
    }
  }

  @media (max-width: 600px) {
    .c-issues {
      display: none;
    }
  }
</style>
