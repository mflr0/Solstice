<script lang="ts">
  import { t } from '../i18n/t.js';

  /** Marks a confidential issue. */
  interface Props {
    label?: string | undefined;
  }
  const { label = t.common.confidential }: Props = $props();
</script>

<svg class="lock" width="11" height="12" viewBox="0 0 11 12" role="img" aria-label={label}>
  <title>{label}</title>
  <path d="M3 5.5V3.8a2.5 2.5 0 0 1 5 0v1.7" />
  <rect x="1.5" y="5.5" width="8" height="5.5" rx="0.5" />
</svg>

<style>
  .lock {
    vertical-align: -1px;
    flex: none;
  }

  .lock path,
  .lock rect {
    fill: none;
    stroke: var(--accent);
    stroke-width: 1.2;
  }
</style>
