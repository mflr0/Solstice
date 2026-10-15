<script lang="ts">
  import { paths } from '../../core/route.js';
  import { t } from '../../i18n/fr.js';
  import EmptyState from '../EmptyState.svelte';
  import WebhooksEdit from './WebhooksEdit.svelte';
  import WebhooksList from './WebhooksList.svelte';

  /**
   * Webhooks: the list (/admin/webhooks), a new one (/admin/webhooks/new),
   * or one webhook with its deliveries (/admin/webhooks/12).
   */
  interface Props {
    /** A webhook's id, "new", or null for the list. */
    item: string | null;
  }
  const { item }: Props = $props();

  const id = $derived(item && /^[1-9]\d{0,15}$/.test(item) ? Number(item) : null);
</script>

{#if item === null}
  <WebhooksList />
{:else if item === 'new' || id !== null}
  {#key item}<WebhooksEdit {id} />{/key}
{:else}
  <EmptyState message={t.hooksAdmin.notFound}>
    <a class="btn" href={paths.admin('webhooks')}>{t.hooksAdmin.backToList}</a>
  </EmptyState>
{/if}
