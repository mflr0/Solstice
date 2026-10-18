<script lang="ts">
  import {
    WEEKDAYS,
    hourLabel,
    newSubscription,
    subscriptionInput,
    weekdayName,
  } from '../../core/account-settings.js';
  import { subscriptionPayload, subscriptionProblems } from '../../core/admin-forms.js';
  import { toggleDay } from '../../core/admin-ops.js';
  import type { MySubscription, SubscriptionInput } from '../../core/admin-types.js';
  import type { SavedFilter } from '../../core/types.js';
  import { formatLocale } from '../../i18n/format.js';
  import { t } from '../../i18n/t.js';
  import { accountApi } from '../../lib/admin-api.js';
  import { errorMessage } from '../../lib/api.js';
  import Modal from '../Modal.svelte';

  /**
   * Subscribing to a saved filter, or changing a subscription's schedule:
   * every day or on chosen weekdays, at an hour, with or without an e-mail
   * when nothing matches.
   */
  interface Props {
    open: boolean;
    /** The subscription to change; null to subscribe to a filter. */
    subscription: MySubscription | null;
    /** The filters one may pick from when subscribing. */
    filters: SavedFilter[] | null;
    onclose: () => void;
    onsaved: (created: boolean) => void;
  }
  const { open, subscription, filters, onclose, onsaved }: Props = $props();

  const HOURS = Array.from({ length: 24 }, (_, h) => h);

  let input = $state<SubscriptionInput>(newSubscription());
  let filterId = $state<number | null>(null);
  let tried = $state(false);
  let error = $state('');
  let busy = $state(false);

  const problems = $derived(subscriptionProblems(input));
  const missingFilter = $derived(subscription === null && filterId === null);
  const locale = $derived(formatLocale());

  $effect(() => {
    if (!open) return;
    input = subscription ? subscriptionInput(subscription) : newSubscription();
    filterId = null;
    tried = false;
    error = '';
  });

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    tried = true;
    if (problems.days || problems.hour || missingFilter) return;
    busy = true;
    error = '';
    try {
      const payload = subscriptionPayload(input);
      if (subscription)
        await accountApi.saveSubscription(subscription.filterId, subscription.id, payload);
      else if (filterId !== null) await accountApi.subscribe(filterId, payload);
      onsaved(subscription === null);
      onclose();
    } catch (err) {
      error = errorMessage(err);
    } finally {
      busy = false;
    }
  }
</script>

<Modal
  {open}
  title={subscription ? t.subscriptionsPage.editTitle : t.subscriptionsPage.newTitle}
  onclose={() => !busy && onclose()}
>
  <form id="subscription-form" class="form" onsubmit={submit} novalidate>
    {#if subscription}
      <p class="filter-name">
        <span class="muted small">{t.subscriptionsPage.filter}</span>
        <strong>{subscription.filterName}</strong>
      </p>
    {:else if filters && filters.length === 0}
      <p class="muted">{t.subscriptionsPage.noFilters}</p>
    {:else}
      <label class="field">
        <span>{t.subscriptionsPage.filter}</span>
        <select
          class="select"
          bind:value={filterId}
          aria-invalid={tried && missingFilter}
          disabled={!filters}
        >
          <option value={null}>{t.subscriptionsPage.filterPlaceholder}</option>
          {#each filters ?? [] as f (f.id)}<option value={f.id}>{f.name}</option>{/each}
        </select>
        {#if tried && missingFilter}
          <span class="field-error">{t.subscriptionsPage.problems.filterRequired}</span>
        {/if}
      </label>
    {/if}

    <fieldset class="choices">
      <legend class="field-label">{t.subscriptionsPage.frequency}</legend>
      {#each ['daily', 'weekly'] as const as s (s)}
        <label class="check">
          <input type="radio" name="subscription-schedule" value={s} bind:group={input.schedule} />
          <span>{t.subscriptionsPage.frequencies[s]}</span>
        </label>
      {/each}
    </fieldset>

    {#if input.schedule === 'weekly'}
      <fieldset class="choices inline">
        <legend class="field-label">{t.subscriptionsPage.days}</legend>
        {#each WEEKDAYS as d (d)}
          <label class="check">
            <input
              type="checkbox"
              checked={input.days.includes(d)}
              onchange={(e) => (input.days = toggleDay(input.days, d, e.currentTarget.checked))}
            />
            <span>{weekdayName(d, locale)}</span>
          </label>
        {/each}
      </fieldset>
      {#if tried && problems.days}
        <p class="field-error">{t.subscriptionsPage.problems[problems.days]}</p>
      {/if}
    {/if}

    <label class="field hour">
      <span>{t.subscriptionsPage.hour}</span>
      <select class="select" bind:value={input.hour}>
        {#each HOURS as h (h)}<option value={h}>{hourLabel(h, locale)}</option>{/each}
      </select>
      {#if tried && problems.hour}
        <span class="field-error">{t.subscriptionsPage.problems[problems.hour]}</span>
      {/if}
    </label>

    <label class="check">
      <input class="checkbox" type="checkbox" bind:checked={input.emptyToo} />
      <span>{t.subscriptionsPage.emptyTooLabel}</span>
    </label>

    {#if error}<p class="error-text" role="alert">{error}</p>{/if}
  </form>
  {#snippet footer()}
    <button type="button" class="btn btn-quiet" disabled={busy} onclick={onclose}
      >{t.common.cancel}</button
    >
    <button
      class="btn btn-primary"
      type="submit"
      form="subscription-form"
      disabled={busy || (subscription === null && filters?.length === 0)}
      >{busy ? t.common.saving : subscription ? t.common.save : t.subscriptionsPage.create}</button
    >
  {/snippet}
</Modal>

<style>
  .form {
    display: grid;
    gap: 14px;
  }

  .form p {
    margin: 0;
  }

  .filter-name {
    display: grid;
    gap: 2px;
  }

  .hour {
    max-width: 200px;
  }
</style>
