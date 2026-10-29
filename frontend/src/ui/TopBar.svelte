<script lang="ts">
  import { SCHEME_SWATCHES, SKINS, type Scheme, type Skin } from '../core/appearance.js';
  import { paths } from '../core/route.js';
  import { t } from '../i18n/t.js';
  import { menu, outside, stackWhenCrowded } from '../lib/actions.js';
  import { LICENSES_PATH, SOURCE_URL } from '../lib/links.js';
  import { live } from '../lib/live.svelte.js';
  import { publicSettings } from '../lib/public-settings.svelte.js';
  import { router } from '../lib/router.svelte.js';
  import { session } from '../lib/session.svelte.js';
  import { updateNotice } from '../lib/updates.svelte.js';
  import { appearance, applyMode, applySkin, openCreate } from '../lib/ui.svelte.js';
  import Logo from './Logo.svelte';
  import Picture from './Picture.svelte';
  import ProjectScope from './ProjectScope.svelte';
  import QuickSearch from './search/QuickSearch.svelte';

  interface Props {
    /** The project to preselect on creation: the page's, else the scoped one. */
    project: string | undefined;
  }
  const { project }: Props = $props();

  let menuOpen = $state(false);
  let themesOpen = $state(false);
  const SCHEMES: readonly Scheme[] = ['light', 'dark'];

  /** Following the system stays on Account → Appearance: a pick here is explicit. */
  function pickLook(skin: Skin, scheme: Scheme) {
    applyMode(scheme);
    applySkin(skin);
  }
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
      case 'boardReports':
      case 'boardSettings':
        return 'boards';
      case 'search':
        return 'search';
      default:
        return '';
    }
  });

  $effect(() => {
    if (session.me?.admin) void updateNotice.load();
  });
  const update = $derived(session.me?.admin ? updateNotice.latest : null);

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

<header class="topbar" use:stackWhenCrowded>
  <div class="topbar-inner">
    <a class="brand" href={paths.home()}>
      <Logo />
      <span class="wordmark">{t.appName}</span>
      {#if instanceName}<span class="instance-name">{instanceName}</span>{/if}
    </a>

    {#if session.me}<ProjectScope />{/if}

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

    <div class="theme-picker" use:outside={() => (themesOpen = false)}>
      <button
        class="topbar-themes"
        title={t.nav.themes}
        aria-label={t.nav.themes}
        aria-haspopup="menu"
        aria-expanded={themesOpen}
        aria-current={router.route.name === 'themes' ? 'page' : undefined}
        onclick={() => (themesOpen = !themesOpen)}
      >
        <svg viewBox="0 0 20 20" width="18" height="18" aria-hidden="true">
          <circle cx="10" cy="10" r="7.25" />
          <path d="M10 2.75a7.25 7.25 0 0 1 0 14.5Z" />
        </svg>
      </button>
      {#if themesOpen}
        <div class="popover theme-menu" role="menu" use:menu={() => (themesOpen = false)}>
          <div class="theme-head" aria-hidden="true">
            <span>{t.nav.themes}</span>
            {#each SCHEMES as scheme (scheme)}<span class="scheme-name">{t.nav.modes[scheme]}</span
              >{/each}
          </div>
          <div role="group" aria-label={t.nav.themes}>
            {#each SKINS as skin (skin)}
              <div class="theme-row">
                <span class="skin-name" aria-hidden="true">{t.nav.skins[skin]}</span>
                {#each SCHEMES as scheme (scheme)}
                  {@const sw = SCHEME_SWATCHES[skin][scheme]}
                  {@const label = t.nav.skinMode(t.nav.skins[skin], t.nav.modes[scheme])}
                  <button
                    class="scheme"
                    role="menuitemradio"
                    aria-checked={appearance.skin === skin && appearance.scheme === scheme}
                    aria-label={label}
                    title={label}
                    style:--sw-bar={sw.bar}
                    style:--sw-paper={sw.paper}
                    style:--sw-ink={sw.ink}
                    style:--sw-accent={sw.accent}
                    onclick={() => pickLook(skin, scheme)}
                  ></button>
                {/each}
              </div>
            {/each}
          </div>
          <div class="sep"></div>
          <a class="item" role="menuitem" href={paths.themes()} onclick={() => (themesOpen = false)}
            >{t.nav.allThemes}</a
          >
        </div>
      {/if}
    </div>

    <button
      class="btn btn-primary topbar-create"
      onclick={() => openCreate(project ? { project } : {})}
    >
      <span class="plus" aria-hidden="true">+</span>
      <span class="label">{t.nav.create}</span>
      {#if appearance.shortcuts}<kbd>c</kbd>{/if}
    </button>

    {#if session.me}
      <div class="account" use:outside={() => (menuOpen = false)}>
        <button
          class="account-button"
          aria-haspopup="menu"
          aria-expanded={menuOpen}
          aria-label={update ? `${t.nav.account} (${t.updates.badge})` : t.nav.account}
          onclick={() => (menuOpen = !menuOpen)}
        >
          <span class="avatar"><Picture user={session.me} /></span>
          {#if update}<span class="update-dot" aria-hidden="true"></span>{/if}
        </button>
        {#if menuOpen}
          <div class="popover account-menu" role="menu" use:menu={() => (menuOpen = false)}>
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
            {#if update}
              <a
                class="item update-item"
                role="menuitem"
                href={paths.admin('updates')}
                onclick={() => (menuOpen = false)}
              >
                <span class="update-dot" aria-hidden="true"></span>
                <span>{t.updates.badge}</span>
                <span class="hint">{update}</span>
              </a>
            {/if}
            <div class="sep"></div>
            <a
              class="item"
              role="menuitem"
              href={SOURCE_URL}
              target="_blank"
              rel="noopener noreferrer">{t.nav.sourceCode}</a
            >
            <a
              class="item"
              role="menuitem"
              href={LICENSES_PATH}
              target="_blank"
              rel="noopener noreferrer">{t.nav.licenses}</a
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

  .account-button {
    position: relative;
  }

  .update-dot {
    width: 8px;
    height: 8px;
    flex: none;
    border-radius: 50%;
    background: var(--accent);
  }

  .account-button .update-dot {
    position: absolute;
    top: 0;
    right: 0;
    box-shadow: 0 0 0 2px var(--bar);
  }

  .theme-head,
  .theme-row {
    display: grid;
    grid-template-columns: 1fr repeat(2, 44px);
    gap: 8px;
    align-items: center;
    padding: 3px 12px;
  }

  .theme-head {
    padding-top: 6px;
    font: 500 11.5px/1 var(--font-body);
    color: var(--muted);
  }

  .scheme-name {
    text-align: center;
  }

  /* Some languages write the mode names in lower case. */
  .scheme-name::first-letter {
    text-transform: uppercase;
  }

  .skin-name {
    overflow: hidden;
    font-size: 13.5px;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  /* A thumbnail of the skin in one mode: its top bar, paper, two lines of ink, its accent. */
  .scheme {
    position: relative;
    width: 44px;
    height: 30px;
    padding: 0;
    border: 1px solid var(--rule-strong);
    border-radius: 4px;
    background:
      linear-gradient(var(--sw-bar), var(--sw-bar)) top / 100% 7px no-repeat,
      var(--sw-paper);
    cursor: pointer;
  }

  .scheme::before {
    content: '';
    position: absolute;
    left: 6px;
    top: 13px;
    width: 16px;
    height: 2px;
    background: var(--sw-ink);
    box-shadow: 0 6px 0 var(--sw-ink);
  }

  .scheme::after {
    content: '';
    position: absolute;
    right: 6px;
    top: 14px;
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--sw-accent);
  }

  .scheme:hover {
    border-color: var(--ink-soft);
  }

  .scheme[aria-checked='true'] {
    border-color: var(--accent);
    box-shadow: 0 0 0 2px var(--accent);
  }

  .scheme:focus-visible {
    outline: 2px solid var(--ink);
    outline-offset: 3px;
  }

  .account-menu .hint {
    font-size: 12px;
    color: var(--muted);
    white-space: nowrap;
  }
</style>
