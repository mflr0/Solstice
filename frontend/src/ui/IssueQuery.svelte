<script lang="ts">
  import type { IssuePage, SearchMode } from '../core/types.js';
  import { t } from '../i18n/fr.js';
  import { api, ApiError, errorMessage } from '../lib/api.js';
  import { debounce, live } from '../lib/live.svelte.js';
  import IssueTable from './IssueTable.svelte';
  import Pager from './Pager.svelte';

  /**
   * A search shown as a ledger. It refetches by itself, a moment after
   * anyone changes an issue, so the list stays current without reloading.
   */
  interface Props {
    q: string;
    project?: string | undefined;
    size?: number | undefined;
    page?: number | undefined;
    empty?: string | undefined;
    ontotal?: ((total: number) => void) | undefined;
    onmode?: ((mode: SearchMode) => void) | undefined;
    onpage?: ((page: number) => void) | undefined;
    showAssignee?: boolean | undefined;
  }
  const {
    q,
    project,
    size = 50,
    page = 1,
    empty = t.search.empty,
    ontotal,
    onmode,
    onpage,
    showAssignee = true,
  }: Props = $props();

  let result = $state<IssuePage | null>(null);
  let error = $state('');
  /** A query the server could not read (JQL syntax): shown as a note, not a failure. */
  let syntax = $state(false);
  let seq = 0;

  async function load() {
    const mine = ++seq;
    try {
      const r = await api.issues({ q, project, page, size });
      if (mine !== seq) return;
      result = r;
      ontotal?.(r.total);
      onmode?.(r.mode ?? 'simple');
      error = '';
      syntax = false;
    } catch (e) {
      if (mine !== seq) return;
      error = errorMessage(e);
      syntax = e instanceof ApiError && e.status === 400;
      if (syntax) {
        ontotal?.(0);
        onmode?.('jql');
      }
    }
  }

  $effect(() => {
    // Read the inputs so the effect reruns when any changes.
    void [q, project, page, size];
    void load();
  });

  $effect(() => live.onChange(debounce(() => void load(), 700)));
</script>

{#if error}
  <p class:syntax class="error-text" role="alert">{error}</p>
{:else if !result}
  <p class="muted loading">{t.common.loading}</p>
{:else if result.issues.length === 0}
  <p class="empty">{empty}</p>
{:else}
  <IssueTable issues={result.issues} {showAssignee} />
  {#if onpage}
    <Pager page={result.page} size={result.size} total={result.total} {onpage} />
  {/if}
{/if}

<style>
  .syntax {
    font: 13px/1.5 var(--mono);
    padding: 8px 12px;
    border-left: 3px solid var(--accent);
    background: var(--accent-wash);
  }

  .loading {
    padding: 10px 0;
  }
</style>
