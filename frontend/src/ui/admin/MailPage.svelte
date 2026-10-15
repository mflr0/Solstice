<script lang="ts">
  import {
    defaultPort,
    hasProblems,
    isEmail,
    mailForm,
    mailPayload,
    mailProblems,
    type MailForm,
  } from '../../core/admin-forms.js';
  import type { MailEntry, MailQueue, MailSecurity, MailSettings } from '../../core/admin-types.js';
  import { formatNumber } from '../../i18n/format.js';
  import { formatDateTime, relative, t } from '../../i18n/t.js';
  import { adminApi } from '../../lib/admin-api.js';
  import { errorMessage } from '../../lib/api.js';
  import { session } from '../../lib/session.svelte.js';
  import { notify } from '../../lib/ui.svelte.js';
  import EmptyState from '../EmptyState.svelte';

  /**
   * Outgoing e-mail: the SMTP server, a test message that shows the
   * server's own refusal, and the queue of messages waiting or failed.
   */

  const SECURITIES: MailSecurity[] = ['starttls', 'tls', 'none'];

  let settings = $state<MailSettings | null>(null);
  let form = $state<MailForm | null>(null);
  let saved = $state('');
  let loadError = $state('');
  let saveError = $state('');
  let saving = $state(false);
  let tried = $state(false);
  let touched = $state<Record<string, boolean>>({});

  let testTo = $state(session.me?.email ?? '');
  let testing = $state(false);
  let testResult = $state<{ ok: boolean; text: string } | null>(null);

  let queue = $state<MailQueue | null>(null);
  let queueError = $state('');
  let retrying = $state(false);

  const dirty = $derived(form !== null && JSON.stringify(form) !== saved);
  const problems = $derived(form ? mailProblems(form) : {});
  const shown = (field: 'host' | 'port' | 'from') =>
    (tried || touched[field]) && problems[field] ? t.mailAdmin.problems[problems[field]] : '';
  const testProblem = $derived(
    testTo.trim() && !isEmail(testTo.trim()) ? t.mailAdmin.toInvalid : '',
  );

  function adopt(s: MailSettings) {
    settings = s;
    form = mailForm(s);
    saved = JSON.stringify(form);
    tried = false;
    touched = {};
  }

  async function loadQueue() {
    try {
      queue = await adminApi.mailQueue();
      queueError = '';
    } catch (e) {
      queueError = errorMessage(e);
    }
  }

  $effect(() => {
    adminApi
      .mail()
      .then(adopt)
      .catch((e: unknown) => (loadError = errorMessage(e)));
    void loadQueue();
  });

  function setSecurity(next: MailSecurity) {
    if (!form) return;
    // The port follows the mode while it is the previous mode's usual one.
    if (!form.port.trim() || Number(form.port) === defaultPort(form.security)) {
      form.port = String(defaultPort(next));
    }
    form.security = next;
  }

  /** Saves; true when the settings are stored. */
  async function save(): Promise<boolean> {
    if (!form) return false;
    tried = true;
    saveError = '';
    if (hasProblems(problems)) return false;
    saving = true;
    try {
      adopt(await adminApi.saveMail(mailPayload(form)));
      notify(t.mailAdmin.saved);
      return true;
    } catch (e) {
      saveError = errorMessage(e);
      return false;
    } finally {
      saving = false;
    }
  }

  async function test() {
    testResult = null;
    if (testProblem) return;
    if (dirty && !(await save())) return;
    testing = true;
    try {
      const r = await adminApi.testMail(testTo.trim());
      testResult = { ok: true, text: t.mailAdmin.testSent(r.to) };
      void loadQueue();
    } catch (e) {
      testResult = { ok: false, text: errorMessage(e) };
    } finally {
      testing = false;
    }
  }

  async function retry() {
    retrying = true;
    try {
      const r = await adminApi.retryMail();
      notify(t.mailAdmin.retried(r.retried));
      await loadQueue();
    } catch (e) {
      notify(errorMessage(e), 'error');
    } finally {
      retrying = false;
    }
  }

  function submit(e: SubmitEvent) {
    e.preventDefault();
    void save();
  }

  const stateClass = (s: MailEntry['status']) =>
    s === 'sent' ? 'done' : s === 'pending' ? 'doing' : 'is-failed';
  const stateLabel = (s: MailEntry['status']) =>
    s === 'sent' || s === 'pending' || s === 'failed' ? t.mailAdmin.states[s] : s;
  const kindLabel = (k: string) => (t.mailAdmin.kinds as Record<string, string>)[k] ?? k;
</script>

<header class="page-head">
  <div>
    <p class="eyebrow">{t.adminArea.groups.integrations}</p>
    <h1 class="page-title">{t.mailAdmin.title}</h1>
  </div>
  {#if settings}
    <span class="stamp" class:done={settings.enabled}
      >{settings.enabled ? t.mailAdmin.on : t.mailAdmin.off}</span
    >
  {/if}
</header>
<p class="lead-text">{t.mailAdmin.intro}</p>

{#if loadError}
  <p class="error-text" role="alert">{loadError}</p>
{:else if !form || !settings}
  <p class="muted loading">{t.common.loading}</p>
{:else}
  <form class="settings-form" onsubmit={submit} novalidate>
    <h2 class="section-title">{t.mailAdmin.server}</h2>
    <label class="check">
      <input class="checkbox" type="checkbox" bind:checked={form.enabled} />
      <span>{t.mailAdmin.enabled}</span>
    </label>
    <p class="form-hint">{t.mailAdmin.enabledHint}</p>

    <div class="form-grid wide-first">
      <label class="field">
        <span>{t.mailAdmin.host}</span>
        <input
          class="input"
          bind:value={form.host}
          placeholder={t.mailAdmin.hostPlaceholder}
          autocomplete="off"
          spellcheck="false"
          aria-invalid={!!shown('host')}
          onblur={() => (touched.host = true)}
        />
        {#if shown('host')}<span class="field-error">{shown('host')}</span>{/if}
      </label>
      <label class="field">
        <span>{t.mailAdmin.port}</span>
        <input
          class="input"
          inputmode="numeric"
          bind:value={form.port}
          aria-invalid={!!shown('port')}
          onblur={() => (touched.port = true)}
        />
        {#if shown('port')}<span class="field-error">{shown('port')}</span>{/if}
      </label>
    </div>

    <fieldset class="field choices">
      <legend class="field-label">{t.mailAdmin.security}</legend>
      {#each SECURITIES as s (s)}
        <label class="check">
          <input
            type="radio"
            name="security"
            checked={form.security === s}
            onchange={() => setSecurity(s)}
          />
          <span>{t.mailAdmin.securities[s]}</span>
        </label>
      {/each}
    </fieldset>

    <div class="form-grid">
      <label class="field">
        <span>{t.mailAdmin.username}</span>
        <input class="input" bind:value={form.username} autocomplete="off" spellcheck="false" />
        <span class="hint">{t.mailAdmin.usernameHint}</span>
      </label>
      <div class="field">
        <span class="field-label" id="mail-password">{t.mailAdmin.password}</span>
        {#if form.passwordAction === 'replace' || !settings.passwordSet}
          <input
            class="input"
            type="password"
            bind:value={form.password}
            aria-labelledby="mail-password"
            autocomplete="new-password"
            oninput={() => form && (form.passwordAction = 'replace')}
          />
          {#if settings.passwordSet}
            <button
              type="button"
              class="link-btn small"
              onclick={() => form && ((form.passwordAction = 'keep'), (form.password = ''))}
              >{t.mailAdmin.keepPassword}</button
            >
          {/if}
        {:else if form.passwordAction === 'remove'}
          <p class="secret-note">
            {t.mailAdmin.passwordRemoved}
            <button
              type="button"
              class="link-btn"
              onclick={() => form && (form.passwordAction = 'keep')}>{t.common.cancel}</button
            >
          </p>
        {:else}
          <p class="secret-note">
            {t.mailAdmin.passwordSet}
            <button
              type="button"
              class="link-btn"
              onclick={() => form && (form.passwordAction = 'replace')}
              >{t.mailAdmin.changePassword}</button
            >
            <button
              type="button"
              class="link-btn"
              onclick={() => form && (form.passwordAction = 'remove')}
              >{t.mailAdmin.removePassword}</button
            >
          </p>
        {/if}
      </div>
    </div>

    <h2 class="section-title">{t.mailAdmin.sender}</h2>
    <div class="form-grid">
      <label class="field">
        <span>{t.mailAdmin.from}</span>
        <input
          class="input"
          type="email"
          bind:value={form.from}
          placeholder={t.mailAdmin.fromPlaceholder}
          spellcheck="false"
          aria-invalid={!!shown('from')}
          onblur={() => (touched.from = true)}
        />
        {#if shown('from')}<span class="field-error">{shown('from')}</span>{/if}
      </label>
      <label class="field">
        <span>{t.mailAdmin.fromName}</span>
        <input class="input" bind:value={form.fromName} placeholder={t.appName} />
        <span class="hint">{t.mailAdmin.fromNameHint}</span>
      </label>
    </div>
    <p class="form-hint">
      {settings.baseUrl ? t.mailAdmin.baseUrl(settings.baseUrl) : t.mailAdmin.baseUrlMissing}
    </p>

    {#if saveError}<p class="error-text" role="alert">{saveError}</p>{/if}
    {#if dirty || saveError}
      <div class="savebar" role="region" aria-label={t.settings.unsaved}>
        <span>{t.settings.unsaved}</span>
        <button type="button" class="btn btn-quiet" onclick={() => settings && adopt(settings)}
          >{t.settings.discard}</button
        >
        <button class="btn btn-primary" type="submit" disabled={saving}
          >{saving ? t.common.saving : t.common.save}</button
        >
      </div>
    {/if}
  </form>

  <section class="card test" aria-labelledby="mail-test">
    <h2 class="card-head" id="mail-test">{t.mailAdmin.testTitle}</h2>
    <div class="card-body">
      <p class="muted small">{t.mailAdmin.testIntro}</p>
      <form
        class="test-row"
        onsubmit={(e) => {
          e.preventDefault();
          void test();
        }}
      >
        <label class="field grow">
          <span>{t.mailAdmin.testTo}</span>
          <input
            class="input"
            type="email"
            bind:value={testTo}
            placeholder={session.me?.email}
            aria-invalid={!!testProblem}
          />
        </label>
        <button class="btn btn-primary" type="submit" disabled={testing || saving || !!testProblem}
          >{testing
            ? t.mailAdmin.testing
            : dirty
              ? t.mailAdmin.saveAndTest
              : t.mailAdmin.test}</button
        >
      </form>
      {#if testProblem}<p class="field-error">{testProblem}</p>{/if}
      {#if testResult?.ok}
        <p class="result ok" role="status">{testResult.text}</p>
      {:else if testResult}
        <div class="result failed" role="alert">
          <strong>{t.mailAdmin.testFailed}</strong>
          <p class="detail">{testResult.text}</p>
        </div>
      {/if}
    </div>
  </section>
{/if}

<h2 class="section-title">
  {t.mailAdmin.queue}
  <span class="actions">
    <button type="button" class="btn btn-small btn-quiet" onclick={() => void loadQueue()}
      >{t.adminArea.refresh}</button
    >
  </span>
</h2>
{#if queueError}
  <p class="error-text" role="alert">{queueError}</p>
{:else if !queue}
  <p class="muted loading">{t.common.loading}</p>
{:else}
  <dl class="stats">
    <div>
      <dt>{t.mailAdmin.pending}</dt>
      <dd>{formatNumber(queue.pending)}</dd>
    </div>
    <div class:is-alert={queue.failed > 0}>
      <dt>{t.mailAdmin.failed}</dt>
      <dd>{formatNumber(queue.failed)}</dd>
    </div>
    <div>
      <dt>{t.mailAdmin.sentLastDay}</dt>
      <dd>{formatNumber(queue.sentLastDay)}</dd>
    </div>
    <div>
      <dt>{t.mailAdmin.batching}</dt>
      <dd>{formatNumber(queue.batching)}</dd>
    </div>
    <div>
      <dt>{t.mailAdmin.digests}</dt>
      <dd>{formatNumber(queue.digests)}</dd>
    </div>
  </dl>
  {#if queue.oldestPending}
    <p class="muted small">{t.mailAdmin.oldestPending(relative(queue.oldestPending))}</p>
  {/if}
  {#if queue.failed > 0}
    <p class="row">
      <button class="btn" disabled={retrying} onclick={() => void retry()}
        >{t.mailAdmin.retry}</button
      >
      <span class="muted small">{t.mailAdmin.retryHint}</span>
    </p>
  {/if}

  {#if queue.recent.length === 0 && queue.errors.length === 0}
    <EmptyState message={t.mailAdmin.queueEmpty} hint={t.mailAdmin.queueEmptyHint} />
  {:else}
    {#each [{ title: t.mailAdmin.errors, list: queue.errors }, { title: t.mailAdmin.recent, list: queue.recent }] as block (block.title)}
      {#if block.list.length}
        <h3 class="sub-title">{block.title}</h3>
        <div class="table-scroll">
          <table class="ledger">
            <thead>
              <tr>
                <th>{t.mailAdmin.colTo}</th>
                <th>{t.mailAdmin.colSubject}</th>
                <th>{t.mailAdmin.colState}</th>
                <th class="num">{t.mailAdmin.colWhen}</th>
              </tr>
            </thead>
            <tbody>
              {#each block.list as m (m.id)}
                <tr>
                  <td class="mono">{m.to}</td>
                  <td>
                    {m.subject}
                    <span class="muted small kind">{kindLabel(m.kind)}</span>
                    {#if m.lastError}<span class="field-error error-detail">{m.lastError}</span
                      >{/if}
                  </td>
                  <td>
                    <span class="stamp {stateClass(m.status)}">{stateLabel(m.status)}</span>
                    {#if m.attempts > 1}<span class="muted small"
                        >{t.mailAdmin.attempts(m.attempts)}</span
                      >{/if}
                  </td>
                  <td class="num muted" title={formatDateTime(m.sent ?? m.created)}
                    >{relative(m.sent ?? m.created)}</td
                  >
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {/if}
    {/each}
  {/if}
{/if}

<style>
  .secret-note {
    margin: 4px 0 0;
    display: flex;
    flex-wrap: wrap;
    gap: 4px 12px;
    align-items: baseline;
  }

  .test {
    margin-top: 28px;
    max-width: 720px;
  }

  .test-row {
    display: flex;
    gap: 10px;
    align-items: flex-end;
    flex-wrap: wrap;
  }

  .test-row .grow {
    min-width: 220px;
  }

  .kind {
    margin-left: 6px;
  }

  .error-detail {
    display: block;
    margin-top: 2px;
    font-family: var(--font-mono);
    overflow-wrap: anywhere;
  }
</style>
