<script lang="ts">
  import {
    findingPlace,
    themeActions,
    themeFindings,
    type AdminThemes,
    type AdminTheme,
    type ThemeFinding,
  } from '../../core/admin-themes.js';
  import { DEFAULT_SKIN, localized, type Scheme, type Skin } from '../../core/appearance.js';
  import { locale } from '../../i18n/index.js';
  import { t } from '../../i18n/t.js';
  import { adminApi } from '../../lib/admin-api.js';
  import { ApiError, errorMessage } from '../../lib/api.js';
  import { loadSkin, reloadSkin, reloadThemes, skinSheets } from '../../lib/skins.svelte.js';
  import { notify } from '../../lib/ui.svelte.js';
  import ConfirmDialog from '../ConfirmDialog.svelte';
  import SkinMock from '../SkinMock.svelte';

  /**
   * The instance's themes: built-in and imported ones, each with its preview.
   * Importing or replacing takes a zip; a refused package shows its findings.
   */
  const m = t.themesAdmin;

  let data = $state<AdminThemes | null>(null);
  let loadError = $state('');
  let fileInput = $state<HTMLInputElement>();
  /** The theme the chosen file replaces; null imports a new one. */
  let replacing = $state<AdminTheme | null>(null);
  let uploading = $state(false);
  let switching = $state<string | null>(null);
  let uploadError = $state('');
  let refused = $state<{ file: string; findings: ThemeFinding[] } | null>(null);
  let removing = $state<AdminTheme | null>(null);
  let previewing = $state<string | null>(null);
  let previewFailed = $state(false);

  const nameOf = (theme: AdminTheme) => localized(theme.name, locale);
  const schemes = (skin: Skin): Scheme[] =>
    skinSheets[skin]?.dark === false ? ['light'] : ['light', 'dark'];
  const fallback = $derived(data?.themes.find((theme) => theme.id === DEFAULT_SKIN));

  async function load() {
    try {
      data = await adminApi.themes();
      loadError = '';
    } catch (e) {
      loadError = errorMessage(e);
    }
  }

  $effect(() => {
    void load();
  });

  // A theme that is off has no stylesheet to fetch: the server only serves enabled ones.
  $effect(() => {
    const theme = data?.themes.find((candidate) => candidate.id === previewing);
    previewFailed = false;
    if (!theme?.enabled) return;
    void loadSkin(theme.id).then((loaded) => {
      if (previewing === theme.id) previewFailed = !loaded;
    });
  });

  /** Brings the page and the theme menus up to date after a change. */
  async function changed() {
    await Promise.all([load(), reloadThemes()]);
  }

  function chooseFile(target: AdminTheme | null) {
    replacing = target;
    fileInput?.click();
  }

  async function upload(e: Event) {
    const input = e.currentTarget as HTMLInputElement;
    const file = input.files?.[0];
    if (!file) return;
    const target = replacing;
    uploading = true;
    uploadError = '';
    refused = null;
    try {
      const theme = target
        ? await adminApi.replaceTheme(target.id, file)
        : await adminApi.importTheme(file);
      await reloadSkin(theme.id);
      await changed();
      notify(target ? m.replaced(nameOf(theme), theme.version) : m.imported(nameOf(theme)));
    } catch (err) {
      const findings = err instanceof ApiError ? themeFindings(err.data) : [];
      if (findings.length) refused = { file: file.name, findings };
      else uploadError = errorMessage(err);
    } finally {
      uploading = false;
      // The same file, fixed, must fire the change event again.
      input.value = '';
    }
  }

  async function setEnabled(theme: AdminTheme, on: boolean) {
    switching = theme.id;
    try {
      await adminApi.switchTheme(theme.id, on);
      await changed();
      notify(on ? m.enabled(nameOf(theme)) : m.disabled(nameOf(theme)));
    } catch (e) {
      notify(errorMessage(e), 'error');
    } finally {
      switching = null;
    }
  }

  async function confirmRemove() {
    if (!removing) return;
    const theme = removing;
    await adminApi.deleteTheme(theme.id);
    if (previewing === theme.id) previewing = null;
    await changed();
    notify(m.deleted(nameOf(theme)));
  }
</script>

<header class="page-head">
  <div>
    <p class="eyebrow">{t.adminArea.groups.general}</p>
    <h1 class="page-title">{t.adminArea.sections.themes}</h1>
  </div>
  <button class="btn btn-primary" disabled={uploading} onclick={() => chooseFile(null)}
    >{uploading ? m.uploading : m.import}</button
  >
</header>
<p class="lead-text">{m.intro}</p>
<input bind:this={fileInput} type="file" accept=".zip,application/zip" hidden onchange={upload} />

{#if uploadError}<p class="error-text" role="alert">{uploadError}</p>{/if}

{#if refused}
  <section class="findings" role="alert" aria-labelledby="theme-findings">
    <div class="findings-head">
      <h2 id="theme-findings" class="section-title">{m.refused(refused.file)}</h2>
      <button class="btn btn-small btn-quiet" onclick={() => (refused = null)}
        >{t.common.close}</button
      >
    </div>
    <ul>
      {#each refused.findings as finding, i (i)}
        <li>
          <span class="place">{findingPlace(finding)}</span>
          <span class="stamp">{finding.rule}</span>
          <span class="message">{finding.message}</span>
        </li>
      {/each}
    </ul>
  </section>
{/if}

{#if loadError}
  <p class="error-text" role="alert">{loadError}</p>
{:else if !data}
  <p class="muted loading">{t.common.loading}</p>
{:else}
  <p class="muted contract">{m.contractNote(data.contract)}</p>
  <ul class="themes">
    {#each data.themes as theme (theme.id)}
      {@const name = nameOf(theme)}
      {@const can = themeActions(theme)}
      {@const open = previewing === theme.id}
      <li class="theme" class:off={!theme.enabled}>
        <div class="head">
          <h2 class="name">{name}</h2>
          <span class="id">{theme.id}</span>
          <span class="stamp">{m.origins[theme.origin]}</span>
          <span class="stamp" class:done={theme.enabled}>{theme.enabled ? m.on : m.off}</span>
        </div>
        <dl class="facts">
          <div>
            <dt>{m.author}</dt>
            <dd>{theme.author}</dd>
          </div>
          <div>
            <dt>{m.version}</dt>
            <dd>{theme.version}</dd>
          </div>
          <div>
            <dt>{m.contract}</dt>
            <dd>{theme.contract}</dd>
          </div>
        </dl>
        <div class="actions" role="group" aria-label={name}>
          <button
            class="btn btn-small"
            aria-expanded={open}
            onclick={() => (previewing = open ? null : theme.id)}
            >{open ? m.hidePreview : m.preview}</button
          >
          <a class="btn btn-small" href={adminApi.themeExportUrl(theme.id)} download>{m.export}</a>
          {#if can.switchOff || !theme.enabled}
            <button
              class="btn btn-small"
              disabled={switching === theme.id}
              onclick={() => void setEnabled(theme, !theme.enabled)}
              >{theme.enabled ? m.disable : m.enable}</button
            >
          {/if}
          {#if can.replace}
            <button class="btn btn-small" disabled={uploading} onclick={() => chooseFile(theme)}
              >{m.replace}</button
            >
          {/if}
          {#if can.remove}
            <button class="btn btn-small" onclick={() => (removing = theme)}
              >{t.common.delete}</button
            >
          {/if}
        </div>
        {#if open}
          <div class="previews">
            {#if skinSheets[theme.id]}
              {#each schemes(theme.id) as scheme (scheme)}
                <figure>
                  <SkinMock skin={theme.id} {scheme} />
                  <figcaption>{t.themes.schemes[scheme]}</figcaption>
                </figure>
              {/each}
            {:else if !theme.enabled}
              <p class="muted">{m.previewOff}</p>
            {:else if previewFailed}
              <p class="error-text" role="alert">{m.previewFailed}</p>
            {:else}
              <p class="muted loading">{t.themes.preparing}</p>
            {/if}
          </div>
        {/if}
      </li>
    {/each}
  </ul>
{/if}

<ConfirmDialog
  open={removing !== null}
  title={m.deleteTitle(removing ? nameOf(removing) : '')}
  confirm={t.common.delete}
  onclose={() => (removing = null)}
  onconfirm={confirmRemove}
>
  <p>{m.deleteBody(fallback ? nameOf(fallback) : DEFAULT_SKIN)}</p>
</ConfirmDialog>

<style>
  .contract {
    margin: 0 0 8px;
    font-size: 0.8125rem;
  }

  .findings {
    display: grid;
    gap: 8px;
    margin: 12px 0;
    padding: 12px 14px;
    border: 1px solid var(--accent);
    border-radius: var(--radius-s);
  }

  .findings-head {
    display: flex;
    gap: 12px;
    align-items: start;
    justify-content: space-between;
  }

  .findings-head .section-title {
    margin: 0;
    overflow-wrap: anywhere;
  }

  .findings ul,
  .themes {
    display: grid;
    margin: 0;
    padding: 0;
    list-style: none;
  }

  .findings ul {
    gap: 8px;
  }

  .findings li {
    display: flex;
    flex-wrap: wrap;
    gap: 4px 8px;
    align-items: baseline;
  }

  .place,
  .id {
    font: 0.8125rem var(--font-mono);
    overflow-wrap: anywhere;
  }

  .message {
    flex-basis: 100%;
    overflow-wrap: anywhere;
  }

  .theme {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    gap: 8px 20px;
    align-items: center;
    padding: 14px 0;
    border-bottom: 1px solid var(--rule);
  }

  .head {
    display: flex;
    flex-wrap: wrap;
    gap: 4px 10px;
    align-items: baseline;
  }

  .name {
    font-size: 1rem;
  }

  .id {
    color: var(--muted);
  }

  .off .name {
    color: var(--muted);
  }

  .facts {
    display: flex;
    flex-wrap: wrap;
    grid-column: 1;
    gap: 2px 18px;
    margin: 0;
    font-size: 0.8125rem;
  }

  .facts div {
    display: flex;
    gap: 6px;
    min-width: 0;
  }

  .facts dt {
    color: var(--muted);
  }

  .facts dd {
    margin: 0;
    overflow-wrap: anywhere;
  }

  .actions {
    display: flex;
    flex-wrap: wrap;
    grid-row: 1 / span 2;
    grid-column: 2;
    gap: 6px;
    justify-content: end;
  }

  .previews {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(min(100%, 27.5rem), 27.5rem));
    grid-column: 1 / -1;
    gap: 16px;
    margin-top: 6px;
  }

  figure {
    display: grid;
    gap: 6px;
    min-width: 0;
    margin: 0;
  }

  figure :global(.skin-preview) {
    display: block;
    width: 100%;
    height: 16.375rem;
    border: 1px solid var(--rule-strong);
    border-radius: var(--radius-s);
  }

  figcaption {
    font-size: 0.78125rem;
    color: var(--muted);
  }

  @media (max-width: 720px) {
    .theme {
      grid-template-columns: minmax(0, 1fr);
    }

    .actions {
      grid-row: auto;
      grid-column: 1;
      justify-content: start;
    }
  }
</style>
