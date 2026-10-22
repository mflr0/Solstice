<script lang="ts">
  import { DEFAULT_COLUMNS, FIXED_COLUMNS, type ColumnChoice } from '../../core/search-columns.js';
  import { tick } from 'svelte';
  import { t } from '../../i18n/t.js';
  import OrderArrows from '../admin/OrderArrows.svelte';
  import FilterPopover from './FilterPopover.svelte';

  /**
   * The results' columns: one box per field, ticked ones first in the order
   * shown, which the arrows change. The key and the summary are always shown.
   */
  interface Props {
    choices: ColumnChoice[];
    chosen: readonly string[];
    onchange: (next: string[]) => void;
  }
  const { choices, chosen, onchange }: Props = $props();

  let open = $state(false);
  const byId = $derived(new Map(choices.map((c) => [c.id, c])));
  const shown = $derived(chosen.flatMap((id) => byId.get(id) ?? []));
  const hidden = $derived(
    choices
      .filter((c) => !FIXED_COLUMNS.includes(c.id) && !chosen.includes(c.id))
      .sort((a, b) => a.name.localeCompare(b.name)),
  );
  const isDefault = $derived(
    chosen.length === DEFAULT_COLUMNS.length && chosen.every((c, i) => c === DEFAULT_COLUMNS[i]),
  );

  const uid = $props.id();

  /** Ticking moves the row to the other list: the focus follows its box. */
  async function toggle(id: string, on: boolean) {
    onchange(on ? [...chosen, id] : chosen.filter((x) => x !== id));
    await tick();
    document.getElementById(`${uid}-${id}`)?.focus();
  }

  /** The focus stays on the arrow, or on the other one once it is disabled at an end. */
  async function move(id: string, by: -1 | 1) {
    const next = [...chosen];
    const i = next.indexOf(id);
    [next[i], next[i + by]] = [next[i + by]!, next[i]!];
    onchange(next);
    await tick();
    const [same, other] = by < 0 ? ['up', 'down'] : ['down', 'up'];
    const arrow = document.getElementById(`order-${same}-col-${id}`) as HTMLButtonElement | null;
    (arrow?.disabled ? document.getElementById(`order-${other}-col-${id}`) : arrow)?.focus();
  }
</script>

<div class="column-picker">
  <FilterPopover label={t.search.columns.button} bind:open wide>
    <div class="body">
      <ul class="list" aria-label={t.search.columns.shown}>
        {#each shown as c, i (c.id)}
          <li>
            <label class="check">
              <input
                id="{uid}-{c.id}"
                type="checkbox"
                checked
                onchange={() => void toggle(c.id, false)}
              />
              <span>{c.name}</span>
            </label>
            <span class="arrows">
              <OrderArrows
                id="col-{c.id}"
                name={c.name}
                first={i === 0}
                last={i === shown.length - 1}
                onmove={(by) => void move(c.id, by)}
              />
            </span>
          </li>
        {/each}
      </ul>
      {#if hidden.length}
        <p class="more">{t.search.columns.more}</p>
        <ul class="list" aria-label={t.search.columns.more}>
          {#each hidden as c (c.id)}
            <li>
              <label class="check">
                <input id="{uid}-{c.id}" type="checkbox" onchange={() => void toggle(c.id, true)} />
                <span>{c.name}</span>
              </label>
            </li>
          {/each}
        </ul>
      {/if}
    </div>
    <div class="foot">
      <button
        type="button"
        class="btn btn-small btn-quiet"
        disabled={isDefault}
        onclick={() => onchange([...DEFAULT_COLUMNS])}>{t.search.columns.reset}</button
      >
    </div>
  </FilterPopover>
</div>

<style>
  .body {
    overflow-y: auto;
    padding: 6px 0;
  }

  .list {
    list-style: none;
    margin: 0;
    padding: 0;
  }

  li {
    display: flex;
    align-items: center;
    gap: 4px;
    padding: 1px 8px 1px 12px;
  }

  .check {
    display: flex;
    flex: 1;
    min-width: 0;
    align-items: center;
    gap: 8px;
    padding: 4px 0;
    font-size: 13.5px;
    cursor: pointer;
  }

  .check span {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .arrows {
    display: inline-flex;
    flex: none;
  }

  .more {
    margin: 6px 0 2px;
    padding: 6px 12px 0;
    border-top: 1px solid var(--rule);
    font-size: 12px;
    color: var(--muted);
  }

  .foot {
    display: flex;
    justify-content: flex-end;
    padding: 6px 8px;
    border-top: 1px solid var(--rule);
  }

  /* The button sits at the right end of the tools: the panel opens leftwards. */
  @media (min-width: 601px) {
    .column-picker :global(.filter-popover.wide) {
      left: auto;
      right: 0;
    }
  }
</style>
