<script lang="ts">
  import { BANNER_MAX, bannerProblems, sameBanner } from '../../core/admin-general.js';
  import type { BannerSettings } from '../../core/admin-types.js';
  import { formatNumber } from '../../i18n/format.js';
  import { t } from '../../i18n/t.js';
  import { adminApi } from '../../lib/admin-api.js';
  import { errorMessage } from '../../lib/api.js';
  import { publicSettings } from '../../lib/public-settings.svelte.js';
  import { notify } from '../../lib/ui.svelte.js';

  /**
   * The announcement shown above every page (maintenance tonight, new
   * version…), with a preview as people will see it.
   */

  const LEVELS: BannerSettings['level'][] = ['info', 'warning'];

  let stored = $state<BannerSettings | null>(null);
  let form = $state<BannerSettings>({ enabled: false, text: '', level: 'info' });
  let loadError = $state('');
  let saveError = $state('');
  let saving = $state(false);
  let tried = $state(false);

  const problems = $derived(bannerProblems(form));
  const dirty = $derived(stored !== null && !sameBanner(stored, form));
  const length = $derived([...form.text.trim()].length);
  const problem = $derived(
    (tried || length > BANNER_MAX) && problems.text ? t.bannerAdmin.problems[problems.text] : '',
  );

  function adopt(b: BannerSettings) {
    stored = { ...b };
    form = { ...b };
    tried = false;
  }

  $effect(() => {
    adminApi
      .settings()
      .then((s) => adopt(s.banner))
      .catch((e: unknown) => {
        loadError = errorMessage(e);
      });
  });

  async function save(next: BannerSettings, message: string) {
    tried = true;
    saveError = '';
    if (bannerProblems(next).text) return;
    saving = true;
    try {
      const s = await adminApi.saveSettings({ banner: { ...next, text: next.text.trim() } });
      adopt(s.banner);
      notify(message);
      await publicSettings.load();
    } catch (e) {
      saveError = errorMessage(e);
    } finally {
      saving = false;
    }
  }

  function submit(e: SubmitEvent) {
    e.preventDefault();
    void save(form, form.enabled ? t.bannerAdmin.shown : t.bannerAdmin.saved);
  }
</script>

<header class="page-head">
  <div>
    <p class="eyebrow">{t.adminArea.groups.general}</p>
    <h1 class="page-title">{t.adminArea.sections.banner}</h1>
  </div>
  {#if stored}
    <span class="stamp" class:done={stored.enabled}
      >{stored.enabled ? t.bannerAdmin.on : t.bannerAdmin.off}</span
    >
  {/if}
</header>
<p class="lead-text">{t.bannerAdmin.intro}</p>

{#if loadError}
  <p class="error-text" role="alert">{loadError}</p>
{:else if !stored}
  <p class="muted loading">{t.common.loading}</p>
{:else}
  <form class="settings-form" onsubmit={submit} novalidate>
    <label class="check">
      <input class="checkbox" type="checkbox" bind:checked={form.enabled} />
      <span>{t.bannerAdmin.enabled}</span>
    </label>

    <div class="field">
      <label class="field-label" for="banner-text">{t.bannerAdmin.text}</label>
      <textarea
        id="banner-text"
        class="textarea plain"
        rows="3"
        bind:value={form.text}
        placeholder={t.bannerAdmin.placeholder}
        aria-invalid={!!problem}
        aria-describedby="banner-count"
      ></textarea>
      <span class="hint count" id="banner-count" class:over={length > BANNER_MAX}
        >{t.bannerAdmin.count(formatNumber(length), formatNumber(BANNER_MAX))}</span
      >
      {#if problem}<span class="field-error">{problem}</span>{/if}
    </div>

    <fieldset class="choices inline">
      <legend class="field-label">{t.bannerAdmin.level}</legend>
      {#each LEVELS as l (l)}
        <label class="check">
          <input type="radio" name="banner-level" value={l} bind:group={form.level} />
          <span>{t.bannerAdmin.levels[l]}</span>
        </label>
      {/each}
    </fieldset>

    <div class="preview" aria-label={t.bannerAdmin.preview}>
      <p class="field-label">{t.bannerAdmin.preview}</p>
      {#if form.text.trim()}
        <div class="app-banner is-{form.level}">
          <p class="app-banner-text">{form.text.trim()}</p>
          <span class="link-btn app-banner-hide" aria-hidden="true">{t.adminArea.hideBanner}</span>
        </div>
        {#if !form.enabled}<p class="form-hint">{t.bannerAdmin.previewOff}</p>{/if}
      {:else}
        <p class="form-hint">{t.bannerAdmin.previewEmpty}</p>
      {/if}
    </div>

    {#if saveError}<p class="error-text" role="alert">{saveError}</p>{/if}
    {#if dirty}
      <div class="savebar" role="region" aria-label={t.settings.unsaved}>
        <span>{t.settings.unsaved}</span>
        <button type="button" class="btn btn-quiet" onclick={() => stored && adopt(stored)}
          >{t.settings.discard}</button
        >
        <button class="btn btn-primary" type="submit" disabled={saving}
          >{saving ? t.common.saving : t.common.save}</button
        >
      </div>
    {:else if stored.enabled}
      <p class="row">
        <button
          type="button"
          class="btn"
          disabled={saving}
          onclick={() => stored && void save({ ...stored, enabled: false }, t.bannerAdmin.hidden)}
          >{t.bannerAdmin.hideNow}</button
        >
      </p>
    {/if}
  </form>
{/if}

<style>
  .textarea.plain {
    min-height: 0;
    font: 0.875rem/1.5 var(--font-body);
  }

  .count {
    justify-self: end;
    font-family: var(--font-mono);
  }

  .count.over {
    color: var(--accent-text);
  }

  .preview {
    display: grid;
    gap: 6px;
  }

  .preview .form-hint {
    margin: 0;
  }

  /* The banner's own look comes from ShellBanner; here it sits in a frame. */
  .preview .app-banner {
    border: 1px solid var(--rule-strong);
    border-radius: var(--radius-s);
  }

  .preview .app-banner.is-warning {
    border-color: var(--accent);
  }
</style>
