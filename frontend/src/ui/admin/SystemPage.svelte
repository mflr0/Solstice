<script lang="ts">
  import { splitUptime } from '../../core/admin-ops.js';
  import type { GeneralSettings, SystemInfo } from '../../core/admin-types.js';
  import { adminPath } from '../../core/areas.js';
  import { formatNumber } from '../../i18n/format.js';
  import { formatDateTime, formatSize, relative, t } from '../../i18n/t.js';
  import { adminApi } from '../../lib/admin-api.js';
  import { errorMessage } from '../../lib/api.js';
  import { updateNotice, updatesApi, type UpdateState } from '../../lib/updates.svelte.js';

  /**
   * What runs and where: version, uptime, database, data directory, time
   * zone, mail, webhooks and the update status. Read-only; each block
   * points to the page that changes it.
   */
  let info = $state<SystemInfo | null>(null);
  let settings = $state<GeneralSettings | null>(null);
  let updates = $state<UpdateState | null>(null);
  let updatesFailed = $state(false);
  let error = $state('');
  let loading = $state(false);

  const uptime = $derived(info ? splitUptime(info.uptimeSeconds) : null);
  const isDev = $derived(!!info && !/^\d/.test(info.version));
  const zone = $derived(
    settings
      ? settings.timeZone ||
          t.systemAdmin.serverZone(Intl.DateTimeFormat().resolvedOptions().timeZone)
      : '',
  );

  async function load() {
    loading = true;
    try {
      info = await adminApi.system();
      error = '';
    } catch (e) {
      error = errorMessage(e);
    } finally {
      loading = false;
    }
    // The time zone and the update status are extras: the page shows without them.
    adminApi
      .settings()
      .then((s) => (settings = s))
      .catch(() => (settings = null));
    updatesApi
      .state()
      .then((s) => {
        updates = s;
        updatesFailed = false;
        updateNotice.set(s);
      })
      .catch(() => (updatesFailed = true));
  }

  $effect(() => {
    void load();
  });
</script>

<header class="page-head">
  <div>
    <p class="eyebrow">{t.adminArea.groups.system}</p>
    <h1 class="page-title">{t.adminArea.sections.system}</h1>
  </div>
  <button type="button" class="btn" disabled={loading} onclick={() => void load()}
    >{t.adminArea.refresh}</button
  >
</header>
<p class="lead-text">{t.systemAdmin.intro}</p>

{#if error}
  <p class="error-text" role="alert">{error}</p>
{:else if !info}
  <p class="muted loading">{t.common.loading}</p>
{:else}
  {#if info.import?.state === 'running'}
    <p class="result failed" role="status">
      <strong>{t.systemAdmin.importRunning}</strong>
      <a href={adminPath('import')}>{t.systemAdmin.importPage}</a>
    </p>
  {/if}

  <h2 class="section-title">{t.systemAdmin.solstice}</h2>
  <dl class="facts">
    <div>
      <dt>{t.systemAdmin.version}</dt>
      <dd>
        <span class="key">{info.version}</span>
        {#if isDev}<span class="muted">({t.systemAdmin.dev})</span>{/if}
      </dd>
    </div>
    <div>
      <dt>{t.systemAdmin.uptime}</dt>
      <dd>
        {#if uptime}{t.systemAdmin.uptimeValue(uptime.days, uptime.hours, uptime.minutes)}{/if}
        <span class="muted small" title={formatDateTime(info.startedAt)}
          >· {t.systemAdmin.started}: {relative(info.startedAt)}</span
        >
      </dd>
    </div>
    <div>
      <dt>{t.systemAdmin.goVersion}</dt>
      <dd class="mono">{info.goVersion}</dd>
    </div>
    {#if settings}
      <div>
        <dt>{t.systemAdmin.timeZone}</dt>
        <dd>{zone}</dd>
      </div>
    {/if}
  </dl>

  <h2 class="section-title">{t.systemAdmin.updates}</h2>
  {#if updates}
    <p>
      {#if updates.newer && updates.latest}
        <span class="stamp doing">{t.systemAdmin.available(updates.latest)}</span>
      {:else if updates.latest && isDev}
        {t.systemAdmin.devBuild}
      {:else if updates.latest}
        {t.systemAdmin.upToDate}
      {:else}
        <span class="muted">{t.systemAdmin.notChecked}</span>
      {/if}
      <a href={adminPath('updates')}>{t.systemAdmin.updatesPage}</a>
    </p>
    {#if updates.checkError}
      <p class="error-text" role="alert">{t.updates.checkErrors[updates.checkError]}</p>
    {:else if updates.checkedAt}
      <p class="muted small">{t.updates.checked(relative(updates.checkedAt))}</p>
    {/if}
  {:else if updatesFailed}
    <p class="muted">{t.systemAdmin.updatesUnknown}</p>
  {:else}
    <p class="muted loading">{t.common.loading}</p>
  {/if}

  <h2 class="section-title">{t.systemAdmin.database}</h2>
  <dl class="facts">
    <div>
      <dt>{t.systemAdmin.databaseServer}</dt>
      <dd>{info.database.server || '—'}</dd>
    </div>
    <div>
      <dt>{t.systemAdmin.databaseName}</dt>
      <dd class="mono">{info.database.name}</dd>
    </div>
    <div>
      <dt>{t.systemAdmin.databaseSize}</dt>
      <dd>{formatSize(info.database.sizeBytes)}</dd>
    </div>
  </dl>

  <h2 class="section-title">{t.systemAdmin.storage}</h2>
  <dl class="facts">
    <div>
      <dt>{t.systemAdmin.dataDir}</dt>
      <dd class="mono">{info.dataDir.path}</dd>
    </div>
    <div>
      <dt>{t.systemAdmin.dataSize}</dt>
      <dd>
        {#if info.dataDir.error}
          <span class="muted" title={info.dataDir.error}>{t.systemAdmin.dataUnknown}</span>
        {:else}
          {formatSize(info.dataDir.sizeBytes)}
        {/if}
      </dd>
    </div>
    <div>
      <dt>{t.systemAdmin.attachmentsSize}</dt>
      <dd>
        {formatNumber(info.counts.attachments)} · {formatSize(info.counts.attachmentBytes)}
      </dd>
    </div>
  </dl>

  <h2 class="section-title">{t.systemAdmin.content}</h2>
  <dl class="stats">
    <div>
      <dt>{t.systemAdmin.usersActive}</dt>
      <dd>{formatNumber(info.counts.usersActive)}</dd>
    </div>
    <div>
      <dt>{t.systemAdmin.usersInactive}</dt>
      <dd>{formatNumber(info.counts.usersInactive)}</dd>
    </div>
    <div>
      <dt>{t.systemAdmin.projects}</dt>
      <dd>{formatNumber(info.counts.projects)}</dd>
    </div>
    <div>
      <dt>{t.systemAdmin.projectsArchived}</dt>
      <dd>{formatNumber(info.counts.projectsArchived)}</dd>
    </div>
    <div>
      <dt>{t.systemAdmin.issues}</dt>
      <dd>{formatNumber(info.counts.issues)}</dd>
    </div>
    <div>
      <dt>{t.systemAdmin.comments}</dt>
      <dd>{formatNumber(info.counts.comments)}</dd>
    </div>
  </dl>

  <h2 class="section-title">
    {t.systemAdmin.mail}
    <span class="actions"><a href={adminPath('mail')}>{t.systemAdmin.mailPage}</a></span>
  </h2>
  <p>
    <span class="stamp" class:done={info.mail.enabled}
      >{info.mail.enabled ? t.systemAdmin.mailOn : t.systemAdmin.mailOff}</span
    >
  </p>
  <dl class="stats">
    <div>
      <dt>{t.mailAdmin.pending}</dt>
      <dd>{formatNumber(info.mail.pending)}</dd>
    </div>
    <div class:is-alert={info.mail.failed > 0}>
      <dt>{t.mailAdmin.failed}</dt>
      <dd>{formatNumber(info.mail.failed)}</dd>
    </div>
    <div>
      <dt>{t.mailAdmin.sentLastDay}</dt>
      <dd>{formatNumber(info.mail.sentLastDay)}</dd>
    </div>
  </dl>
  {#if info.mail.oldestPending}
    <p class="muted small">{t.mailAdmin.oldestPending(relative(info.mail.oldestPending))}</p>
  {/if}

  <h2 class="section-title">
    {t.systemAdmin.webhooks}
    <span class="actions"><a href={adminPath('webhooks')}>{t.systemAdmin.webhooksPage}</a></span>
  </h2>
  <p class="muted">{t.systemAdmin.webhooksCount(info.webhooks.total)}</p>
  {#if info.webhooks.failing.length === 0}
    {#if info.webhooks.total > 0}<p>{t.systemAdmin.webhooksFine}</p>{/if}
  {:else}
    <h3 class="sub-title">{t.systemAdmin.webhooksFailing}</h3>
    <div class="table-scroll">
      <table class="ledger">
        <tbody>
          {#each info.webhooks.failing as w (w.id)}
            <tr>
              <td><a href={adminPath('webhooks', w.id)}>{w.name}</a></td>
              <td>
                <span class="stamp is-failed">{t.systemAdmin.failures(w.failed)}</span>
              </td>
              <td class="mono">{w.lastError}</td>
              <td class="num muted" title={formatDateTime(w.lastAttempt)}
                >{relative(w.lastAttempt)}</td
              >
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}

  <h2 class="section-title">{t.systemAdmin.config}</h2>
  <dl class="facts">
    <div>
      <dt>{t.systemAdmin.configFile}</dt>
      <dd>
        {#if info.config.found}
          <span class="mono">{info.config.path}</span>
        {:else}
          <span class="muted">{t.systemAdmin.configNone}</span>
        {/if}
      </dd>
    </div>
    <div>
      <dt>{t.systemAdmin.address}</dt>
      <dd class="mono">{info.config.addr}</dd>
    </div>
  </dl>

  <h2 class="section-title">{t.systemAdmin.migrations}</h2>
  <p class="muted">
    {t.systemAdmin.migrationsSummary(info.migrations.count, info.migrations.latest)}
  </p>
  <details>
    <summary>{t.systemAdmin.migrations}</summary>
    <ul class="migrations mono">
      {#each info.migrations.list as m (m.version)}
        <li>
          {m.version}
          <span class="muted">{t.systemAdmin.migrationApplied(relative(m.appliedAt))}</span>
        </li>
      {/each}
    </ul>
  </details>
{/if}

<style>
  .facts {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(13.75rem, 1fr));
    gap: 12px 28px;
    margin: 4px 0 8px;
  }

  .facts dt {
    font-size: 0.78125rem;
    color: var(--muted);
  }

  .facts dd {
    margin: 2px 0 0;
    overflow-wrap: anywhere;
  }

  .stats {
    margin-top: 4px;
  }

  .result {
    margin-bottom: 14px;
  }

  .migrations {
    margin: 8px 0 0;
    padding-left: 20px;
    display: grid;
    gap: 2px;
  }

  details summary {
    cursor: pointer;
    color: var(--ink-soft);
  }
</style>
