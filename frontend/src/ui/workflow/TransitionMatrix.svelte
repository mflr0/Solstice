<script lang="ts">
  import type { StatusRef } from '../../core/types.js';
  import {
    defaultName,
    matrix,
    rename,
    toggleCell,
    type DraftTransition,
  } from '../../core/workflow.js';
  import { t } from '../../i18n/t.js';
  import { autofocus } from '../../lib/actions.js';
  import Stamp from '../Stamp.svelte';

  /**
   * The transitions as a table: a row per status it starts from, a column
   * per status it leads to, a box per pair, and a last row for transitions
   * allowed from any status. A checked box shows the transition's name,
   * which can be changed in place.
   */
  interface Props {
    statuses: StatusRef[];
    transitions: DraftTransition[];
    /** The saved transitions, so a box checked again gets its own back. */
    saved: DraftTransition[];
    readonly: boolean;
    onchange: (list: DraftTransition[]) => void;
  }
  const { statuses, transitions, saved, readonly, onchange }: Props = $props();

  const rows = $derived(matrix(statuses, transitions));
  let editing = $state<string | null>(null);

  function label(from: StatusRef | null, to: StatusRef): string {
    return from ? t.workflows.cell(from.name, to.name) : t.workflows.cellAny(to.name);
  }

  function shown(tr: DraftTransition, to: StatusRef): string {
    return tr.name.trim() || defaultName(to);
  }

  function nameKey(e: KeyboardEvent) {
    if (e.key === 'Enter' || e.key === 'Escape') {
      e.preventDefault();
      e.stopPropagation();
      const key = editing;
      editing = null;
      // Back to the name's button, so the keyboard does not lose its place.
      queueMicrotask(() => document.getElementById(`tn-${key}`)?.focus());
    }
  }
</script>

{#if statuses.length === 0}
  <p class="empty">{t.workflows.noStatuses}</p>
{:else}
  <div class="scroll">
    <table class="ledger matrix" style:--n={statuses.length}>
      <thead>
        <tr>
          <td class="corner"><span>{t.workflows.fromTo}</span></td>
          {#each statuses as to (to.id)}
            <th scope="col"><Stamp status={to} /></th>
          {/each}
        </tr>
      </thead>
      <tbody>
        {#each rows as row (row.from?.id ?? '*')}
          <tr class:any={row.from === null}>
            <th scope="row">
              {#if row.from}<Stamp status={row.from} />{:else}<span class="any-label"
                  >{t.workflows.anyStatusRow}</span
                >{/if}
            </th>
            {#each row.cells as c (c.to.id)}
              <td class:self={c.self} class:on={c.transitions.length > 0}>
                {#if c.self}
                  <span class="dash" aria-hidden="true">·</span>
                {:else if readonly}
                  {#if c.transitions.length}
                    <span class="sr-only">{label(row.from, c.to)}{t.common.labelSep}</span>
                    {#each c.transitions as tr (tr.key)}
                      <span class="tname static">{shown(tr, c.to)}</span>
                    {/each}
                  {/if}
                {:else}
                  <div class="cell">
                    <input
                      type="checkbox"
                      class="checkbox"
                      aria-label={label(row.from, c.to)}
                      checked={c.transitions.length > 0}
                      onchange={() =>
                        onchange(toggleCell(transitions, row.from?.id ?? null, c.to, saved))}
                    />
                    {#each c.transitions as tr (tr.key)}
                      {#if editing === tr.key}
                        <input
                          class="input name"
                          value={tr.name}
                          placeholder={defaultName(c.to)}
                          aria-label={t.workflows.transitionName}
                          oninput={(e) =>
                            onchange(rename(transitions, tr.key, e.currentTarget.value))}
                          onkeydown={nameKey}
                          onblur={() => (editing = null)}
                          use:autofocus
                        />
                      {:else}
                        <button
                          type="button"
                          class="tname"
                          id="tn-{tr.key}"
                          aria-label={`${t.workflows.renameLabel(shown(tr, c.to))} (${label(row.from, c.to)})`}
                          onclick={() => (editing = tr.key)}>{shown(tr, c.to)}</button
                        >
                      {/if}
                    {/each}
                  </div>
                {/if}
              </td>
            {/each}
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
{/if}

<style>
  /* Positioned, so hidden labels (.sr-only) stay inside the scrolling box. */
  .scroll {
    position: relative;
    overflow-x: auto;
    padding-bottom: 4px;
  }

  /* Equal columns, the row headers aside; past the page width, it scrolls. */
  .matrix {
    --head: 190px;
    --col: 128px;
    table-layout: fixed;
    width: max(100%, calc(var(--head) + var(--n) * var(--col)));
  }

  @media (max-width: 760px) {
    .matrix {
      --head: 118px;
      --col: 112px;
    }
  }

  .corner {
    width: var(--head);
    padding: 0 10px 8px 0;
    vertical-align: bottom;
    border-bottom: 1px solid var(--rule-strong);
  }

  .corner span {
    font: 500 11px/1.2 var(--font-body);
    color: var(--muted);
    white-space: nowrap;
  }

  /* Long status names wrap rather than spill over the next column. */
  .matrix th :global(.stamp) {
    white-space: normal;
    overflow-wrap: anywhere;
  }

  .matrix thead th {
    vertical-align: bottom;
    text-align: left;
  }

  .matrix th[scope='row'] {
    padding: 8px 12px 8px 0;
    border-bottom: 1px solid var(--rule);
    text-align: left;
    overflow-wrap: anywhere;
  }

  /* Where the matrix scrolls sideways, the row headers stay in view. */
  @media (max-width: 760px) {
    .matrix th[scope='row'] {
      position: sticky;
      left: 0;
      z-index: 1;
      background: var(--paper);
    }
  }

  .matrix tbody tr:hover td {
    background: none;
  }

  .matrix td {
    vertical-align: top;
    padding: 8px 10px;
    border-left: 1px solid var(--rule);
  }

  .matrix td.self {
    background: var(--paper-sunk);
    text-align: center;
  }

  .matrix tbody tr:hover td.self {
    background: var(--paper-sunk);
  }

  .matrix td.on {
    background: var(--accent-wash);
  }

  .matrix tbody tr:hover td.on {
    background: var(--accent-wash);
  }

  tr.any th,
  tr.any td {
    border-top: 1px solid var(--rule-strong);
  }

  .any-label {
    font: italic 500 12.5px/1.3 var(--font-body);
    color: var(--ink-soft);
  }

  .dash {
    color: var(--muted);
  }

  .cell {
    display: flex;
    flex-wrap: wrap;
    align-items: flex-start;
    gap: 4px 6px;
  }

  .checkbox {
    margin: 2px 0 0;
    flex: none;
  }

  .tname {
    flex: 1;
    min-width: 0;
    border: 1px solid transparent;
    background: none;
    padding: 0 2px;
    margin: 0 -2px;
    font: 12.5px/1.35 var(--font-body);
    color: var(--ink);
    text-align: left;
    overflow-wrap: anywhere;
    text-decoration: underline dotted var(--muted);
    text-underline-offset: 3px;
    cursor: text;
  }

  .tname:hover {
    border-color: var(--rule-strong);
    text-decoration: none;
  }

  .tname.static {
    display: block;
    text-decoration: none;
    cursor: default;
  }

  .name {
    flex: 1 1 100%;
    min-width: 0;
    height: 26px;
    padding: 2px 6px;
    font-size: 12.5px;
  }
</style>
