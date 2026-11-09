<script lang="ts">
  import { isOverdue } from '../core/dates.js';
  import { columnDirection, columnSort } from '../core/filters.js';
  import type { SortTerm } from '../core/jql.js';
  import { paths } from '../core/route.js';
  import type { IssueSummary } from '../core/types.js';
  import { formatDay, t } from '../i18n/t.js';
  import { listKeys } from '../lib/actions.js';
  import { isPlainClick, router, showIssue } from '../lib/router.svelte.js';
  import { session } from '../lib/session.svelte.js';
  import Highlight from './Highlight.svelte';
  import Lock from './Lock.svelte';
  import PriorityMark from './PriorityMark.svelte';
  import RelTime from './RelTime.svelte';
  import Stamp from './Stamp.svelte';
  import TypeMark from './TypeMark.svelte';
  import Who from './Who.svelte';

  /** Issues ruled like a ledger: one line each, key first, dates on the right. */
  interface Props {
    issues: IssueSummary[];
    /** Hidden when every row is assigned to the same person anyway. */
    showAssignee?: boolean | undefined;
    /**
     * The columns chosen after key and summary, in order (the type is drawn
     * before the key); the usual set when absent.
     */
    columns?: readonly string[] | undefined;
    /** Column names by id, for the columns the table has no name of its own for. */
    names?: Record<string, string> | undefined;
    /**
     * With onselect, a checkbox per row and one for the whole page, and x
     * selects the focused row (j and k move between rows, o or Enter opens, always).
     */
    selected?: readonly string[] | undefined;
    onselect?: ((keys: string[]) => void) | undefined;
    /** The query's order, which the headers show. */
    sort?: readonly SortTerm[] | undefined;
    /** With it, the headers of the columns ORDER BY takes are buttons that report a click. */
    onsort?: ((column: string) => void) | undefined;
    /** The words of the query's `text ~`, marked in the summaries. */
    textSearch?: string | undefined;
    /** Never wider than its box: fixed column widths, and the summary gives way. */
    fixed?: boolean | undefined;
  }
  const {
    issues,
    showAssignee = true,
    columns,
    names = {},
    selected = [],
    onselect,
    sort = [],
    onsort,
    textSearch = '',
    fixed = false,
  }: Props = $props();
  const allSelected = $derived(issues.length > 0 && issues.every((i) => selected.includes(i.key)));
  const someSelected = $derived(!allSelected && issues.some((i) => selected.includes(i.key)));

  function toggle(key: string, on: boolean) {
    onselect?.(on ? [...selected, key] : selected.filter((k) => k !== key));
  }

  function toggleAll(on: boolean) {
    const page = issues.map((i) => i.key);
    onselect?.(
      on
        ? [...selected, ...page.filter((k) => !selected.includes(k))]
        : selected.filter((k) => !page.includes(k)),
    );
  }
  const showDue = $derived(issues.some((i) => i.due));
  const showType = $derived(!columns || columns.includes('issuetype'));
  /** What follows the summary. */
  const rest = $derived(
    (
      columns ?? ['status', 'priority', ...(showAssignee ? ['assignee'] : []), 'duedate', 'updated']
    ).filter((c) => c !== 'issuetype' && (c !== 'duedate' || showDue)),
  );
  const headers: Record<string, string> = {
    status: t.table.status,
    priority: t.table.priority,
    assignee: t.table.assignee,
    duedate: t.table.due,
    updated: t.table.updated,
    reporter: t.search.fields.reporter,
    created: t.search.fields.created,
    resolutiondate: t.search.fields.resolved,
    resolution: t.search.fields.resolution,
  };

  function ariaSort(column: string): 'ascending' | 'descending' | undefined {
    if (!onsort) return undefined;
    const desc = columnDirection(column, sort);
    return desc === null ? undefined : desc ? 'descending' : 'ascending';
  }

  // The modal hands the focus back to the row when it closes.
  function openRow(row: HTMLElement) {
    if (!row.dataset.key) return;
    row.focus({ preventScroll: true });
    showIssue(row.dataset.key);
  }

  function open(e: MouseEvent) {
    if ((e.target as Element).closest('a, button, label, input')) return;
    const row = e.currentTarget as HTMLElement;
    if (isPlainClick(e)) openRow(row);
    else if (row.dataset.key) router.navigate(paths.issue(row.dataset.key));
  }

  function openKey(e: MouseEvent) {
    if (!isPlainClick(e)) return;
    const row = (e.currentTarget as Element).closest<HTMLElement>('tr');
    if (!row) return;
    e.preventDefault();
    openRow(row);
  }

  function selectRow(row: HTMLElement) {
    const key = row.dataset.key;
    if (key) toggle(key, !selected.includes(key));
  }
</script>

{#snippet label(column: string, name: string)}
  {#if onsort && columnSort(column) !== null}
    {@const desc = columnDirection(column, sort)}
    <button
      type="button"
      class="sort"
      class:sorted={desc !== null}
      onclick={() => onsort?.(column)}
    >
      {name}<span class="arrow" aria-hidden="true">{desc === null ? '↕' : desc ? '↓' : '↑'}</span>
    </button>
  {:else}
    {name}
  {/if}
{/snippet}

<!-- The browser leaves a control partly in view where it is, its ring cut by the scroll box. -->
<div
  class="issue-table"
  class:fixed
  onfocusin={(e) => (e.target as Element).scrollIntoView({ block: 'nearest', inline: 'nearest' })}
  use:listKeys={{ rows: 'tbody tr', open: openRow, select: onselect ? selectRow : undefined }}
>
  <table class="ledger">
    <thead>
      <tr>
        {#if onselect}
          <th class="c-pick">
            <label class="pick">
              <input
                type="checkbox"
                checked={allSelected}
                indeterminate={someSelected}
                onchange={(e) => toggleAll(e.currentTarget.checked)}
              />
              <span class="sr-only">{t.search.bulk.selectPage}</span>
            </label>
          </th>
        {/if}
        {#if showType}
          <th class="c-type" aria-sort={ariaSort('issuetype')}
            >{@render label('issuetype', t.table.type)}</th
          >
        {/if}
        <th class="c-key" aria-sort={ariaSort('issuekey')}
          >{@render label('issuekey', t.table.key)}</th
        >
        <th class="c-summary" aria-sort={ariaSort('summary')}
          >{@render label('summary', t.table.summary)}</th
        >
        {#each rest as c (c)}
          {@const name = headers[c] ?? names[c] ?? c}
          {#if c === 'priority'}
            <th class="c-priority" aria-sort={ariaSort(c)}>{@render label(c, name)}</th>
          {:else if c === 'status'}
            <th class="c-status" aria-sort={ariaSort(c)}>{@render label(c, name)}</th>
          {:else if c === 'assignee'}
            <th class="c-assignee" aria-sort={ariaSort(c)}>{@render label(c, name)}</th>
          {:else if c === 'duedate'}
            <th class="c-due" aria-sort={ariaSort(c)}>{@render label(c, name)}</th>
          {:else if c === 'updated'}
            <th class="num c-updated" aria-sort={ariaSort(c)}>{@render label(c, name)}</th>
          {:else}
            <th class="c-extra" aria-sort={ariaSort(c)}>{@render label(c, name)}</th>
          {/if}
        {/each}
      </tr>
    </thead>
    <tbody>
      {#each issues as issue (issue.key)}
        <tr onclick={open} class:picked={selected.includes(issue.key)} data-key={issue.key}>
          {#if onselect}
            <td class="c-pick">
              <label class="pick">
                <input
                  type="checkbox"
                  checked={selected.includes(issue.key)}
                  onchange={(e) => toggle(issue.key, e.currentTarget.checked)}
                />
                <span class="sr-only">{t.search.bulk.select(issue.key)}</span>
              </label>
            </td>
          {/if}
          {#if showType}<td class="c-type"><TypeMark type={issue.type} /></td>{/if}
          <td class="c-key"
            ><a class="key" href={paths.issue(issue.key)} onclick={openKey}>{issue.key}</a></td
          >
          <td class="c-summary">
            <span class="summary"
              >{#if textSearch}<Highlight
                  text={issue.summary}
                  search={textSearch}
                  fulltext
                />{:else}{issue.summary}{/if}</span
            >
            {#if issue.confidential}<Lock />{/if}
          </td>
          {#each rest as c (c)}
            {#if c === 'status'}
              <td class="c-status"><Stamp status={issue.status} /></td>
            {:else if c === 'priority'}
              <td class="c-priority"><PriorityMark priority={issue.priority} /></td>
            {:else if c === 'assignee'}
              <td class="c-who c-assignee"><Who user={issue.assignee} /></td>
            {:else if c === 'reporter'}
              <td class="c-who c-extra"><Who user={issue.reporter} /></td>
            {:else if c === 'duedate'}
              {@const late =
                !!issue.due && issue.status.category !== 'done' && isOverdue(issue.due)}
              <td class="c-due" class:late class:is-overdue={late}>
                {issue.due ? formatDay(issue.due) : ''}
              </td>
            {:else if c === 'updated'}
              <td class="num muted c-updated"><RelTime iso={issue.updated} /></td>
            {:else if c === 'created'}
              <td class="num muted c-extra"><RelTime iso={issue.created} /></td>
            {:else if c === 'resolutiondate'}
              <td class="num muted c-extra">
                {#if issue.resolved}<RelTime iso={issue.resolved} />{/if}
              </td>
            {:else if c === 'resolution'}
              <td class="c-extra"
                >{issue.resolution ? session.resolutionName(issue.resolution) : ''}</td
              >
            {:else}
              {@const text = issue.values?.[c] ?? ''}
              <td class="c-extra"><span class="text" title={text || undefined}>{text}</span></td>
            {/if}
          {/each}
        </tr>
      {/each}
    </tbody>
  </table>
</div>

<style>
  /* Too many chosen columns scroll sideways here rather than squeeze the summary. */
  .issue-table {
    container: issue-table / inline-size;
    overflow-x: auto;
    /* A scroll box clips at its padding edge: this leaves room for the focus ring. */
    margin: -6px;
    padding: 6px;
    scroll-padding: 6px;
  }

  tbody tr {
    cursor: pointer;
  }

  .sort {
    display: inline-flex;
    align-items: baseline;
    gap: 3px;
    padding: 0;
    border: 0;
    background: none;
    font: inherit;
    letter-spacing: inherit;
    text-transform: inherit;
    color: inherit;
    cursor: pointer;
    white-space: nowrap;
  }

  .sort:hover,
  .sorted {
    color: var(--ink);
  }

  .arrow {
    opacity: 0;
  }

  .sort:hover .arrow,
  .sort:focus-visible .arrow,
  .sorted .arrow {
    opacity: 1;
  }

  tbody tr:focus-visible {
    outline: 2px solid var(--focus);
    outline-offset: -2px;
  }

  .c-pick {
    width: 28px;
    padding-right: 0;
  }

  .pick {
    display: inline-flex;
    padding: 4px;
    margin: -4px;
    cursor: pointer;
  }

  .picked {
    background: var(--accent-wash);
  }

  td.c-type {
    width: 28px;
    padding-right: 0;
  }

  .c-summary {
    width: 100%;
    max-width: 0;
  }

  .c-summary :global(.lock) {
    margin-left: 6px;
  }

  .summary {
    display: inline-block;
    max-width: calc(100% - 24px);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    vertical-align: bottom;
  }

  .c-who {
    max-width: 11.25rem;
    white-space: nowrap;
  }

  .c-due {
    white-space: nowrap;
    font-size: 0.78125rem;
  }

  .late {
    color: var(--accent-text);
  }

  .c-extra {
    white-space: nowrap;
  }

  .c-extra .text {
    display: inline-block;
    max-width: 13.75rem;
    overflow: hidden;
    text-overflow: ellipsis;
    vertical-align: bottom;
  }

  /* The widths hold each column's usual widest content; longer text is cut. */
  .fixed {
    overflow-x: clip;
  }

  .fixed table {
    table-layout: fixed;
  }

  .fixed :is(th, td) {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .fixed .c-pick {
    width: 1.75rem;
  }

  .fixed .c-type {
    width: 3.5rem;
  }

  .fixed .c-key {
    width: 5.75rem;
  }

  .fixed .c-summary {
    width: auto;
    max-width: none;
  }

  .fixed .c-status {
    width: 8rem;
  }

  .fixed .c-status :global(.stamp) {
    max-width: 100%;
    overflow: hidden;
    text-overflow: ellipsis;
    vertical-align: middle;
  }

  .fixed .c-priority {
    width: 6.5rem;
  }

  .fixed .c-assignee {
    width: 10rem;
  }

  .fixed .c-due {
    width: 9.5rem;
  }

  .fixed .c-updated {
    width: 8.5rem;
  }

  .fixed .c-extra {
    width: 9rem;
  }

  @container issue-table (width >= 570px) {
    .c-summary {
      min-width: 10em;
    }
  }

  @container issue-table (width >= 920px) {
    .c-summary {
      min-width: 16em;
    }
  }

  /* Columns give way by the list's own width, not the window's: beside the
     saved filters, a 1024 px window leaves the list about 720 px. */
  @container issue-table (width < 920px) {
    .c-priority,
    .c-assignee,
    .c-extra {
      display: none;
    }
  }

  /* Fixed on a phone: the key says enough without the type, and the summary needs the room. */
  @container issue-table (width < 570px) {
    .fixed .c-type {
      display: none;
    }

    .fixed :is(th, td) {
      padding-inline: 6px;
    }

    .fixed .c-key {
      width: 5.25rem;
    }

    .fixed .c-status {
      width: 6rem;
    }
  }

  /* A phone: type, key, summary and status; the summary may take two lines. */
  @container issue-table (width < 570px) {
    .c-due,
    .c-updated {
      display: none;
    }

    .summary {
      display: -webkit-inline-box;
      white-space: normal;
      -webkit-box-orient: vertical;
      -webkit-line-clamp: 2;
      line-clamp: 2;
    }
  }
</style>
