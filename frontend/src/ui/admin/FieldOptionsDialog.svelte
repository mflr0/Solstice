<script lang="ts">
  import type { FieldOption, FieldRow } from '../../core/admin-types.js';
  import {
    arrowAfterMove,
    moveItem,
    optionProblem,
    optionTargets,
    typeKind,
  } from '../../core/catalogs.js';
  import { t } from '../../i18n/t.js';
  import { adminApi } from '../../lib/admin-api.js';
  import { errorMessage } from '../../lib/api.js';
  import { notify } from '../../lib/ui.svelte.js';
  import Dropdown from '../Dropdown.svelte';
  import Modal from '../Modal.svelte';
  import OrderArrows from './OrderArrows.svelte';

  /**
   * The options of a choice field: add, rename, reorder, disable and delete.
   * Each action is saved at once. An option some issues have chosen is
   * deleted only once their value is moved to another option or cleared.
   */
  interface Props {
    open: boolean;
    field: FieldRow | null;
    onclose: () => void;
    /** Called after any change, so the list can refresh its counts. */
    onchanged: () => void;
  }
  const { open, field, onclose, onchanged }: Props = $props();

  let detail = $state<FieldRow | null>(null);
  let loadError = $state('');
  let error = $state('');
  let busy = $state(false);
  let adding = $state('');
  let renaming = $state<{ id: number; value: string } | null>(null);
  let removing = $state<FieldOption | null>(null);
  let how = $state<'move' | 'clear'>('move');
  let moveTo = $state('');

  const options = $derived(detail && Array.isArray(detail.options) ? detail.options : []);
  const cascading = $derived(field !== null && typeKind(field.typeKey) === 'cascadingselect');
  const addProblem = $derived(adding === '' ? null : optionProblem(adding, options, null));
  const renameProblem = $derived(
    renaming ? optionProblem(renaming.value, options, renaming.id) : null,
  );
  const targets = $derived(removing ? optionTargets(options, removing) : []);

  $effect(() => {
    if (!open || !field) return;
    detail = null;
    loadError = '';
    error = '';
    adding = '';
    renaming = null;
    removing = null;
    const id = field.id;
    adminApi
      .field(id)
      .then((f) => {
        if (field?.id === id) detail = f;
      })
      .catch((e: unknown) => {
        loadError = errorMessage(e);
      });
  });

  /** Runs one change and keeps the field it returns. */
  async function run(action: () => Promise<FieldRow>, focus?: string): Promise<boolean> {
    busy = true;
    error = '';
    try {
      detail = await action();
      onchanged();
      return true;
    } catch (e) {
      error = errorMessage(e);
      return false;
    } finally {
      busy = false;
      if (focus) queueMicrotask(() => document.getElementById(focus)?.focus());
    }
  }

  async function add(e: SubmitEvent) {
    e.preventDefault();
    if (!field || !adding.trim() || addProblem) return;
    const value = adding.trim();
    if (await run(() => adminApi.addOption(field.id, value))) {
      adding = '';
      queueMicrotask(() => document.getElementById('option-new')?.focus());
    }
  }

  async function rename(e: SubmitEvent) {
    e.preventDefault();
    if (!field || !renaming || renameProblem) return;
    const { id, value } = renaming;
    const before = options.find((o) => o.id === id);
    if (before && before.value === value.trim()) {
      renaming = null;
      return;
    }
    if (await run(() => adminApi.updateOption(field.id, id, { value: value.trim() }))) {
      renaming = null;
    }
  }

  function toggle(o: FieldOption) {
    if (field) void run(() => adminApi.updateOption(field.id, o.id, { disabled: !o.disabled }));
  }

  function step(o: FieldOption, by: -1 | 1) {
    if (!field || busy) return;
    const from = options.findIndex((x) => x.id === o.id);
    const to = from + by;
    if (to < 0 || to >= options.length) return;
    const dir = arrowAfterMove(by, to, options.length);
    const ids = moveItem(options, from, to).map((x) => x.id);
    void run(() => adminApi.orderOptions(field.id, ids), `order-${dir}-opt-${o.id}`);
  }

  function askRemove(o: FieldOption) {
    removing = o;
    renaming = null;
    error = '';
    how = optionTargets(options, o).length > 0 ? 'move' : 'clear';
    moveTo = '';
  }

  async function remove() {
    if (!field || !removing) return;
    const o = removing;
    const used = o.issues > 0;
    if (used && how === 'move' && !moveTo) {
      error = t.fieldsAdmin.chooseTarget;
      return;
    }
    const choice = !used ? {} : how === 'move' ? { moveTo: Number(moveTo) } : { clear: true };
    if (await run(() => adminApi.deleteOption(field.id, o.id, choice))) {
      notify(t.fieldsAdmin.optionRemoved(o.value));
      removing = null;
    }
  }
</script>

<Modal
  {open}
  title={t.fieldsAdmin.optionsTitle(field?.name ?? '')}
  onclose={() => !busy && onclose()}
>
  {#if loadError}
    <p class="error-text" role="alert">{loadError}</p>
  {:else if !detail}
    <p class="muted loading">{t.common.loading}</p>
  {:else if removing}
    <div class="removing">
      <h3 class="sub-title">{t.fieldsAdmin.deleteOptionTitle(removing.value)}</h3>
      {#if removing.issues === 0}
        <p>{t.fieldsAdmin.deleteOptionUnused}</p>
      {:else}
        <p>{t.fieldsAdmin.deleteOptionUsed(removing.issues)}</p>
        <fieldset class="choices">
          <legend class="sr-only">{t.fieldsAdmin.moveLabel}</legend>
          <label class="check">
            <input
              type="radio"
              name="option-how"
              value="move"
              bind:group={how}
              disabled={targets.length === 0}
            />
            <span>{t.fieldsAdmin.howMove}</span>
          </label>
          <label class="check">
            <input type="radio" name="option-how" value="clear" bind:group={how} />
            <span>{t.fieldsAdmin.howClear}</span>
          </label>
        </fieldset>
        {#if targets.length === 0}
          <p class="form-hint">{t.fieldsAdmin.noTarget}</p>
        {:else if how === 'move'}
          <div class="field">
            <span>{t.fieldsAdmin.moveLabel}</span>
            <Dropdown
              label={t.fieldsAdmin.moveLabel}
              value={moveTo}
              options={targets.map((x) => ({ value: String(x.id), label: x.value }))}
              placeholder={t.catalogAdmin.choose}
              onchange={(v) => (moveTo = v)}
            />
          </div>
        {/if}
      {/if}
      {#if removing.children > 0}
        <p class="muted small">{t.fieldsAdmin.cascadeNote}</p>
      {/if}
    </div>
  {:else}
    <div class="options">
      <p>{t.fieldsAdmin.optionsIntro}</p>
      {#if cascading}<p class="muted small">{t.fieldsAdmin.cascadeNote}</p>{/if}

      {#if options.length === 0}
        <p class="muted">{t.fieldsAdmin.noOptions}</p>
      {:else}
        <ul class="option-list">
          {#each options as o, i (o.id)}
            <li class:is-disabled={o.disabled}>
              <span class="order">
                <OrderArrows
                  id="opt-{o.id}"
                  name={o.value}
                  first={i === 0}
                  last={i === options.length - 1}
                  disabled={busy}
                  onmove={(by) => step(o, by)}
                />
              </span>
              {#if renaming?.id === o.id}
                <form class="rename" onsubmit={rename}>
                  <!-- svelte-ignore a11y_autofocus -->
                  <input
                    class="input"
                    bind:value={renaming.value}
                    aria-label={t.fieldsAdmin.renameOption(o.value)}
                    aria-invalid={!!renameProblem}
                    autocomplete="off"
                    autofocus
                    onkeydown={(e) =>
                      e.key === 'Escape' &&
                      (e.preventDefault(), e.stopPropagation(), (renaming = null))}
                  />
                  <button class="btn btn-small btn-primary" disabled={busy || !!renameProblem}
                    >{t.common.save}</button
                  >
                  <button
                    type="button"
                    class="btn btn-small btn-quiet"
                    onclick={() => (renaming = null)}>{t.common.cancel}</button
                  >
                  {#if renameProblem}
                    <span class="field-error">{t.fieldsAdmin.problems[renameProblem]}</span>
                  {/if}
                </form>
              {:else}
                <span class="name">
                  <strong>{o.value}</strong>
                  {#if o.disabled}<span class="chip">{t.fieldsAdmin.disabledStamp}</span>{/if}
                  <span class="muted small">
                    {o.issues > 0
                      ? t.catalogAdmin.issues(o.issues)
                      : t.catalogAdmin.unused}{o.children > 0
                      ? ` · ${t.fieldsAdmin.subOptions(o.children)}`
                      : ''}
                  </span>
                </span>
                <span class="row-actions">
                  <button
                    type="button"
                    class="link-btn"
                    disabled={busy}
                    onclick={() => (renaming = { id: o.id, value: o.value })}
                    >{t.common.edit}</button
                  >
                  <button type="button" class="link-btn" disabled={busy} onclick={() => toggle(o)}
                    >{o.disabled ? t.fieldsAdmin.enable : t.fieldsAdmin.disable}</button
                  >
                  <button
                    type="button"
                    class="link-btn"
                    disabled={busy}
                    onclick={() => askRemove(o)}>{t.common.delete}</button
                  >
                </span>
              {/if}
            </li>
          {/each}
        </ul>
      {/if}

      <form class="add" onsubmit={add}>
        <label class="field grow">
          <span class="sr-only">{t.fieldsAdmin.addOption}</span>
          <input
            id="option-new"
            class="input"
            bind:value={adding}
            placeholder={t.fieldsAdmin.optionPlaceholder}
            autocomplete="off"
            aria-invalid={!!addProblem}
          />
        </label>
        <button class="btn" disabled={busy || adding.trim() === '' || !!addProblem}
          >{t.fieldsAdmin.addOption}</button
        >
      </form>
      {#if addProblem}<span class="field-error">{t.fieldsAdmin.problems[addProblem]}</span>{/if}
    </div>
  {/if}
  {#if error}<p class="error-text" role="alert">{error}</p>{/if}
  {#snippet footer()}
    {#if removing}
      <button type="button" class="btn btn-quiet" disabled={busy} onclick={() => (removing = null)}
        >{t.common.cancel}</button
      >
      <button type="button" class="btn btn-danger" disabled={busy} onclick={() => void remove()}
        >{removing.issues === 0
          ? t.common.delete
          : how === 'move'
            ? t.fieldsAdmin.moveAndDelete
            : t.fieldsAdmin.clearAndDelete}</button
      >
    {:else}
      <button type="button" class="btn" disabled={busy} onclick={onclose}>{t.common.close}</button>
    {/if}
  {/snippet}
</Modal>

<style>
  .options,
  .removing {
    display: grid;
    gap: 12px;
  }

  .options p,
  .removing p,
  .removing h3 {
    margin: 0;
  }

  .option-list {
    display: grid;
    margin: 0;
    padding: 0;
    list-style: none;
    border-top: var(--frame) solid var(--rule);
  }

  .option-list li {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 6px 12px;
    padding: 6px 0;
    border-bottom: var(--frame) solid var(--rule);
  }

  .order {
    white-space: nowrap;
  }

  .name {
    display: inline-flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 2px 8px;
    flex: 1;
    min-width: 0;
  }

  li.is-disabled .name strong {
    color: var(--muted);
    font-weight: 500;
  }

  .rename {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 6px;
    flex: 1;
  }

  .rename .input {
    flex: 1;
    min-width: 12ch;
  }

  .add {
    display: flex;
    gap: 8px;
    align-items: center;
  }

  .add .grow {
    flex: 1;
  }
</style>
