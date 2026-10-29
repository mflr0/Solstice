<script lang="ts">
  import { onMount } from 'svelte';
  import { MODES, SKINS, type Scheme, type Skin } from '../../core/appearance.js';
  import { t } from '../../i18n/t.js';
  import { loadSkin, skinSheets } from '../../lib/skins.svelte.js';
  import { appearance, applyMode, applySkin } from '../../lib/ui.svelte.js';
  import SkinMock from '../SkinMock.svelte';

  /** The theme gallery: every skin drawn live, in each of its schemes. */

  onMount(() => {
    document.title = `${t.themes.title} — ${t.appName}`;
    for (const skin of SKINS) void loadSkin(skin);
  });

  function schemes(skin: Skin): Scheme[] {
    return skinSheets[skin]?.dark === false ? ['light'] : ['light', 'dark'];
  }

  /** Applies the skin; picking the other scheme's preview switches the mode too. */
  function choose(skin: Skin, scheme: Scheme) {
    applySkin(skin);
    if (schemes(skin).length > 1 && scheme !== appearance.scheme) applyMode(scheme);
  }
</script>

<div class="page" data-title={t.themes.title}>
  <header class="page-head">
    <div>
      <p class="eyebrow">{t.themes.eyebrow}</p>
      <h1 class="page-title">{t.themes.title}</h1>
      <p class="intro muted">{t.themes.intro}</p>
    </div>
  </header>

  {#each SKINS as skin (skin)}
    {@const current = appearance.skin === skin}
    <section class="theme" class:current aria-labelledby="theme-{skin}">
      <div class="about">
        <h2 id="theme-{skin}" class="name">{t.nav.skins[skin]}</h2>
        <p class="description">{t.themes.descriptions[skin]}</p>
        {#if current}
          <p class="in-use"><span class="stamp done">{t.themes.current}</span></p>
        {:else}
          <button class="btn btn-primary" onclick={() => applySkin(skin)}>{t.themes.use}</button>
        {/if}
        {#if schemes(skin).length > 1}
          <div class="modes" role="group" aria-label="{t.nav.mode} — {t.nav.skins[skin]}">
            {#each MODES as mode (mode)}
              <button
                class="btn btn-small"
                class:on={current && appearance.mode === mode}
                aria-pressed={current && appearance.mode === mode}
                onclick={() => {
                  applySkin(skin);
                  applyMode(mode);
                }}>{t.nav.modes[mode]}</button
              >
            {/each}
          </div>
        {:else if skinSheets[skin]}
          <p class="muted small">{t.themes.lightOnly}</p>
        {/if}
      </div>

      <div class="previews">
        {#each schemes(skin) as scheme (scheme)}
          <button
            class="preview"
            class:active={current && appearance.scheme === scheme}
            aria-label={t.themes.apply(t.nav.skins[skin], t.themes.schemes[scheme])}
            onclick={() => choose(skin, scheme)}
          >
            {#if skinSheets[skin]}
              <SkinMock {skin} {scheme} />
            {:else}
              <span class="placeholder muted loading">{t.themes.preparing}</span>
            {/if}
            <span class="caption">{t.themes.schemes[scheme]}</span>
          </button>
        {/each}
      </div>
    </section>
  {/each}
</div>

<style>
  .intro {
    max-width: 62ch;
    margin: 8px 0 0;
  }

  .theme {
    display: grid;
    grid-template-columns: 14.375rem minmax(0, 1fr);
    gap: 28px;
    padding: 22px 0;
    border-bottom: 1px solid var(--rule);
  }

  .about {
    display: grid;
    gap: 10px;
    align-content: start;
    justify-items: start;
  }

  .name {
    font-size: 1.3125rem;
  }

  .description {
    margin: 0;
    color: var(--ink-soft);
  }

  .in-use {
    margin: 0;
  }

  .modes {
    display: flex;
    gap: 4px;
  }

  .small {
    margin: 0;
    font-size: 0.78125rem;
  }

  .previews {
    display: grid;
    grid-template-columns: repeat(2, 27.5rem);
    gap: 16px;
  }

  .preview {
    display: grid;
    gap: 6px;
    padding: 0;
    border: none;
    background: none;
    text-align: left;
    cursor: pointer;
  }

  .preview :global(.skin-preview),
  .placeholder {
    display: block;
    width: 27.5rem;
    height: 16.375rem;
    border: 1px solid var(--rule-strong);
    border-radius: var(--radius-s);
  }

  .placeholder {
    padding: 16px;
  }

  .preview:hover :global(.skin-preview),
  .preview:focus-visible :global(.skin-preview) {
    border-color: var(--ink-soft);
  }

  .preview.active :global(.skin-preview) {
    outline: 2px solid var(--accent);
    outline-offset: 2px;
  }

  .caption {
    font-size: 0.78125rem;
    color: var(--muted);
  }

  @media (max-width: 1180px) {
    .theme {
      grid-template-columns: minmax(0, 1fr);
      gap: 14px;
    }
  }

  @media (max-width: 980px) {
    .previews {
      grid-template-columns: minmax(0, 27.5rem);
    }

    .preview :global(.skin-preview),
    .placeholder {
      width: 100%;
    }
  }
</style>
