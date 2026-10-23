<script lang="ts">
  import { paths } from '../../core/route.js';
  import { t } from '../../i18n/t.js';
  import type { BulkResult } from '../../lib/api.js';
  import Modal from '../Modal.svelte';

  /** The issues a bulk change left as they were, each with the server's reason. */
  interface Props {
    results: BulkResult[] | null;
    onclose: () => void;
  }
  const { results, onclose }: Props = $props();
  const refused = $derived((results ?? []).filter((r) => !r.ok));
</script>

<Modal open={refused.length > 0} title={t.search.bulk.failed(refused.length)} {onclose}>
  <ul class="refused">
    {#each refused as r (r.key)}
      <li>
        <a class="key" href={paths.issue(r.key)}>{r.key}</a>
        <span>{r.error || t.errors.generic}</span>
      </li>
    {/each}
  </ul>
  {#snippet footer()}
    <button type="button" class="btn btn-primary" onclick={onclose}>{t.common.close}</button>
  {/snippet}
</Modal>

<style>
  .refused {
    margin: 0;
    padding: 0;
    list-style: none;
    display: grid;
    gap: 8px;
  }

  .refused li {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr);
    gap: 10px;
    overflow-wrap: anywhere;
  }
</style>
