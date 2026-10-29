<script lang="ts">
  import { hasProblems } from '../../core/admin-forms.js';
  import type { CatalogItem, FieldRow, FieldType, FieldTypeInfo } from '../../core/admin-types.js';
  import {
    FIELD_TYPE_GROUPS,
    fieldChanges,
    fieldContextPayload,
    fieldFormOf,
    fieldProblems,
    hasOptions,
    newFieldForm,
    parseOptions,
    type FieldForm,
  } from '../../core/catalogs.js';
  import type { AdminProjectRow } from '../../core/project-admin.js';
  import { t } from '../../i18n/t.js';
  import { adminApi } from '../../lib/admin-api.js';
  import { errorMessage } from '../../lib/api.js';
  import Dropdown from '../Dropdown.svelte';
  import Modal from '../Modal.svelte';
  import FieldContextPicker from './FieldContextPicker.svelte';

  /**
   * Creating a custom field (its type, first options and context), or
   * changing one: name, description and where it applies. The type and the
   * options of an existing field do not change here.
   */
  interface Props {
    open: boolean;
    /** The field to change; null to create one. */
    field: FieldRow | null;
    /** Every field, to spot names already taken. */
    fields: FieldRow[];
    /** The types the server creates; null while loading or unknown (all are offered). */
    types: FieldTypeInfo[] | null;
    projects: AdminProjectRow[] | null;
    issueTypes: CatalogItem[] | null;
    onclose: () => void;
    onsaved: (name: string, created: boolean) => void;
  }
  const { open, field, fields, types, projects, issueTypes, onclose, onsaved }: Props = $props();

  let form = $state<FieldForm>(newFieldForm());
  let tried = $state(false);
  let touched = $state<Record<string, boolean>>({});
  let error = $state('');
  let busy = $state(false);

  const problems = $derived(fieldProblems(form, fields, field?.id ?? null));
  const shown = (f: 'name' | 'options') => {
    const p = problems[f];
    return (tried || touched[f]) && p ? t.fieldsAdmin.problems[p] : '';
  };
  const groups = $derived(
    FIELD_TYPE_GROUPS.map((g) => ({
      ...g,
      types: g.types.filter((x) => !types || types.some((k) => k.type === x)),
    })).filter((g) => g.types.length > 0),
  );

  $effect(() => {
    if (!open) return;
    form = field ? fieldFormOf(field) : newFieldForm();
    tried = false;
    touched = {};
    error = '';
  });

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    tried = true;
    error = '';
    if (hasProblems(problems)) return;
    busy = true;
    try {
      if (field) {
        const changes = fieldChanges(field, form);
        if (Object.keys(changes).length === 0) {
          onclose();
          return;
        }
        await adminApi.updateField(field.id, changes);
      } else {
        await adminApi.createField({
          name: form.name.trim(),
          ...(form.description.trim() ? { description: form.description.trim() } : {}),
          type: form.type,
          ...(hasOptions(form.type) ? { options: parseOptions(form.options) } : {}),
          context: fieldContextPayload(form),
        });
      }
      onsaved(form.name.trim(), field === null);
      onclose();
    } catch (err) {
      error = errorMessage(err);
    } finally {
      busy = false;
    }
  }

  const typeName = (type: FieldType | null) =>
    type ? t.fieldsAdmin.types[type] : t.fieldsAdmin.otherType;
</script>

<Modal
  {open}
  title={field ? t.fieldsAdmin.editTitle(field.name) : t.fieldsAdmin.newTitle}
  onclose={() => !busy && onclose()}
>
  <form id="field-form" class="field-form" onsubmit={submit} novalidate>
    <label class="field">
      <span>{t.fieldsAdmin.nameLabel}</span>
      <!-- svelte-ignore a11y_autofocus -->
      <input
        class="input"
        bind:value={form.name}
        placeholder={t.fieldsAdmin.namePlaceholder}
        autocomplete="off"
        autofocus
        aria-invalid={!!shown('name')}
        onblur={() => (touched.name = true)}
      />
      {#if shown('name')}<span class="field-error">{shown('name')}</span>{/if}
    </label>

    <label class="field">
      <span>{t.fieldsAdmin.description}</span>
      <textarea class="textarea short" bind:value={form.description} rows="2"></textarea>
      <span class="hint">{t.fieldsAdmin.descriptionHint}</span>
    </label>

    {#if field}
      <div class="field">
        <span class="field-label">{t.fieldsAdmin.typeLabel}</span>
        <span>{typeName(field.type)}</span>
        <span class="hint">{t.fieldsAdmin.typeFixed}</span>
      </div>
    {:else}
      <div class="field">
        <span>{t.fieldsAdmin.typeLabel}</span>
        <Dropdown
          label={t.fieldsAdmin.typeLabel}
          value={form.type ?? ''}
          options={groups.flatMap((g) =>
            g.types.map((x) => ({
              value: x,
              label: t.fieldsAdmin.types[x],
              group: t.fieldsAdmin.groups[g.group],
            })),
          )}
          onchange={(v) => (form.type = v as FieldType)}
        />
      </div>
      {#if hasOptions(form.type)}
        <label class="field">
          <span>{t.fieldsAdmin.optionsLabel}</span>
          <textarea
            class="textarea short"
            bind:value={form.options}
            rows="4"
            aria-invalid={!!shown('options')}
            onblur={() => (touched.options = true)}
          ></textarea>
          {#if shown('options')}
            <span class="field-error">{shown('options')}</span>
          {:else}
            <span class="hint">{t.fieldsAdmin.optionsHint}</span>
          {/if}
        </label>
      {/if}
    {/if}

    <section class="where" aria-labelledby="field-context">
      <h3 class="sub-title" id="field-context">{t.fieldsAdmin.contextTitle}</h3>
      <FieldContextPicker
        bind:form
        {projects}
        {issueTypes}
        projectsProblem={tried && problems.projects
          ? t.fieldsAdmin.problems[problems.projects]
          : ''}
        typesProblem={tried && problems.issueTypes
          ? t.fieldsAdmin.problems[problems.issueTypes]
          : ''}
      />
    </section>

    {#if error}<p class="error-text" role="alert">{error}</p>{/if}
  </form>
  {#snippet footer()}
    <button type="button" class="btn btn-quiet" disabled={busy} onclick={onclose}
      >{t.common.cancel}</button
    >
    <button class="btn btn-primary" type="submit" form="field-form" disabled={busy}
      >{busy ? t.common.saving : field ? t.common.save : t.fieldsAdmin.create}</button
    >
  {/snippet}
</Modal>

<style>
  .field-form {
    display: grid;
    gap: 14px;
  }

  .textarea.short {
    min-height: 0;
    font-family: var(--font-body);
    font-size: 0.875rem;
  }

  .where {
    display: grid;
    gap: 8px;
    padding-top: 6px;
    border-top: var(--frame) solid var(--rule);
  }

  .where .sub-title {
    margin: 0;
  }
</style>
