<script lang="ts">
  import { tick } from 'svelte';
  import type { LayoutField, LayoutForm, LayoutZone } from '../../core/admin-types.js';
  import { arrowAfterMove } from '../../core/catalogs.js';
  import { t } from '../../i18n/t.js';
  import { pinnedHeight } from '../../lib/actions.js';
  import { errorMessage } from '../../lib/api.js';
  import { touchDrag, type TouchDragHandlers } from '../../lib/touch-drag.js';
  import OrderArrows from '../admin/OrderArrows.svelte';
  import CardMenu, { type MenuGroup } from '../board/CardMenu.svelte';
  import {
    dropRow,
    FIXED_FIELDS,
    hiddenRows,
    hide,
    layoutRows,
    setRequired,
    show,
    stepRow,
    zoneRows,
    type DropTarget,
    type LayoutChoice,
  } from './layout-rows.js';

  /**
   * Edits one form's template on a mock of the form: the new issue dialog,
   * or the issue page with its main and side columns. Shown fields are rows
   * in order; hidden ones wait in a tray below. Rows move by mouse, by touch,
   * or by keyboard (the arrows and each row's menu). Changes wait for « Save ».
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
  const nameOf = (field: string) => names.get(field)?.name ?? field;
  const hidden = $derived(hiddenRows(rows, nameOf));

  $effect(() => {
    ondirty?.(dirty);
  });

  const rowId = (field: string) => `layout-${form}-${field}`;

  async function focusLater(id: string, selector = '') {
    await tick();
    const el = document.getElementById(id);
    (selector ? el?.querySelector<HTMLElement>(selector) : el)?.focus();
  }

  function step(row: LayoutField, by: -1 | 1) {
    rows = stepRow(rows, row.field, by);
    const peers = zoneRows(rows, row.zone);
    const at = peers.findIndex((r) => r.field === row.field);
    void focusLater(`order-${arrowAfterMove(by, at, peers.length)}-${rowId(row.field)}`);
  }

  function hideRow(field: string) {
    rows = hide(rows, field);
    void focusLater(`tray-${rowId(field)}`, 'button');
  }

  function showRow(field: string, zone?: LayoutZone) {
    rows = show(rows, field, zone);
    void focusLater(rowId(field), '.card-menu-toggle');
  }

  function moveTo(field: string, zone: LayoutZone) {
    rows = dropRow(rows, field, { zone });
    void focusLater(rowId(field), '.card-menu-toggle');
  }

  function rowMenu(r: LayoutField): MenuGroup[] {
    const name = nameOf(r.field);
    const own = { label: name, items: [{ label: t.layouts.hide, run: () => hideRow(r.field) }] };
    if (form === 'create') return [own];
    return [
      {
        label: t.layouts.moveTo,
        items: ZONES.map((z) => ({
          label: t.layouts.zones[z],
          disabled: r.zone === z,
          run: () => moveTo(r.field, z),
        })),
      },
      own,
    ];
  }

  function land(target: DropTarget | null) {
    if (drag && target) rows = dropRow(rows, drag, target);
    drag = null;
    over = null;
  }

  function targetAt(x: number, y: number): DropTarget | null {
    const el = document.elementFromPoint(x, y);
    if (el?.closest('.tray')) return { tray: true };
    const row = el?.closest<HTMLElement>('.layout-row[data-field]');
    if (row) return { field: row.dataset['field'], zone: zoneOf(row) };
    const list = el?.closest<HTMLElement>('.layout-list');
    return list ? { zone: zoneOf(list) } : null;
  }

  const zoneOf = (el: HTMLElement) => (el.dataset['zone'] || undefined) as LayoutZone | undefined;

  const touch: TouchDragHandlers = {
    selector: '[data-field][draggable="true"]',
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

  /** Mouse drag handlers of a drop target; `target` is where a drop lands. */
  function dropZone(target: DropTarget) {
    return {
      ondragover: (e: DragEvent) => {
        if (drag === null) return;
        e.preventDefault();
        e.stopPropagation();
        over = target;
      },
      ondrop: (e: DragEvent) => {
        e.preventDefault();
        e.stopPropagation();
        land(target);
      },
    };
  }

  function dragStart(field: string) {
    return (e: DragEvent) => {
      drag = field;
      e.dataTransfer?.setData('text/plain', field);
    };
  }

  const dragEnd = () => ((drag = null), (over = null));

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

{#snippet lock()}
  <svg class="lock" viewBox="0 0 16 16" width="12" height="12" aria-hidden="true">
    <rect x="3" y="7" width="10" height="7" rx="1.5" />
    <path d="M5.5 7V5a2.5 2.5 0 0 1 5 0v2" fill="none" stroke="currentColor" stroke-width="1.5" />
  </svg>
{/snippet}

{#snippet plus()}
  <svg viewBox="0 0 16 16" width="12" height="12" aria-hidden="true">
    <path d="M8 3v10M3 8h10" fill="none" stroke="currentColor" stroke-width="1.6" />
  </svg>
{/snippet}

{#snippet row(r: LayoutField, i: number, count: number)}
  {@const id = rowId(r.field)}
  {@const note = names.get(r.field)?.note}
  <li
    class="layout-row"
    class:is-dragged={drag === r.field}
    class:is-over={over?.field === r.field && drag !== null && drag !== r.field}
    {id}
    data-field={r.field}
    data-zone={r.zone ?? ''}
    draggable="true"
    ondragstart={dragStart(r.field)}
    ondragend={dragEnd}
    {...dropZone({ field: r.field, zone: r.zone })}
  >
    <span class="grip" aria-hidden="true">⋮⋮</span>
    <span class="name" id="{id}-name">
      <span class="label">{nameOf(r.field)}</span>
      {#if note}<span class="note">{note}</span>{/if}
    </span>
    {#if form === 'create'}
      <button
        type="button"
        class="req"
        aria-pressed={!!r.required}
        aria-describedby="{id}-name"
        onclick={() => (rows = setRequired(rows, r.field, !r.required))}
        >{t.layouts.required}</button
      >
    {/if}
    <span class="arrows">
      <OrderArrows
        {id}
        name={nameOf(r.field)}
        first={i === 0}
        last={i === count - 1}
        onmove={(by) => step(r, by)}
      />
    </span>
    <CardMenu label={t.layouts.actionsOf(nameOf(r.field))} groups={() => rowMenu(r)} />
  </li>
{/snippet}

{#snippet list(zone: LayoutZone | undefined)}
  {@const shown = zoneRows(rows, zone)}
  <ol
    class="layout-list"
    class:is-over={over !== null && !over.field && !over.tray && over.zone === zone}
    data-zone={zone ?? ''}
    aria-label={zone ? t.layouts.zones[zone] : t.layouts.tabs.create}
    {...dropZone({ zone })}
  >
    {#each shown as r, i (r.field)}
      {@render row(r, i, shown.length)}
    {/each}
    {#if shown.length === 0}
      <li class="layout-empty">{t.layouts.emptyZone}</li>
    {/if}
  </ol>
{/snippet}

<form class="layout-editor" onsubmit={save} novalidate use:touchDrag={touch}>
  <p class="hint">{t.layouts.orderHint}</p>
  <div class="mock" class:is-view={form === 'view'}>
    <div class="mock-head">
      <span class="mock-title">{form === 'create' ? t.layouts.mockCreate : t.layouts.mockView}</span
      >
      <span class="fixed">
        {@render lock()}
        <span>
          <span class="fixed-names"
            >{form === 'create'
              ? FIXED_FIELDS.map((f) => t.layouts.fields[f]).join(' · ')
              : t.layouts.fields.summary}</span
          >
          <span class="note">{form === 'create' ? t.layouts.fixedCreate : t.layouts.fixedView}</span
          >
        </span>
      </span>
    </div>

    {#if form === 'create'}
      {@render list(undefined)}
    {:else}
      <div class="columns">
        {#each ZONES as zone (zone)}
          <section class="column column-{zone}" aria-labelledby="layout-zone-{zone}">
            <h3 class="column-title" id="layout-zone-{zone}">{t.layouts.zones[zone]}</h3>
            {@render list(zone)}
          </section>
        {/each}
      </div>
    {/if}

    <section
      class="tray"
      class:is-over={over?.tray && drag !== null}
      aria-labelledby="layout-tray-{form}"
      {...dropZone({ tray: true })}
    >
      <h3 class="column-title" id="layout-tray-{form}">{t.layouts.hidden}</h3>
      {#if hidden.length === 0}
        <p class="tray-empty">{t.layouts.trayEmpty}</p>
      {:else}
        <ul class="chips">
          {#each hidden as r (r.field)}
            {@const name = nameOf(r.field)}
            <li
              class="tray-chip"
              class:is-dragged={drag === r.field}
              id="tray-{rowId(r.field)}"
              data-field={r.field}
              draggable="true"
              ondragstart={dragStart(r.field)}
              ondragend={dragEnd}
            >
              <span>{name}</span>
              {#if form === 'view'}
                <CardMenu
                  label={t.layouts.showOf(name)}
                  glyph={plus}
                  groups={() => [
                    {
                      label: t.layouts.addTo,
                      items: ZONES.map((z) => ({
                        label: t.layouts.zones[z],
                        run: () => showRow(r.field, z),
                      })),
                    },
                  ]}
                />
              {:else}
                <button
                  type="button"
                  class="btn btn-quiet btn-small add"
                  aria-label={t.layouts.showOf(name)}
                  title={t.layouts.showOf(name)}
                  onclick={() => showRow(r.field)}>{@render plus()}</button
                >
              {/if}
            </li>
          {/each}
        </ul>
      {/if}
    </section>
  </div>

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
    color: var(--muted);
    font-size: 0.8125rem;
  }

  .mock {
    container-type: inline-size;
    border: 1px solid var(--rule-strong);
    border-radius: var(--radius-m);
    background: var(--paper-raised);
    overflow: hidden;
  }

  .mock-head {
    display: grid;
    gap: 6px;
    padding: 10px 14px;
    background: var(--paper-sunk);
    border-bottom: 1px solid var(--rule);
  }

  .mock-title {
    font: 600 0.6875rem/1.2 var(--font-body);
    letter-spacing: 0.06em;
    text-transform: uppercase;
    color: var(--muted);
  }

  .fixed {
    display: flex;
    align-items: baseline;
    gap: 8px;
    color: var(--ink-soft);
  }

  .fixed > span {
    display: grid;
  }

  .lock {
    flex: none;
    fill: currentColor;
    color: var(--muted);
  }

  .note {
    color: var(--muted);
    font-size: 0.75rem;
  }

  .columns {
    display: grid;
    grid-template-columns: minmax(0, 2fr) minmax(0, 1fr);
  }

  .column-side {
    border-left: 1px solid var(--rule);
  }

  @container (max-width: 40rem) {
    .columns {
      grid-template-columns: minmax(0, 1fr);
    }

    .column-side {
      border-left: none;
      border-top: 1px solid var(--rule);
    }
  }

  .column-title {
    margin: 0;
    padding: 10px 14px 4px;
    font: 600 0.6875rem/1.2 var(--font-body);
    letter-spacing: 0.06em;
    text-transform: uppercase;
    color: var(--muted);
  }

  .layout-list {
    list-style: none;
    margin: 0;
    padding: 0 0 6px;
    min-height: 2.75rem;
  }

  .layout-list.is-over,
  .tray.is-over {
    box-shadow: inset 0 0 0 2px var(--accent);
  }

  .layout-row,
  :global(body > .layout-row.touch-ghost) {
    display: flex;
    align-items: center;
    gap: 8px;
    min-height: 2.25rem;
    padding: 3px 8px 3px 10px;
  }

  .layout-row + .layout-row {
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
    background: var(--paper-raised);
    box-shadow: var(--shadow-pop);
  }

  .layout-row.is-dragged,
  .tray-chip.is-dragged {
    opacity: 0.45;
  }

  .layout-row.is-over {
    box-shadow: inset 0 2px 0 var(--accent);
  }

  .layout-empty,
  .tray-empty {
    margin: 0;
    padding: 8px 14px;
    color: var(--muted);
    font-size: 0.8125rem;
  }

  .grip {
    flex: none;
    width: 12px;
    color: var(--muted);
    font-size: 0.6875rem;
    letter-spacing: -2px;
    cursor: grab;
  }

  .name {
    flex: 1;
    min-width: 0;
    display: grid;
    overflow-wrap: anywhere;
  }

  .arrows {
    display: inline-flex;
    flex: none;
  }

  /* Quiet until the row is pointed at or holds the focus. */
  .arrows,
  .req[aria-pressed='false'],
  .layout-row :global(.card-menu-toggle) {
    opacity: 0.4;
  }

  .layout-row:hover .arrows,
  .layout-row:focus-within .arrows,
  .layout-row:hover .req,
  .layout-row:focus-within .req,
  .layout-row:hover :global(.card-menu-toggle),
  .layout-row:focus-within :global(.card-menu-toggle) {
    opacity: 1;
  }

  .req {
    flex: none;
    border: 1px dashed var(--rule-strong);
    border-radius: 999px;
    background: none;
    padding: 1px 9px;
    font: 500 0.75rem/1.5 var(--font-body);
    color: var(--muted);
    cursor: pointer;
  }

  .req[aria-pressed='true'] {
    border-style: solid;
    border-color: var(--accent);
    color: var(--accent-text);
  }

  .tray {
    border-top: 1px solid var(--rule);
    background: var(--paper-sunk);
    padding-bottom: 10px;
  }

  .chips {
    list-style: none;
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    margin: 0;
    padding: 4px 14px 0;
  }

  .tray-chip {
    display: inline-flex;
    align-items: center;
    gap: 2px;
    max-width: 100%;
    padding: 1px 2px 1px 10px;
    border: 1px solid var(--rule-strong);
    border-radius: 999px;
    background: var(--paper-raised);
    font-size: 0.8125rem;
    cursor: grab;
  }

  .tray-chip > span {
    overflow-wrap: anywhere;
  }

  .add {
    padding: 0 4px;
    color: var(--muted);
  }

  .add svg,
  .tray-chip :global(svg) {
    display: block;
  }

  .savebar {
    margin-top: 16px;
  }

  @media (pointer: coarse) {
    .arrows,
    .req[aria-pressed='false'],
    .layout-row :global(.card-menu-toggle) {
      opacity: 1;
    }

    [draggable='true'] {
      touch-action: pan-y;
      user-select: none;
      -webkit-user-select: none;
    }
  }
</style>
