<script lang="ts">
  import { hourLabel, sortedDays, weekdayName } from '../../core/account-settings.js';
  import type { MySubscription, NotificationPrefs } from '../../core/admin-types.js';
  import { paths } from '../../core/route.js';
  import type { SavedFilter } from '../../core/types.js';
  import { formatList, formatLocale } from '../../i18n/format.js';
  import { formatDateTime, relative, t } from '../../i18n/t.js';
  import { accountApi } from '../../lib/admin-api.js';
  import { api, errorMessage } from '../../lib/api.js';
  import { notify } from '../../lib/ui.svelte.js';
  import ConfirmDialog from '../ConfirmDialog.svelte';
  import EmptyState from '../EmptyState.svelte';
  import SubscriptionDialog from './SubscriptionDialog.svelte';

  /**
   * The saved filters one receives by e-mail, daily or on chosen weekdays:
   * subscribe to a filter, change a schedule, send one now, unsubscribe.
   */
  let subs = $state<MySubscription[] | null>(null);
  let prefs = $state<NotificationPrefs | null>(null);
  let filters = $state<SavedFilter[] | null>(null);
  let error = $state('');
  let dialog = $state<{ subscription: MySubscription | null } | null>(null);
  let removing = $state<MySubscription | null>(null);
  let running = $state<number | null>(null);

  const locale = $derived(formatLocale());

  function schedule(s: MySubscription): string {
    const hour = hourLabel(s.hour, locale);
    if (s.schedule === 'daily') return t.subscriptionsPage.daily(hour);
    const days = formatList(sortedDays(s.days).map((d) => weekdayName(d, locale)));
    return t.subscriptionsPage.weekly(days, hour);
  }

  async function load() {
    try {
      subs = await accountApi.mySubscriptions();
      error = '';
    } catch (e) {
      error = errorMessage(e);
    }
  }

  $effect(() => {
    void load();
    // The time zone, whether mail goes out, and the filters to pick from are extras.
    accountApi
      .notifications()
      .then((p) => (prefs = p))
      .catch(() => (prefs = null));
    api
      .filters()
      .then((f) => (filters = f))
      .catch(() => (filters = []));
  });

  async function run(s: MySubscription) {
    running = s.id;
    try {
      const r = await accountApi.runSubscription(s.filterId, s.id);
      notify(t.subscriptionsPage.ran(r.emails));
      void load();
    } catch (e) {
      notify(errorMessage(e), 'error');
    } finally {
      running = null;
    }
  }

  function saved(created: boolean) {
    notify(created ? t.subscriptionsPage.created : t.subscriptionsPage.saved);
    void load();
  }
</script>

<header class="page-head">
  <div>
    <p class="eyebrow">{t.account.title}</p>
    <h1 class="page-title">{t.account.sections.subscriptions}</h1>
  </div>
  {#if subs && subs.length > 0}
    <button class="btn btn-primary" onclick={() => (dialog = { subscription: null })}
      >{t.subscriptionsPage.newSubscription}</button
    >
  {/if}
</header>
<p class="lead-text">{t.subscriptionsPage.intro}</p>

{#if prefs && !prefs.mailEnabled}
  <p class="result failed" role="note">{t.subscriptionsPage.mailOff}</p>
{/if}

{#if error}
  <p class="error-text" role="alert">{error}</p>
{:else if !subs}
  <p class="muted loading">{t.common.loading}</p>
{:else if subs.length === 0}
  <EmptyState message={t.subscriptionsPage.empty} hint={t.subscriptionsPage.emptyHint}>
    <button class="btn btn-primary" onclick={() => (dialog = { subscription: null })}
      >{t.subscriptionsPage.newSubscription}</button
    >
    <a class="btn" href={paths.search()}>{t.subscriptionsPage.openSearch}</a>
  </EmptyState>
{:else}
  <div class="table-scroll">
    <table class="ledger">
      <thead>
        <tr>
          <th>{t.subscriptionsPage.colFilter}</th>
          <th>{t.subscriptionsPage.colSchedule}</th>
          <th class="c-next">{t.subscriptionsPage.colNext}</th>
          <th class="c-last">{t.subscriptionsPage.colLast}</th>
          <th><span class="sr-only">{t.common.actions}</span></th>
        </tr>
      </thead>
      <tbody>
        {#each subs as s (s.id)}
          <tr>
            <td>
              <a class="name" href="/issues?filter={s.filterId}">{s.filterName}</a>
              {#if s.group}<span class="muted small">{t.subscriptionsPage.group(s.group.name)}</span
                >{/if}
            </td>
            <td>
              {schedule(s)}
              {#if s.emptyToo}<span class="muted small">{t.subscriptionsPage.emptyToo}</span>{/if}
            </td>
            <td class="c-next muted" title={formatDateTime(s.nextRun)}>{relative(s.nextRun)}</td>
            <td class="c-last muted" title={s.lastRun ? formatDateTime(s.lastRun) : undefined}>
              {s.lastRun ? relative(s.lastRun) : t.subscriptionsPage.notSent}
            </td>
            <td class="row-actions">
              {#if s.canEdit}
                <button class="link-btn" onclick={() => (dialog = { subscription: s })}
                  >{t.common.edit}</button
                >
                {#if prefs?.mailEnabled}
                  <button class="link-btn" disabled={running === s.id} onclick={() => void run(s)}
                    >{t.subscriptionsPage.run}</button
                  >
                {/if}
                <button class="link-btn" onclick={() => (removing = s)}
                  >{t.subscriptionsPage.unsubscribe}</button
                >
              {/if}
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
  {#if prefs?.timeZone}<p class="muted small">
      {t.subscriptionsPage.timeZone(prefs.timeZone)}
    </p>{/if}
{/if}

<SubscriptionDialog
  open={dialog !== null}
  subscription={dialog?.subscription ?? null}
  {filters}
  onclose={() => (dialog = null)}
  onsaved={saved}
/>

<ConfirmDialog
  open={removing !== null}
  title={t.subscriptionsPage.unsubscribeTitle(removing?.filterName ?? '')}
  confirm={t.subscriptionsPage.unsubscribe}
  onclose={() => (removing = null)}
  onconfirm={async () => {
    if (!removing) return;
    await accountApi.unsubscribe(removing.filterId, removing.id);
    notify(t.subscriptionsPage.unsubscribed);
    void load();
  }}
>
  <p>{t.subscriptionsPage.unsubscribeBody}</p>
</ConfirmDialog>

<style>
  .name {
    font-weight: 600;
    color: var(--ink);
    text-decoration: none;
  }

  .name:hover {
    text-decoration: underline;
  }

  .name + .muted {
    display: block;
  }

  @media (max-width: 760px) {
    .c-last {
      display: none;
    }
  }

  @media (max-width: 600px) {
    .c-next {
      display: none;
    }
  }
</style>
