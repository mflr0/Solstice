<script lang="ts">
  import { jqlValue, narrowedJql, type CountField } from '../../core/dashboard.js';
  import { paths } from '../../core/route.js';
  import type { Facets } from '../../core/types.js';
  import { formatNumber } from '../../i18n/format.js';
  import { t } from '../../i18n/t.js';
  import { api, errorMessage } from '../../lib/api.js';
  import { debounce, live } from '../../lib/live.svelte.js';
  import { session } from '../../lib/session.svelte.js';

  /**
   * A search's issues counted by one field, one row per value with a bar,
   * most first. Each row opens the search narrowed to its value.
   */
  interface Props {
    query: string;
    field: CountField;
  }
  const { query, field }: Props = $props();

  let result = $state<Facets | null>(null);
  let error = $state('');
  let seq = 0;

  async function load() {
    const mine = ++seq;
    try {
      const r = await api.facets({ q: query, fields: field });
      if (mine !== seq) return;
      result = r;
      error = '';
    } catch (e) {
      if (mine === seq) error = errorMessage(e);
    }
  }

  $effect(() => {
    void [query, field];
    void load();
  });

  $effect(() => live.onChange(debounce(() => void load(), 700)));

  /** A counted value's name, and how JQL writes it. */
  function describe(id: string, users: Facets['users']): { label: string; value: string } {
    if (id === '') {
      const label = field === 'assignee' ? t.search.special.unassigned : t.home.none;
      return { label, value: jqlValue('') };
    }
    const meta = session.meta;
    // Shown in the viewer's language; JQL keeps the stored name.
    const named = <T extends { id: string; name: string }>(
      list: T[] | undefined,
      shown: (x: T) => string = (x) => x.name,
    ) => {
      const x = list?.find((y) => y.id === id);
      return x ? { label: shown(x), value: jqlValue(x.name) } : undefined;
    };
    let found: { label: string; value: string } | undefined;
    switch (field) {
      case 'status':
        found = named(meta?.statuses);
        break;
      case 'type':
        found = named(meta?.issueTypes, (x) => session.typeName(x));
        break;
      case 'priority':
        found = named(meta?.priorities, (x) => session.priorityName(x));
        break;
      case 'resolution':
        found = named(meta?.resolutions, (x) => session.resolutionName(x));
        break;
      case 'assignee':
      case 'reporter': {
        const u = users[id];
        if (u) found = { label: u.name, value: jqlValue(u.username) };
        break;
      }
    }
    return found ?? { label: id, value: jqlValue(id) };
  }

  const rows = $derived.by(() => {
    if (!result) return [];
    const counts = result.facets[field] ?? {};
    const users = result.users ?? {};
    return Object.entries(counts)
      .map(([id, n]) => ({ id, n, ...describe(id, users) }))
      .sort((a, b) => b.n - a.n || a.label.localeCompare(b.label));
  });
  const max = $derived(Math.max(1, ...rows.map((r) => r.n)));
</script>

{#if error}
  <p class="error-text" role="alert">{error}</p>
{:else if !result}
  <p class="muted loading">{t.common.loading}</p>
{:else if rows.length === 0}
  <p class="empty">{t.home.emptyCount}</p>
{:else}
  <ul class="counts">
    {#each rows as r (r.id)}
      {@const jql = narrowedJql(query, field, r.value)}
      <li>
        {#if jql !== null}
          <a class="label" href={paths.search(jql)}>{r.label}</a>
        {:else}
          <span class="label">{r.label}</span>
        {/if}
        <span class="bar" aria-hidden="true"><span style:width="{(r.n / max) * 100}%"></span></span>
        <span class="n">{formatNumber(r.n)}</span>
      </li>
    {/each}
  </ul>
{/if}

<style>
  .counts {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    gap: 2px;
  }

  li {
    display: grid;
    grid-template-columns: minmax(0, calc(14 * var(--font-size))) minmax(0, 1fr) auto;
    align-items: center;
    gap: 12px;
    padding: 5px 0;
    border-bottom: 1px solid var(--rule);
    font-size: 13.5px;
  }

  .label {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .bar {
    height: 8px;
    background: var(--accent-wash);
  }

  .bar span {
    display: block;
    height: 100%;
    background: var(--accent);
  }

  .n {
    min-width: 3ch;
    text-align: right;
    font: 500 12.5px var(--font-mono);
  }

  .loading {
    padding: 10px 0;
  }

  @media (max-width: 600px) {
    li {
      grid-template-columns: minmax(0, 1fr) calc(4 * var(--font-size)) auto;
    }
  }
</style>
