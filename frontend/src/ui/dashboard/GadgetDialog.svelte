<script lang="ts">
  import {
    COUNT_FIELDS,
    DEFAULT_SIZE,
    GADGET_KINDS,
    MAX_SIZE,
    onFilter,
    type CountField,
    type Gadget,
    type GadgetKind,
  } from '../../core/dashboard.js';
  import { DEFAULT_COLUMNS, type ColumnChoice } from '../../core/search-columns.js';
  import type { SavedFilter } from '../../core/types.js';
  import { t } from '../../i18n/t.js';
  import { api, errorMessage } from '../../lib/api.js';
  import Modal from '../Modal.svelte';
  import ColumnPicker from '../search/ColumnPicker.svelte';

  /**
   * Adding a gadget, or changing one: what it shows (a saved filter's
   * results or counts, or a built-in list), and how.
   */
  interface Props {
    open: boolean;
    /** The gadget to change; null to add one. */
    gadget: Gadget | null;
    choices: ColumnChoice[];
    onclose: () => void;
    /** Saves; a failure comes back as an error to show. */
    onsave: (g: Gadget) => Promise<void>;
  }
  const { open, gadget, choices, onclose, onsave }: Props = $props();

  let kind = $state<GadgetKind>('results');
  let filterId = $state<number | null>(null);
  let field = $state<CountField>('status');
  let size = $state(DEFAULT_SIZE);
  let columns = $state<string[]>([...DEFAULT_COLUMNS]);
  let filters = $state<SavedFilter[] | null>(null);
  let error = $state('');
  let busy = $state(false);

  /** An unavailable gadget's filter is not offered, nor known: kept as it is. */
  const keepsHidden = $derived(
    gadget?.unavailable === true && onFilter(gadget.kind) && onFilter(kind),
  );

  $effect(() => {
    if (!open) return;
    kind = gadget?.kind ?? 'results';
    filterId = gadget?.filter?.id ?? null;
    field = gadget?.config.field ?? 'status';
    size = gadget?.config.size || DEFAULT_SIZE;
    columns = gadget?.config.columns ? [...gadget.config.columns] : [...DEFAULT_COLUMNS];
    error = '';
    api.filters().then(
      (list) => {
        filters = list;
        if (filterId === null && !keepsHidden) filterId = list[0]?.id ?? null;
      },
      (e: unknown) => (error = errorMessage(e)),
    );
  });

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    const f = filters?.find((x) => x.id === filterId);
    if (onFilter(kind) && !f && !keepsHidden) return;
    busy = true;
    error = '';
    try {
      await onsave({
        id: gadget?.id ?? 0,
        kind,
        filter: onFilter(kind) && f ? { id: f.id, name: f.name, query: f.query } : null,
        unavailable: onFilter(kind) && !f,
        config:
          kind === 'count'
            ? { field }
            : { size: Math.min(MAX_SIZE, Math.max(1, Math.round(size) || DEFAULT_SIZE)), columns },
      });
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
  title={gadget ? t.home.editTitle : t.home.addTitle}
  onclose={() => !busy && onclose()}
>
  <form id="gadget-form" class="gadget-form" onsubmit={submit} novalidate>
    <label class="field">
      <span>{t.home.kindLabel}</span>
      <select class="select" bind:value={kind}>
        {#each GADGET_KINDS as k (k)}
          <option value={k}>{t.home.kinds[k]}</option>
        {/each}
      </select>
    </label>

    {#if onFilter(kind)}
      <label class="field">
        <span>{t.home.filter}</span>
        {#if filters === null}
          <span class="muted">{t.common.loading}</span>
        {:else if filters.length === 0 && !keepsHidden}
          <span class="hint">{t.home.noFilters}</span>
        {:else}
          <select class="select" bind:value={filterId}>
            {#if keepsHidden}
              <option value={null}>{t.home.unavailableTitle}</option>
            {/if}
            {#each filters as f (f.id)}
              <option value={f.id}>{f.name}</option>
            {/each}
          </select>
        {/if}
      </label>
    {/if}

    {#if kind === 'count'}
      <label class="field">
        <span>{t.home.field}</span>
        <select class="select" bind:value={field}>
          {#each COUNT_FIELDS as f (f)}
            <option value={f}>{t.search.fields[f]}</option>
          {/each}
        </select>
      </label>
    {:else}
      <label class="field">
        <span>{t.home.size}</span>
        <input class="input size" type="number" min="1" max={MAX_SIZE} bind:value={size} />
      </label>
      <div class="field">
        <span class="field-label">{t.home.columns}</span>
        <ColumnPicker {choices} chosen={columns} onchange={(next) => (columns = next)} />
      </div>
    {/if}

    {#if error}<p class="error-text" role="alert">{error}</p>{/if}
  </form>
  {#snippet footer()}
    <button type="button" class="btn btn-quiet" disabled={busy} onclick={onclose}
      >{t.common.cancel}</button
    >
    <button
      class="btn btn-primary"
      type="submit"
      form="gadget-form"
      disabled={busy || (onFilter(kind) && filterId === null && !keepsHidden)}
      >{busy ? t.common.saving : gadget ? t.common.save : t.common.add}</button
    >
  {/snippet}
</Modal>

<style>
  .gadget-form {
    display: grid;
    gap: 14px;
  }

  .size {
    width: calc(7 * var(--font-size));
  }
</style>
