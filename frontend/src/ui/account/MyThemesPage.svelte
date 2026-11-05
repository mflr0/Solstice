<script lang="ts">
  import { onMount } from 'svelte';
  import { DEFAULT_SKIN, localized, type Scheme, type Skin } from '../../core/appearance.js';
  import { focusOrderAfterRemoval } from '../../core/focus.js';
  import { locale } from '../../i18n/index.js';
  import { t } from '../../i18n/t.js';
  import { refocusLater } from '../../lib/actions.js';
  import { errorMessage } from '../../lib/api.js';
  import {
    deleteLocalTheme,
    listLocalThemes,
    saveLocalTheme,
    unpackLocalTheme,
    type LocalTheme,
  } from '../../lib/local-themes.js';
  import {
    forgetSkin,
    loadSkin,
    loadThemes,
    reloadSkin,
    skinSheets,
    themes,
  } from '../../lib/skins.svelte.js';
  import { appearance, applySkin, notify } from '../../lib/ui.svelte.js';
  import ConfirmDialog from '../ConfirmDialog.svelte';
  import SkinMock from '../SkinMock.svelte';
  import ThemeContractFlag from '../ThemeContractFlag.svelte';

  /**
   * The themes kept in this browser: import a zip, once its user says they
   * trust it, then use, preview, export or delete them.
   */
  const m = t.account.myThemes;

  let list = $state.raw<LocalTheme[]>([]);
  let loadError = $state('');
  let fileInput = $state<HTMLInputElement>();
  /** The zip waiting for the user to say they trust it. */
  let choosing = $state<File | null>(null);
  /** The package waiting for the user to say it replaces the one of the same id. */
  // Raw: IndexedDB can't clone a proxy.
  let replacing = $state.raw<{ old: LocalTheme; next: LocalTheme } | null>(null);
  let removing = $state.raw<LocalTheme | null>(null);
  let previewFailed = $state<Record<string, boolean>>({});

  const nameOf = (theme: LocalTheme) => localized(theme.manifest.name, locale);
  const schemes = (skin: Skin): Scheme[] =>
    skinSheets[skin]?.dark === false ? ['light'] : ['light', 'dark'];
  const fallbackName = $derived.by(() => {
    const fallback = themes.list.find((theme) => theme.id === DEFAULT_SKIN);
    return fallback ? localized(fallback.name, locale) : DEFAULT_SKIN;
  });

  async function refresh() {
    try {
      list = await listLocalThemes();
      loadError = '';
    } catch (e) {
      loadError = errorMessage(e);
    }
  }

  onMount(() => {
    void loadThemes();
    void refresh();
  });

  $effect(() => {
    for (const theme of list) {
      if (skinSheets[theme.id]) continue;
      void loadSkin(theme.id).then((loaded) => {
        previewFailed[theme.id] = !loaded;
      });
    }
  });

  function chosen(e: Event) {
    const input = e.currentTarget as HTMLInputElement;
    choosing = input.files?.[0] ?? null;
    // The same file must fire the change event again.
    input.value = '';
  }

  async function save(theme: LocalTheme, done: string) {
    await saveLocalTheme(theme);
    await reloadSkin(theme.id);
    await refresh();
    notify(done);
  }

  async function importChosen() {
    if (!choosing) return;
    const next = await unpackLocalTheme(choosing);
    if (!next) throw new Error(m.unreadable);
    const old = list.find((theme) => theme.id === next.id);
    if (old) replacing = { old, next };
    else await save(next, m.imported(nameOf(next)));
  }

  async function confirmReplace() {
    if (!replacing) return;
    await save(replacing.next, m.replaced(nameOf(replacing.next)));
  }

  const deleteId = (theme: LocalTheme) => `my-theme-delete-${theme.id}`;

  async function confirmRemove() {
    if (!removing) return;
    const theme = removing;
    const focus = focusOrderAfterRemoval(list, theme, deleteId, 'my-theme-import');
    await deleteLocalTheme(theme.id);
    if (appearance.skin === theme.id) applySkin(DEFAULT_SKIN);
    forgetSkin(theme.id);
    await refresh();
    notify(m.deleted(nameOf(theme)));
    refocusLater(focus);
  }

  function exportZip(theme: LocalTheme) {
    const url = URL.createObjectURL(theme.zip);
    const link = document.createElement('a');
    link.href = url;
    link.download = `${theme.manifest.id}.zip`;
    link.click();
    setTimeout(() => URL.revokeObjectURL(url), 1000);
  }
</script>

<header class="page-head">
  <div>
    <p class="eyebrow">{t.account.title}</p>
    <h1 class="page-title">{t.account.sections.themes}</h1>
  </div>
  <button
    type="button"
    id="my-theme-import"
    class="btn btn-primary"
    onclick={() => fileInput?.click()}>{m.import}</button
  >
</header>
<p class="lead-text">{m.hint}</p>

<input
  bind:this={fileInput}
  type="file"
  accept=".zip,application/zip"
  aria-label={m.import}
  hidden
  onchange={chosen}
/>

{#if loadError}
  <p class="error-text" role="alert">{loadError}</p>
{:else if list.length === 0}
  <p class="muted">{m.empty}</p>
{:else}
  <ul class="themes">
    {#each list as theme (theme.id)}
      {@const name = nameOf(theme)}
      <li class="theme">
        <div class="head">
          <div class="about">
            <h2 class="section-title">
              {name}
              {#if theme.manifest.version}<span class="muted small">v{theme.manifest.version}</span
                >{/if}
            </h2>
            <p class="muted small">{localized(theme.manifest.description, locale)}</p>
            <ThemeContractFlag theme={theme.manifest.contract} server={themes.contract} />
          </div>
          <div class="actions">
            {#if appearance.skin === theme.id}
              <span class="stamp done">{t.themes.current}</span>
            {:else}
              <button type="button" class="btn btn-small" onclick={() => applySkin(theme.id)}
                >{m.use}</button
              >
            {/if}
            <button type="button" class="btn btn-small" onclick={() => exportZip(theme)}
              >{m.export}</button
            >
            <button
              type="button"
              id={deleteId(theme)}
              class="btn btn-small btn-quiet"
              onclick={() => (removing = theme)}>{t.common.delete}</button
            >
          </div>
        </div>
        <div class="previews">
          {#if skinSheets[theme.id]}
            {#each schemes(theme.id) as scheme (scheme)}
              <figure>
                <SkinMock skin={theme.id} {scheme} />
                <figcaption>{t.themes.schemes[scheme]}</figcaption>
              </figure>
            {/each}
          {:else if previewFailed[theme.id]}
            <p class="error-text" role="alert">{m.previewFailed}</p>
          {:else}
            <p class="muted loading">{t.themes.preparing}</p>
          {/if}
        </div>
      </li>
    {/each}
  </ul>
{/if}

<ConfirmDialog
  open={choosing !== null}
  title={m.trustTitle}
  confirm={m.import}
  danger={false}
  onclose={() => (choosing = null)}
  onconfirm={importChosen}
>
  <p class="file-name">{choosing?.name}</p>
  <p>{m.trustBody}</p>
</ConfirmDialog>

<ConfirmDialog
  open={replacing !== null}
  title={replacing
    ? m.replaceTitle(
        nameOf(replacing.old),
        replacing.old.manifest.version,
        replacing.next.manifest.version,
      )
    : ''}
  confirm={m.replace}
  danger={false}
  onclose={() => (replacing = null)}
  onconfirm={confirmReplace}
>
  <p>{m.replaceBody}</p>
</ConfirmDialog>

<ConfirmDialog
  open={removing !== null}
  title={m.deleteTitle(removing ? nameOf(removing) : '')}
  confirm={t.common.delete}
  onclose={() => (removing = null)}
  onconfirm={confirmRemove}
>
  <p>{m.deleteBody(fallbackName)}</p>
</ConfirmDialog>

<style>
  .themes {
    display: grid;
    gap: 24px;
    margin: 0;
    padding: 0;
    list-style: none;
  }

  .theme {
    display: grid;
    gap: 12px;
    padding-bottom: 24px;
    border-bottom: 1px solid var(--rule);
  }

  .head {
    display: flex;
    flex-wrap: wrap;
    justify-content: space-between;
    gap: 12px;
  }

  .about {
    display: grid;
    gap: 4px;
    min-width: 0;
    overflow-wrap: anywhere;
  }

  .about p {
    margin: 0;
  }

  .actions {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px;
  }

  .previews {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(min(100%, 27.5rem), 27.5rem));
    gap: 16px;
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

  .file-name {
    font-weight: 600;
    overflow-wrap: anywhere;
  }
</style>
