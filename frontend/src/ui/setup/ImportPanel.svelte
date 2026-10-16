<script lang="ts">
  import { IMPORT_PHASES, sourceEngine } from '../../core/setup.js';
  import type { SetupMessages } from '../../i18n/setup/index.js';
  import {
    setupApi,
    setupErrorMessage,
    type ImportState,
    type SetupAccount,
    type SetupFinished,
    type SetupStatus,
    type SourceProbe,
  } from '../../lib/setup-api.js';
  import AdminForm from './AdminForm.svelte';
  import TimeZoneField from './TimeZoneField.svelte';

  /**
   * Importing a Jira: the source, a test, the import running on the server
   * (followed by polling), its report, then who administers Solstice.
   */
  interface Props {
    m: SetupMessages;
    num: (n: number) => string;
    status: SetupStatus;
    ondone: (result: SetupFinished) => void;
    onback: () => void;
  }
  const { m, num, status, ondone, onback }: Props = $props();

  const browserTZ = Intl.DateTimeFormat().resolvedOptions().timeZone;
  // svelte-ignore state_referenced_locally
  let form = $state({
    url: '',
    jiraHome: '',
    tz: status.tz && status.tz !== 'Local' ? status.tz : browserTZ,
  });
  let probe = $state<SourceProbe | null>(null);
  // svelte-ignore state_referenced_locally
  let job = $state<ImportState | null>(status.import ?? null);
  // svelte-ignore state_referenced_locally
  let editing = $state(!status.import);
  let testing = $state(false);
  let starting = $state(false);
  let error = $state('');

  let q = $state('');
  let accounts = $state<SetupAccount[] | null>(null);
  let chosen = $state('');
  let choosing = $state(false);
  let chooseError = $state('');
  let creating = $state(false);

  const engineName = (e: string) => (e === 'mysql' ? 'MySQL' : 'PostgreSQL');
  const phaseName = (p: string) => (m.import.phases as Record<string, string>)[p] ?? p;
  const stepName = (s: string) => (m.import.report as Record<string, string>)[s] ?? s;
  const duration = (s: number) => m.duration(Math.floor(s / 60), s % 60);
  const anySkipped = $derived(job?.steps.some((s) => s.skipped > 0) ?? false);

  // A changed field voids the last test.
  $effect(() => {
    void [form.url, form.jiraHome, form.tz];
    probe = null;
  });

  // Follow the import while it runs.
  $effect(() => {
    if (job?.state !== 'running') return;
    const timer = setInterval(async () => {
      try {
        job = await setupApi.importState();
      } catch {
        // The server may be busy: the next tick tries again.
      }
    }, 1000);
    return () => clearInterval(timer);
  });

  // Once done, the accounts to choose from (searched as one types).
  $effect(() => {
    if (job?.state !== 'done') return;
    const search = q.trim();
    const timer = setTimeout(async () => {
      try {
        accounts = await setupApi.accounts(search);
        if (!chosen && accounts[0]?.admin) chosen = accounts[0].username;
      } catch (e) {
        chooseError = setupErrorMessage(e, m.errors.generic);
      }
    }, 200);
    return () => clearTimeout(timer);
  });

  async function test() {
    testing = true;
    error = '';
    try {
      probe = await setupApi.testSource({ ...form });
    } catch (e) {
      error = setupErrorMessage(e, m.errors.generic);
    } finally {
      testing = false;
    }
  }

  async function start(e: SubmitEvent) {
    e.preventDefault();
    starting = true;
    error = '';
    try {
      job = await setupApi.startImport({ ...form });
      editing = false;
    } catch (err) {
      error = setupErrorMessage(err, m.errors.generic);
    } finally {
      starting = false;
    }
  }

  async function choose(e: SubmitEvent) {
    e.preventDefault();
    if (!chosen) return;
    choosing = true;
    chooseError = '';
    try {
      ondone(await setupApi.finish({ existing: chosen }));
    } catch (err) {
      chooseError = setupErrorMessage(err, m.errors.generic);
    } finally {
      choosing = false;
    }
  }
</script>

{#if creating}
  <AdminForm
    {m}
    afterImport={true}
    {ondone}
    onback={() => (creating = false)}
    backLabel={m.import.pickInstead}
  />
{:else if editing || !job}
  <form onsubmit={start}>
    <div>
      <h1 class="page-title" id="setup-step-title" tabindex="-1">{m.import.title}</h1>
      <p class="lead">{m.import.lead}</p>
    </div>
    <label class="field">
      <span>{m.import.url}</span>
      <input
        class="input mono"
        bind:value={form.url}
        required
        autocomplete="off"
        spellcheck="false"
        placeholder="postgres://jira:…@db-jira:5432/jira"
        aria-describedby="source-url-hint"
      />
      <span class="hint" id="source-url-hint">{m.import.urlHint}</span>
    </label>
    <TimeZoneField label={m.import.tz} hint={m.import.tzHint} bind:value={form.tz} id="source-tz" />
    <label class="field">
      <span>{m.import.home}</span>
      <input
        class="input mono"
        bind:value={form.jiraHome}
        autocomplete="off"
        spellcheck="false"
        placeholder="/var/atlassian/application-data/jira"
        aria-describedby="source-home-hint"
      />
      <span class="hint" id="source-home-hint">{m.import.homeHint}</span>
    </label>

    <div aria-live="polite">
      {#if error}
        <p class="error-text" role="alert">{error}</p>
      {:else if probe}
        <p class="setup-ok">
          {m.import.found(
            engineName(probe.engine),
            num(probe.projects),
            num(probe.issues),
            num(probe.users),
          )}
          {#if probe.attachments}{m.import.attachmentsFound}{/if}
        </p>
        {#if probe.warning}<p class="setup-warn">{probe.warning}</p>{/if}
      {/if}
    </div>
    {#if status.imported || job}<p class="muted">{m.import.replaceNote}</p>{/if}

    <div class="setup-actions">
      <button class="btn btn-quiet" type="button" onclick={onback}>{m.back}</button>
      <div class="end">
        <button
          class="btn"
          type="button"
          onclick={test}
          disabled={testing || starting || !sourceEngine(form.url)}
          >{testing ? m.import.testing : m.import.test}</button
        >
        <button class="btn btn-primary" disabled={starting || testing || !probe}
          >{starting ? m.import.starting : m.import.start}</button
        >
      </div>
    </div>
  </form>
{:else if job.state === 'running'}
  <div class="setup-body">
    <div>
      <h1 class="page-title" id="setup-step-title" tabindex="-1">{m.import.running}</h1>
      <p class="lead">{m.import.runningLead}</p>
    </div>
    <div class="progress">
      <progress
        max={job.total || IMPORT_PHASES.length}
        value={job.done}
        aria-label={m.import.progressLabel}
      ></progress>
      <span class="rows mono">{m.import.rows(num(job.rows))}</span>
    </div>
    <ol class="phases">
      {#each IMPORT_PHASES as p, i (p)}
        {@const state = i < job.done ? 'done' : p === job.phase ? 'current' : 'todo'}
        <li class="phase {state}" aria-current={state === 'current' ? 'step' : undefined}>
          <span class="mark" aria-hidden="true"
            >{state === 'done' ? '✓' : state === 'current' ? '›' : ''}</span
          >
          {phaseName(p)}
          {#if state !== 'todo'}
            <span class="sr-only"
              >({state === 'done' ? m.import.phaseDone : m.import.phaseCurrent})</span
            >
          {/if}
        </li>
      {/each}
    </ol>
    <p class="sr-only" aria-live="polite">{job.phase ? phaseName(job.phase) : ''}</p>
  </div>
{:else if job.state === 'failed'}
  <div class="setup-body">
    <div>
      <h1 class="page-title" id="setup-step-title" tabindex="-1">{m.import.failed}</h1>
      <p class="lead">{m.import.failedLead}</p>
    </div>
    <p class="error-text" role="alert">{job.error}</p>
    <div class="setup-actions">
      <button class="btn btn-quiet" type="button" onclick={onback}>{m.back}</button>
      <button class="btn btn-primary" type="button" onclick={() => (editing = true)}
        >{m.retry}</button
      >
    </div>
  </div>
{:else}
  <form onsubmit={choose}>
    <div>
      <h1 class="page-title" id="setup-step-title" tabindex="-1">{m.import.doneTitle}</h1>
      <p class="lead">{m.import.doneLead(duration(job.seconds))}</p>
    </div>

    <details>
      <summary>{m.import.reportCaption}</summary>
      <div class="table-scroll">
        <table class="ledger report">
          <thead>
            <tr>
              <th scope="col">{m.import.reportItem}</th>
              <th scope="col" class="num">{m.import.reportImported}</th>
              {#if anySkipped}<th scope="col" class="num">{m.import.reportSkipped}</th>{/if}
            </tr>
          </thead>
          <tbody>
            {#each job.steps as s (s.name)}
              <tr>
                <td>{stepName(s.name)}</td>
                <td class="num">{num(s.imported)}</td>
                {#if anySkipped}<td class="num">{s.skipped ? num(s.skipped) : ''}</td>{/if}
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
      {#if anySkipped}<p class="muted">{m.import.skippedHint}</p>{/if}
    </details>

    {#if job.warnings.length > 0}
      <section class="warnings" aria-labelledby="import-warnings">
        <h2 class="section-title" id="import-warnings">{m.import.warnings}</h2>
        {#if m.import.warningsNote}<p class="muted">{m.import.warningsNote}</p>{/if}
        <ul lang="en">
          {#each job.warnings as w, i (i)}<li>{w}</li>{/each}
        </ul>
      </section>
    {/if}

    <section class="choose" aria-labelledby="import-choose">
      <h2 class="section-title" id="import-choose">{m.import.chooseTitle}</h2>
      <p class="muted">{m.import.chooseLead}</p>
      <input
        class="input"
        type="search"
        bind:value={q}
        placeholder={m.import.search}
        aria-label={m.import.search}
      />
      {#if accounts && accounts.length === 0}
        <p class="empty">{m.import.noAccounts}</p>
      {:else if accounts}
        <fieldset class="accounts">
          <legend class="sr-only">{m.import.accounts}</legend>
          {#each accounts as a (a.username)}
            <label class="account" class:on={chosen === a.username}>
              <input type="radio" name="admin" value={a.username} bind:group={chosen} />
              <span class="who">
                <strong>{a.name}</strong>
                <span class="key">{a.username}</span>
                {#if a.email}<span class="muted">{a.email}</span>{/if}
              </span>
              {#if a.admin}<span class="chip">{m.import.jiraAdmin}</span>{/if}
            </label>
          {/each}
        </fieldset>
      {/if}
    </section>

    {#if chooseError}<p class="error-text" role="alert">{chooseError}</p>{/if}

    <div class="setup-actions">
      <button class="link-btn" type="button" onclick={() => (creating = true)}
        >{m.import.createInstead}</button
      >
      <button class="btn btn-primary" disabled={!chosen || choosing}
        >{choosing ? m.import.choosing : m.import.choose}</button
      >
    </div>
  </form>
{/if}

<style>
  .progress {
    display: grid;
    gap: 8px;
  }

  progress {
    width: 100%;
    height: 8px;
    appearance: none;
    border: 1px solid var(--rule-strong);
    border-radius: var(--radius-xs);
    background: var(--paper-sunk);
    overflow: hidden;
  }

  progress::-webkit-progress-bar {
    background: var(--paper-sunk);
  }

  progress::-webkit-progress-value {
    background: var(--accent);
    transition: width 0.4s ease;
  }

  progress::-moz-progress-bar {
    background: var(--accent);
  }

  .rows {
    color: var(--ink-soft);
  }

  .phases {
    list-style: none;
    margin: 0;
    padding: 0;
    columns: 2 220px;
    column-gap: 24px;
  }

  .phase {
    display: flex;
    gap: 8px;
    padding: 3px 0;
    color: var(--muted);
    break-inside: avoid;
  }

  .phase .mark {
    width: 1em;
    flex: none;
    text-align: center;
  }

  .phase.done {
    color: var(--ink-soft);
  }

  .phase.done .mark {
    color: var(--stamp-done);
  }

  .phase.current {
    color: var(--ink);
    font-weight: 600;
  }

  .phase.current .mark {
    color: var(--accent-text);
  }

  .report tbody tr {
    cursor: default;
  }

  .warnings ul {
    margin: 0;
    padding-left: 20px;
    display: grid;
    gap: 4px;
  }

  .section-title {
    margin-top: 6px;
  }

  .choose {
    display: grid;
    gap: 10px;
  }

  .choose p {
    margin: 0;
  }

  .accounts {
    border: none;
    margin: 0;
    padding: 0;
    display: grid;
    gap: 4px;
    max-height: 320px;
    overflow: auto;
  }

  .account {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 8px 10px;
    border: var(--frame) solid var(--rule);
    border-radius: var(--radius-s);
    background: var(--paper-raised);
    cursor: pointer;
  }

  .account.on {
    border-color: var(--accent);
  }

  .account input {
    accent-color: var(--accent);
    margin: 0;
  }

  .account:has(input:focus-visible) {
    outline: 2px solid var(--focus);
    outline-offset: 2px;
  }

  .who {
    display: flex;
    flex-wrap: wrap;
    gap: 2px 10px;
    align-items: baseline;
    min-width: 0;
    flex: 1;
  }

  .who .key {
    font-size: 12.5px;
    color: var(--ink-soft);
  }

  .who .muted {
    font-size: 12.5px;
    overflow-wrap: anywhere;
  }
</style>
