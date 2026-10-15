<script lang="ts">
  import type { CatalogItem } from '../../core/admin-types.js';
  import { hasProblems } from '../../core/admin-forms.js';
  import {
    CATALOG_KINDS,
    catalogForm,
    catalogPayload,
    catalogProblems,
    normalizeColor,
    subtaskLocked,
    type CatalogForm,
    type CatalogSection,
  } from '../../core/catalogs.js';
  import { t } from '../../i18n/t.js';
  import { adminApi } from '../../lib/admin-api.js';
  import { errorMessage } from '../../lib/api.js';
  import Modal from '../Modal.svelte';
  import PriorityMark from '../PriorityMark.svelte';
  import TypeMark from '../TypeMark.svelte';

  /** Creating or changing one item of a catalogue, with the fields its kind has. */
  interface Props {
    section: CatalogSection;
    open: boolean;
    /** The item to change; null to create one. */
    item: CatalogItem | null;
    /** The whole catalogue, to spot names already taken. */
    items: CatalogItem[];
    onclose: () => void;
    onsaved: (item: CatalogItem, created: boolean) => void;
  }
  const { section, open, item, items, onclose, onsaved }: Props = $props();

  let form = $state<CatalogForm>(catalogForm(null));
  let tried = $state(false);
  let touched = $state<Record<string, boolean>>({});
  let error = $state('');
  let busy = $state(false);

  const m = $derived(t.catalogAdmin.kinds[section]);
  const problems = $derived(catalogProblems(section, form, items, item?.id ?? null));
  const shown = (f: 'name' | 'color' | 'outward' | 'inward') => {
    const p = problems[f];
    return (tried || touched[f]) && p ? t.catalogAdmin.problems[p] : '';
  };
  const color = $derived(normalizeColor(form.color));
  const locked = $derived(subtaskLocked(item));
  const formId = $derived(`catalog-form-${section}`);

  $effect(() => {
    if (!open) return;
    form = catalogForm(item);
    tried = false;
    touched = {};
    error = '';
  });

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    tried = true;
    error = '';
    if (hasProblems(problems)) return;
    const payload = catalogPayload(section, form, item);
    if (item && Object.keys(payload).length === 0) {
      onclose();
      return;
    }
    busy = true;
    try {
      const kind = CATALOG_KINDS[section];
      const saved = item
        ? await adminApi.updateCatalogItem(kind, item.id, payload)
        : await adminApi.createCatalogItem(kind, payload);
      onsaved(saved, item === null);
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
  title={item ? m.editTitle(item.name) : m.newItem}
  onclose={() => !busy && onclose()}
>
  <form id={formId} class="catalog-form" onsubmit={submit} novalidate>
    <label class="field">
      <span>{t.catalogAdmin.name}</span>
      <!-- svelte-ignore a11y_autofocus -->
      <input
        class="input"
        bind:value={form.name}
        placeholder={m.namePlaceholder}
        autocomplete="off"
        autofocus
        aria-invalid={!!shown('name')}
        onblur={() => (touched.name = true)}
      />
      {#if shown('name')}<span class="field-error">{shown('name')}</span>{/if}
    </label>

    {#if section === 'linkTypes'}
      <div class="form-grid">
        <label class="field">
          <span>{t.catalogAdmin.outward}</span>
          <input
            class="input"
            bind:value={form.outward}
            placeholder={t.catalogAdmin.outwardPlaceholder}
            autocomplete="off"
            aria-invalid={!!shown('outward')}
            onblur={() => (touched.outward = true)}
          />
          {#if shown('outward')}<span class="field-error">{shown('outward')}</span>{/if}
        </label>
        <label class="field">
          <span>{t.catalogAdmin.inward}</span>
          <input
            class="input"
            bind:value={form.inward}
            placeholder={t.catalogAdmin.inwardPlaceholder}
            autocomplete="off"
            aria-invalid={!!shown('inward')}
            onblur={() => (touched.inward = true)}
          />
          {#if shown('inward')}<span class="field-error">{shown('inward')}</span>{/if}
        </label>
      </div>
      <div class="preview" aria-live="polite">
        <span class="field-label">{t.catalogAdmin.preview}</span>
        <p>
          {t.catalogAdmin.linkPreview(
            'DEMO-1',
            form.outward.trim() || t.catalogAdmin.outwardPlaceholder,
            'DEMO-2',
          )}
        </p>
        <p>
          {t.catalogAdmin.linkPreview(
            'DEMO-2',
            form.inward.trim() || t.catalogAdmin.inwardPlaceholder,
            'DEMO-1',
          )}
        </p>
      </div>
    {:else}
      <label class="field">
        <span>{t.catalogAdmin.description}</span>
        <textarea class="textarea short" bind:value={form.description} rows="2"></textarea>
        <span class="hint">{t.catalogAdmin.descriptionHint}</span>
      </label>
    {/if}

    {#if section === 'issueTypes'}
      <label class="check">
        <input class="checkbox" type="checkbox" bind:checked={form.subtask} disabled={locked} />
        <span>{t.catalogAdmin.subtaskLabel}</span>
      </label>
      {#if locked}<p class="form-hint">{t.catalogAdmin.subtaskLocked}</p>{/if}
      <div class="preview">
        <span class="field-label">{t.catalogAdmin.preview}</span>
        <p>
          <TypeMark
            type={{ id: '', name: form.name.trim() || m.namePlaceholder, subtask: form.subtask }}
            withName
          />
        </p>
      </div>
    {/if}

    {#if section === 'priorities'}
      <div class="field">
        <span class="field-label" id="{formId}-color">{t.catalogAdmin.color}</span>
        <div class="color-row">
          <input
            class="swatch"
            type="color"
            value={color || '#888888'}
            aria-label={t.catalogAdmin.colorPicker}
            oninput={(e) => (form.color = e.currentTarget.value)}
          />
          <input
            class="input mono color-text"
            bind:value={form.color}
            placeholder="#d04437"
            spellcheck="false"
            aria-labelledby="{formId}-color"
            aria-invalid={!!shown('color')}
            onblur={() => (touched.color = true)}
          />
          {#if form.color}
            <button type="button" class="link-btn small" onclick={() => (form.color = '')}
              >{t.catalogAdmin.noColor}</button
            >
          {/if}
        </div>
        {#if shown('color')}
          <span class="field-error">{shown('color')}</span>
        {:else}
          <span class="hint">{t.catalogAdmin.colorHint}</span>
        {/if}
      </div>
      <div class="preview">
        <span class="field-label">{t.catalogAdmin.preview}</span>
        <p>
          <PriorityMark
            priority={{
              id: item ? String(item.id) : '',
              name: form.name.trim() || m.namePlaceholder,
              ...(color ? { color } : {}),
            }}
            withName
          />
        </p>
      </div>
      <label class="check">
        <input
          class="checkbox"
          type="checkbox"
          bind:checked={form.default}
          disabled={item?.default === true}
        />
        <span>{t.catalogAdmin.defaultLabel}</span>
      </label>
      <p class="form-hint">
        {item?.default ? t.catalogAdmin.defaultKeep : t.catalogAdmin.defaultHint}
      </p>
    {/if}

    {#if error}<p class="error-text" role="alert">{error}</p>{/if}
  </form>
  {#snippet footer()}
    <button type="button" class="btn btn-quiet" disabled={busy} onclick={onclose}
      >{t.common.cancel}</button
    >
    <button class="btn btn-primary" type="submit" form={formId} disabled={busy}
      >{busy ? t.common.saving : item ? t.common.save : t.catalogAdmin.create}</button
    >
  {/snippet}
</Modal>

<style>
  .catalog-form {
    display: grid;
    gap: 14px;
  }

  .textarea.short {
    min-height: 0;
    font-family: var(--font-body);
    font-size: 14px;
  }

  .catalog-form .form-hint {
    margin-top: -10px;
  }

  .preview {
    display: grid;
    gap: 4px;
    padding: 8px 12px;
    border: 1px dashed var(--rule-strong);
    border-radius: var(--radius-s);
    background: var(--paper);
  }

  .preview p {
    margin: 0;
  }

  .color-row {
    display: flex;
    gap: 8px;
    align-items: center;
    flex-wrap: wrap;
  }

  .swatch {
    width: 40px;
    height: 32px;
    padding: 2px;
    border: var(--frame) solid var(--rule-strong);
    border-radius: var(--radius-xs);
    background: var(--paper-raised);
    cursor: pointer;
  }

  .color-text {
    width: 12ch;
  }
</style>
