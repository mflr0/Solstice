<script lang="ts">
  import {
    cardFilterParams,
    hasCardFilter,
    toggleIn,
    type CardFacets,
    type CardFilter,
  } from '../../core/board.js';
  import type { QuickFilter } from '../../core/board-types.js';
  import type { PriorityRef, TypeRef, UserRef } from '../../core/types.js';
  import { t } from '../../i18n/t.js';
  import { router } from '../../lib/router.svelte.js';
  import { session } from '../../lib/session.svelte.js';
  import Avatar from '../Avatar.svelte';
  import PriorityMark from '../PriorityMark.svelte';
  import FilterPopover from '../search/FilterPopover.svelte';
  import TypeMark from '../TypeMark.svelte';
  import QuickFilters from './QuickFilters.svelte';

  /**
   * The bar above a board: the cards' type, assignee and priority, each a
   * checklist of the values on the loaded cards, then the board's quick
   * filters. The choice lives in the address (?type=…&assignee=…&priority=…).
   */
  interface Props {
    filter: CardFilter;
    facets: CardFacets;
    quickFilters: QuickFilter[];
    quick: number[];
  }
  const { filter, facets, quickFilters, quick }: Props = $props();

  interface Row {
    key: string;
    label: string;
    checked: boolean;
    toggle: () => void;
    type?: TypeRef;
    user?: UserRef;
    priority?: PriorityRef;
  }

  let open = $state({ type: false, assignee: false, priority: false });

  function set(next: CardFilter) {
    router.setParams(cardFilterParams(next));
  }

  function summarize(names: string[]): string {
    if (names.length <= 2) return names.join(', ');
    return `${names.slice(0, 2).join(', ')} ${t.search.andMore(names.length - 2)}`;
  }

  const userName = (a: CardFilter['assignees'][number]) =>
    a === 'me'
      ? t.boards.filterMe
      : a === 'none'
        ? t.boards.unassigned
        : (facets.users.find((u) => u.id === a)?.name ?? '');
  const priorityName = (id: string) => {
    const p = facets.priorities.find((x) => x.id === id);
    return p ? session.priorityName(p) : t.boards.filterNoPriority;
  };

  const typeRows = $derived<Row[]>(
    facets.types.map((ty) => ({
      key: ty.id,
      label: session.typeName(ty),
      checked: filter.types.includes(ty.id),
      toggle: () => set({ ...filter, types: toggleIn(filter.types, ty.id) }),
      type: ty,
    })),
  );
  const assigneeRows = $derived<Row[]>(
    (['me', ...facets.users.map((u) => u.id), 'none'] as const).map((a) => {
      const user = typeof a === 'number' ? facets.users.find((u) => u.id === a) : undefined;
      return {
        key: String(a),
        label: userName(a),
        checked: filter.assignees.includes(a),
        toggle: () => set({ ...filter, assignees: toggleIn(filter.assignees, a) }),
        ...(user ? { user } : {}),
      };
    }),
  );
  const priorityRows = $derived<Row[]>([
    ...facets.priorities.map((p) => ({
      key: p.id,
      label: session.priorityName(p),
      checked: filter.priorities.includes(p.id),
      toggle: () => set({ ...filter, priorities: toggleIn(filter.priorities, p.id) }),
      priority: p,
    })),
    ...(facets.noPriority
      ? [
          {
            key: 'none',
            label: t.boards.filterNoPriority,
            checked: filter.priorities.includes('none'),
            toggle: () => set({ ...filter, priorities: toggleIn(filter.priorities, 'none') }),
          },
        ]
      : []),
  ]);

  function clearAll() {
    router.setParams({
      ...cardFilterParams({ types: [], assignees: [], priorities: [] }),
      quick: null,
    });
  }
</script>

{#snippet checklist(label: string, rows: Row[])}
  <ul class="card-options" aria-label={label}>
    {#each rows as row (row.key)}
      <li>
        <label class="item card-option">
          <input type="checkbox" checked={row.checked} onchange={row.toggle} />
          <!-- The marks of types and priorities carry the name for screen readers. -->
          {#if row.type}
            <TypeMark type={row.type} withName />
          {:else if row.priority}
            <PriorityMark priority={row.priority} withName />
          {:else}
            {#if row.user}<Avatar user={row.user} />{/if}
            <span class="label">{row.label}</span>
          {/if}
        </label>
      </li>
    {/each}
  </ul>
{/snippet}

<div class="card-filters" role="group" aria-label={t.boards.cardFilters}>
  <FilterPopover
    label={t.boards.filterType}
    summary={summarize(filter.types.map((id) => typeRows.find((r) => r.key === id)?.label ?? ''))}
    active={filter.types.length > 0}
    bind:open={open.type}
    onclear={() => set({ ...filter, types: [] })}
  >
    {@render checklist(t.boards.filterType, typeRows)}
  </FilterPopover>
  <FilterPopover
    label={t.boards.filterAssignee}
    summary={summarize(filter.assignees.map(userName))}
    active={filter.assignees.length > 0}
    bind:open={open.assignee}
    onclear={() => set({ ...filter, assignees: [] })}
  >
    {@render checklist(t.boards.filterAssignee, assigneeRows)}
  </FilterPopover>
  <FilterPopover
    label={t.boards.filterPriority}
    summary={summarize(filter.priorities.map(priorityName))}
    active={filter.priorities.length > 0}
    bind:open={open.priority}
    onclear={() => set({ ...filter, priorities: [] })}
  >
    {@render checklist(t.boards.filterPriority, priorityRows)}
  </FilterPopover>
  <QuickFilters filters={quickFilters} selected={quick} inline />
  {#if hasCardFilter(filter) || quick.length}
    <button type="button" class="link-btn clear" onclick={clearAll}>{t.boards.clearFilters}</button>
  {/if}
</div>

<style>
  .card-filters {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 6px 10px;
    margin: 0 0 14px;
  }

  .card-options {
    list-style: none;
    margin: 0;
    padding: 4px 0;
    overflow: auto;
    min-height: 0;
  }

  .card-option input {
    margin: 0;
    flex: none;
  }

  .label {
    min-width: 0;
    overflow-wrap: anywhere;
  }

  .clear {
    font-size: 0.78125rem;
  }

  @media (max-width: 600px) {
    .card-option {
      padding-top: 9px;
      padding-bottom: 9px;
    }
  }
</style>
