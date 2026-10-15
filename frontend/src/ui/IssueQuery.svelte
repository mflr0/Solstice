<script lang="ts">
  import type { IssuePage } from '../core/types.js';
  import { t } from '../i18n/fr.js';
  import { api, ApiError, errorMessage } from '../lib/api.js';
  import { debounce, live } from '../lib/live.svelte.js';
  import Illustration from './Illustration.svelte';
  import IssueTable from './IssueTable.svelte';
  import Pager from './Pager.svelte';

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
  }: Props = $props();

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
      const r = await api.issues(
        jql === undefined ? { q, project, page, size } : { jql, project, page, size },
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
    void [q, jql, project, page, size];
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
  <IssueTable issues={result.issues} {showAssignee} />
  {#if onpage}
    <Pager page={result.page} size={result.size} total={result.total} {onpage} />
  {/if}
{/if}

<style>
  .syntax {
    font: 13px/1.5 var(--font-mono);
    padding: 8px 12px;
    border-left: 3px solid var(--accent);
    background: var(--accent-wash);
  }

  .loading {
    padding: 10px 0;
  }
</style>
