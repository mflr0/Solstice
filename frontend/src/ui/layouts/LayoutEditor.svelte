<script lang="ts">
  import { tick } from 'svelte';
  import type { LayoutField, LayoutForm, LayoutZone } from '../../core/admin-types.js';
  import { arrowAfterMove } from '../../core/catalogs.js';
  import { t } from '../../i18n/t.js';
  import { pinnedHeight } from '../../lib/actions.js';
  import { errorMessage } from '../../lib/api.js';
  import { touchDrag, type TouchDragHandlers } from '../../lib/touch-drag.js';
  import OrderArrows from '../admin/OrderArrows.svelte';
  import Dropdown from '../Dropdown.svelte';
  import {
    dropRow,
    FIXED_FIELDS,
    layoutRows,
    setRequired,
    setShown,
    setZone,
    stepRow,
    zoneRows,
    type DropTarget,
    type LayoutChoice,
  } from './layout-rows.js';

  /**
   * Edits one form's template: the order of its fields, which it shows and,
   * in the create form, which a new issue needs; in the view, which zone each
   * goes to. Rows move by mouse, by touch, or by keyboard (the arrows and the
   * zone select). Changes wait for « Save ».
   */
  interface Props {
    form: LayoutForm;
    /** The fields the form can place, named. */
    fields: LayoutChoice[];
    /** The template as saved, or what it inherits. */
    template: LayoutField[];
    /** Saves the rows; the page then passes the saved template back. */
    onsave: (fields: LayoutField[]) => Promise<unknown>;
    ondirty?: ((dirty: boolean) => void) | undefined;
  }
  const { form, fields, template, onsave, ondirty }: Props = $props();

  const ZONES: LayoutZone[] = ['main', 'side'];

  const baseline = $derived(layoutRows(form, template, fields));
  let rows = $derived(baseline);
  let saving = $state(false);
  let error = $state('');
  let drag = $state<string | null>(null);
  let over = $state<DropTarget | null>(null);

  const dirty = $derived(JSON.stringify(rows) !== JSON.stringify(baseline));
  const names = $derived(new Map(fields.map((f) => [f.id, f])));

  $effect(() => {
    ondirty?.(dirty);
  });

  const rowId = (field: string) => `layout-${form}-${field}`;
  const nameOf = (field: string) => names.get(field)?.name ?? field;

  async function step(row: LayoutField, by: -1 | 1) {
    rows = stepRow(rows, row.field, by);
    const peers = zoneRows(rows, row.zone);
    const at = peers.findIndex((r) => r.field === row.field);
    const dir = arrowAfterMove(by, at, peers.length);
    await tick();
    document.getElementById(`order-${dir}-${rowId(row.field)}`)?.focus();
  }

  async function moveTo(row: LayoutField, zone: LayoutZone) {
    rows = setZone(rows, row.field, zone);
    await tick();
    document.getElementById(`zone-${rowId(row.field)}`)?.focus();
  }

  function land(target: DropTarget | null) {
    if (drag && target) rows = dropRow(rows, drag, target);
    drag = null;
    over = null;
  }

  function targetAt(x: number, y: number): DropTarget | null {
    const el = document.elementFromPoint(x, y);
    const row = el?.closest<HTMLElement>('.layout-row[data-field]');
    if (row) return { field: row.dataset['field'], zone: row.dataset['zone'] as LayoutZone };
    const list = el?.closest<HTMLElement>('.layout-list[data-zone]');
    return list ? { zone: list.dataset['zone'] as LayoutZone } : null;
  }

  const touch: TouchDragHandlers = {
    selector: '.layout-row[data-field]',
    start(el) {
      drag = el.dataset['field'] ?? null;
      return drag !== null;
    },
    move(x, y) {
      over = targetAt(x, y);
    },
    drop(x, y) {
      land(targetAt(x, y));
    },
    cancel() {
      drag = null;
      over = null;
    },
  };

  function discard() {
    rows = baseline;
    error = '';
  }

  async function save(e: SubmitEvent) {
    e.preventDefault();
    saving = true;
    error = '';
    try {
      await onsave(rows);
    } catch (err) {
      error = errorMessage(err);
    } finally {
      saving = false;
    }
  }
</script>

{#snippet row(r: LayoutField, i: number, count: number)}
  {@const id = rowId(r.field)}
  {@const choice = names.get(r.field)}
  <li
    class="layout-row"
    class:is-hidden={r.hidden}
    class:is-dragged={drag === r.field}
    class:is-over={over?.field === r.field && drag !== null && drag !== r.field}
    data-field={r.field}
    data-zone={r.zone ?? ''}
    draggable="true"
    ondragstart={(e) => {
      drag = r.field;
      e.dataTransfer?.setData('text/plain', r.field);
    }}
    ondragover={(e) => {
      if (drag === null) return;
      e.preventDefault();
      e.stopPropagation();
      over = { field: r.field, zone: r.zone };
    }}
    ondrop={(e) => {
      e.preventDefault();
      e.stopPropagation();
      land({ field: r.field, zone: r.zone });
    }}
    ondragend={() => ((drag = null), (over = null))}
  >
    <span class="order">
      <span class="grip" aria-hidden="true">⋮⋮</span>
      <OrderArrows
        {id}
        name={nameOf(r.field)}
        first={i === 0}
        last={i === count - 1}
        onmove={(by) => void step(r, by)}
      />
    </span>
    <span class="name" id="{id}-name">
      <span class="label">{nameOf(r.field)}</span>
      {#if choice?.note}<span class="muted small note">{choice.note}</span>{/if}
    </span>
    <span class="controls">
      {#if form === 'view'}
        <span class="zone">
          <Dropdown
            id="zone-{id}"
            label={t.layouts.zoneOf(nameOf(r.field))}
            value={r.zone ?? 'side'}
            options={ZONES.map((z) => ({ value: z, label: t.layouts.zones[z] }))}
            onchange={(v) => void moveTo(r, v as LayoutZone)}
          />
        </span>
      {/if}
      <label class="switch">
        <input
          class="checkbox"
          type="checkbox"
          role="switch"
          id="shown-{id}"
          aria-describedby="{id}-name"
          checked={!r.hidden}
          onchange={(e) => (rows = setShown(rows, r.field, e.currentTarget.checked))}
        />
        <span>{t.layouts.shown}</span>
      </label>
      {#if form === 'create'}
        <label class="switch" class:is-off={r.hidden}>
          <input
            class="checkbox"
            type="checkbox"
            role="switch"
            id="required-{id}"
            aria-describedby="{id}-name"
            checked={!!r.required}
            disabled={r.hidden}
            onchange={(e) => (rows = setRequired(rows, r.field, e.currentTarget.checked))}
          />
          <span>{t.layouts.required}</span>
        </label>
      {/if}
    </span>
  </li>
{/snippet}

{#snippet fixedRow(label: string, note: string)}
  <li class="layout-row is-fixed">
    <span class="order"><span class="stamp">{t.layouts.fixed}</span></span>
    <span class="name">
      <span class="label">{label}</span>
      <span class="muted small note">{note}</span>
    </span>
  </li>
{/snippet}

<form class="layout-editor" onsubmit={save} novalidate use:touchDrag={touch}>
  <p class="muted small hint">{t.layouts.orderHint}</p>
  {#if form === 'create'}
    {@const list = zoneRows(rows)}
    <ol class="layout-list" aria-label={t.layouts.tabs.create}>
      {#each FIXED_FIELDS as f (f)}
        {@render fixedRow(t.layouts.fields[f], t.layouts.fixedRequired)}
      {/each}
      {#each list as r, i (r.field)}
        {@render row(r, i, list.length)}
      {/each}
    </ol>
  {:else}
    <div class="zones">
      {#each ZONES as zone (zone)}
        {@const list = zoneRows(rows, zone)}
        <section class="zone-box" aria-labelledby="layout-zone-{zone}">
          <h3 class="zone-title" id="layout-zone-{zone}">{t.layouts.zones[zone]}</h3>
          <ol
            class="layout-list"
            class:is-over={over !== null && !over.field && over.zone === zone}
            data-zone={zone}
            aria-labelledby="layout-zone-{zone}"
            ondragover={(e) => {
              if (drag === null) return;
              e.preventDefault();
              over = { zone };
            }}
            ondrop={(e) => {
              e.preventDefault();
              land({ zone });
            }}
          >
            {#if zone === 'main'}
              {@render fixedRow(t.layouts.header, t.layouts.headerNote)}
            {/if}
            {#each list as r, i (r.field)}
              {@render row(r, i, list.length)}
            {/each}
            {#if list.length === 0}
              <li class="layout-empty muted small">{t.layouts.emptyZone}</li>
            {/if}
          </ol>
        </section>
      {/each}
    </div>
  {/if}

  {#if error}<p class="error-text" role="alert">{error}</p>{/if}
  {#if dirty || error}
    <div
      class="savebar"
      role="region"
      aria-label={t.settings.unsaved}
      use:pinnedHeight={'--savebar-height'}
    >
      <span>{t.settings.unsaved}</span>
      <button type="button" class="btn btn-quiet" onclick={discard}>{t.settings.discard}</button>
      <button class="btn btn-primary" type="submit" disabled={saving}
        >{saving ? t.common.saving : t.common.save}</button
      >
    </div>
  {/if}
</form>

<style>
  .hint {
    margin: 0 0 10px;
  }

  .zones {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(min(100%, 22rem), 1fr));
    gap: 16px;
    align-items: start;
  }

  .zone-title {
    margin: 0 0 6px;
    font: 600 0.8125rem/1.3 var(--font-body);
    color: var(--ink-soft);
  }

  .layout-list {
    list-style: none;
    margin: 0;
    padding: 0;
    border: 1px solid var(--rule);
    border-radius: var(--radius-s);
    min-height: 3rem;
  }

  .layout-list.is-over {
    box-shadow: 0 0 0 2px var(--accent);
  }

  .layout-row,
  :global(body > .layout-row.touch-ghost) {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 6px 12px;
    padding: 6px 10px;
    background: var(--paper-raised);
  }

  .layout-row + .layout-row,
  .layout-row + .layout-empty {
    border-top: 1px solid var(--rule);
  }

  :global(body > .layout-row.touch-ghost) {
    position: fixed;
    top: 0;
    left: 0;
    z-index: 70;
    margin: 0;
    list-style: none;
    pointer-events: none;
    box-shadow: var(--shadow-pop);
    opacity: 0.9;
  }

  .layout-row.is-fixed {
    background: var(--paper-sunk);
  }

  .layout-row.is-hidden .label {
    color: var(--muted);
    text-decoration: line-through;
  }

  .layout-row.is-dragged {
    opacity: 0.45;
  }

  .layout-row.is-over {
    box-shadow: inset 0 2px 0 var(--accent);
  }

  .layout-empty {
    padding: 10px;
  }

  .order {
    display: inline-flex;
    align-items: center;
    min-width: 4.75rem;
    white-space: nowrap;
  }

  .grip {
    display: inline-block;
    width: 14px;
    color: var(--muted);
    font-size: 0.6875rem;
    letter-spacing: -2px;
    cursor: grab;
  }

  .name {
    flex: 1 1 10rem;
    min-width: 0;
    display: grid;
    overflow-wrap: anywhere;
  }

  .controls {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 6px 14px;
    margin-left: auto;
  }

  .zone {
    min-width: 7rem;
  }

  .switch {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    white-space: nowrap;
  }

  .switch.is-off {
    color: var(--muted);
  }

  .savebar {
    margin-top: 16px;
  }

  @media (pointer: coarse) {
    .layout-row[data-field] {
      touch-action: pan-y;
      user-select: none;
      -webkit-user-select: none;
    }
  }
</style>
