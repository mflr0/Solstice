<script lang="ts">
  import type { StatusRow } from '../../core/admin-types.js';
  import { hasProblems } from '../../core/admin-forms.js';
  import { statusPayload, statusProblems } from '../../core/catalogs.js';
  import { STATUS_CATEGORIES } from '../../core/types.js';
  import { t } from '../../i18n/t.js';
  import { adminApi } from '../../lib/admin-api.js';
  import { errorMessage } from '../../lib/api.js';
  import Modal from '../Modal.svelte';

  /** Creating a status, or renaming one and changing its category. */
  interface Props {
    open: boolean;
    /** The status to change; null to create one. */
    status: StatusRow | null;
    /** Every status, to spot names already taken. */
    statuses: StatusRow[];
    onclose: () => void;
    onsaved: (name: string, created: boolean) => void;
  }
  const { open, status, statuses, onclose, onsaved }: Props = $props();

  let name = $state('');
  let category = $state<StatusRow['category']>('todo');
  let tried = $state(false);
  let touched = $state(false);
  let error = $state('');
  let busy = $state(false);

  const problems = $derived(statusProblems(name, statuses, status?.id ?? null));
  const shown = $derived(
    (tried || touched) && problems.name ? t.statusesAdmin.problems[problems.name] : '',
  );

  $effect(() => {
    if (!open) return;
    name = status?.name ?? '';
    category = status?.category ?? 'todo';
    tried = false;
    touched = false;
    error = '';
  });

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    tried = true;
    error = '';
    if (hasProblems(problems)) return;
    const changes = status ? statusPayload(status, name, category) : null;
    if (changes && Object.keys(changes).length === 0) {
      onclose();
      return;
    }
    busy = true;
    try {
      if (status && changes) await adminApi.updateStatus(status.id, changes);
      else await adminApi.createStatus({ name: name.trim(), category });
      onsaved(name.trim(), status === null);
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
  title={status ? t.statusesAdmin.editTitle(status.name) : t.statusesAdmin.newItem}
  onclose={() => !busy && onclose()}
>
  <form id="status-form" class="status-form" onsubmit={submit} novalidate>
    <label class="field">
      <span>{t.catalogAdmin.name}</span>
      <!-- svelte-ignore a11y_autofocus -->
      <input
        class="input"
        bind:value={name}
        placeholder={t.statusesAdmin.namePlaceholder}
        autocomplete="off"
        autofocus
        aria-invalid={!!shown}
        onblur={() => (touched = true)}
      />
      {#if shown}<span class="field-error">{shown}</span>{/if}
    </label>

    <fieldset class="field choices">
      <legend class="field-label">{t.statusesAdmin.category}</legend>
      <div class="choices inline">
        {#each STATUS_CATEGORIES as c (c)}
          <label class="check">
            <input type="radio" name="status-category" value={c} bind:group={category} />
            <span class="stamp {c}" data-category={c}>{t.categories[c]}</span>
          </label>
        {/each}
      </div>
      <span class="hint">{t.statusesAdmin.categoryHint}</span>
    </fieldset>

    {#if error}<p class="error-text" role="alert">{error}</p>{/if}
  </form>
  {#snippet footer()}
    <button type="button" class="btn btn-quiet" disabled={busy} onclick={onclose}
      >{t.common.cancel}</button
    >
    <button class="btn btn-primary" type="submit" form="status-form" disabled={busy}
      >{busy ? t.common.saving : status ? t.common.save : t.catalogAdmin.create}</button
    >
  {/snippet}
</Modal>

<style>
  .status-form {
    display: grid;
    gap: 14px;
  }
</style>
