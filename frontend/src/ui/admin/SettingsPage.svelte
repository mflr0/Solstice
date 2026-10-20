<script lang="ts">
  import { hasProblems } from '../../core/admin-forms.js';
  import {
    canReset,
    generalForm,
    generalPatch,
    generalProblems,
    PASSWORD_LENGTH,
    SESSION_DAYS,
    type GeneralForm,
  } from '../../core/admin-general.js';
  import type { GeneralField, GeneralSettings } from '../../core/admin-types.js';
  import { guessBaseURL, timeZones } from '../../core/setup.js';
  import { t } from '../../i18n/t.js';
  import { LOCALE_NAMES, LOCALES } from '../../i18n/index.js';
  import { adminApi } from '../../lib/admin-api.js';
  import { ApiError, errorMessage } from '../../lib/api.js';
  import { publicSettings } from '../../lib/public-settings.svelte.js';
  import { notify } from '../../lib/ui.svelte.js';

  /**
   * The instance as a whole: its name, public address, time zone, default
   * language, how long a sign-in lasts and how long passwords must be. Each
   * field says where its value comes from when it is not set here.
   */

  type Field = keyof GeneralForm;

  const zones = timeZones();

  let settings = $state<GeneralSettings | null>(null);
  let form = $state<GeneralForm | null>(null);
  let loadError = $state('');
  let unavailable = $state(false);
  let saveError = $state('');
  let saving = $state(false);
  let tried = $state(false);
  let touched = $state<Partial<Record<Field, boolean>>>({});

  const problems = $derived(form ? generalProblems(form, zones.length ? zones : null) : {});
  const dirty = $derived(
    !!form && !!settings && Object.keys(generalPatch(form, settings)).length > 0,
  );
  const shown = (f: Field) =>
    (tried || touched[f]) && problems[f] ? t.settingsAdmin.problems[problems[f]] : '';

  function adopt(s: GeneralSettings) {
    settings = s;
    form = generalForm(s);
    tried = false;
    touched = {};
  }

  $effect(() => {
    adminApi
      .settings()
      .then(adopt)
      .catch((e: unknown) => {
        if (e instanceof ApiError && e.status === 404) unavailable = true;
        else loadError = errorMessage(e);
      });
  });

  /** Where a value comes from, when not from this page. */
  function sourceNote(field: GeneralField): string {
    const s = settings?.sources[field];
    if (!s || s === 'database') return '';
    return t.settingsAdmin.sources[s];
  }

  async function send(patch: Parameters<typeof adminApi.saveSettings>[0], message: string) {
    saving = true;
    saveError = '';
    try {
      adopt(await adminApi.saveSettings(patch));
      notify(message);
      void publicSettings.load();
    } catch (e) {
      saveError = errorMessage(e);
    } finally {
      saving = false;
    }
  }

  function submit(e: SubmitEvent) {
    e.preventDefault();
    if (!form || !settings) return;
    tried = true;
    if (hasProblems(problems)) return;
    void send(generalPatch(form, settings), t.settingsAdmin.saved);
  }

  function reset(field: GeneralField) {
    void send({ [field]: null }, t.settingsAdmin.resetDone);
  }
</script>

{#snippet origin(field: GeneralField)}
  {#if sourceNote(field)}
    <span class="hint source">{sourceNote(field)}</span>
  {:else if canReset(settings?.sources[field])}
    <button type="button" class="link-btn hint" disabled={saving} onclick={() => reset(field)}
      >{t.settingsAdmin.reset}</button
    >
  {/if}
{/snippet}

<header class="page-head">
  <div>
    <p class="eyebrow">{t.adminArea.groups.general}</p>
    <h1 class="page-title">{t.adminArea.sections.settings}</h1>
  </div>
</header>
<p class="lead-text">{t.settingsAdmin.intro}</p>

{#if unavailable}
  <p class="empty">{t.adminArea.unavailable}</p>
{:else if loadError}
  <p class="error-text" role="alert">{loadError}</p>
{:else if !form || !settings}
  <p class="muted loading">{t.common.loading}</p>
{:else}
  <form class="settings-form" onsubmit={submit} novalidate>
    <h2 class="section-title">{t.settingsAdmin.identity}</h2>
    <div class="field">
      <label class="field-label" for="set-name">{t.settingsAdmin.instanceName}</label>
      <input
        id="set-name"
        class="input"
        bind:value={form.instanceName}
        maxlength="100"
        aria-invalid={!!shown('instanceName')}
        onblur={() => (touched.instanceName = true)}
      />
      {#if shown('instanceName')}
        <span class="field-error">{shown('instanceName')}</span>
      {:else}
        <span class="hint">{t.settingsAdmin.instanceNameHint}</span>
      {/if}
      {@render origin('instanceName')}
    </div>

    <div class="field">
      <label class="field-label" for="set-url">{t.settingsAdmin.baseURL}</label>
      <input
        id="set-url"
        class="input mono"
        type="url"
        bind:value={form.baseURL}
        placeholder={guessBaseURL(location.origin)}
        spellcheck="false"
        aria-invalid={!!shown('baseURL')}
        onblur={() => (touched.baseURL = true)}
      />
      {#if shown('baseURL')}
        <span class="field-error">{shown('baseURL')}</span>
      {:else}
        <span class="hint">{t.settingsAdmin.baseURLHint}</span>
      {/if}
      {#if form.baseURL.trim() === ''}
        <button
          type="button"
          class="link-btn hint"
          onclick={() => form && (form.baseURL = guessBaseURL(location.origin))}
          >{t.settingsAdmin.useThisAddress(guessBaseURL(location.origin))}</button
        >
      {/if}
      {@render origin('baseURL')}
    </div>

    <h2 class="section-title">{t.settingsAdmin.regional}</h2>
    <div class="form-grid">
      <div class="field">
        <label class="field-label" for="set-zone">{t.settingsAdmin.timeZone}</label>
        <input
          id="set-zone"
          class="input"
          bind:value={form.timeZone}
          list="admin-zones"
          placeholder={t.settingsAdmin.serverZone}
          autocomplete="off"
          spellcheck="false"
          aria-invalid={!!shown('timeZone')}
          onblur={() => (touched.timeZone = true)}
        />
        <datalist id="admin-zones">
          {#each zones as z (z)}<option value={z}></option>{/each}
        </datalist>
        {#if shown('timeZone')}
          <span class="field-error">{shown('timeZone')}</span>
        {:else}
          <span class="hint">{t.settingsAdmin.timeZoneHint}</span>
        {/if}
        {@render origin('timeZone')}
      </div>
      <div class="field">
        <label class="field-label" for="set-lang">{t.settingsAdmin.defaultLanguage}</label>
        <select id="set-lang" class="select" bind:value={form.defaultLanguage}>
          {#each settings.languages.length ? settings.languages : LOCALES as l (l)}
            <option value={l} lang={l}>{(LOCALE_NAMES as Record<string, string>)[l] ?? l}</option>
          {/each}
        </select>
        <span class="hint">{t.settingsAdmin.defaultLanguageHint}</span>
        {@render origin('defaultLanguage')}
      </div>
    </div>

    <h2 class="section-title">{t.settingsAdmin.security}</h2>
    <div class="form-grid">
      <div class="field">
        <label class="field-label" for="set-days">{t.settingsAdmin.sessionDays}</label>
        <input
          id="set-days"
          class="input"
          type="number"
          inputmode="numeric"
          min={SESSION_DAYS.min}
          max={SESSION_DAYS.max}
          bind:value={form.sessionDays}
          aria-invalid={!!shown('sessionDays')}
          onblur={() => (touched.sessionDays = true)}
        />
        {#if shown('sessionDays')}
          <span class="field-error">{shown('sessionDays')}</span>
        {:else}
          <span class="hint">{t.settingsAdmin.sessionDaysHint}</span>
        {/if}
        {@render origin('sessionDays')}
      </div>
      <div class="field">
        <label class="field-label" for="set-pwlen">{t.settingsAdmin.passwordMinLength}</label>
        <input
          id="set-pwlen"
          class="input"
          type="number"
          inputmode="numeric"
          min={PASSWORD_LENGTH.min}
          max={PASSWORD_LENGTH.max}
          bind:value={form.passwordMinLength}
          aria-invalid={!!shown('passwordMinLength')}
          onblur={() => (touched.passwordMinLength = true)}
        />
        {#if shown('passwordMinLength')}
          <span class="field-error">{shown('passwordMinLength')}</span>
        {:else}
          <span class="hint">{t.settingsAdmin.passwordMinLengthHint}</span>
        {/if}
        {@render origin('passwordMinLength')}
      </div>
    </div>

    {#if saveError}<p class="error-text" role="alert">{saveError}</p>{/if}
    {#if dirty}
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
{/if}

<style>
  .source {
    font-style: italic;
  }

  .field > .link-btn.hint {
    justify-self: start;
    font-size: 12px;
  }

  .mono {
    font-family: var(--font-mono);
    font-size: 13px;
  }
</style>
