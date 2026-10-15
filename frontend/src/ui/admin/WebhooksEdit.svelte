<script lang="ts">
  import {
    hasProblems,
    newWebhookForm,
    webhookPayload,
    webhookProblems,
    type WebhookForm,
  } from '../../core/admin-forms.js';
  import {
    deliveryTone,
    groupEvents,
    groupState,
    toggleEvent,
    toggleGroup,
  } from '../../core/admin-integrations.js';
  import type { Webhook, WebhookDelivery } from '../../core/admin-types.js';
  import { paths } from '../../core/route.js';
  import { formatNumber } from '../../i18n/format.js';
  import { t } from '../../i18n/t.js';
  import { adminApi } from '../../lib/admin-api.js';
  import { api, errorMessage } from '../../lib/api.js';
  import { router } from '../../lib/router.svelte.js';
  import { notify } from '../../lib/ui.svelte.js';
  import ConfirmDialog from '../ConfirmDialog.svelte';
  import JqlEditor from '../search/JqlEditor.svelte';
  import WebhooksDeliveries from './WebhooksDeliveries.svelte';

  /**
   * One webhook: where it calls, on which events, for which issues, signed
   * with which secret; a test call; then its deliveries. With no id, a new
   * webhook.
   */
  interface Props {
    id: number | null;
  }
  const { id }: Props = $props();

  let events = $state<string[] | null>(null);
  let hook = $state<Webhook | null>(null);
  let form = $state<WebhookForm | null>(null);
  let saved = $state('');
  let loadError = $state('');
  let saveError = $state('');
  let saving = $state(false);
  let tried = $state(false);
  let touched = $state<Record<string, boolean>>({});
  let deleting = $state(false);

  /** The filter as last loaded; the editor's text is form.jql. */
  let jqlValue = $state('');
  let jqlError = $state<string | null>(null);
  let jqlPreview = $state('');

  let testing = $state(false);
  let testResult = $state<WebhookDelivery | null>(null);
  let testError = $state('');
  let deliveriesKey = $state(0);

  const isNew = $derived(id === null);
  const groups = $derived(groupEvents(events ?? []));
  const dirty = $derived(form !== null && (isNew || JSON.stringify(form) !== saved));
  const problems = $derived(form ? webhookProblems(form) : {});
  const shown = (f: 'name' | 'url' | 'events') =>
    (tried || touched[f]) && problems[f] ? t.hooksAdmin.problems[problems[f]] : '';

  function adopt(w: Webhook) {
    hook = w;
    form = {
      name: w.name,
      url: w.url,
      events: [...w.events],
      jql: w.jql,
      enabled: w.enabled,
      excludeBody: w.excludeBody,
      secret: '',
      secretAction: w.hasSecret ? 'keep' : 'replace',
    };
    jqlValue = w.jql;
    jqlError = null;
    jqlPreview = '';
    saved = JSON.stringify(form);
    tried = false;
    touched = {};
  }

  $effect(() => {
    const wanted = id;
    adminApi
      .webhookEvents()
      .then((e) => (events = e))
      .catch((e: unknown) => (loadError = errorMessage(e)));
    if (wanted === null) {
      form = newWebhookForm();
      saved = JSON.stringify(form);
      document.getElementById('hook-name')?.focus();
    } else {
      adminApi
        .webhook(wanted)
        .then(adopt)
        .catch((e: unknown) => (loadError = errorMessage(e)));
    }
  });

  // The name field takes the focus on a new webhook, once the form shows.
  $effect(() => {
    if (isNew && form && events) document.getElementById('hook-name')?.focus();
  });

  async function previewJql(jql: string) {
    if (!form) return;
    form.jql = jql;
    jqlPreview = '';
    jqlError = null;
    if (!jql.trim()) return;
    try {
      const page = await api.issues({ jql: jql.trim(), size: 1 });
      jqlPreview = t.hooksAdmin.jqlMatches(page.total);
    } catch (e) {
      jqlError = errorMessage(e);
    }
  }

  /** Saves; true when stored. */
  async function save(): Promise<boolean> {
    if (!form) return false;
    tried = true;
    saveError = '';
    if (hasProblems(problems)) return false;
    saving = true;
    try {
      const payload = webhookPayload(form);
      if (id === null) {
        const created = await adminApi.createWebhook(payload);
        notify(t.hooksAdmin.created(created.name));
        router.navigate(paths.admin('webhooks', created.id), { replace: true });
      } else {
        adopt(await adminApi.saveWebhook(id, payload));
        notify(t.hooksAdmin.saved);
      }
      return true;
    } catch (e) {
      saveError = errorMessage(e);
      return false;
    } finally {
      saving = false;
    }
  }

  async function test() {
    if (id === null) return;
    testResult = null;
    testError = '';
    if (dirty && !(await save())) return;
    testing = true;
    try {
      testResult = await adminApi.testWebhook(id);
      deliveriesKey++;
    } catch (e) {
      testError = errorMessage(e);
    } finally {
      testing = false;
    }
  }

  function submit(e: SubmitEvent) {
    e.preventDefault();
    void save();
  }

  function discard() {
    if (hook) adopt(hook);
    else router.navigate(paths.admin('webhooks'));
  }

  const eventLabel = (e: string) => (t.hooksAdmin.eventNames as Record<string, string>)[e] ?? e;
</script>

<header class="page-head">
  <div>
    <p class="eyebrow"><a href={paths.admin('webhooks')}>← {t.hooksAdmin.title}</a></p>
    <h1 class="page-title">{isNew ? t.hooksAdmin.newHook : (hook?.name ?? t.hooksAdmin.title)}</h1>
  </div>
  {#if hook}
    <span class="stamp" class:done={hook.enabled}
      >{hook.enabled ? t.hooksAdmin.enabledState : t.hooksAdmin.disabledState}</span
    >
  {/if}
</header>
{#if isNew}<p class="lead-text">{t.hooksAdmin.newIntro}</p>{/if}

{#if loadError}
  <p class="error-text" role="alert">{loadError}</p>
  <p><a class="btn" href={paths.admin('webhooks')}>{t.hooksAdmin.backToList}</a></p>
{:else if !form || !events}
  <p class="muted loading">{t.common.loading}</p>
{:else}
  <form class="settings-form" onsubmit={submit} novalidate>
    <h2 class="section-title">{t.hooksAdmin.destination}</h2>
    <label class="field">
      <span>{t.hooksAdmin.name}</span>
      <input
        id="hook-name"
        class="input"
        bind:value={form.name}
        maxlength="255"
        autocomplete="off"
        aria-invalid={!!shown('name')}
        onblur={() => (touched.name = true)}
      />
      {#if shown('name')}<span class="field-error">{shown('name')}</span>{/if}
    </label>
    <label class="field">
      <span>{t.hooksAdmin.url}</span>
      <input
        class="input mono"
        type="url"
        bind:value={form.url}
        placeholder={t.hooksAdmin.urlPlaceholder}
        spellcheck="false"
        autocomplete="off"
        aria-invalid={!!shown('url')}
        onblur={() => (touched.url = true)}
      />
      {#if shown('url')}
        <span class="field-error">{shown('url')}</span>
      {:else}
        <span class="hint">{t.hooksAdmin.urlHint}</span>
      {/if}
    </label>
    <label class="check">
      <input class="checkbox" type="checkbox" bind:checked={form.enabled} />
      <span>{t.hooksAdmin.enabled}</span>
    </label>
    <label class="check">
      <input class="checkbox" type="checkbox" bind:checked={form.excludeBody} />
      <span>{t.hooksAdmin.excludeBody}</span>
    </label>
    <p class="form-hint">{t.hooksAdmin.excludeBodyHint}</p>

    <h2 class="section-title">{t.hooksAdmin.events}</h2>
    {#if shown('events')}<p class="field-error" role="alert">{shown('events')}</p>{/if}
    <div class="event-groups">
      {#each groups as g (g.group)}
        {@const state = groupState(form.events, g.events)}
        <fieldset class="choices">
          <legend class="field-label">
            {t.hooksAdmin.groups[g.group]}
            {#if g.events.length > 1}
              <button
                type="button"
                class="link-btn all"
                onclick={() =>
                  form &&
                  (form.events = toggleGroup(form.events, g.events, state !== 'all', events ?? []))}
                >{state === 'all' ? t.hooksAdmin.noneInGroup : t.hooksAdmin.allInGroup}</button
              >
            {/if}
          </legend>
          {#each g.events as e (e)}
            <label class="check">
              <input
                class="checkbox"
                type="checkbox"
                checked={form.events.includes(e)}
                onchange={(ev) => {
                  if (!form) return;
                  form.events = toggleEvent(form.events, e, ev.currentTarget.checked, events ?? []);
                  touched.events = true;
                }}
              />
              <span>{eventLabel(e)} <code class="code muted">{e}</code></span>
            </label>
          {/each}
        </fieldset>
      {/each}
    </div>

    <h2 class="section-title">{t.hooksAdmin.jql}</h2>
    <div class="jql">
      <JqlEditor
        value={jqlValue}
        bind:draft={form.jql}
        error={jqlError}
        onsubmit={(jql) => void previewJql(jql)}
      />
    </div>
    <p class="form-hint jql-hint">{jqlPreview || t.hooksAdmin.jqlHint}</p>

    <h2 class="section-title">{t.hooksAdmin.secret}</h2>
    <div class="field">
      <span class="field-label" id="hook-secret">{t.hooksAdmin.secretLabel}</span>
      {#if form.secretAction === 'replace' || !hook?.hasSecret}
        <input
          class="input mono"
          type="password"
          bind:value={form.secret}
          aria-labelledby="hook-secret"
          autocomplete="new-password"
          maxlength="255"
          oninput={() => form && (form.secretAction = 'replace')}
        />
        {#if hook?.hasSecret}
          <button
            type="button"
            class="link-btn small keep"
            onclick={() => form && ((form.secretAction = 'keep'), (form.secret = ''))}
            >{t.hooksAdmin.keepSecret}</button
          >
        {/if}
      {:else if form.secretAction === 'remove'}
        <p class="secret-note">
          {t.hooksAdmin.secretRemoved}
          <button
            type="button"
            class="link-btn"
            onclick={() => form && (form.secretAction = 'keep')}>{t.common.cancel}</button
          >
        </p>
      {:else}
        <p class="secret-note">
          {t.hooksAdmin.secretSet}
          <button
            type="button"
            class="link-btn"
            onclick={() => form && (form.secretAction = 'replace')}
            >{t.hooksAdmin.changeSecret}</button
          >
          <button
            type="button"
            class="link-btn"
            onclick={() => form && (form.secretAction = 'remove')}
            >{t.hooksAdmin.removeSecret}</button
          >
        </p>
      {/if}
      <span class="hint">{t.hooksAdmin.secretHint}</span>
    </div>

    {#if saveError}<p class="error-text" role="alert">{saveError}</p>{/if}
    {#if dirty || saveError}
      <div class="savebar" role="region" aria-label={t.settings.unsaved}>
        <span>{isNew ? t.hooksAdmin.notCreated : t.settings.unsaved}</span>
        <button type="button" class="btn btn-quiet" onclick={discard}
          >{isNew ? t.common.cancel : t.settings.discard}</button
        >
        <button class="btn btn-primary" type="submit" disabled={saving}
          >{saving ? t.common.saving : isNew ? t.hooksAdmin.create : t.common.save}</button
        >
      </div>
    {/if}
  </form>

  {#if hook && id !== null}
    <section class="card test" aria-labelledby="hook-test">
      <h2 class="card-head" id="hook-test">{t.hooksAdmin.testTitle}</h2>
      <div class="card-body">
        <p class="muted small">{t.hooksAdmin.testIntro}</p>
        <p>
          <button class="btn btn-primary" disabled={testing || saving} onclick={() => void test()}
            >{testing
              ? t.hooksAdmin.testing
              : dirty
                ? t.hooksAdmin.saveAndTest
                : t.hooksAdmin.test}</button
          >
        </p>
        {#if testError}
          <div class="result failed" role="alert">
            <strong>{t.hooksAdmin.testFailed}</strong>
            <p class="detail">{testError}</p>
          </div>
        {:else if testResult}
          {@const tone = deliveryTone(testResult)}
          <div
            class="result"
            class:failed={tone === 'failed'}
            role={tone === 'failed' ? 'alert' : 'status'}
          >
            <strong
              >{tone === 'failed' ? t.hooksAdmin.testFailed : t.hooksAdmin.testOk}</strong
            >
            <p class="detail">
              {testResult.statusCode !== null
                ? t.hooksAdmin.httpAnswer(testResult.statusCode)
                : t.hooksAdmin.noAnswer}{#if testResult.durationMs !== null}
                · {t.hooksAdmin.duration(formatNumber(testResult.durationMs))}{/if}
            </p>
            {#if testResult.error}<p class="detail">{testResult.error}</p>{/if}
          </div>
        {/if}
      </div>
    </section>

    {#key deliveriesKey}<WebhooksDeliveries {id} />{/key}

    <h2 class="section-title">{t.hooksAdmin.dangerTitle}</h2>
    <p class="row danger">
      <button class="btn btn-danger" onclick={() => (deleting = true)}
        >{t.hooksAdmin.delete}</button
      >
      <span class="muted small">{t.hooksAdmin.deleteHint}</span>
    </p>

    <ConfirmDialog
      open={deleting}
      title={t.hooksAdmin.deleteTitle(hook.name)}
      confirm={t.hooksAdmin.delete}
      onclose={() => (deleting = false)}
      onconfirm={async () => {
        await adminApi.deleteWebhook(id);
        notify(t.hooksAdmin.deleted(hook?.name ?? ''));
        router.navigate(paths.admin('webhooks'), { replace: true });
      }}
    >
      <p>{t.hooksAdmin.deleteBody}</p>
    </ConfirmDialog>
  {/if}
{/if}

<style>
  .event-groups {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
    gap: 16px 24px;
  }

  .event-groups legend {
    display: flex;
    gap: 10px;
    align-items: baseline;
  }

  .all {
    font-size: 12px;
    font-weight: 400;
  }

  .code {
    margin-left: 4px;
    font: 11.5px var(--font-mono);
    overflow-wrap: anywhere;
  }

  .jql-hint {
    margin-top: -6px;
  }

  .secret-note {
    margin: 4px 0 0;
    display: flex;
    flex-wrap: wrap;
    gap: 4px 12px;
    align-items: baseline;
  }

  .keep {
    justify-self: start;
  }

  .test {
    margin-top: 28px;
    max-width: 720px;
  }

  .danger {
    margin: 0;
  }
</style>
