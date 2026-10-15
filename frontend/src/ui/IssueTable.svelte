<script lang="ts">
  import { isOverdue } from '../core/dates.js';
  import { paths } from '../core/route.js';
  import type { IssueSummary } from '../core/types.js';
  import { formatDay, t } from '../i18n/t.js';
  import { router } from '../lib/router.svelte.js';
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
  }
  const { issues, showAssignee = true }: Props = $props();
  const showDue = $derived(issues.some((i) => i.due));

  function open(e: MouseEvent, key: string) {
    if ((e.target as Element).closest('a, button')) return;
    router.navigate(paths.issue(key));
  }
</script>

<table class="ledger">
  <thead>
    <tr>
      <th class="c-type"><span class="sr-only">{t.table.type}</span></th>
      <th class="c-key">{t.table.key}</th>
      <th class="c-summary">{t.table.summary}</th>
      <th class="c-status">{t.table.status}</th>
      <th class="c-priority"><span class="sr-only">{t.table.priority}</span></th>
      {#if showAssignee}<th class="c-assignee">{t.table.assignee}</th>{/if}
      {#if showDue}<th class="c-due">{t.table.due}</th>{/if}
      <th class="num c-updated">{t.table.updated}</th>
    </tr>
  </thead>
  <tbody>
    {#each issues as issue (issue.key)}
      <tr onclick={(e) => open(e, issue.key)}>
        <td class="c-type"><TypeMark type={issue.type} /></td>
        <td class="c-key"><a class="key" href={paths.issue(issue.key)}>{issue.key}</a></td>
        <td class="c-summary">
          <span class="summary">{issue.summary}</span>
          {#if issue.confidential}<Lock />{/if}
        </td>
        <td class="c-status"><Stamp status={issue.status} /></td>
        <td class="c-priority"><PriorityMark priority={issue.priority} /></td>
        {#if showAssignee}<td class="c-who c-assignee"><Who user={issue.assignee} /></td>{/if}
        {#if showDue}
          {@const late = !!issue.due && issue.status.category !== 'done' && isOverdue(issue.due)}
          <td class="c-due" class:late class:is-overdue={late}>
            {issue.due ? formatDay(issue.due) : ''}
          </td>
        {/if}
        <td class="num muted c-updated"><RelTime iso={issue.updated} /></td>
      </tr>
    {/each}
  </tbody>
</table>

<style>
  tbody tr {
    cursor: pointer;
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

  @media (max-width: 760px) {
    .c-priority,
    .c-assignee {
      display: none;
    }
  }

  /* A phone: type, key, summary and status; the summary may take two lines. */
  @media (max-width: 600px) {
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
