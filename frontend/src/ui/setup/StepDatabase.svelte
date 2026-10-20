<script lang="ts">
  import { databaseSQL } from '../../core/setup.js';
  import type { SetupMessages } from '../../i18n/setup/index.js';
  import {
    setupApi,
    setupErrorMessage,
    type ConfigSave,
    type DatabaseProbe,
  } from '../../lib/setup-api.js';
  import ConfigNote from './ConfigNote.svelte';

  interface Props {
    m: SetupMessages;
    onsaved: (config: ConfigSave, existing: boolean) => Promise<void>;
    onback: () => void;
  }
  const { m, onsaved, onback }: Props = $props();

  let form = $state({
    host: 'localhost',
    port: 3306,
    name: 'solstice',
    user: 'solstice',
    password: '',
  });
  let probe = $state<DatabaseProbe | null>(null);
  let testing = $state(false);
  let saving = $state(false);
  let error = $state('');
  /** Saved, but the configuration file needs the administrator's hand. */
  let unsaved = $state<{ config: ConfigSave; existing: boolean } | null>(null);

  // A changed field voids the last test.
  $effect(() => {
    void [form.host, form.port, form.name, form.user, form.password];
    probe = null;
  });

  async function test() {
    testing = true;
    error = '';
    try {
      const p = await setupApi.testDatabase({ ...form, port: Number(form.port) || 3306 });
      probe = p;
    } catch (e) {
      error = setupErrorMessage(e);
    } finally {
      testing = false;
    }
  }

  async function save(e: SubmitEvent) {
    e.preventDefault();
    saving = true;
    error = '';
    try {
      const res = await setupApi.saveDatabase({ ...form, port: Number(form.port) || 3306 });
      if (res.config.written) await onsaved(res.config, res.existing);
      else unsaved = { config: res.config, existing: res.existing };
    } catch (err) {
      error = setupErrorMessage(err);
    } finally {
      saving = false;
    }
  }
</script>

<form onsubmit={save}>
  <div>
    <h1 class="page-title" id="setup-step-title" tabindex="-1">{m.database.title}</h1>
    <p class="lead">{m.database.lead}</p>
  </div>

  {#if unsaved}
    <ConfigNote {m} config={unsaved.config} />
    <div class="setup-actions">
      <span></span>
      <button
        class="btn btn-primary"
        type="button"
        onclick={() => unsaved && onsaved(unsaved.config, unsaved.existing)}>{m.next}</button
      >
    </div>
  {:else}
    <div class="setup-row narrow-last">
      <label class="field">
        <span>{m.database.host}</span>
        <input
          class="input"
          bind:value={form.host}
          required
          autocomplete="off"
          spellcheck="false"
        />
      </label>
      <label class="field">
        <span>{m.database.port}</span>
        <input class="input" type="number" min="1" max="65535" bind:value={form.port} required />
      </label>
    </div>
    <label class="field">
      <span>{m.database.name}</span>
      <input class="input" bind:value={form.name} required autocomplete="off" spellcheck="false" />
    </label>
    <div class="setup-row">
      <label class="field">
        <span>{m.database.user}</span>
        <input
          class="input"
          bind:value={form.user}
          required
          autocomplete="off"
          spellcheck="false"
        />
      </label>
      <label class="field">
        <span>{m.database.password}</span>
        <input
          class="input"
          type="password"
          bind:value={form.password}
          autocomplete="new-password"
        />
      </label>
    </div>

    <details>
      <summary>{m.database.prepareTitle}</summary>
      <p>{m.database.prepareLead}</p>
      <pre class="setup-code">{databaseSQL(form.name, form.user)}</pre>
    </details>

    <div aria-live="polite">
      {#if error}
        <p class="error-text" role="alert">{error}</p>
      {:else if probe}
        <p class="setup-ok">
          {m.database.ok(probe.server)}
          {probe.accounts > 0
            ? m.database.existing(probe.accounts)
            : probe.tables > 0
              ? m.database.upgrade
              : m.database.empty}
        </p>
      {/if}
    </div>

    <div class="setup-actions">
      <button class="btn btn-quiet" type="button" onclick={onback}>{m.back}</button>
      <div class="end">
        <button class="btn" type="button" onclick={test} disabled={testing || saving}
          >{testing ? m.database.testing : m.database.test}</button
        >
        <button class="btn btn-primary" disabled={saving || testing}
          >{saving
            ? m.database.saving
            : probe && probe.accounts > 0
              ? m.database.saveExisting
              : m.database.save}</button
        >
      </div>
    </div>
  {/if}
</form>
