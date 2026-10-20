<script lang="ts">
  import { untrack } from 'svelte';
  import {
    EMPTY,
    emptyFilters,
    FACET_FIELDS,
    facetJql,
    isBlank,
    matchPreset,
    ME,
    rangeDays,
    rangeSet,
    sameValue,
    toggle,
    valueOf,
    type DateField,
    type DateRange,
    type FilterState,
    type ListField,
    type PeopleRole,
  } from '../../core/filters.js';
  import type { StatusCategory } from '../../core/types.js';
  import { categoryLabels, t } from '../../i18n/t.js';
  import { formatList } from '../../i18n/format.js';
  import { api } from '../../lib/api.js';
  import { session } from '../../lib/session.svelte.js';
  import DateRangePicker from './DateRangePicker.svelte';
  import FilterPopover from './FilterPopover.svelte';
  import { labelOf, loadOptions, specialValues } from './sources.svelte.js';
  import ValuePicker from './ValuePicker.svelte';

  /**
   * Search without a syntax: a text field and one button per criterion,
   * each opening a list to tick. Every change is applied at once; burst
   * names the interaction (one popover, the text field) so the page can
   * keep a single history entry for it.
   */
  interface Props {
    filters: FilterState;
    onchange: (next: FilterState, burst: string) => void;
    /** A project the search is already limited to (project page). */
    project?: string | undefined;
    autofocus?: boolean | undefined;
  }
  const { filters, onchange, project, autofocus = false }: Props = $props();

  type Extra = Exclude<ListField, (typeof MAIN)[number]> | DateField;
  const MAIN = ['people', 'project', 'type', 'status', 'assignee', 'priority', 'labels'] as const;
  /** The roles offered for "People"; the assignee and the reporter hand over to their own filters. */
  const ROLES = ['participant', 'assignee', 'reporter', 'creator', 'watcher', 'commenter'] as const;
  const EXTRAS: Extra[] = [
    'reporter',
    'resolution',
    'sprint',
    'epic',
    'fixVersion',
    'component',
    'created',
    'updated',
    'due',
    'resolved',
  ];
  const DATES: readonly string[] = ['created', 'updated', 'due', 'resolved'];
  const isDate = (f: Extra): f is DateField => DATES.includes(f);
  const OPEN: StatusCategory[] = ['todo', 'doing'];
  const DONE: StatusCategory[] = ['done'];

  const main = $derived(MAIN.filter((f) => !(project && f === 'project')));
  /** Extra criteria added through "More", shown even without a value. */
  let added = $state<Extra[]>([]);
  let openId = $state<string | null>(null);
  let opened = 0;
  let burst = '';
  const extras = $derived(EXTRAS.filter((f) => added.includes(f) || hasValue(f)));

  // The text field follows the filters (back, forward) but not while typing.
  let text = $state(untrack(() => filters.text));
  let timer: ReturnType<typeof setTimeout> | undefined;
  let textInput: HTMLInputElement | undefined = $state();
  $effect(() => {
    const v = filters.text;
    if (v !== untrack(() => text).trim()) text = v;
  });
  $effect(() => {
    if (autofocus) textInput?.focus();
  });

  function hasValue(f: ListField | DateField): boolean {
    if (DATES.includes(f)) return rangeSet(filters.dates[f as DateField]);
    if (f === 'status' && filters.categories.length) return true;
    return !!filters.values[f as ListField]?.length;
  }

  /** Opens one popover at a time; each opening starts a new burst. */
  function openFlag(id: string) {
    return {
      get: () => openId === id,
      set: (v: boolean) => {
        if (v) {
          openId = id;
          burst = `${id}:${++opened}`;
          void loadCounts(id);
        } else if (openId === id) {
          openId = null;
        }
      },
    };
  }

  const moreFlag = openFlag('more');

  function change(next: FilterState, b = burst) {
    onchange(next, b);
  }

  function setValues(field: ListField, values: string[]) {
    const next = { ...filters, values: { ...filters.values, [field]: values } };
    if (values.length === 0) delete next.values[field];
    if (field === 'people') next.peopleRole = peopleRole;
    // A status and a status category do not mix: one replaces the other.
    if (field === 'status' && values.length) next.categories = [];
    change(next);
  }

  function setCategories(cats: StatusCategory[]) {
    const values = { ...filters.values };
    if (cats.length) delete values.status;
    change({ ...filters, values, categories: cats });
  }

  function setRange(field: DateField, r: DateRange | undefined) {
    const dates = { ...filters.dates };
    if (r) dates[field] = r;
    else delete dates[field];
    change({ ...filters, dates });
  }

  function clearField(f: ListField | DateField) {
    if (DATES.includes(f)) return setRange(f as DateField, undefined);
    const next = { ...filters, values: { ...filters.values } };
    delete next.values[f as ListField];
    if (f === 'status') next.categories = [];
    if (f === 'people') {
      next.peopleRole = 'participant';
      roleDraft = null;
    }
    change(next, `clear:${f}:${++opened}`);
  }

  /**
   * The role of "People". Chosen before anyone is picked, it changes no
   * query yet, so it is kept here until then.
   */
  let roleDraft = $state<PeopleRole | null>(null);
  const peopleRole = $derived<PeopleRole>(
    filters.values.people?.length ? filters.peopleRole : (roleDraft ?? filters.peopleRole),
  );

  function setRole(role: (typeof ROLES)[number]) {
    const people = (filters.values.people ?? []).filter((v) => v !== EMPTY);
    if (role === 'assignee' || role === 'reporter') {
      // One filter per field: the people chosen go to that field's own filter.
      const values = { ...filters.values };
      delete values.people;
      const merged = [...(values[role] ?? [])];
      for (const v of people) if (!merged.some((m) => sameValue(m, v))) merged.push(v);
      if (merged.length) values[role] = merged;
      roleDraft = null;
      if (people.length) change({ ...filters, values, peopleRole: 'participant' });
      if (role === 'reporter' && !added.includes('reporter')) added = [...added, 'reporter'];
      openFlag(role).set(true);
      return;
    }
    roleDraft = role;
    if (!filters.values.people?.length) return;
    // Nobody takes part in everything: "No watchers" and "No comments" only fit those roles.
    const keep = role === 'watcher' || role === 'commenter' ? filters.values.people : people;
    const values: FilterState['values'] = { ...filters.values, people: keep };
    if (!keep.length) delete values.people;
    change({ ...filters, values, peopleRole: role });
  }

  function peopleSummary(): string {
    const names = (filters.values.people ?? []).map((v) => labelOf('people', v, peopleRole));
    if (!names.length) return '';
    const role = peopleRole === 'participant' ? '' : ` · ${t.search.roles[peopleRole] ?? ''}`;
    return summarize(names) + role;
  }

  /** "Issues I’m watching" is "People", as watcher, with Me. */
  const watchingMe = $derived(
    filters.peopleRole === 'watcher' && (filters.values.people ?? []).includes(ME),
  );
  const peopleBusy = $derived(filters.peopleRole !== 'watcher' && !!filters.values.people?.length);

  function toggleWatching() {
    const current = filters.peopleRole === 'watcher' ? (filters.values.people ?? []) : [];
    const people = toggle(current, ME);
    const values: FilterState['values'] = { ...filters.values, people };
    if (!people.length) delete values.people;
    roleDraft = null;
    change(
      { ...filters, values, peopleRole: people.length ? 'watcher' : 'participant' },
      `watch:${++opened}`,
    );
  }

  const sameSet = (a: StatusCategory[], b: StatusCategory[]) =>
    a.length === b.length && a.every((c) => b.includes(c));

  function categorySummary(cats: StatusCategory[]): string {
    if (cats.length === 0 || cats.length === 3) return '';
    if (sameSet(cats, OPEN)) return t.search.special.open;
    if (sameSet(cats, DONE)) return t.search.special.done;
    return t.search.categoryOnly(formatList(cats.map((c) => categoryLabels[c])));
  }

  /** "In progress, To do", "In progress, To do +1". */
  function summarize(names: string[]): string {
    if (names.length <= 2) return names.join(', ');
    return `${names.slice(0, 2).join(', ')} ${t.search.andMore(names.length - 2)}`;
  }

  function listSummary(field: ListField): string {
    const names = (filters.values[field] ?? []).map((v) => labelOf(field, v));
    if (field === 'status') {
      const c = categorySummary(filters.categories);
      if (c) names.unshift(c);
    }
    return summarize(names);
  }

  function dateSummary(field: DateField): string {
    const r = filters.dates[field];
    if (!r || !rangeSet(r)) return '';
    const preset = matchPreset(field, r);
    if (preset) return t.search.datePresets[preset] ?? preset;
    if (r.empty) return t.search.noDate;
    const days = rangeDays(r);
    const from = r.from
      ? days.from
        ? t.search.day(days.from)
        : `${r.from.op} ${r.from.value}`
      : '';
    const to = r.to ? (days.to ? t.search.day(days.to) : `${r.to.op} ${r.to.value}`) : '';
    if (from && to) return t.search.between(from, to);
    if (from) return days.from ? t.search.since(from) : from;
    return days.to ? t.search.until(to) : to;
  }

  /**
   * Issues per value of the field whose list is open, counted on the search
   * without that field's own filter: ticking a value shows what it adds.
   * Fetched when the list opens; it stays right while it is open, since only
   * that field changes then.
   */
  let counts = $state<{ field: string; values: Record<string, number> } | null>(null);
  let countSeq = 0;

  async function loadCounts(id: string) {
    const mine = ++countSeq;
    counts = null;
    const field = FACET_FIELDS.find((f) => f === id);
    if (!field) return;
    try {
      const r = await api.facets({ jql: facetJql(filters, field), project, fields: field });
      if (mine === countSeq) counts = { field, values: r.facets[field] ?? {} };
    } catch {
      // The lists work without their counts.
    }
  }

  const countsFor = (field: ListField) => (counts?.field === field ? counts.values : null);

  /** Issues in some status categories, from the status counts. */
  function categoryCount(cats: StatusCategory[]): number | undefined {
    const c = countsFor('status');
    if (!c) return undefined;
    return (session.meta?.statuses ?? [])
      .filter((st) => cats.includes(st.category))
      .reduce((n, st) => n + (c[st.id] ?? 0), 0);
  }

  function shortcutsFor(field: ListField) {
    if (field !== 'status') return [];
    return [
      {
        label: t.search.special.open,
        checked: sameSet(filters.categories, OPEN),
        toggle: () => setCategories(sameSet(filters.categories, OPEN) ? [] : OPEN),
        count: categoryCount(OPEN),
      },
      {
        label: t.search.special.done,
        checked: sameSet(filters.categories, DONE),
        toggle: () => setCategories(sameSet(filters.categories, DONE) ? [] : DONE),
        count: categoryCount(DONE),
      },
    ];
  }

  /** Fields whose values may be typed in freely (anything the server will match by name). */
  const FREE: ListField[] = ['labels', 'fixVersion', 'component', 'epic'];

  function scope(): string[] {
    return project ? [valueOf(project)] : (filters.values.project ?? []);
  }

  function typed() {
    clearTimeout(timer);
    timer = setTimeout(commitText, 350);
  }

  function commitText() {
    clearTimeout(timer);
    if (text.trim() === filters.text.trim()) return;
    change({ ...filters, text: text.trim() }, 'text');
  }

  function toggleExtra(f: Extra) {
    if (added.includes(f) || hasValue(f)) {
      added = added.filter((x) => x !== f);
      if (hasValue(f)) clearField(f);
      return;
    }
    added = [...added, f];
    // Straight to the new criterion's list.
    openFlag(f).set(true);
  }

  function clearAll() {
    added = [];
    text = '';
    change({ ...emptyFilters(), order: filters.order }, `clear:all:${++opened}`);
  }
</script>

{#snippet listFilter(field: ListField)}
  {@const flag = openFlag(field)}
  <FilterPopover
    label={t.search.fields[field]}
    summary={listSummary(field)}
    active={hasValue(field)}
    bind:open={flag.get, flag.set}
    onclear={() => clearField(field)}
  >
    <ValuePicker
      label={t.search.fields[field]}
      selected={filters.values[field] ?? []}
      onchange={(v) => setValues(field, v)}
      load={(q) => loadOptions(field, q, scope())}
      special={specialValues(field)}
      shortcuts={shortcutsFor(field)}
      labelOf={(v) => labelOf(field, v)}
      free={FREE.includes(field)
        ? (s) => (field === 'epic' ? s.toUpperCase() : valueOf(s))
        : undefined}
      onreset={() => clearField(field)}
      counts={countsFor(field)}
    />
  </FilterPopover>
{/snippet}

{#snippet peopleFilter()}
  {@const flag = openFlag('people')}
  <FilterPopover
    label={t.search.fields.people}
    summary={peopleSummary()}
    active={hasValue('people')}
    bind:open={flag.get, flag.set}
    onclear={() => clearField('people')}
  >
    <div class="role">
      <label class="role-field">
        <span>{t.search.role}</span>
        <select
          class="select"
          value={peopleRole}
          onchange={(e) => setRole(e.currentTarget.value as (typeof ROLES)[number])}
        >
          {#each ROLES as r (r)}
            <option value={r}>{t.search.roles[r]}</option>
          {/each}
        </select>
      </label>
      <p class="role-hint">{t.search.roleHint}</p>
    </div>
    <ValuePicker
      label={t.search.fields.people}
      selected={filters.values.people ?? []}
      onchange={(v) => setValues('people', v)}
      load={(q) => loadOptions('people', q, scope())}
      special={specialValues('people', peopleRole)}
      labelOf={(v) => labelOf('people', v, peopleRole)}
      onreset={() => clearField('people')}
    />
  </FilterPopover>
{/snippet}

{#snippet dateFilter(field: DateField)}
  {@const flag = openFlag(field)}
  <FilterPopover
    label={t.search.fields[field]}
    summary={dateSummary(field)}
    active={hasValue(field)}
    bind:open={flag.get, flag.set}
    onclear={() => clearField(field)}
    wide
  >
    <DateRangePicker
      {field}
      label={t.search.fields[field]}
      range={filters.dates[field]}
      onchange={(r) => setRange(field, r)}
    />
  </FilterPopover>
{/snippet}

<div class="filter-bar" role="group" aria-label={t.search.filters}>
  <div class="filter-text">
    <span class="glass" aria-hidden="true">⌕</span>
    <input
      bind:this={textInput}
      class="input"
      type="search"
      bind:value={text}
      oninput={typed}
      onkeydown={(e) => e.key === 'Enter' && commitText()}
      onblur={commitText}
      placeholder={t.search.textPlaceholder}
      aria-label={t.search.text}
      autocomplete="off"
      spellcheck="false"
    />
  </div>
  <div class="filter-buttons">
    {#each main as f (f)}
      {#if f === 'people'}
        {@render peopleFilter()}
      {:else}
        {@render listFilter(f)}
      {/if}
    {/each}
    {#each extras as f (f)}
      {#if isDate(f)}
        {@render dateFilter(f)}
      {:else}
        {@render listFilter(f)}
      {/if}
    {/each}
    <FilterPopover label={t.search.more} bind:open={moreFlag.get, moreFlag.set} wide>
      <ul class="more-list" aria-label={t.search.moreLabel}>
        {#each EXTRAS as f (f)}
          <li>
            <label class="item">
              <input
                type="checkbox"
                class="checkbox"
                checked={added.includes(f) || hasValue(f)}
                onchange={() => toggleExtra(f)}
              />
              <span>{t.search.fields[f]}</span>
            </label>
          </li>
        {/each}
        <li class="sep" role="presentation"></li>
        <li>
          <label class="item" class:busy={peopleBusy}>
            <input
              type="checkbox"
              class="checkbox"
              checked={watchingMe}
              disabled={peopleBusy}
              aria-describedby={peopleBusy ? 'watching-busy' : undefined}
              onchange={toggleWatching}
            />
            <span>{t.search.watching}</span>
          </label>
          {#if peopleBusy}<p class="busy-note" id="watching-busy">{t.search.watchingBusy}</p>{/if}
        </li>
      </ul>
    </FilterPopover>
    {#if !isBlank(filters)}
      <button type="button" class="link-btn clear-all" onclick={clearAll}
        >{t.search.clearAll}</button
      >
    {/if}
  </div>
</div>

<style>
  .filter-bar {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px 10px;
  }

  .filter-text {
    position: relative;
    flex: 1 1 240px;
    min-width: 0;
    max-width: 420px;
  }

  .filter-text .glass {
    position: absolute;
    left: 9px;
    top: 50%;
    transform: translateY(-52%);
    color: var(--muted);
    font-size: 16px;
    pointer-events: none;
  }

  .filter-text .input {
    height: 32px;
    padding-left: 28px;
  }

  .filter-buttons {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 6px;
    min-width: 0;
    flex: 1 1 auto;
  }

  .role {
    padding: 8px 8px 2px;
    border-bottom: 1px solid var(--rule);
    flex: none;
  }

  .role-field {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 12.5px;
    color: var(--ink-soft);
  }

  .role-field .select {
    flex: 1;
    height: 28px;
    padding: 2px 6px;
    font-size: 13px;
  }

  .role-hint,
  .busy-note {
    margin: 4px 0 6px;
    font-size: 11.5px;
    line-height: 1.35;
    color: var(--muted);
  }

  .busy-note {
    margin: -2px 12px 6px 36px;
  }

  .more-list .item.busy {
    cursor: default;
    color: var(--muted);
  }

  .more-list {
    list-style: none;
    margin: 0;
    padding: 4px 0;
    overflow: auto;
  }

  .more-list .item {
    cursor: pointer;
  }

  .more-list input {
    margin: 0;
  }

  .clear-all {
    font-size: 12.5px;
    margin-left: 4px;
  }

  @media (max-width: 600px) {
    .filter-text {
      flex-basis: 100%;
      max-width: none;
    }
  }
</style>
