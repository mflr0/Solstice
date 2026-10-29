<script lang="ts">
  import { hourLabel } from '../../core/account-settings.js';
  import { prefsPayload } from '../../core/admin-forms.js';
  import type { Delivery, NotificationPrefs } from '../../core/admin-types.js';
  import { adminPath } from '../../core/areas.js';
  import { formatLocale } from '../../i18n/format.js';
  import { t } from '../../i18n/t.js';
  import { accountApi } from '../../lib/admin-api.js';
  import { errorMessage } from '../../lib/api.js';
  import { session } from '../../lib/session.svelte.js';
  import { notify } from '../../lib/ui.svelte.js';
  import Dropdown from '../Dropdown.svelte';

  /**
   * What Solstice sends by e-mail: when (each change, a daily digest, never),
   * for which events, one's own changes or not, and the due-date reminder.
   * E-mails link here (/settings/notifications).
   */

  const DELIVERIES: Delivery[] = ['immediate', 'digest', 'none'];
  const EVENTS = [
    'created',
    'assigned',
    'commented',
    'mentioned',
    'status',
    'updated',
    'attachments',
    'worklogs',
    'deleted',
  ] as const;
  const HOURS = Array.from({ length: 24 }, (_, h) => h);

  let prefs = $state<NotificationPrefs | null>(null);
  let saved = $state('');
  let loadError = $state('');
  let saveError = $state('');
  let saving = $state(false);

  const dirty = $derived(prefs !== null && JSON.stringify(prefsPayload(prefs)) !== saved);
  const hour = (h: number) => hourLabel(h, formatLocale());

  function adopt(p: NotificationPrefs) {
    // Groups the server does not list yet start on, as its defaults do.
    for (const e of EVENTS) if (p.events[e] === undefined) p.events[e] = true;
    prefs = p;
    saved = JSON.stringify(prefsPayload(p));
  }

  $effect(() => {
    accountApi
      .notifications()
      .then(adopt)
      .catch((e: unknown) => (loadError = errorMessage(e)));
  });

  async function save(e: SubmitEvent) {
    e.preventDefault();
    if (!prefs) return;
    saving = true;
    saveError = '';
    try {
      adopt(await accountApi.saveNotifications(prefsPayload(prefs)));
      notify(t.account.notificationsSaved);
    } catch (err) {
      saveError = errorMessage(err);
    } finally {
      saving = false;
    }
  }

  function discard() {
    if (prefs) prefs = { ...prefs, ...JSON.parse(saved) };
    saveError = '';
  }
</script>

<header class="page-head">
  <div>
    <p class="eyebrow">{t.account.title}</p>
    <h1 class="page-title">{t.account.sections.notifications}</h1>
  </div>
</header>
<p class="lead-text">{t.account.notificationsIntro}</p>

{#if loadError}
  <p class="error-text" role="alert">{loadError}</p>
{:else if !prefs}
  <p class="muted loading">{t.common.loading}</p>
{:else}
  {#if !prefs.mailEnabled}
    <div class="result failed mail-off" role="status">
      <strong>{t.account.mailOff}</strong>
      <p class="detail-text">{t.account.mailOffHint}</p>
      {#if session.me?.admin}
        <p><a href={adminPath('mail')}>{t.account.mailOffAdmin}</a></p>
      {/if}
    </div>
  {/if}

  <form class="settings-form" onsubmit={save} novalidate>
    <h2 class="section-title">{t.account.delivery}</h2>
    <fieldset class="deliveries">
      <legend class="sr-only">{t.account.delivery}</legend>
      {#each DELIVERIES as d (d)}
        <label class="delivery" class:on={prefs.delivery === d}>
          <input type="radio" name="delivery" value={d} bind:group={prefs.delivery} />
          <span class="delivery-text">
            <strong>{t.account.deliveries[d].title}</strong>
            <span class="muted small">{t.account.deliveries[d].hint}</span>
          </span>
        </label>
      {/each}
    </fieldset>
    {#if prefs.delivery === 'digest'}
      <div class="field hour">
        <span>{t.account.digestHour}</span>
        <Dropdown
          label={t.account.digestHour}
          value={String(prefs.digestHour)}
          options={HOURS.map((h) => ({ value: String(h), label: hour(h) }))}
          onchange={(v) => prefs && (prefs.digestHour = Number(v))}
        />
      </div>
    {/if}
    {#if prefs.delivery !== 'none'}
      <p class="form-hint">{t.account.timeZone(prefs.timeZone)}</p>
    {/if}

    <h2 class="section-title">{t.account.events}</h2>
    <fieldset class="choices" disabled={prefs.delivery === 'none'}>
      <legend class="form-hint legend">
        {prefs.delivery === 'none' ? t.account.eventsOff : t.account.eventsHint}
      </legend>
      {#each EVENTS as e (e)}
        <label class="check">
          <input class="checkbox" type="checkbox" bind:checked={prefs.events[e]} />
          <span>
            {t.account.eventLabels[e].label}
            <span class="muted small">{t.account.eventLabels[e].hint}</span>
          </span>
        </label>
      {/each}
    </fieldset>
    <label class="check">
      <input
        class="checkbox"
        type="checkbox"
        bind:checked={prefs.ownChanges}
        disabled={prefs.delivery === 'none'}
      />
      <span>
        {t.account.ownChanges}
        <span class="muted small">{t.account.ownChangesHint}</span>
      </span>
    </label>

    <h2 class="section-title">{t.account.reminder}</h2>
    <label class="check">
      <input class="checkbox" type="checkbox" bind:checked={prefs.dueReminder} />
      <span>
        {t.account.dueReminder}
        <span class="muted small">{t.account.dueReminderHint}</span>
      </span>
    </label>
    {#if prefs.dueReminder}
      <div class="field hour">
        <span>{t.account.reminderHour}</span>
        <Dropdown
          label={t.account.reminderHour}
          value={String(prefs.reminderHour)}
          options={HOURS.map((h) => ({ value: String(h), label: hour(h) }))}
          onchange={(v) => prefs && (prefs.reminderHour = Number(v))}
        />
      </div>
      <p class="form-hint">{t.account.timeZone(prefs.timeZone)}</p>
    {/if}

    {#if saveError}<p class="error-text" role="alert">{saveError}</p>{/if}
    {#if dirty}
      <div class="savebar" role="region" aria-label={t.settings.unsaved}>
        <span>{t.settings.unsaved}</span>
        <button type="button" class="btn btn-quiet" onclick={discard}>{t.settings.discard}</button>
        <button class="btn btn-primary" type="submit" disabled={saving}
          >{saving ? t.common.saving : t.common.save}</button
        >
      </div>
    {/if}
  </form>
{/if}

<style>
  .mail-off {
    max-width: 45rem;
    margin-bottom: 18px;
    display: grid;
    gap: 4px;
  }

  .mail-off p {
    margin: 0;
  }

  .deliveries {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 8px;
    margin: 0;
    padding: 0;
    border: none;
    min-width: 0;
  }

  .delivery {
    display: flex;
    gap: 8px;
    align-items: baseline;
    padding: 10px 12px;
    border: var(--frame) solid var(--rule-strong);
    border-radius: var(--radius-m);
    background: var(--paper-raised);
    cursor: pointer;
  }

  .delivery.on {
    border-color: var(--accent);
    box-shadow: 0 0 0 1px var(--accent);
  }

  .delivery input {
    flex: none;
    transform: translateY(1px);
  }

  .delivery-text {
    display: grid;
    gap: 3px;
    line-height: 1.35;
  }

  .hour {
    max-width: 12.5rem;
  }

  .legend {
    margin: 0 0 6px;
  }

  .check .small {
    display: block;
  }

  @media (max-width: 760px) {
    .deliveries {
      grid-template-columns: minmax(0, 1fr);
    }
  }
</style>
