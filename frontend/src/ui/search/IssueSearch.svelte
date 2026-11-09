<script lang="ts">
  import {
    clickSort,
    emptyFilters,
    filtersToJql,
    jqlToFilters,
    readSort,
    type FilterState,
    type SortField,
  } from '../../core/filters.js';
  import type { SortTerm } from '../../core/jql.js';
  import { readLocation, writeLocation, type SearchLocation } from '../../core/filters-url.js';
  import {
    exportColumns,
    NATIVE_COLUMNS,
    readStoredColumns,
    storeColumns,
    viewColumns,
    type ColumnChoice,
  } from '../../core/search-columns.js';
  import { renderJqlError, t } from '../../i18n/t.js';
  import { api, errorMessage } from '../../lib/api.js';
  import { router } from '../../lib/router.svelte.js';
  import { notify } from '../../lib/ui.svelte.js';
  import CardMenu from '../board/CardMenu.svelte';
  import IssueQuery from '../IssueQuery.svelte';
  import ColumnPicker from './ColumnPicker.svelte';
  import FilterBar from './FilterBar.svelte';
  import JqlEditor from './JqlEditor.svelte';

  /**
   * A search kept in the address: the filter bar (or the JQL editor) with
   * the tools at its end, the count and the sort, then the results. Every change is a new address, so
   * back and forward walk through the searches and any link can be shared.
   */
  interface Props {
    /** A project the search is limited to (project page). */
    project?: string | undefined;
    /** The query when the address gives none; null while it is not known yet. */
    fallback: string | null;
    autofocus?: boolean | undefined;
    /** Results can be selected and changed together. */
    selectable?: boolean | undefined;
    /** The view's own columns (an open saved filter's); null or absent for the stored choice. */
    columns?: readonly string[] | null | undefined;
    /** Takes a change of the columns instead of storing it in the browser. */
    oncolumns?: ((next: string[]) => void) | undefined;
    /** Where the choice is kept, under the column picker. */
    columnsNote?: string | undefined;
    /** Hears the columns shown, for saving them with a new filter. */
    onshown?: ((columns: string[]) => void) | undefined;
  }
  let {
    project,
    fallback,
    autofocus = false,
    selectable = false,
    columns,
    oncolumns,
    columnsNote,
    onshown,
  }: Props = $props();

  const read = $derived(readLocation(router.search));
  const loc = $derived(read.loc);
  const jql = $derived(loc.jql ?? fallback);
  const reading = $derived(jql === null ? null : jqlToFilters(jql));
  const filters = $derived<FilterState>(reading?.ok ? reading.state : emptyFilters());
  const readable = $derived(reading?.ok ?? true);
  const visual = $derived(loc.mode !== 'jql' && readable);
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

  /** The columns the server knows; null until known, [] if it could not say. */
  let choices = $state<ColumnChoice[] | null>(null);
  let stored = $state(readStoredColumns());
  const chosen = $derived.by(() => {
    if (choices === null) return [];
    // Without the list, only what the table draws by itself is safe to ask for.
    if (choices.length === 0)
      return viewColumns(columns, stored).filter((c) => NATIVE_COLUMNS.has(c));
    return viewColumns(
      columns,
      stored,
      choices.map((c) => c.id),
    );
  });
  $effect(() => {
    onshown?.(chosen);
  });
  let exporting = $state(false);
  const names = $derived(Object.fromEntries((choices ?? []).map((c) => [c.id, c.name])));

  /** The order in words, after the count; empty for one that takes more. */
  const sortText = $derived.by(() => {
    const s = reading?.ok ? readSort(reading.state.order) : null;
    if (!s) return '';
    const name = Object.hasOwn(t.search.sorts, s.field)
      ? t.search.sorts[s.field as SortField]
      : s.column && names[s.column];
    return name ? t.search.sortedBy(name, t.search.sortWays[s.way][s.desc ? 'desc' : 'asc']) : '';
  });

  $effect(() => {
    api.issueColumns().then(
      (list) => (choices = list),
      () => (choices = []),
    );
  });

  function setColumns(next: string[]) {
    if (oncolumns) return oncolumns(next);
    storeColumns(next);
    stored = JSON.stringify(next);
  }

  async function exportCsv() {
    if (jql === null || exporting) return;
    exporting = true;
    try {
      const tz = Intl.DateTimeFormat().resolvedOptions().timeZone;
      const file = await api.exportIssues({
        jql,
        project,
        columns: exportColumns(chosen).join(','),
        tz,
      });
      const url = URL.createObjectURL(file);
      const a = document.createElement('a');
      a.href = url;
      a.download = `solstice-${new Date().toLocaleDateString('sv', { timeZone: tz })}.csv`;
      a.click();
      setTimeout(() => URL.revokeObjectURL(url), 0);
    } catch (e) {
      notify(errorMessage(e), 'error');
    } finally {
      exporting = false;
    }
  }

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
      refused = r.reason === 'syntax' ? renderJqlError(r.error) : t.search.tooComplex;
      return;
    }
    go({ jql: q === (jql ?? '').trim() ? loc.jql : q, mode: null, page: loc.page });
  }

  function showJql() {
    go({ jql, mode: 'jql', page: loc.page });
  }

  function setOrder(order: SortTerm[]) {
    applyFilters({ ...filters, order }, `sort:${Date.now()}`);
  }
</script>

{#snippet tools()}
  {#if choices?.length}
    <ColumnPicker {choices} {chosen} note={columnsNote} onchange={setColumns} />
  {/if}
  <button
    type="button"
    class="btn btn-small btn-quiet mode-toggle"
    onclick={() => (visual ? showJql() : showFilters())}
    >{visual ? t.search.modeJql : t.search.modeFilters}</button
  >
  <CardMenu
    label={t.search.moreActions}
    groups={() => [
      {
        label: t.search.resultsMenu,
        items: [{ label: t.search.exportCsv, run: () => void exportCsv(), disabled: exporting }],
      },
    ]}
  />
{/snippet}

<div class="issue-search">
  {#if jql === null}
    <p class="muted loading">{t.common.loading}</p>
  {:else if visual}
    <FilterBar {filters} onchange={applyFilters} {project} {autofocus} {tools} />
  {:else}
    <div class="jql-tools">{@render tools()}</div>
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

  <p class="search-summary" aria-live="polite">
    {#if total !== null}<strong>{t.search.results(total)}</strong>{sortText}{/if}
  </p>

  {#if refused || tooComplex}
    <p class="refused" role="status">
      <strong>{refused || t.search.tooComplex}</strong>
      {#if !refused || refused === t.search.tooComplex}— {t.search.tooComplexHint}{/if}
    </p>
  {/if}
  {#if visual && error}
    <p class="error-text" role="alert">{error}</p>
  {/if}

  {#if jql !== null && choices !== null}
    <div class="results">
      <IssueQuery
        {jql}
        {project}
        columns={chosen}
        {names}
        {selectable}
        sort={filters.order}
        textSearch={filters.text}
        onsort={visual ? (column) => setOrder(clickSort(column, filters.order)) : undefined}
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
  .jql-tools {
    display: flex;
    justify-content: flex-end;
    align-items: center;
    gap: 6px;
    margin-bottom: 8px;
  }

  .mode-toggle {
    height: 2rem;
    font-family: var(--font-mono);
  }

  .issue-search :global(.card-menu-toggle) {
    height: 2rem;
    padding: 0 8px;
  }

  .search-summary {
    min-height: 1.25rem;
    margin: 12px 0 0;
    font-size: 0.8125rem;
    color: var(--ink-soft);
  }

  .search-summary strong {
    font-weight: 600;
    color: var(--ink);
  }

  .refused {
    margin: 10px 0 0;
    padding: 6px 10px;
    border-left: 3px solid var(--stamp-doing);
    background-color: var(--paper-sunk);
    font-size: 0.8125rem;
    color: var(--ink-soft);
  }

  .results {
    margin-top: 14px;
  }
</style>
