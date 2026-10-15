<script lang="ts">
  import { matches } from '../../core/admin-forms.js';
  import { deliveryTone, toneClass } from '../../core/admin-integrations.js';
  import type { Webhook } from '../../core/admin-types.js';
  import { paths } from '../../core/route.js';
  import { formatNumber } from '../../i18n/format.js';
  import { formatDateTime, relative, t } from '../../i18n/fr.js';
  import { adminApi } from '../../lib/admin-api.js';
  import { errorMessage } from '../../lib/api.js';
  import EmptyState from '../EmptyState.svelte';

  /** Every webhook: where it calls, what for, and how its last call went. */
  type Show = 'all' | 'enabled' | 'disabled' | 'failing';
  const SHOWS: Show[] = ['all', 'enabled', 'disabled', 'failing'];

  let hooks = $state<Webhook[] | null>(null);
  let error = $state('');
  let q = $state('');
  let show = $state<Show>('all');

  const failing = (w: Webhook) => !!w.lastDelivery && deliveryTone(w.lastDelivery) === 'failed';

  const list = $derived(
    (hooks ?? []).filter((w) => {
      if (show === 'enabled' && !w.enabled) return false;
      if (show === 'disabled' && w.enabled) return false;
      if (show === 'failing' && !failing(w)) return false;
      return matches(q, w.name, w.url);
    }),
  );

  $effect(() => {
    adminApi
      .webhooks()
      .then((h) => (hooks = h))
      .catch((e: unknown) => (error = errorMessage(e)));
  });

  function clear() {
    q = '';
    show = 'all';
  }
</script>

<header class="page-head">
  <div>
    <p class="eyebrow">{t.adminArea.groups.integrations}</p>
    <h1 class="page-title">
      {t.hooksAdmin.title}
      {#if hooks?.length}<span class="count muted">{formatNumber(hooks.length)}</span>{/if}
    </h1>
  </div>
  <a class="btn btn-primary" href={paths.admin('webhooks', 'new')}>{t.hooksAdmin.newHook}</a>
</header>
<p class="lead-text">{t.hooksAdmin.intro}</p>

{#if error}
  <p class="error-text" role="alert">{error}</p>
{:else if !hooks}
  <p class="muted loading">{t.common.loading}</p>
{:else if hooks.length === 0}
  <EmptyState message={t.hooksAdmin.empty} hint={t.hooksAdmin.emptyHint}>
    <a class="btn btn-primary" href={paths.admin('webhooks', 'new')}>{t.hooksAdmin.newHook}</a>
  </EmptyState>
{:else}
  <div class="list-tools">
    <input
      class="input search"
      type="search"
      bind:value={q}
      placeholder={t.hooksAdmin.search}
      aria-label={t.hooksAdmin.search}
    />
    <select class="select" bind:value={show} aria-label={t.hooksAdmin.showLabel}>
      {#each SHOWS as s (s)}<option value={s}>{t.hooksAdmin.show[s]}</option>{/each}
    </select>
  </div>

  {#if list.length === 0}
    <EmptyState kind="search" message={t.adminArea.noMatch}>
      <button class="btn" onclick={clear}>{t.adminArea.clearSearch}</button>
    </EmptyState>
  {:else}
    <div class="table-scroll">
      <table class="ledger">
        <thead>
          <tr>
            <th>{t.hooksAdmin.name}</th>
            <th class="num c-events">{t.hooksAdmin.events}</th>
            <th>{t.hooksAdmin.state}</th>
            <th class="c-last">{t.hooksAdmin.lastDelivery}</th>
            <th><span class="sr-only">{t.common.actions}</span></th>
          </tr>
        </thead>
        <tbody>
          {#each list as w (w.id)}
            {@const d = w.lastDelivery}
            <tr class:is-off={!w.enabled}>
              <td class="c-name">
                <a class="name" href={paths.admin('webhooks', w.id)}>{w.name}</a>
                <span class="mono muted url" title={w.url}>{w.url}</span>
                {#if w.jql}<span class="muted small jql" title={w.jql}
                    >{t.hooksAdmin.filtered}</span
                  >{/if}
              </td>
              <td class="num c-events" title={w.events.join(', ')}>
                {formatNumber(w.events.length)}
              </td>
              <td>
                <span class="stamp" class:done={w.enabled}
                  >{w.enabled ? t.hooksAdmin.enabledState : t.hooksAdmin.disabledState}</span
                >
              </td>
              <td class="c-last">
                {#if d}
                  {@const tone = deliveryTone(d)}
                  <span class="stamp {toneClass(tone)}">{t.hooksAdmin.tones[tone]}</span>
                  <span class="muted small" title={formatDateTime(d.attempted ?? d.created)}
                    >{relative(d.attempted ?? d.created)}</span
                  >
                {:else}
                  <span class="muted small">{t.hooksAdmin.never}</span>
                {/if}
              </td>
              <td class="row-actions">
                <a href={paths.admin('webhooks', w.id)}>{t.common.edit}</a>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
{/if}

<style>
  .count {
    margin-left: 6px;
    font: 500 14px/1 var(--font-mono);
  }

  .c-name {
    max-width: 0;
    width: 50%;
  }

  .name {
    display: block;
    font-weight: 600;
    text-decoration: none;
  }

  .name:hover {
    text-decoration: underline;
  }

  .url {
    display: block;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    overflow-wrap: normal;
  }

  .c-last .small {
    margin-left: 6px;
    white-space: nowrap;
  }

  @media (max-width: 760px) {
    .c-events {
      display: none;
    }
  }

  @media (max-width: 600px) {
    .c-last {
      display: none;
    }

    .c-name {
      width: auto;
    }
  }
</style>
