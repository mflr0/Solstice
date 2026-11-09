<script lang="ts">
  import { tick, untrack } from 'svelte';
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
    type DateField,
    type DateRange,
    type FilterState,
    type ListField,
    type PeopleRole,
  } from '../../core/filters.js';
  import { quote } from '../../core/jql.js';
  import { matches } from '../../core/text.js';
  import type { StatusCategory } from '../../core/types.js';
  import { categoryLabels, t } from '../../i18n/t.js';
  import { formatList } from '../../i18n/format.js';
  import { api } from '../../lib/api.js';
  import { autofocus as focusOnMount } from '../../lib/actions.js';
  import { session } from '../../lib/session.svelte.js';
  import Dropdown from '../Dropdown.svelte';
  import DateRangePicker from './DateRangePicker.svelte';
  import FilterPopover from './FilterPopover.svelte';
  import { hasMarks, labelOf, loadOptions, markOf, specialValues } from './sources.svelte.js';
  import ValuePicker from './ValuePicker.svelte';

  /**
   * Search without a syntax: a text field, one chip per criterion in use,
   * and "+ Filter", which picks a field and then its values. Every change is
   * applied at once, so a criterion exists only once it has a value; burst
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

  type Field = ListField | DateField;
  const GROUPS: { id: 'common' | 'dates' | 'planning'; fields: Field[] }[] = [
    {
      id: 'common',
      fields: [
        'people',
        'project',
        'type',
        'status',
        'assignee',
        'reporter',
        'priority',
        'labels',
        'resolution',
      ],
    },
    { id: 'dates', fields: ['created', 'updated', 'due', 'resolved'] },
    { id: 'planning', fields: ['sprint', 'epic', 'fixVersion', 'component'] },
  ];
  /** The roles offered for "People"; the assignee and the reporter hand over to their own filters. */
  const ROLES = ['participant', 'assignee', 'reporter', 'creator', 'watcher', 'commenter'] as const;
  const DATES: readonly string[] = ['created', 'updated', 'due', 'resolved'];
  const isDate = (f: string): f is DateField => DATES.includes(f);
  const OPEN: StatusCategory[] = ['todo', 'doing'];
  const DONE: StatusCategory[] = ['done'];

  const groups = $derived(
    GROUPS.map((g) => ({ ...g, fields: g.fields.filter((f) => !(project && f === 'project')) })),
  );
  const fields = $derived(groups.flatMap((g) => g.fields));
  let openId = $state<string | null>(null);
  let opened = 0;
  let burst = '';
  let bar: HTMLDivElement | undefined = $state();
  /** The criteria in use, and the one whose list is open: its last value may just have been unticked. */
  const chips = $derived(fields.filter((f) => hasValue(f) || openId === f));

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
          if ((fields as string[]).includes(id) && !hasValue(id as Field)) refocusAdd();
        }
      },
    };
  }

  /**
   * A chip closed without a value goes away, and the focus with it if it
   * was there: it moves to "+ Filter".
   */
  function refocusAdd() {
    if (!bar?.contains(document.activeElement)) return;
    void tick().then(() => {
      const at = document.activeElement;
      if (at && at !== document.body && at.isConnected) return;
      bar?.querySelector<HTMLElement>('.add-filter .filter-button')?.focus();
    });
  }

  /** The field "+ Filter" shows the values of, and the search over the fields. */
  let addField = $state<Field | null>(null);
  let fieldQuery = $state('');
  let fieldActive = $state(0);
  const uid = $props.id();
  const fieldListId = `fb-${uid}`;
  const addFlag = {
    get: () => openId === 'add',
    set: (v: boolean) => {
      if (v) {
        addField = null;
        fieldQuery = '';
        fieldActive = 0;
      }
      openFlag('add').set(v);
    },
  };
  const shownGroups = $derived(
    groups
      .map((g) => ({
        ...g,
        fields: g.fields.filter((f) => matches(fieldQuery, t.search.fields[f])),
      }))
      .filter((g) => g.fields.length),
  );
  const shownFields = $derived(shownGroups.flatMap((g) => g.fields));

  /** Shows a field's values; the popover keeps its burst. */
  function chooseField(f: Field) {
    addField = f;
    fieldActive = Math.max(0, shownFields.indexOf(f));
    void loadCounts(f);
  }

  function fieldKeydown(e: KeyboardEvent) {
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      fieldActive = Math.min(shownFields.length - 1, fieldActive + 1);
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      fieldActive = Math.max(0, fieldActive - 1);
    } else if (e.key === 'Home' && !fieldQuery) {
      e.preventDefault();
      fieldActive = 0;
    } else if (e.key === 'End' && !fieldQuery) {
      e.preventDefault();
      fieldActive = shownFields.length - 1;
    } else if (e.key === 'Enter') {
      e.preventDefault();
      const f = shownFields[fieldActive];
      if (f) chooseField(f);
    }
  }

  $effect(() => {
    if (fieldActive < 0 || fieldActive >= shownFields.length) return;
    document.getElementById(`${fieldListId}-${fieldActive}`)?.scrollIntoView({ block: 'nearest' });
  });

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
      if (openId === 'add') chooseField(role);
      else openFlag(role).set(true);
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
    return project ? [quote(project)] : (filters.values.project ?? []);
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

  function clearAll() {
    text = '';
    change({ ...emptyFilters(), order: filters.order }, `clear:all:${++opened}`);
  }
</script>

{#snippet listPicker(field: ListField)}
  <ValuePicker
    label={t.search.fields[field]}
    selected={filters.values[field] ?? []}
    onchange={(v) => setValues(field, v)}
    load={(q) => loadOptions(field, q, scope())}
    special={specialValues(field)}
    shortcuts={shortcutsFor(field)}
    labelOf={(v) => labelOf(field, v)}
    free={FREE.includes(field) ? (s) => (field === 'epic' ? s.toUpperCase() : quote(s)) : undefined}
    onreset={() => clearField(field)}
    counts={countsFor(field)}
    markOf={hasMarks(field) ? (v) => markOf(field, v) : undefined}
  />
{/snippet}

{#snippet peoplePicker()}
  <div class="role">
    <div class="role-field">
      <span>{t.search.role}</span>
      <Dropdown
        label={t.search.role}
        value={peopleRole}
        options={ROLES.map((r) => ({ value: r, label: t.search.roles[r] ?? r }))}
        onchange={(v) => setRole(v as (typeof ROLES)[number])}
      />
    </div>
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
    markOf={(v) => markOf('people', v)}
  />
{/snippet}

{#snippet picker(field: Field)}
  {#if isDate(field)}
    <DateRangePicker
      {field}
      label={t.search.fields[field]}
      range={filters.dates[field]}
      onchange={(r) => setRange(field, r)}
    />
  {:else if field === 'people'}
    {@render peoplePicker()}
  {:else}
    {@render listPicker(field)}
  {/if}
{/snippet}

{#snippet chip(field: Field)}
  {@const flag = openFlag(field)}
  <FilterPopover
    label={t.search.fields[field]}
    summary={isDate(field)
      ? dateSummary(field)
      : field === 'people'
        ? peopleSummary()
        : listSummary(field)}
    active={hasValue(field)}
    bind:open={flag.get, flag.set}
    onclear={() => {
      clearField(field);
      refocusAdd();
    }}
    wide={isDate(field)}
  >
    {@render picker(field)}
  </FilterPopover>
{/snippet}

<div class="filter-bar" role="group" aria-label={t.search.filters} bind:this={bar}>
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
    {#each chips as f (f)}
      {@render chip(f)}
    {/each}
    <div class="add-filter">
      <FilterPopover
        label={t.search.addFilter}
        name={t.search.addFilterLabel}
        lead="+"
        bind:open={addFlag.get, addFlag.set}
        wide
        pinned
      >
        <div class="add-panel">
          <div class="add-fields">
            <input
              class="input field-search"
              type="text"
              bind:value={fieldQuery}
              oninput={() => (fieldActive = 0)}
              onkeydown={fieldKeydown}
              placeholder={t.search.findField}
              aria-label={t.search.findFieldLabel}
              role="combobox"
              aria-expanded="true"
              aria-controls={fieldListId}
              aria-autocomplete="list"
              aria-activedescendant={shownFields[fieldActive]
                ? `${fieldListId}-${fieldActive}`
                : undefined}
              autocomplete="off"
              spellcheck="false"
              use:focusOnMount
            />
            <ul
              class="field-list"
              id={fieldListId}
              role="listbox"
              aria-label={t.search.addFilterLabel}
            >
              {#each shownGroups as g (g.id)}
                <li class="group-label" role="presentation">{t.search.fieldGroups[g.id]}</li>
                {#each g.fields as f (f)}
                  {@const i = shownFields.indexOf(f)}
                  <!-- Keyboard use goes through the search field (aria-activedescendant). -->
                  <!-- svelte-ignore a11y_click_events_have_key_events -->
                  <li
                    id="{fieldListId}-{i}"
                    class="item field-option"
                    class:active={i === fieldActive}
                    role="option"
                    aria-selected={addField === f}
                    onclick={() => chooseField(f)}
                    onpointerdown={(e) => e.preventDefault()}
                    onmouseenter={() => (fieldActive = i)}
                  >
                    <span>{t.search.fields[f]}</span>
                    {#if hasValue(f)}<span class="in-use">{t.search.inUse}</span>{/if}
                  </li>
                {/each}
              {/each}
              {#if shownGroups.length === 0}
                <li class="none" role="presentation">{t.search.noField}</li>
              {/if}
            </ul>
            <ul class="more-list">
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
                {#if peopleBusy}<p class="busy-note" id="watching-busy">
                    {t.search.watchingBusy}
                  </p>{/if}
              </li>
            </ul>
          </div>
          <div class="add-values">
            {#if addField}
              {#key addField}
                {@render picker(addField)}
              {/key}
            {:else}
              <p class="pick-field">{t.search.pickField}</p>
            {/if}
          </div>
        </div>
      </FilterPopover>
    </div>
    {#if !isBlank(filters)}
      <button type="button" class="link-btn clear-all" onclick={clearAll}
        >{t.search.clearAll}</button
      >
    {/if}
  </div>
</div>

<style>
  .filter-bar {
    position: relative;
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px 10px;
  }

  .filter-text {
    position: relative;
    flex: 1 1 15rem;
    min-width: 0;
    max-width: 26.25rem;
  }

  .filter-text .glass {
    position: absolute;
    left: 9px;
    top: 50%;
    transform: translateY(-52%);
    color: var(--muted);
    font-size: 1rem;
    pointer-events: none;
  }

  .filter-text .input {
    height: 2rem;
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
    font-size: 0.78125rem;
    color: var(--ink-soft);
  }

  .role-field :global(.select) {
    flex: 1;
    height: 1.75rem;
    padding-block: 2px;
    padding-left: 6px;
    font-size: 0.8125rem;
  }

  .role-hint,
  .busy-note {
    margin: 4px 0 6px;
    font-size: 0.71875rem;
    line-height: 1.35;
    color: var(--muted);
  }

  .busy-note {
    margin: -2px 12px 6px 36px;
  }

  .add-filter {
    display: contents;
  }

  .add-filter :global(.filter-popover.wide) {
    width: min(33rem, calc(100vw - 16px));
    height: min(27.5rem, 70vh);
  }

  .add-panel {
    display: grid;
    grid-template-columns: 12.5rem minmax(0, 1fr);
    flex: 1;
    min-height: 0;
  }

  .add-fields {
    display: flex;
    flex-direction: column;
    min-height: 0;
    border-right: 1px solid var(--rule);
  }

  .field-search {
    margin: 8px 8px 4px;
    width: calc(100% - 16px);
    height: 1.875rem;
    flex: none;
  }

  .field-list {
    list-style: none;
    margin: 0;
    padding: 2px 0 4px;
    overflow: auto;
    min-height: 0;
    flex: 1;
  }

  .field-option[aria-selected='true'] {
    background: var(--paper-sunk);
    font-weight: 600;
  }

  .in-use {
    margin-left: auto;
    font-size: 0.71875rem;
    font-weight: 400;
    color: var(--muted);
  }

  /* A skin may paint the chosen row dark: the note takes the row's colour. */
  .field-option[aria-selected='true'] .in-use {
    color: inherit;
  }

  .none,
  .pick-field {
    margin: 0;
    padding: 8px 12px;
    color: var(--muted);
    font-size: 0.8125rem;
  }

  .pick-field {
    padding-top: 14px;
  }

  .more-list {
    flex: none;
    list-style: none;
    margin: 0;
    padding: 4px 0;
    border-top: 1px solid var(--rule);
  }

  .more-list .item {
    cursor: pointer;
  }

  .more-list .item.busy {
    cursor: default;
    color: var(--muted);
  }

  .more-list input {
    margin: 0;
  }

  .add-values {
    display: flex;
    flex-direction: column;
    min-width: 0;
    min-height: 0;
    overflow: auto;
  }

  .clear-all {
    font-size: 0.78125rem;
    margin-left: 4px;
  }

  @media (max-width: 600px) {
    .filter-text {
      flex-basis: 100%;
      max-width: none;
    }

    .add-filter :global(.filter-popover.wide) {
      width: auto;
      height: 78vh;
    }

    .add-panel {
      grid-template-columns: minmax(0, 1fr);
      grid-template-rows: minmax(0, 2fr) minmax(0, 3fr);
    }

    .add-fields {
      border-right: none;
      border-bottom: 1px solid var(--rule);
    }
  }
</style>
