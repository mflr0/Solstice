<script lang="ts">
  import type { SearchMode } from '../../core/types.js';
  import { t } from '../../i18n/fr.js';
  import { router } from '../../lib/router.svelte.js';
  import IssueQuery from '../IssueQuery.svelte';
  import SearchBox from '../SearchBox.svelte';

  /** Search across every project; the query lives in the address. */
  const q = $derived(router.param('q') ?? '');
  const page = $derived(Number(router.param('page') ?? '1') || 1);
  // Follows the address (back/forward) until the user types.
  let draft = $derived(q);
  let total = $state(0);
  let mode = $state<SearchMode>('simple');

  $effect(() => {
    document.title = `${q || t.search.title} — ${t.appName}`;
  });

  function search(v: string) {
    router.setParams({ q: v, page: null });
  }
</script>

<div class="page">
  <header class="page-head">
    <h1>{t.search.title}</h1>
    <span class="count muted">{t.search.results(total)}</span>
  </header>
  <SearchBox bind:value={draft} onsearch={search} {mode} autofocus />
  <div class="results">
    <IssueQuery
      {q}
      {page}
      ontotal={(n) => (total = n)}
      onmode={(m) => (mode = m)}
      onpage={(p) => router.setParams({ page: String(p) }, { replace: false })}
    />
  </div>
</div>

<style>
  .count {
    font: 13px var(--mono);
  }

  .results {
    margin-top: 18px;
  }
</style>
