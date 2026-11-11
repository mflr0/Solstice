<script lang="ts">
  import { untrack, type Snippet } from 'svelte';
  import { presentOptions } from '../../core/board.js';
  import {
    facetJql,
    hasAssignee,
    quickUsers,
    sameValue,
    toggleAssignee,
    withoutField,
    type FilterState,
    type ListField,
  } from '../../core/filters.js';
  import { quote } from '../../core/jql.js';
  import { matches } from '../../core/text.js';
  import type { Facets } from '../../core/types.js';
  import { t } from '../../i18n/t.js';
  import { api } from '../../lib/api.js';
  import { refs } from '../../lib/refs.svelte.js';
  import { session } from '../../lib/session.svelte.js';
  import Avatar from '../Avatar.svelte';
  import FilterPopover from './FilterPopover.svelte';
  import ListPicker from './ListPicker.svelte';
  import {
    catalog,
    chosenNames,
    facetCounts,
    labelOf,
    loadOptions,
    rememberEpics,
    type Option,
  } from './sources.svelte.js';

  /**
   * One-click filters under the filter bar: the avatars of the busiest
   * assignees, oneself first, then a list each for type, status, priority
   * and epic. They change the same filters as the bar. On a board, the
   * values and their counts come from its cards, and the board's own quick
   * filters close the row.
   */
  interface Props {
    filters: FilterState;
    onchange: (next: FilterState, burst: string) => void;
    /** A project the search is already limited to (project page). */
    project?: string | undefined;
    /** A board's counts (cardFacets) in place of the search's; null while its cards load. */
    cards?: Facets | null | undefined;
    /** What follows the lists (the board's quick filters). */
    children?: Snippet | undefined;
  }
  const { filters, onchange, project, cards, children }: Props = $props();
  const onBoard = $derived(cards !== undefined);

  const LISTS = ['type', 'status', 'priority', 'epic'] as const satisfies readonly ListField[];

  let openId = $state<string | null>(null);
  let opened = 0;
  let burst = '';

  /** The assignees counted on the search without its own assignee filter. */
  let facet = $state<Facets | null>(null);
  /** Everyone counted so far, by id: a chosen person keeps their avatar once their count is 0. */
  let known = $state<Facets['users']>({});
  let facetSeq = 0;
  function remember(r: Facets) {
    for (const u of Object.values(r.users)) {
      refs.users[u.username.toLowerCase()] = u.name;
      catalog.people[u.username.toLowerCase()] = u;
    }
    rememberEpics(r);
    known = { ...untrack(() => known), ...r.users };
    facet = r;
  }

  $effect(() => {
    if (cards !== undefined) {
      if (cards) remember(cards);
      return;
    }
    const jql = facetJql(filters, 'assignee');
    const mine = ++facetSeq;
    // The epics counted name a chosen epic before its list is opened.
    api.facets({ jql, project, fields: 'assignee,epic' }).then(
      (r) => {
        if (mine === facetSeq) remember(r);
      },
      // Without the counts, the row shows oneself only.
      () => undefined,
    );
  });

  const people = $derived(
    quickUsers(facet?.facets.assignee ?? {}, known, session.me, filters.values.assignee),
  );
  /** Chosen assignees without an avatar here, named on "+n". */
  const hidden = $derived(
    (filters.values.assignee ?? []).filter((v) => !people.users.some((u) => sameValue(u.value, v))),
  );

  function hasValue(f: ListField): boolean {
    if (f === 'status' && filters.categories.length) return true;
    return !!filters.values[f]?.length;
  }

  /** "Bug", "Bug +1". */
  function summary(f: ListField): string {
    const names = chosenNames(filters, f);
    if (names.length <= 1) return names[0] ?? '';
    return `${names[0] ?? ''} ${t.search.andMore(names.length - 1)}`;
  }

  /** Issues per value of the open list, as in the filter bar. */
  let counts = $state<{ field: ListField; values: Record<string, number> } | null>(null);
  let countSeq = 0;

  async function loadCounts(field: ListField) {
    const mine = ++countSeq;
    counts = null;
    if (onBoard) return;
    try {
      const values = await facetCounts(filters, field, project);
      if (values && mine === countSeq) counts = { field, values };
    } catch {
      // The lists work without their counts.
    }
  }

  function openFlag(field: ListField) {
    return {
      get: () => openId === field,
      set: (v: boolean) => {
        if (v) {
          openId = field;
          burst = `quick:${field}:${++opened}`;
          void loadCounts(field);
        } else if (openId === field) {
          openId = null;
        }
      },
    };
  }

  const moreFlag = openFlag('assignee');

  /** A list's values on a board: those its cards have, the busiest first for people. */
  function boardLoad(field: ListField, src: Facets): (q: string) => Promise<Option[]> {
    const n = src.facets[field] ?? {};
    return async (q) => {
      if (field === 'assignee') {
        return Object.values(src.users)
          .filter((u) => matches(q, u.name, u.username))
          .sort((a, b) => (n[b.id] ?? 0) - (n[a.id] ?? 0) || a.name.localeCompare(b.name))
          .map((u) => ({
            value: quote(u.username),
            label: u.name,
            hint: u.username,
            facets: [String(u.id)],
            user: u,
          }));
      }
      if (field === 'epic') {
        return Object.values(src.epics)
          .filter((e) => matches(q, e.summary, e.key))
          .map((e) => ({ value: e.key, label: e.summary, hint: e.key, facets: [e.key] }));
      }
      return presentOptions(await loadOptions(field, q, []), n);
    };
  }

  function countsOf(field: ListField): Record<string, number> | null {
    if (cards !== undefined) return cards?.facets[field] ?? null;
    return counts?.field === field ? counts.values : null;
  }

  const clear = (f: ListField) => onchange(withoutField(filters, f), `clear:${f}:${++opened}`);
</script>

{#snippet list(field: ListField)}
  <ListPicker
    {filters}
    {field}
    {project}
    onchange={(next) => onchange(next, burst)}
    onreset={() => clear(field)}
    counts={countsOf(field)}
    load={cards ? boardLoad(field, cards) : undefined}
  />
{/snippet}

<div class="quick-filters" role="group" aria-label={t.search.quickFilters}>
  <div class="people">
    {#each people.users as u (u.id)}
      {@const pressed = hasAssignee(filters, u.value)}
      <button
        type="button"
        class="btn btn-small person"
        class:pressed
        aria-pressed={pressed}
        aria-label={u.name}
        title={u.name}
        onclick={() => onchange(toggleAssignee(filters, u.value), `person:${++opened}`)}
      >
        <Avatar user={u} />
      </button>
    {/each}
    <FilterPopover
      label={people.more > 0 ? t.search.andMore(people.more) : '+'}
      name={t.search.moreAssignees}
      summary={hidden.map((v) => labelOf('assignee', v)).join(', ')}
      active={hidden.length > 0}
      bind:open={moreFlag.get, moreFlag.set}
    >
      {@render list('assignee')}
    </FilterPopover>
  </div>
  {#each LISTS as f (f)}
    {@const flag = openFlag(f)}
    <FilterPopover
      label={t.search.fields[f]}
      summary={summary(f)}
      active={hasValue(f)}
      bind:open={flag.get, flag.set}
      onclear={() => clear(f)}
    >
      {@render list(f)}
    </FilterPopover>
  {/each}
  {@render children?.()}
</div>

<style>
  .quick-filters {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 6px;
    margin-top: 8px;
  }

  .people {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 4px;
    margin-right: 6px;
  }

  .person {
    padding: 0 2px;
    min-width: 0;
  }

  /* Pressed: the button's frame and fill, as an active filter. */
  .person.pressed {
    border-color: var(--ink);
    background-color: var(--paper-sunk);
    box-shadow: inset 0 0 0 1px var(--ink);
  }
</style>
