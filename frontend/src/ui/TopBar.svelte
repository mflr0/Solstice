<script lang="ts">
  import { initials } from '../core/marks.js';
  import { paths } from '../core/route.js';
  import { t } from '../i18n/t.js';
  import { locale, LOCALE_NAMES, LOCALES } from '../i18n/index.js';
  import { outside } from '../lib/actions.js';
  import { chooseLocale } from '../lib/locale.js';
  import { SOURCE_URL } from '../lib/links.js';
  import { live } from '../lib/live.svelte.js';
  import { publicSettings } from '../lib/public-settings.svelte.js';
  import { router } from '../lib/router.svelte.js';
  import { session } from '../lib/session.svelte.js';
  import { MODES, SKINS, SWATCHES } from '../core/appearance.js';
  import { appearance, applyMode, applySkin, openCreate } from '../lib/ui.svelte.js';
  import Logo from './Logo.svelte';
  import QuickSearch from './search/QuickSearch.svelte';

  interface Props {
    /** The project the current page is about, to preselect it on creation. */
    project: string | undefined;
  }
  const { project }: Props = $props();

  let menuOpen = $state(false);
  /** The instance's own name, when the administrators gave it one. */
  const instanceName = $derived.by(() => {
    const n = publicSettings.value.instanceName.trim();
    return n && n !== t.appName ? n : '';
  });
  let nav: HTMLElement | undefined = $state();
  const section = $derived.by(() => {
    switch (router.route.name) {
      case 'home':
        return 'home';
      case 'projects':
      case 'project':
      case 'projectSettings':
        return 'projects';
      case 'boards':
      case 'board':
      case 'backlog':
      case 'boardSettings':
        return 'boards';
      case 'search':
        return 'search';
      default:
        return '';
    }
  });

  // On a narrow bar the links scroll sideways: bring the current one into view.
  $effect(() => {
    const link = section && nav?.querySelector<HTMLElement>('[aria-current="page"]');
    if (!nav || !link) return;
    const box = nav.getBoundingClientRect();
    const r = link.getBoundingClientRect();
    if (r.right > box.right) nav.scrollLeft += r.right - box.right;
    else if (r.left < box.left) nav.scrollLeft -= box.left - r.left;
  });
</script>

<header class="topbar">
  <div class="topbar-inner">
    <a class="brand" href={paths.home()}>
      <Logo />
      <span class="wordmark">{t.appName}</span>
      {#if instanceName}<span class="instance-name">{instanceName}</span>{/if}
    </a>

    <nav class="topbar-nav" bind:this={nav} aria-label={t.common.mainNav}>
      <a class="nav-link" href={paths.home()} aria-current={section === 'home' ? 'page' : undefined}
        >{t.nav.work}</a
      >
      <a
        class="nav-link"
        href={paths.projects()}
        aria-current={section === 'projects' ? 'page' : undefined}>{t.nav.projects}</a
      >
      <a
        class="nav-link"
        href={paths.boards()}
        aria-current={section === 'boards' ? 'page' : undefined}>{t.nav.boards}</a
      >
      <a
        class="nav-link"
        href={paths.search()}
        aria-current={section === 'search' ? 'page' : undefined}>{t.nav.search}</a
      >
    </nav>

    <QuickSearch />

    <span
      class="live-dot"
      class:on={live.connected}
      title={live.connected ? t.nav.live : t.nav.offline}
      role="img"
      aria-label={live.connected ? t.nav.live : t.nav.offline}
    ></span>

    <a
      class="topbar-themes"
      href={paths.themes()}
      title={t.nav.themes}
      aria-label={t.nav.themes}
      aria-current={router.route.name === 'themes' ? 'page' : undefined}
    >
      <svg viewBox="0 0 20 20" width="18" height="18" aria-hidden="true">
        <circle cx="10" cy="10" r="7.25" />
        <path d="M10 2.75a7.25 7.25 0 0 1 0 14.5Z" />
      </svg>
    </a>

    <button
      class="btn btn-primary topbar-create"
      onclick={() => openCreate(project ? { project } : {})}
    >
      <span class="plus" aria-hidden="true">+</span>
      <span class="label">{t.nav.create}</span> <kbd>c</kbd>
    </button>

    {#if session.me}
      <div class="account" use:outside={() => (menuOpen = false)}>
        <button
          class="account-button"
          aria-haspopup="menu"
          aria-expanded={menuOpen}
          aria-label={t.nav.account}
          onclick={() => (menuOpen = !menuOpen)}
        >
          <span class="avatar">{initials(session.me.name)}</span>
        </button>
        {#if menuOpen}
          <div class="popover account-menu" role="menu">
            <div class="who">
              <strong>{session.me.name}</strong>
              <span class="key muted">{session.me.username}</span>
            </div>
            <div class="sep"></div>
            <a
              class="item"
              role="menuitem"
              href={paths.person(session.me.username)}
              onclick={() => (menuOpen = false)}>{t.nav.myPage}</a
            >
            <a
              class="item"
              role="menuitem"
              href={paths.account('profile')}
              onclick={() => (menuOpen = false)}>{t.account.menu}</a
            >
            {#if session.me.admin}
              <a
                class="item"
                role="menuitem"
                href={paths.admin('overview')}
                onclick={() => (menuOpen = false)}>{t.adminArea.title}</a
              >
            {/if}
            <div class="sep"></div>
            <div class="group-label">{t.nav.skin}</div>
            {#each SKINS as skin (skin)}
              <button
                class="item"
                role="menuitemradio"
                aria-checked={appearance.skin === skin}
                onclick={() => applySkin(skin)}
              >
                <span
                  class="swatch"
                  aria-hidden="true"
                  style:--sw-paper={SWATCHES[skin].paper}
                  style:--sw-ink={SWATCHES[skin].ink}
                  style:--sw-accent={SWATCHES[skin].accent}
                ></span>
                <span>{t.nav.skins[skin]}</span>
                <span class="hint">{t.nav.skinHints[skin]}</span>
              </button>
            {/each}
            <a class="item" role="menuitem" href={paths.themes()} onclick={() => (menuOpen = false)}
              >{t.nav.allThemes}</a
            >
            <div class="sep"></div>
            <div class="group-label">{t.nav.mode}</div>
            {#each MODES as mode (mode)}
              <button
                class="item"
                role="menuitemradio"
                aria-checked={appearance.mode === mode}
                onclick={() => applyMode(mode)}>{t.nav.modes[mode]}</button
              >
            {/each}
            <div class="sep"></div>
            <div class="group-label">{t.nav.language}</div>
            <!-- Each language in its own name and tongue, whatever the interface's. -->
            {#each LOCALES as l (l)}
              <button
                class="item"
                role="menuitemradio"
                lang={l}
                aria-checked={locale === l}
                onclick={() => void chooseLocale(l)}>{LOCALE_NAMES[l]}</button
              >
            {/each}
            <div class="sep"></div>
            <a
              class="item"
              role="menuitem"
              href={SOURCE_URL}
              target="_blank"
              rel="noopener noreferrer">{t.nav.sourceCode}</a
            >
            <div class="sep"></div>
            <button class="item" role="menuitem" onclick={() => void session.signOut()}
              >{t.nav.logout}</button
            >
          </div>
        {/if}
      </div>
    {/if}
  </div>
</header>

<style>
  .instance-name {
    align-self: center;
    max-width: 18ch;
    overflow: hidden;
    padding-left: 8px;
    border-left: 1px solid currentColor;
    font: 500 12.5px/1.2 var(--font-body);
    color: var(--bar-ink-soft);
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  @media (max-width: 860px) {
    .instance-name {
      display: none;
    }
  }

  .account-menu .who {
    display: grid;
    gap: 2px;
    padding: 8px 12px 6px;
  }

  /* A thumbnail of each skin: its paper, two lines of ink, its accent. */
  .swatch {
    position: relative;
    width: 24px;
    height: 17px;
    flex: none;
    border: 1px solid var(--rule-strong);
    border-radius: 3px;
    background: var(--sw-paper);
  }

  .swatch::before {
    content: '';
    position: absolute;
    left: 4px;
    top: 5px;
    width: 9px;
    height: 1.5px;
    background: var(--sw-ink);
    box-shadow: 0 4px 0 var(--sw-ink);
  }

  .swatch::after {
    content: '';
    position: absolute;
    right: 4px;
    top: 5px;
    width: 5px;
    height: 5px;
    border-radius: 50%;
    background: var(--sw-accent);
  }

  .account-menu .hint {
    font-size: 12px;
    color: var(--muted);
    white-space: nowrap;
  }
</style>
