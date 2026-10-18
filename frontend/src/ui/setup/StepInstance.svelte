<script lang="ts">
  import { guessBaseURL } from '../../core/setup.js';
  import type { SetupMessages } from '../../i18n/setup/index.js';
  import {
    setupApi,
    setupErrorMessage,
    type ConfigSave,
    type SetupStatus,
  } from '../../lib/setup-api.js';
  import ConfigNote from './ConfigNote.svelte';
  import TimeZoneField from './TimeZoneField.svelte';

  interface Props {
    m: SetupMessages;
    status: SetupStatus;
    onsaved: (config: ConfigSave) => void;
    onback: () => void;
  }
  const { m, status, onsaved, onback }: Props = $props();

  const browserTZ = Intl.DateTimeFormat().resolvedOptions().timeZone;
  // Prefilled once, from the server's settings or the browser's.
  // svelte-ignore state_referenced_locally
  let baseURL = $state(status.baseURL || guessBaseURL(location.origin));
  // svelte-ignore state_referenced_locally
  let tz = $state(status.tz && status.tz !== 'Local' ? status.tz : browserTZ);
  let saving = $state(false);
  let error = $state('');
  let unsaved = $state<ConfigSave | null>(null);

  async function save(e: SubmitEvent) {
    e.preventDefault();
    saving = true;
    error = '';
    try {
      const res = await setupApi.saveInstance(baseURL.trim(), tz.trim());
      if (res.config.written) onsaved(res.config);
      else unsaved = res.config;
    } catch (err) {
      error = setupErrorMessage(err, m.errors.generic);
    } finally {
      saving = false;
    }
  }
</script>

<form onsubmit={save}>
  <div>
    <h1 class="page-title" id="setup-step-title" tabindex="-1">{m.instance.title}</h1>
    <p class="lead">{m.instance.lead}</p>
  </div>

  {#if unsaved}
    <ConfigNote {m} config={unsaved} />
    <div class="setup-actions">
      <span></span>
      <button class="btn btn-primary" type="button" onclick={() => unsaved && onsaved(unsaved)}
        >{m.next}</button
      >
    </div>
  {:else}
    <label class="field">
      <span>{m.instance.baseURL}</span>
      <input
        class="input"
        type="url"
        bind:value={baseURL}
        required
        spellcheck="false"
        aria-describedby="base-url-hint"
      />
      <span class="hint" id="base-url-hint">{m.instance.baseURLHint}</span>
    </label>

    <TimeZoneField
      label={m.instance.tz}
      hint={m.instance.tzHint}
      bind:value={tz}
      id="instance-tz"
    />

    <dl class="setup-facts">
      <dt>{m.instance.dataDir}</dt>
      <dd>
        <span class="mono">{status.dataDir}</span>
        <div class="muted">{m.instance.dataDirHint}</div>
      </dd>
      <dt>{m.instance.configFile}</dt>
      <dd><span class="mono">{status.configPath}</span></dd>
    </dl>

    {#if error}<p class="error-text" role="alert">{error}</p>{/if}

    <div class="setup-actions">
      <button class="btn btn-quiet" type="button" onclick={onback}>{m.back}</button>
      <button class="btn btn-primary" disabled={saving}
        >{saving ? m.instance.saving : m.instance.save}</button
      >
    </div>
  {/if}
</form>
