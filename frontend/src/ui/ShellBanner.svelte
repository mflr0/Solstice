<script lang="ts">
  import { t } from '../i18n/t.js';
  import { publicSettings } from '../lib/public-settings.svelte.js';

  /**
   * The administrators' announcement, above every page (maintenance
   * tonight…). One may hide it; it comes back when its text changes.
   */
  const STORAGE_KEY = 'solstice.banner.hidden';

  function hiddenText(): string | null {
    try {
      return sessionStorage.getItem(STORAGE_KEY);
    } catch {
      return null;
    }
  }

  let hidden = $state(hiddenText());
  const banner = $derived(publicSettings.value.banner);

  $effect(() => {
    if (!publicSettings.loaded) void publicSettings.load();
  });

  function hide() {
    if (!banner) return;
    hidden = banner.text;
    try {
      sessionStorage.setItem(STORAGE_KEY, banner.text);
    } catch {
      // Private browsing: hidden until the page reloads.
    }
  }
</script>

{#if banner && banner.text && hidden !== banner.text}
  <div class="app-banner is-{banner.level}" role={banner.level === 'warning' ? 'alert' : 'status'}>
    <p class="app-banner-text">{banner.text}</p>
    <button type="button" class="link-btn app-banner-hide" onclick={hide}
      >{t.adminArea.hideBanner}</button
    >
  </div>
{/if}

<style>
  /* Global: the banner page of the administration previews the same look. */
  :global(.app-banner) {
    display: flex;
    gap: 8px 16px;
    align-items: baseline;
    justify-content: center;
    flex-wrap: wrap;
    padding: 8px 16px;
    border-bottom: 1px solid var(--rule-strong);
    background: var(--paper-sunk);
    color: var(--ink);
    font-size: 13.5px;
    line-height: 1.45;
  }

  :global(.app-banner.is-warning) {
    background: color-mix(in srgb, var(--accent) 12%, var(--paper));
    border-bottom-color: var(--accent);
  }

  :global(.app-banner-text) {
    margin: 0;
    max-width: 90ch;
    overflow-wrap: anywhere;
  }

  :global(.app-banner-hide) {
    font-size: 12.5px;
  }
</style>
