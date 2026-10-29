<script lang="ts">
  import type { ImportCheck, ImportView } from '../../core/admin-types.js';
  import { engineName, sourceEngine, wizardTimeZone } from '../../core/setup.js';
  import type { SetupMessages } from '../../i18n/setup/index.js';
  import {
    setupApi,
    setupErrorMessage,
    type SetupAccount,
    type SetupFinished,
    type SetupStatus,
  } from '../../lib/setup-api.js';
  import AdminForm from './AdminForm.svelte';
  import ImportFields from './ImportFields.svelte';
  import ImportProgress from './ImportProgress.svelte';
  import ImportReport from './ImportReport.svelte';

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

  // svelte-ignore state_referenced_locally
  let form = $state({
    url: '',
    jiraHome: '',
    tz: wizardTimeZone(status.tz),
  });
  let probe = $state<ImportCheck | null>(null);
  // svelte-ignore state_referenced_locally
  let job = $state<ImportView | null>(status.import ?? null);
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

  const duration = (s: number) => m.duration(Math.floor(s / 60), s % 60);

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
        chooseError = setupErrorMessage(e);
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
      error = setupErrorMessage(e);
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
      error = setupErrorMessage(err);
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
      chooseError = setupErrorMessage(err);
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
    <ImportFields {m} bind:form />

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
    <ImportProgress {m} {num} {job} />
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

    <ImportReport {m} {num} {job} />

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
    max-height: 20rem;
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
    font-size: 0.78125rem;
    color: var(--ink-soft);
  }

  .who .muted {
    font-size: 0.78125rem;
    overflow-wrap: anywhere;
  }
</style>
