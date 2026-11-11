<script lang="ts">
  import { untrack } from 'svelte';
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
  import type { Facets } from '../../core/types.js';
  import { t } from '../../i18n/t.js';
  import { api } from '../../lib/api.js';
  import { refs } from '../../lib/refs.svelte.js';
  import { session } from '../../lib/session.svelte.js';
  import Avatar from '../Avatar.svelte';
  import FilterPopover from './FilterPopover.svelte';
  import ListPicker from './ListPicker.svelte';
  import { catalog, chosenNames, facetCounts, labelOf, rememberEpics } from './sources.svelte.js';

  /**
   * One-click filters under the filter bar: the avatars of the busiest
   * assignees, oneself first, then a list each for type, status, priority
   * and epic. They change the same filters as the bar.
   */
  interface Props {
    filters: FilterState;
    onchange: (next: FilterState, burst: string) => void;
    /** A project the search is already limited to (project page). */
    project?: string | undefined;
  }
  const { filters, onchange, project }: Props = $props();

  const LISTS = ['type', 'status', 'priority', 'epic'] as const satisfies readonly ListField[];

  let openId = $state<string | null>(null);
  let opened = 0;
  let burst = '';

  /** The assignees counted on the search without its own assignee filter. */
  let facet = $state<Facets | null>(null);
  /** Everyone counted so far, by id: a chosen person keeps their avatar once their count is 0. */
  let known = $state<Facets['users']>({});
  let facetSeq = 0;
  $effect(() => {
    const jql = facetJql(filters, 'assignee');
    const mine = ++facetSeq;
    // The epics counted name a chosen epic before its list is opened.
    api.facets({ jql, project, fields: 'assignee,epic' }).then(
      (r) => {
        if (mine !== facetSeq) return;
        for (const u of Object.values(r.users)) {
          refs.users[u.username.toLowerCase()] = u.name;
          catalog.people[u.username.toLowerCase()] = u;
        }
        rememberEpics(r);
        known = { ...untrack(() => known), ...r.users };
        facet = r;
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

  const clear = (f: ListField) => onchange(withoutField(filters, f), `clear:${f}:${++opened}`);
</script>

{#snippet list(field: ListField)}
  <ListPicker
    {filters}
    {field}
    {project}
    onchange={(next) => onchange(next, burst)}
    onreset={() => clear(field)}
    counts={counts?.field === field ? counts.values : null}
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
