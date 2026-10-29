<script lang="ts">
  import { serverColumns } from '../core/search-columns.js';
  import type { IssuePage } from '../core/types.js';
  import { t } from '../i18n/t.js';
  import { api, ApiError, errorMessage } from '../lib/api.js';
  import { debounce, live } from '../lib/live.svelte.js';
  import Illustration from './Illustration.svelte';
  import IssueTable from './IssueTable.svelte';
  import MoveDialog from './issue/MoveDialog.svelte';
  import Pager from './Pager.svelte';
  import BulkDialog from './search/BulkDialog.svelte';

  /**
   * A search shown as a ledger. It refetches by itself, a moment after
   * anyone changes an issue, so the list stays current without reloading.
   */
  interface Props {
    /** A query in either syntax; jql, when given, is read as JQL whatever it looks like. */
    q?: string | undefined;
    jql?: string | undefined;
    project?: string | undefined;
    size?: number | undefined;
    page?: number | undefined;
    empty?: string | undefined;
    ontotal?: ((total: number) => void) | undefined;
    onpage?: ((page: number) => void) | undefined;
    /** Takes over showing a query the server could not read (null once it can). */
    onerror?: ((message: string | null, pos?: number) => void) | undefined;
    showAssignee?: boolean | undefined;
    /** The chosen columns (IssueTable's), the usual set when absent. */
    columns?: readonly string[] | undefined;
    names?: Record<string, string> | undefined;
    /** Rows can be selected and changed together (edit, status, move, delete). */
    selectable?: boolean | undefined;
  }
  const {
    q = '',
    jql,
    project,
    size = 50,
    page = 1,
    empty = t.search.empty,
    ontotal,
    onpage,
    onerror,
    showAssignee = true,
    columns,
    names,
    selectable = false,
  }: Props = $props();

  let selected = $state<string[]>([]);
  let bulk = $state<'edit' | 'transition' | 'move' | 'delete' | null>(null);

  /** The columns the server sends values for, as the request names them. */
  const extra = $derived(columns ? serverColumns(columns).join(',') : '');

  let result = $state<IssuePage | null>(null);
  let error = $state('');
  /** A query the server could not read (JQL syntax): shown as a note, not a failure. */
  let syntax = $state(false);
  /** The parent shows the error: nothing to draw here. */
  let handled = $state(false);
  let seq = 0;

  async function load() {
    const mine = ++seq;
    try {
      const more = extra
        ? { columns: extra, tz: Intl.DateTimeFormat().resolvedOptions().timeZone }
        : {};
      const r = await api.issues(
        jql === undefined
          ? { q, project, page, size, ...more }
          : { jql, project, page, size, ...more },
      );
      if (mine !== seq) return;
      result = r;
      ontotal?.(r.total);
      error = '';
      syntax = false;
      onerror?.(null);
    } catch (e) {
      if (mine !== seq) return;
      error = errorMessage(e);
      syntax = e instanceof ApiError && e.status === 400;
      if (syntax) {
        ontotal?.(0);
        if (onerror) {
          // The server says where in the query it stopped (a character offset).
          const data = (e as ApiError).data;
          const pos =
            data && typeof data === 'object' && 'pos' in data && typeof data.pos === 'number'
              ? data.pos
              : undefined;
          onerror(error, pos);
          result = null;
          error = '';
          handled = true;
          return;
        }
      }
    }
    handled = false;
  }

  $effect(() => {
    // Read the inputs so the effect reruns when any changes.
    void [q, jql, project, page, size, extra];
    selected = [];
    void load();
  });

  $effect(() => live.onChange(debounce(() => void load(), 700)));
</script>

{#if handled}
  <!-- The query could not be read: the parent says why. -->
{:else if error}
  <p class:syntax class="error-text" role="alert">{error}</p>
{:else if !result}
  <p class="muted loading">{t.common.loading}</p>
{:else if result.issues.length === 0}
  <Illustration kind="search" />
  <p class="empty">{empty}</p>
{:else}
  {#if selectable && selected.length}
    <div class="bulk-bar" role="toolbar" aria-label={t.search.bulk.actions}>
      <span class="bulk-count" aria-live="polite">{t.search.bulk.selected(selected.length)}</span>
      <button type="button" class="btn btn-small" onclick={() => (bulk = 'edit')}
        >{t.search.bulk.edit}</button
      >
      <button type="button" class="btn btn-small" onclick={() => (bulk = 'transition')}
        >{t.search.bulk.transition}</button
      >
      <button type="button" class="btn btn-small" onclick={() => (bulk = 'move')}
        >{t.search.bulk.move}</button
      >
      <button type="button" class="btn btn-small btn-danger" onclick={() => (bulk = 'delete')}
        >{t.search.bulk.delete}</button
      >
      <button type="button" class="btn btn-small btn-quiet" onclick={() => (selected = [])}
        >{t.search.bulk.clear}</button
      >
    </div>
  {/if}
  <IssueTable
    issues={result.issues}
    {showAssignee}
    {columns}
    {names}
    {selected}
    onselect={selectable ? (keys) => (selected = keys) : undefined}
  />
  {#if onpage}
    <Pager page={result.page} size={result.size} total={result.total} {onpage} />
  {/if}
{/if}

{#if selectable}
  <BulkDialog
    op={bulk === 'move' ? null : bulk}
    keys={selected}
    onclose={() => (bulk = null)}
    ondone={(refused) => {
      selected = refused;
      void load();
    }}
  />
  <MoveDialog
    open={bulk === 'move'}
    keys={selected}
    onclose={() => (bulk = null)}
    ondone={(refused) => {
      selected = refused;
      void load();
    }}
  />
{/if}

<style>
  .bulk-bar {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px;
    margin-bottom: 10px;
    padding: 6px 10px;
    border-left: 3px solid var(--accent);
    background: var(--accent-wash);
  }

  .bulk-count {
    font: 0.78125rem var(--font-mono);
    margin-right: auto;
  }

  .syntax {
    font: 0.8125rem/1.5 var(--font-mono);
    padding: 8px 12px;
    border-left: 3px solid var(--accent);
    background: var(--accent-wash);
  }

  .loading {
    padding: 10px 0;
  }
</style>
