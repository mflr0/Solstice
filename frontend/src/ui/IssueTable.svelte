<script lang="ts">
  import { isOverdue } from '../core/dates.js';
  import { paths } from '../core/route.js';
  import { shortcutFor } from '../core/shortcuts.js';
  import type { IssueSummary } from '../core/types.js';
  import { formatDay, t } from '../i18n/t.js';
  import { router } from '../lib/router.svelte.js';
  import { appearance, ignoreShortcut } from '../lib/ui.svelte.js';
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
     * With onselect, a checkbox per row and one for the whole page, and the
     * search results' keys: j and k move between rows, o or Enter opens, x selects.
     */
    selected?: readonly string[] | undefined;
    onselect?: ((keys: string[]) => void) | undefined;
  }
  const {
    issues,
    showAssignee = true,
    columns,
    names = {},
    selected = [],
    onselect,
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
    resolution: t.search.fields.resolution,
  };

  function open(e: MouseEvent, key: string) {
    if ((e.target as Element).closest('a, button, label, input')) return;
    router.navigate(paths.issue(key));
  }

  let body: HTMLTableSectionElement | undefined = $state();

  function keys(e: KeyboardEvent) {
    if (!onselect || !body || ignoreShortcut(e)) return;
    const action = shortcutFor(e.key, 'results', appearance.shortcuts);
    if (!action) return;
    const rows = [...body.rows];
    const current = rows.findIndex((r) => r.contains(document.activeElement));
    if (action === 'next' || action === 'previous') {
      if (!rows.length) return;
      e.preventDefault();
      const step = action === 'next' ? 1 : -1;
      const to = current < 0 ? 0 : Math.min(Math.max(current + step, 0), rows.length - 1);
      rows[to]?.focus();
      return;
    }
    // Enter on a link or a checkbox in the row keeps its own meaning.
    if (current < 0 || (e.key === 'Enter' && document.activeElement !== rows[current])) return;
    const issue = issues[current];
    if (!issue) return;
    e.preventDefault();
    if (action === 'open') router.navigate(paths.issue(issue.key));
    else if (action === 'select') toggle(issue.key, !selected.includes(issue.key));
  }
</script>

<svelte:window onkeydown={keys} />

<div class="issue-table">
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
        {#if showType}<th class="c-type"><span class="sr-only">{t.table.type}</span></th>{/if}
        <th class="c-key">{t.table.key}</th>
        <th class="c-summary">{t.table.summary}</th>
        {#each rest as c (c)}
          {@const name = headers[c] ?? names[c] ?? c}
          {#if c === 'priority'}
            <th class="c-priority"><span class="sr-only">{name}</span></th>
          {:else if c === 'status'}
            <th class="c-status">{name}</th>
          {:else if c === 'assignee'}
            <th class="c-assignee">{name}</th>
          {:else if c === 'duedate'}
            <th class="c-due">{name}</th>
          {:else if c === 'updated'}
            <th class="num c-updated">{name}</th>
          {:else}
            <th class="c-extra">{name}</th>
          {/if}
        {/each}
      </tr>
    </thead>
    <tbody bind:this={body}>
      {#each issues as issue (issue.key)}
        <tr
          onclick={(e) => open(e, issue.key)}
          class:picked={selected.includes(issue.key)}
          tabindex={onselect ? -1 : undefined}
        >
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
          <td class="c-key"><a class="key" href={paths.issue(issue.key)}>{issue.key}</a></td>
          <td class="c-summary">
            <span class="summary">{issue.summary}</span>
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
            {:else if c === 'resolution'}
              <td class="c-extra">{issue.resolution?.name ?? ''}</td>
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
  .issue-table {
    container: issue-table / inline-size;
  }

  tbody tr {
    cursor: pointer;
  }

  tbody tr:focus-visible {
    outline: 2px solid var(--accent);
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

  .c-type {
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
    max-width: 180px;
    white-space: nowrap;
  }

  .c-due {
    white-space: nowrap;
    font-size: 12.5px;
  }

  .late {
    color: var(--accent-text);
  }

  .c-extra {
    white-space: nowrap;
  }

  .c-extra .text {
    display: inline-block;
    max-width: 220px;
    overflow: hidden;
    text-overflow: ellipsis;
    vertical-align: bottom;
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
