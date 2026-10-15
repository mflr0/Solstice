<script lang="ts">
  import {
    currentSort,
    emptyFilters,
    filtersToJql,
    jqlToFilters,
    naturalDesc,
    SORTS,
    sortBy,
    type FilterState,
    type SortField,
  } from '../../core/filters.js';
  import { readLocation, writeLocation, type SearchLocation } from '../../core/filters-url.js';
  import { t } from '../../i18n/t.js';
  import { router } from '../../lib/router.svelte.js';
  import IssueQuery from '../IssueQuery.svelte';
  import FilterBar from './FilterBar.svelte';
  import JqlEditor from './JqlEditor.svelte';

  /**
   * A search kept in the address: the filter bar (or the JQL editor), the
   * sort, the count, then the results. Every change is a new address, so
   * back and forward walk through the searches and any link can be shared.
   */
  interface Props {
    /** A project the search is limited to (project page). */
    project?: string | undefined;
    /** The query when the address gives none; null while it is not known yet. */
    fallback: string | null;
    autofocus?: boolean | undefined;
  }
  const { project, fallback, autofocus = false }: Props = $props();

  const read = $derived(readLocation(router.search));
  const loc = $derived(read.loc);
  const jql = $derived(loc.jql ?? fallback);
  const reading = $derived(jql === null ? null : jqlToFilters(jql));
  const filters = $derived<FilterState>(reading?.ok ? reading.state : emptyFilters());
  const readable = $derived(reading?.ok ?? true);
  const visual = $derived(loc.mode !== 'jql' && readable);
  const sort = $derived(currentSort(filters.order));
  /** The address holds a query the filters cannot show: it opens in the editor. */
  const tooComplex = $derived(
    loc.mode !== 'jql' && reading !== null && !reading.ok && reading.reason === 'complex',
  );

  let total = $state<number | null>(null);
  let error = $state<string | null>(null);
  /** Where in the query the server stopped, when it says. */
  let errorPos = $state<number | null>(null);
  /** Why the filters could not take over the query (a switch was refused). */
  let refused = $state('');
  let draft = $state('');
  let lastBurst: string | null = null;

  // Former ?q= addresses become JQL ones, in place.
  $effect(() => {
    if (read.legacy) router.navigate(router.path + writeLocation(loc), { replace: true });
  });

  function go(next: Partial<SearchLocation>, burst: string | null = null) {
    const target: SearchLocation = { ...loc, page: 1, ...next };
    const replace = burst !== null && burst === lastBurst;
    lastBurst = burst;
    refused = '';
    router.navigate(router.path + writeLocation(target), { replace });
  }

  function applyFilters(next: FilterState, burst: string) {
    go({ jql: filtersToJql(next), mode: null }, burst);
  }

  function runJql(q: string) {
    go({ jql: q.trim(), mode: 'jql' });
  }

  function showFilters() {
    const q = draft.trim();
    const r = jqlToFilters(q);
    if (!r.ok) {
      refused = r.reason === 'syntax' ? r.error.message : t.search.tooComplex;
      return;
    }
    go({ jql: q === (jql ?? '').trim() ? loc.jql : q, mode: null, page: loc.page });
  }

  function showJql() {
    go({ jql, mode: 'jql', page: loc.page });
  }

  function setSort(field: SortField, desc: boolean) {
    applyFilters({ ...filters, order: sortBy(field, desc) }, `sort:${Date.now()}`);
  }
</script>

<div class="issue-search">
  {#if jql === null}
    <p class="muted loading">{t.common.loading}</p>
  {:else if visual}
    <FilterBar {filters} onchange={applyFilters} {project} {autofocus} />
  {:else}
    <JqlEditor
      value={jql}
      bind:draft
      {error}
      {errorPos}
      onsubmit={runJql}
      {project}
      autofocus={autofocus || loc.mode === 'jql'}
    />
  {/if}

  <div class="search-tools">
    <span class="count" aria-live="polite">{total === null ? '' : t.search.results(total)}</span>
    {#if visual && jql !== null}
      <div class="sort">
        <label class="sort-field">
          <span>{t.search.sortBy}</span>
          <select
            class="select"
            value={sort?.field ?? ''}
            onchange={(e) => {
              const f = e.currentTarget.value as SortField;
              setSort(f, naturalDesc(f));
            }}
          >
            {#if !sort}<option value="" disabled>{t.search.customSort}</option>{/if}
            {#each SORTS as s (s.field)}
              <option value={s.field}>{t.search.sorts[s.field]}</option>
            {/each}
          </select>
        </label>
        {#if sort}
          <button
            type="button"
            class="btn btn-small btn-quiet sort-dir"
            aria-label={sort.desc ? t.search.descending : t.search.ascending}
            title={sort.desc ? t.search.descending : t.search.ascending}
            onclick={() => setSort(sort.field, !sort.desc)}
            ><span aria-hidden="true">{sort.desc ? '↓' : '↑'}</span></button
          >
        {/if}
      </div>
    {/if}
    <div class="mode-switch" role="group" aria-label={t.search.mode}>
      <button
        type="button"
        class="btn btn-small"
        aria-pressed={visual}
        onclick={() => !visual && showFilters()}>{t.search.modeFilters}</button
      >
      <button
        type="button"
        class="btn btn-small"
        aria-pressed={!visual}
        onclick={() => visual && showJql()}>{t.search.modeJql}</button
      >
    </div>
  </div>

  {#if refused || tooComplex}
    <p class="refused" role="status">
      <strong>{refused || t.search.tooComplex}</strong>
      {#if !refused || refused === t.search.tooComplex}— {t.search.tooComplexHint}{/if}
    </p>
  {/if}
  {#if visual && error}
    <p class="error-text" role="alert">{error}</p>
  {/if}

  {#if jql !== null}
    <div class="results">
      <IssueQuery
        {jql}
        {project}
        page={loc.page}
        ontotal={(n) => (total = n)}
        onerror={(m, at) => {
          error = m;
          errorPos = at ?? null;
        }}
        onpage={(p) => go({ page: p })}
      />
    </div>
  {/if}
</div>

<style>
  .search-tools {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px 14px;
    margin: 12px 0 0;
    padding-bottom: 10px;
    border-bottom: 1px solid var(--rule);
  }

  .count {
    font: 12.5px var(--font-mono);
    color: var(--muted);
    margin-right: auto;
  }

  .sort {
    display: flex;
    align-items: center;
    gap: 2px;
  }

  .sort-field {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 12.5px;
    color: var(--ink-soft);
    white-space: nowrap;
  }

  .sort-field .select {
    width: auto;
    height: 28px;
    padding: 2px 6px;
    font-size: 13px;
  }

  .sort-dir {
    font-size: 14px;
    min-width: 26px;
    justify-content: center;
  }

  .mode-switch {
    display: inline-flex;
  }

  .mode-switch .btn {
    border-radius: 0;
  }

  .mode-switch .btn:first-child {
    border-radius: var(--radius-s) 0 0 var(--radius-s);
  }

  .mode-switch .btn:last-child {
    margin-left: calc(-1 * var(--frame));
    border-radius: 0 var(--radius-s) var(--radius-s) 0;
  }

  .refused {
    margin: 10px 0 0;
    padding: 6px 10px;
    border-left: 3px solid var(--stamp-doing);
    background-color: var(--paper-sunk);
    font-size: 13px;
    color: var(--ink-soft);
  }

  .results {
    margin-top: 14px;
  }
</style>
