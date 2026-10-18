<script lang="ts">
  import { formatNumber } from '../i18n/format.js';
  import { t } from '../i18n/t.js';

  /** "51–100 sur 158", with previous / next. */
  interface Props {
    page: number;
    size: number;
    total: number;
    onpage: (page: number) => void;
  }
  const { page, size, total, onpage }: Props = $props();
  const first = $derived(total === 0 ? 0 : (page - 1) * size + 1);
  const last = $derived(Math.min(total, page * size));
  const pages = $derived(Math.max(1, Math.ceil(total / size)));
</script>

{#if total > size}
  <nav class="pager" aria-label={t.common.pagination}>
    <span class="range"
      >{t.common.range(formatNumber(first), formatNumber(last), formatNumber(total))}</span
    >
    <button class="btn btn-small" disabled={page <= 1} onclick={() => onpage(page - 1)}
      >← {t.common.previous}</button
    >
    <button class="btn btn-small" disabled={page >= pages} onclick={() => onpage(page + 1)}
      >{t.common.next} →</button
    >
  </nav>
{/if}

<style>
  .pager {
    display: flex;
    justify-content: flex-end;
    align-items: center;
    gap: 8px;
    margin-top: 14px;
  }

  .range {
    font: 12.5px var(--font-mono);
    color: var(--muted);
    margin-right: 6px;
  }
</style>
