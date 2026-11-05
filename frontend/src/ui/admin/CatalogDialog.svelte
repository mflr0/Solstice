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
  import { typeLetter } from '../../core/marks.js';
  import type { TypeFill, TypeIcon } from '../../core/types.js';
  import { t } from '../../i18n/t.js';
  import { adminApi } from '../../lib/admin-api.js';
  import { errorMessage } from '../../lib/api.js';
  import Modal from '../Modal.svelte';
  import PriorityMark from '../PriorityMark.svelte';
  import TypeMark from '../TypeMark.svelte';
  import { TYPE_ICONS } from '../type-icons.js';

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

  const FILLS: (TypeFill | '')[] = ['', 'filled', 'subtle', 'outline'];
  const ICONS: (TypeIcon | '')[] = ['', ...(Object.keys(TYPE_ICONS) as TypeIcon[])];
  let iconGrid = $state<HTMLElement>();

  /** Arrow keys move the choice through the icon grid, as in a radio group. */
  function iconKey(e: KeyboardEvent, i: number) {
    const cols = iconGrid
      ? getComputedStyle(iconGrid).gridTemplateColumns.split(' ').length
      : ICONS.length;
    const to = {
      ArrowLeft: i - 1,
      ArrowRight: i + 1,
      ArrowUp: i - cols,
      ArrowDown: i + cols,
      Home: 0,
      End: ICONS.length - 1,
    }[e.key];
    if (to === undefined) return;
    e.preventDefault();
    const next = Math.max(0, Math.min(ICONS.length - 1, to));
    form.icon = ICONS[next] ?? '';
    iconGrid?.querySelectorAll<HTMLElement>('[role="radio"]')[next]?.focus();
  }

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

{#snippet colorField()}
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
{/snippet}

<Modal {open} title={item ? m.editTitle(item.name) : m.newItem} onclose={() => !busy && onclose()}>
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
      {@render colorField()}
      <fieldset class="field">
        <legend class="field-label">{t.catalogAdmin.fill}</legend>
        <div class="fill-row">
          {#each FILLS as f (f)}
            <label class="check">
              <input type="radio" name="{formId}-fill" value={f} bind:group={form.fill} />
              <span>{f ? t.catalogAdmin.fills[f] : t.catalogAdmin.fillDefault}</span>
            </label>
          {/each}
        </div>
      </fieldset>
      <div class="field">
        <span class="field-label" id="{formId}-icon">{t.catalogAdmin.icon}</span>
        <div
          class="icon-grid"
          role="radiogroup"
          aria-labelledby="{formId}-icon"
          bind:this={iconGrid}
        >
          {#each ICONS as ic, i (ic)}
            <button
              type="button"
              class="icon-choice"
              role="radio"
              aria-checked={form.icon === ic}
              aria-label={ic ? t.catalogAdmin.icons[ic] : t.catalogAdmin.iconLetter}
              title={ic ? t.catalogAdmin.icons[ic] : t.catalogAdmin.iconLetter}
              tabindex={form.icon === ic ? 0 : -1}
              onclick={() => (form.icon = ic)}
              onkeydown={(e) => iconKey(e, i)}
            >
              {#if ic}<svg viewBox="0 0 16 16" aria-hidden="true"
                  >{#each TYPE_ICONS[ic] as path, j (j)}<path d={path.d} />{/each}</svg
                >{:else}<span aria-hidden="true"
                  >{typeLetter(form.name.trim() || m.namePlaceholder)}</span
                >{/if}
            </button>
          {/each}
        </div>
      </div>
      <div class="preview">
        <span class="field-label">{t.catalogAdmin.preview}</span>
        <p>
          <TypeMark
            type={{
              id: item ? String(item.id) : '',
              name: form.name.trim() || m.namePlaceholder,
              subtask: form.subtask,
              ...(color ? { color } : {}),
              ...(form.fill ? { fill: form.fill } : {}),
              ...(form.icon ? { icon: form.icon } : {}),
            }}
            withName
            asStored
          />
        </p>
      </div>
    {/if}

    {#if section === 'priorities'}
      {@render colorField()}
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
            asStored
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

    {#if section === 'resolutions'}
      <label class="check">
        <input class="checkbox" type="checkbox" bind:checked={form.default} />
        <span>{t.catalogAdmin.kinds.resolutions.defaultLabel}</span>
      </label>
      <p class="form-hint">{t.catalogAdmin.kinds.resolutions.defaultHint}</p>
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
    font-size: 0.875rem;
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

  fieldset.field {
    margin: 0;
    padding: 0;
    border: 0;
    min-width: 0;
  }

  .fill-row {
    display: flex;
    gap: 6px 16px;
    flex-wrap: wrap;
  }

  .icon-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, 2.25rem);
    gap: 4px;
  }

  .icon-choice {
    display: grid;
    place-items: center;
    width: 2.25rem;
    height: 2.25rem;
    padding: 0;
    border: var(--frame) solid var(--rule);
    border-radius: var(--radius-xs);
    background: var(--paper-raised);
    color: var(--ink);
    font: 600 0.8125rem/1 var(--font-mono);
    cursor: pointer;
  }

  .icon-choice[aria-checked='true'] {
    border-color: var(--accent);
    box-shadow: inset 0 0 0 1px var(--accent);
  }

  .icon-choice svg {
    width: 1rem;
    height: 1rem;
    fill: none;
    stroke: currentColor;
    stroke-width: 1.6;
    stroke-linecap: round;
    stroke-linejoin: round;
  }
</style>
