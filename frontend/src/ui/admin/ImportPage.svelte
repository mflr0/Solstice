<script lang="ts">
  import { importReady } from '../../core/admin-ops.js';
  import type { ImportCheck, ImportSource, ImportView } from '../../core/admin-types.js';
  import { browserTimeZone, engineName, sourceEngine } from '../../core/setup.js';
  import { formatNumber } from '../../i18n/format.js';
  import { locale } from '../../i18n/index.js';
  import { setupCatalogues } from '../../i18n/setup/index.js';
  import { t } from '../../i18n/t.js';
  import { adminApi } from '../../lib/admin-api.js';
  import { errorMessage } from '../../lib/api.js';
  import ImportFields from '../setup/ImportFields.svelte';
  import ImportProgress from '../setup/ImportProgress.svelte';
  import ImportReport from '../setup/ImportReport.svelte';

  /**
   * Importing a Jira from the administration. It always replaces what
   * Solstice holds (there is no merge), so the page says so first, asks for
   * a backup and for the database's name, then follows the import on the
   * server: Solstice is in maintenance while it runs.
   */
  const m = setupCatalogues[locale];
  const num = (n: number) => formatNumber(n);
  const duration = (s: number) => m.duration(Math.floor(s / 60), s % 60);

  let form = $state<ImportSource>({ url: '', jiraHome: '', tz: browserTimeZone() });
  let database = $state('');
  let job = $state<ImportView | null>(null);
  let loaded = $state(false);
  let loadError = $state('');
  let editing = $state(false);
  let probe = $state<ImportCheck | null>(null);
  let backup = $state(false);
  let typed = $state('');
  let testing = $state(false);
  let starting = $state(false);
  let error = $state('');

  const ready = $derived(importReady({ tested: probe !== null, backup, typed, database }));

  async function load() {
    try {
      const s = await adminApi.importState();
      database = s.confirm;
      job = s.job;
      loadError = '';
    } catch (e) {
      loadError = errorMessage(e);
    } finally {
      loaded = true;
    }
  }

  $effect(() => {
    void load();
  });

  // A changed field voids the last test.
  $effect(() => {
    void [form.url, form.jiraHome, form.tz];
    probe = null;
  });

  // Follow the import while it runs; the server answers this page only.
  $effect(() => {
    if (job?.state !== 'running') return;
    const timer = setInterval(async () => {
      try {
        job = (await adminApi.importState()).job;
      } catch {
        // The server may be busy: the next tick tries again.
      }
    }, 1500);
    return () => clearInterval(timer);
  });

  async function test() {
    testing = true;
    error = '';
    try {
      probe = await adminApi.testImport({ ...form });
    } catch (e) {
      error = errorMessage(e);
    } finally {
      testing = false;
    }
  }

  async function start(e: SubmitEvent) {
    e.preventDefault();
    if (!ready) return;
    starting = true;
    error = '';
    try {
      job = await adminApi.startImport({ ...form, confirm: typed.trim() });
      editing = false;
      backup = false;
      typed = '';
    } catch (err) {
      error = errorMessage(err);
    } finally {
      starting = false;
    }
  }

  function again() {
    editing = true;
    probe = null;
    error = '';
  }
</script>

<header class="page-head">
  <div>
    <p class="eyebrow">{t.adminArea.groups.system}</p>
    <h1 class="page-title">{t.adminArea.sections.import}</h1>
  </div>
</header>
<p class="lead-text">{t.importAdmin.intro}</p>

{#if loadError}
  <p class="error-text" role="alert">{loadError}</p>
{:else if !loaded}
  <p class="muted loading">{t.common.loading}</p>
{:else if job?.state === 'running'}
  <section class="stage" aria-labelledby="import-running">
    <h2 class="section-title" id="import-running">{m.import.running}</h2>
    <p class="muted">{t.importAdmin.runningNote}</p>
    <ImportProgress {m} {num} {job} />
  </section>
{:else if job && !editing}
  <section class="stage" aria-labelledby="import-last">
    {#if job.state === 'failed'}
      <h2 class="section-title" id="import-last">{m.import.failed}</h2>
      <p class="muted">{m.import.failedLead}</p>
      <p class="error-text" role="alert">{job.error}</p>
    {:else}
      <h2 class="section-title" id="import-last">{t.importAdmin.lastTitle}</h2>
      <p class="result" role="status">
        <strong>{m.import.doneTitle}</strong>
        {m.import.doneLead(duration(job.seconds))}
      </p>
      <p class="muted">{t.importAdmin.doneNote}</p>
      <ImportReport {m} {num} {job} />
    {/if}
    <div class="row">
      {#if job.state === 'done'}
        <button class="btn btn-primary" onclick={() => location.reload()}
          >{t.importAdmin.reload}</button
        >
      {/if}
      <button class="btn" onclick={again}
        >{job.state === 'failed' ? m.retry : t.importAdmin.again}</button
      >
    </div>
  </section>
{:else}
  <section class="warning result failed" role="note" aria-labelledby="import-warn">
    <strong id="import-warn">{t.importAdmin.warnTitle}</strong>
    <p>{t.importAdmin.warn}</p>
    <details>
      <summary>{t.importAdmin.keepsTitle}</summary>
      <ul>
        <li>{t.importAdmin.keepSettings}</li>
        <li>{t.importAdmin.keepAccounts}</li>
        <li>{t.importAdmin.keepYou}</li>
      </ul>
    </details>
  </section>

  <form class="settings-form" onsubmit={start}>
    <h2 class="section-title">{t.importAdmin.sourceTitle}</h2>
    <p class="form-hint">{m.import.lead}</p>
    <ImportFields {m} bind:form />

    <div aria-live="polite">
      {#if probe}
        <p class="result" role="status">
          {m.import.found(
            engineName(probe.engine),
            num(probe.projects),
            num(probe.issues),
            num(probe.users),
          )}
          {#if probe.attachments}{m.import.attachmentsFound}{/if}
        </p>
        {#if probe.warning}<p class="result failed">{probe.warning}</p>{/if}
      {/if}
    </div>
    <div>
      <button
        class="btn"
        type="button"
        onclick={() => void test()}
        disabled={testing || starting || !sourceEngine(form.url)}
        >{testing ? m.import.testing : m.import.test}</button
      >
    </div>

    <label class="check">
      <input class="checkbox" type="checkbox" bind:checked={backup} />
      <span>{t.importAdmin.backup}</span>
    </label>
    <label class="field">
      <span>{t.importAdmin.typeName(database)}</span>
      <input
        class="input mono"
        bind:value={typed}
        autocomplete="off"
        spellcheck="false"
        aria-describedby="import-confirm-hint"
      />
      <span class="hint" id="import-confirm-hint">{t.importAdmin.typeNameHint}</span>
    </label>

    {#if error}<p class="error-text" role="alert">{error}</p>{/if}
    <div>
      <button class="btn btn-danger" type="submit" disabled={!ready || starting || testing}
        >{starting ? m.import.starting : t.importAdmin.start}</button
      >
    </div>
  </form>
{/if}

<style>
  .stage {
    display: grid;
    gap: 12px;
    max-width: 720px;
  }

  .stage p,
  .warning p {
    margin: 0;
  }

  .warning {
    display: grid;
    gap: 8px;
    max-width: 720px;
    margin-bottom: 22px;
  }

  .warning ul {
    margin: 6px 0 0;
    padding-left: 20px;
    display: grid;
    gap: 4px;
  }

  .warning summary {
    cursor: pointer;
    color: var(--ink-soft);
  }

  .row {
    display: flex;
    gap: 10px;
    flex-wrap: wrap;
  }
</style>
