<script lang="ts">
  import { DEFAULT_COLUMNS, FIXED_COLUMNS, type ColumnChoice } from '../../core/search-columns.js';
  import { moveItem } from '../../core/catalogs.js';
  import { tick } from 'svelte';
  import { t } from '../../i18n/t.js';
  import OrderArrows from '../admin/OrderArrows.svelte';
  import FilterPopover from './FilterPopover.svelte';

  /**
   * The results' columns: the shown ones in order, which dragging or the
   * arrows change, then a box to find the others. The key and the summary
   * are always shown.
   */
  interface Props {
    choices: ColumnChoice[];
    chosen: readonly string[];
    /** Where the choice is kept, in the footer. */
    note?: string | undefined;
    /** Drawn in place rather than in a popover (inside a dialog, which would clip it). */
    inline?: boolean | undefined;
    onchange: (next: string[]) => void;
  }
  const { choices, chosen, note, inline = false, onchange }: Props = $props();

  let open = $state(false);
  let query = $state('');
  let find: HTMLInputElement | undefined = $state();
  let dragged = $state<number | null>(null);
  let over = $state<number | null>(null);
  const byId = $derived(new Map(choices.map((c) => [c.id, c])));
  const shown = $derived(chosen.flatMap((id) => byId.get(id) ?? []));
  const others = $derived(
    choices
      .filter((c) => !FIXED_COLUMNS.includes(c.id) && !chosen.includes(c.id))
      .sort((a, b) => a.name.localeCompare(b.name)),
  );
  const hidden = $derived.by(() => {
    const q = query.trim().toLocaleLowerCase();
    return q ? others.filter((c) => c.name.toLocaleLowerCase().includes(q)) : others;
  });
  const isDefault = $derived(
    chosen.length === DEFAULT_COLUMNS.length && chosen.every((c, i) => c === DEFAULT_COLUMNS[i]),
  );

  const uid = $props.id();

  /** Ticking moves the row to the other list: the focus follows its box, or goes to the search. */
  async function toggle(id: string, on: boolean) {
    onchange(on ? [...chosen, id] : chosen.filter((x) => x !== id));
    await tick();
    (document.getElementById(`${uid}-${id}`) ?? find)?.focus();
  }

  /** The focus stays on the arrow, or on the other one once it is disabled at an end. */
  async function move(id: string, by: -1 | 1) {
    const i = chosen.indexOf(id);
    onchange(moveItem(chosen, i, i + by));
    await tick();
    const [same, other] = by < 0 ? ['up', 'down'] : ['down', 'up'];
    const arrow = document.getElementById(`order-${same}-col-${id}`) as HTMLButtonElement | null;
    (arrow?.disabled ? document.getElementById(`order-${other}-col-${id}`) : arrow)?.focus();
  }

  function drop(target: number) {
    if (dragged !== null && dragged !== target) {
      const from = chosen.indexOf(shown[dragged]!.id);
      const to = chosen.indexOf(shown[target]!.id);
      onchange(moveItem(chosen, from, to));
    }
    dragged = null;
    over = null;
  }
</script>

{#snippet panel()}
  <div class="body">
    <div class="head">
      <strong>{t.search.columns.shownHead}</strong>
      <span class="muted">{t.search.columns.fixed}</span>
    </div>
    <ul class="list" aria-label={t.search.columns.shown}>
      {#each shown as c, i (c.id)}
        <li
          class:is-dragged={dragged === i}
          class:is-over={over === i && dragged !== null && dragged !== i}
          draggable="true"
          ondragstart={(e) => {
            dragged = i;
            e.dataTransfer?.setData('text/plain', c.id);
          }}
          ondragover={(e) => {
            if (dragged === null) return;
            e.preventDefault();
            over = i;
          }}
          ondragleave={() => over === i && (over = null)}
          ondrop={(e) => {
            e.preventDefault();
            drop(i);
          }}
          ondragend={() => ((dragged = null), (over = null))}
        >
          <span class="grip" aria-hidden="true">⋮⋮</span>
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
    {#if others.length}
      <label class="find">
        <span class="sr-only">{t.search.columns.find}</span>
        <input
          bind:this={find}
          bind:value={query}
          class="input"
          type="search"
          placeholder={t.search.columns.add}
        />
      </label>
      <ul class="list more" aria-label={t.search.columns.more}>
        {#each hidden as c (c.id)}
          <li>
            <label class="check">
              <input id="{uid}-{c.id}" type="checkbox" onchange={() => void toggle(c.id, true)} />
              <span>{c.name}</span>
              {#if c.id.startsWith('customfield_')}
                <span class="tag muted">{t.search.columns.custom}</span>
              {/if}
            </label>
          </li>
        {:else}
          <li class="none muted">{t.search.columns.none}</li>
        {/each}
      </ul>
    {/if}
  </div>
  <div class="foot">
    {#if note}<span class="muted">{note}</span>{/if}
    <button
      type="button"
      class="btn btn-small btn-quiet"
      disabled={isDefault}
      onclick={() => onchange([...DEFAULT_COLUMNS])}>{t.search.columns.reset}</button
    >
  </div>
{/snippet}

<div class="column-picker">
  {#if inline}
    <div class="inline">{@render panel()}</div>
  {:else}
    <FilterPopover label={t.search.columns.button} summary={String(shown.length)} bind:open wide>
      {@render panel()}
    </FilterPopover>
  {/if}
</div>

<style>
  .inline {
    border: 1px solid var(--rule);
    border-radius: var(--radius-s);
  }

  .body {
    overflow-y: auto;
    padding: 6px 0;
  }

  .head {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    justify-content: space-between;
    gap: 2px 8px;
    padding: 4px 12px;
    font-size: 0.84375rem;
  }

  .head .muted,
  .foot .muted,
  .tag {
    font-size: 0.75rem;
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

  li[draggable='true'] {
    padding-left: 4px;
  }

  .grip {
    flex: none;
    width: 14px;
    color: var(--muted);
    font-size: 0.6875rem;
    letter-spacing: -2px;
    text-align: center;
    cursor: grab;
  }

  li.is-dragged {
    opacity: 0.45;
  }

  li.is-over {
    box-shadow: inset 0 2px 0 var(--accent);
  }

  .check {
    display: flex;
    flex: 1;
    min-width: 0;
    align-items: center;
    gap: 8px;
    padding: 4px 0;
    font-size: 0.84375rem;
    cursor: pointer;
  }

  .check span {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .check .tag {
    flex: none;
    margin-left: auto;
  }

  .arrows {
    display: inline-flex;
    flex: none;
  }

  .find {
    display: block;
    margin-top: 6px;
    padding: 8px 12px 4px;
    border-top: 1px solid var(--rule);
  }

  .find .input {
    width: 100%;
  }

  .more li {
    padding-left: 30px;
  }

  .more .none {
    padding: 6px 12px;
    font-size: 0.8125rem;
  }

  .foot {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: 8px;
    padding: 6px 8px 6px 12px;
    border-top: 1px solid var(--rule);
  }

  .foot .muted {
    margin-right: auto;
  }

  /* The button sits at the right end of the tools: the panel opens leftwards. */
  @media (min-width: 601px) {
    .column-picker :global(.filter-popover.wide) {
      left: auto;
      right: 0;
    }
  }
</style>
