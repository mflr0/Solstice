<script lang="ts">
  import type { Category } from '../../core/admin-types.js';
  import { categoryProblem } from '../../core/project-admin.js';
  import { t } from '../../i18n/t.js';
  import { adminApi } from '../../lib/admin-api.js';
  import { errorMessage } from '../../lib/api.js';
  import Modal from '../Modal.svelte';

  /** Creating a project category, or changing one: its name and description. */
  interface Props {
    open: boolean;
    /** The category to change; null to create one. */
    category: Category | null;
    /** Every category, to spot a name already taken. */
    categories: Category[];
    onclose: () => void;
    onsaved: (category: Category, created: boolean) => void;
  }
  const { open, category, categories, onclose, onsaved }: Props = $props();

  let name = $state('');
  let description = $state('');
  let tried = $state(false);
  let touched = $state(false);
  let error = $state('');
  let busy = $state(false);

  const problem = $derived(categoryProblem(name, categories, category?.id ?? null));
  const shown = $derived(
    (tried || touched) && problem ? t.projectsAdmin.categoryProblems[problem] : '',
  );

  $effect(() => {
    if (!open) return;
    name = category?.name ?? '';
    description = category?.description ?? '';
    tried = false;
    touched = false;
    error = '';
  });

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    tried = true;
    error = '';
    if (problem) return;
    const input = { name: name.trim(), description: description.trim() };
    busy = true;
    try {
      const saved = category
        ? await adminApi.updateCategory(category.id, input)
        : await adminApi.createCategory(input);
      onsaved(saved, category === null);
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
  title={category ? t.projectsAdmin.categoryTitle(category.name) : t.projectsAdmin.newCategory}
  onclose={() => !busy && onclose()}
>
  <form id="category-form" class="category-form" onsubmit={submit} novalidate>
    <label class="field">
      <span>{t.projectsAdmin.categoryName}</span>
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
      <span>{t.projectsAdmin.categoryDescription}</span>
      <textarea class="textarea short" bind:value={description} rows="2"></textarea>
    </label>
    {#if error}<p class="error-text" role="alert">{error}</p>{/if}
  </form>
  {#snippet footer()}
    <button type="button" class="btn btn-quiet" disabled={busy} onclick={onclose}
      >{t.common.cancel}</button
    >
    <button class="btn btn-primary" type="submit" form="category-form" disabled={busy}
      >{busy ? t.common.saving : t.common.save}</button
    >
  {/snippet}
</Modal>

<style>
  .category-form {
    display: grid;
    gap: 14px;
  }

  .short {
    min-height: 0;
    font: 0.875rem/1.4 var(--font-body);
  }
</style>
