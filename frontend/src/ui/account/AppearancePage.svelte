<script lang="ts">
  import { MODES, SKINS, SWATCHES } from '../../core/appearance.js';
  import { paths } from '../../core/route.js';
  import { t } from '../../i18n/t.js';
  import { LOCALE_NAMES, LOCALES, locale, type Locale } from '../../i18n/index.js';
  import { LOCALE_STORAGE_KEY, negotiate } from '../../i18n/locales.js';
  import { accountApi } from '../../lib/admin-api.js';
  import { errorMessage } from '../../lib/api.js';
  import { chooseLocale } from '../../lib/locale.js';
  import { session } from '../../lib/session.svelte.js';
  import { appearance, applyMode, applySkin, notify } from '../../lib/ui.svelte.js';

  /**
   * The interface language (kept on the account) and the look (kept on this
   * browser): skin and light or dark mode.
   */

  /** The account follows the browser's language when it chose none. */
  const following = $derived(!session.me?.locale);
  const browserLocale = negotiate(null, typeof navigator !== 'undefined' ? navigator.languages : []);
  let busy = $state(false);

  async function pick(l: Locale) {
    if (busy) return;
    if (l === locale && !following) return;
    busy = true;
    if (l === locale) {
      // Same language, now chosen explicitly: no reload needed.
      try {
        if (session.me) session.me = await accountApi.updateProfile({ locale: l });
      } catch (e) {
        notify(errorMessage(e), 'error');
      } finally {
        busy = false;
      }
      return;
    }
    await chooseLocale(l);
  }

  async function followBrowser() {
    if (busy || following) return;
    busy = true;
    try {
      if (session.me) session.me = await accountApi.updateProfile({ locale: null });
      try {
        localStorage.removeItem(LOCALE_STORAGE_KEY);
      } catch {
        // Private browsing: nothing stored anyway.
      }
      if (browserLocale !== locale) location.reload();
      else busy = false;
    } catch (e) {
      notify(errorMessage(e), 'error');
      busy = false;
    }
  }
</script>

<header class="page-head">
  <div>
    <p class="eyebrow">{t.account.title}</p>
    <h1 class="page-title">{t.account.sections.appearance}</h1>
  </div>
</header>
<p class="lead-text">{t.account.appearanceIntro}</p>

<section class="settings-form" aria-labelledby="lang-title">
  <h2 class="section-title" id="lang-title">{t.account.language}</h2>
  <fieldset class="choices" disabled={busy}>
    <legend class="sr-only">{t.account.language}</legend>
    {#each LOCALES as l (l)}
      <label class="check">
        <input
          type="radio"
          name="locale"
          checked={!following && locale === l}
          onchange={() => void pick(l)}
        />
        <span lang={l}>{LOCALE_NAMES[l]}</span>
      </label>
    {/each}
    <label class="check">
      <input type="radio" name="locale" checked={following} onchange={() => void followBrowser()} />
      <span>
        {t.account.followBrowser}
        <span class="muted small">{t.account.followBrowserHint(LOCALE_NAMES[browserLocale])}</span>
      </span>
    </label>
  </fieldset>
  <p class="form-hint">{t.account.languageHint}</p>
</section>

<section class="settings-form look" aria-labelledby="theme-title">
  <h2 class="section-title" id="theme-title">{t.account.theme}</h2>
  <div class="skins" role="group" aria-labelledby="theme-title">
    {#each SKINS as skin (skin)}
      <button
        type="button"
        class="skin"
        aria-pressed={appearance.skin === skin}
        onclick={() => applySkin(skin)}
      >
        <span
          class="swatch"
          aria-hidden="true"
          style:--sw-paper={SWATCHES[skin].paper}
          style:--sw-ink={SWATCHES[skin].ink}
          style:--sw-accent={SWATCHES[skin].accent}
        ></span>
        <span class="skin-text">
          <span class="skin-name">{t.nav.skins[skin]}</span>
          <span class="muted small">{t.nav.skinHints[skin]}</span>
        </span>
      </button>
    {/each}
  </div>

  <fieldset class="choices inline">
    <legend class="field-label">{t.nav.mode}</legend>
    {#each MODES as mode (mode)}
      <label class="check">
        <input
          type="radio"
          name="mode"
          checked={appearance.mode === mode}
          onchange={() => applyMode(mode)}
        />
        <span>{t.nav.modes[mode]}</span>
      </label>
    {/each}
  </fieldset>
  <p class="form-hint">{t.account.themeHint}</p>

  <div class="card gallery">
    <div class="card-body">
      <p>{t.account.galleryHint}</p>
      <p><a class="btn btn-primary" href={paths.themes()}>{t.account.gallery}</a></p>
    </div>
  </div>
</section>

<style>
  .look {
    margin-top: 28px;
  }

  .skins {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
    gap: 8px;
  }

  .skin {
    display: flex;
    gap: 10px;
    align-items: center;
    padding: 8px 10px;
    border: var(--frame) solid var(--rule-strong);
    border-radius: var(--radius-m);
    background: var(--paper-raised);
    color: var(--ink);
    text-align: left;
    font: inherit;
    cursor: pointer;
  }

  .skin:hover {
    border-color: var(--ink-soft);
  }

  .skin[aria-pressed='true'] {
    border-color: var(--accent);
    box-shadow: 0 0 0 1px var(--accent);
  }

  .skin-text {
    display: grid;
    gap: 1px;
    min-width: 0;
  }

  .skin-name {
    font-weight: 600;
  }

  /* A thumbnail of each skin, as in the account menu. */
  .swatch {
    position: relative;
    width: 30px;
    height: 22px;
    flex: none;
    border: 1px solid var(--rule-strong);
    border-radius: 3px;
    background: var(--sw-paper);
  }

  .swatch::before {
    content: '';
    position: absolute;
    left: 5px;
    top: 7px;
    width: 11px;
    height: 1.5px;
    background: var(--sw-ink);
    box-shadow: 0 5px 0 var(--sw-ink);
  }

  .swatch::after {
    content: '';
    position: absolute;
    right: 5px;
    top: 7px;
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: var(--sw-accent);
  }

  .gallery {
    max-width: 720px;
  }
</style>
