<script lang="ts">
  import type { HistoryEntry, HistoryItem } from '../../core/types.js';
  import { DURATION_FIELDS, fieldLabel, formatDuration, t } from '../../i18n/fr.js';
  import { api, errorMessage } from '../../lib/api.js';
  import RelTime from '../RelTime.svelte';
  import Who from '../Who.svelte';

  /** What changed, when and by whom, newest first. */
  interface Props {
    issueKey: string;
    tick: number;
  }
  const { issueKey, tick }: Props = $props();

  let entries = $state<HistoryEntry[] | null>(null);
  let error = $state('');

  $effect(() => {
    void tick;
    const key = issueKey;
    api
      .history(key)
      .then((h) => {
        if (key === issueKey) entries = [...h].reverse();
      })
      .catch((e: unknown) => (error = errorMessage(e)));
  });

  /** Long texts (description…) are folded rather than printed whole. */
  const LONG = 140;

  function value(item: HistoryItem, v: string | null): string | null {
    if (v === null || v === '') return null;
    if (DURATION_FIELDS.has(item.field.toLowerCase()) && /^\d+$/.test(v)) {
      return formatDuration(Number(v));
    }
    return v;
  }
</script>

{#if error}
  <p class="error-text" role="alert">{error}</p>
{:else if !entries}
  <p class="muted">{t.common.loading}</p>
{:else if entries.length === 0}
  <p class="empty">{t.history.empty}</p>
{:else}
  <ol class="history">
    {#each entries as e (e.id)}
      <li>
        <header>
          <Who user={e.author} empty="—" />
          <span class="muted"><RelTime iso={e.created} /></span>
        </header>
        <ul class="items">
          {#each e.items as item, i (i)}
            {@const from = value(item, item.from)}
            {@const to = value(item, item.to)}
            <li>
              <span class="field">{fieldLabel(item.field)}</span>
              {#if (from && from.length > LONG) || (to && to.length > LONG)}
                <details>
                  <summary>{t.history.changed} — {t.history.showText}</summary>
                  {#if from}<pre class="old">{from}</pre>{/if}
                  {#if to}<pre class="new">{to}</pre>{/if}
                </details>
              {:else if from && to}
                <span class="old">{from}</span> <span class="arrow" aria-hidden="true">→</span>
                <span class="new">{to}</span>
              {:else if to}
                <span class="verb">{t.history.added}</span> <span class="new">{to}</span>
              {:else if from}
                <span class="verb">{t.history.removed}</span> <span class="old">{from}</span>
              {:else}
                <span class="verb">{t.history.changed}</span>
              {/if}
            </li>
          {/each}
        </ul>
      </li>
    {/each}
  </ol>
{/if}

<style>
  .history {
    list-style: none;
    margin: 0;
    padding: 0;
  }

  .history > li {
    padding: 10px 0 8px;
    border-bottom: 1px solid var(--rule);
  }

  header {
    display: flex;
    gap: 12px;
    align-items: center;
    font-size: 13px;
    margin-bottom: 4px;
  }

  .items {
    list-style: none;
    margin: 0;
    padding: 0 0 0 27px;
    display: grid;
    gap: 3px;
    font-size: 13.5px;
  }

  .field {
    display: inline-block;
    min-width: 130px;
    color: var(--muted);
    font-size: 12.5px;
  }

  .old {
    text-decoration: line-through;
    text-decoration-color: var(--rule-strong);
    color: var(--ink-soft);
  }

  .arrow,
  .verb {
    color: var(--muted);
  }

  details {
    display: inline-block;
    vertical-align: top;
  }

  summary {
    cursor: pointer;
    color: var(--ink-soft);
  }

  pre {
    white-space: pre-wrap;
    font: 12px/1.5 var(--mono);
    background: var(--paper-sunk);
    border: 1px solid var(--rule);
    padding: 6px 8px;
    margin: 6px 0;
    max-height: 220px;
    overflow: auto;
  }

  pre.old {
    text-decoration: none;
    border-left: 3px solid var(--rule-strong);
  }

  pre.new {
    border-left: 3px solid var(--stamp-done);
  }
</style>
