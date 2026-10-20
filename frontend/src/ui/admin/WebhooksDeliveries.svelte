<script lang="ts">
  import { deliveryTone, toneClass } from '../../core/admin-integrations.js';
  import type { WebhookDelivery } from '../../core/admin-types.js';
  import { formatNumber } from '../../i18n/format.js';
  import { formatDateTime, labelOf, relative, t } from '../../i18n/t.js';
  import { adminApi } from '../../lib/admin-api.js';
  import { errorMessage } from '../../lib/api.js';

  /**
   * The last deliveries of a webhook, newest first: the event sent, how the
   * target answered, and when the next attempt comes for those still waiting.
   * The edit page re-creates it after a test call.
   */
  interface Props {
    id: number;
  }

  const { id }: Props = $props();

  let deliveries = $state<WebhookDelivery[] | null>(null);
  let loadError = $state('');

  $effect(() => {
    let current = true;
    adminApi
      .deliveries(id)
      .then((list) => {
        if (current) deliveries = list;
      })
      .catch((err: unknown) => {
        if (current) loadError = errorMessage(err);
      });
    return () => {
      current = false;
    };
  });

  const eventName = (event: string) => labelOf(t.hooksAdmin.eventNames, event);
</script>

<h2 class="section-title">{t.hookDeliveries.title}</h2>
<p class="muted">{t.hookDeliveries.intro}</p>
{#if loadError}
  <p class="error-text" role="alert">{loadError}</p>
{:else if deliveries === null}
  <p class="muted">{t.common.loading}</p>
{:else if deliveries.length === 0}
  <p class="muted">{t.hookDeliveries.empty}</p>
{:else}
  <div class="table-scroll">
    <table class="ledger">
      <thead>
        <tr>
          <th>{t.hookDeliveries.event}</th>
          <th>{t.hookDeliveries.result}</th>
          <th>{t.hookDeliveries.answer}</th>
          <th class="num">{t.hookDeliveries.attempts}</th>
          <th>{t.hookDeliveries.when}</th>
        </tr>
      </thead>
      <tbody>
        {#each deliveries as d (d.id)}
          {@const tone = deliveryTone(d)}
          {@const at = d.attempted ?? d.created}
          <tr>
            <td>{eventName(d.event)}</td>
            <td><span class="stamp {toneClass(tone)}">{t.hooksAdmin.tones[tone]}</span></td>
            <td class="c-answer">
              {d.statusCode !== null
                ? t.hooksAdmin.httpAnswer(d.statusCode)
                : t.hooksAdmin.noAnswer}{#if d.durationMs !== null}
                · {t.hooksAdmin.duration(formatNumber(d.durationMs))}{/if}
              {#if d.error}<span class="muted small error-text" title={d.error}>{d.error}</span
                >{/if}
              {#if tone === 'waiting' && d.nextAttempt}<span class="muted small"
                  >{t.hookDeliveries.retry(formatDateTime(d.nextAttempt))}</span
                >{/if}
            </td>
            <td class="num">{formatNumber(d.attempts)}</td>
            <td class="c-when"><span title={formatDateTime(at)}>{relative(at)}</span></td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
{/if}

<style>
  .c-answer {
    max-width: 0;
    width: 45%;
  }

  .c-answer .small {
    display: block;
  }

  .c-answer .error-text {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .c-when {
    white-space: nowrap;
  }
</style>
