<script lang="ts">
  import { onMount } from 'svelte';
  import { localized, type Scheme, type Skin, type ThemeManifest } from '../core/appearance.js';
  import { paths } from '../core/route.js';
  import { bindingWorks, formatShortcut } from '../core/shortcuts.js';
  import { locale } from '../i18n/index.js';
  import { t } from '../i18n/t.js';
  import { menu, outside, pinnedHeight, stackWhenCrowded } from '../lib/actions.js';
  import { LICENSES_PATH, SOURCE_URL } from '../lib/links.js';
  import { live } from '../lib/live.svelte.js';
  import { localThemeManifests } from '../lib/local-themes.js';
  import { publicSettings } from '../lib/public-settings.svelte.js';
  import { router } from '../lib/router.svelte.js';
  import { session } from '../lib/session.svelte.js';
  import { updateNotice } from '../lib/updates.svelte.js';
  import { isSkinLoaded, loadSkin, loadThemes, themes } from '../lib/skins.svelte.js';
  import {
    appearance,
    applyMode,
    applySkin,
    mac,
    openCreate,
    toggleScheme,
  } from '../lib/ui.svelte.js';
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
  let mine = $state.raw<ThemeManifest[]>([]);
  // The bar outlives the pages where themes get imported and deleted: read again on each opening.
  const readMine = () => localThemeManifests().then((list) => (mine = list));
  onMount(() => {
    void loadThemes();
    void readMine();
  });
  const SCHEMES: readonly Scheme[] = ['light', 'dark'];

  /** Following the system stays on Account → Appearance: a pick here is explicit. */
  function pickLook(skin: Skin, scheme: Scheme, item: HTMLElement) {
    applyMode(scheme);
    pickSkin(skin, item);
  }
  /** Switches skin and keeps the mode, following the system included. */
  function pickSkin(skin: Skin, item: HTMLElement) {
    applySkin(skin);
    // The page is hidden while a skin's stylesheet loads, which drops focus;
    // a skin that hides the theme button on this screen hides the menu too.
    if (!isSkinLoaded(skin))
      void loadSkin(skin).then(() => {
        if (item.getClientRects().length) item.focus();
        else themesOpen = false;
      });
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
      case 'view':
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
      default:
        return '';
    }
  });

  $effect(() => {
    if (session.me?.admin) void updateNotice.load();
  });
  const update = $derived(session.me?.admin ? updateNotice.latest : null);
  const createKey = $derived(
    appearance.bindings
      .find((s) => s.action === 'create')
      ?.keys.find((k) => bindingWorks(k, appearance.shortcuts, mac)),
  );

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

<header class="topbar" use:stackWhenCrowded use:pinnedHeight={'--topbar-height'}>
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
        href={session.scope ? paths.project(session.scope) : paths.projects()}
        aria-current={section === 'projects' ? 'page' : undefined}>{t.nav.projects}</a
      >
      <a
        class="nav-link"
        href={paths.boards()}
        aria-current={section === 'boards' ? 'page' : undefined}>{t.nav.boards}</a
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
        onclick={() => {
          themesOpen = !themesOpen;
          void loadThemes();
          void readMine();
        }}
      >
        <svg viewBox="0 0 20 20" width="18" height="18" aria-hidden="true">
          <path
            d="M10 2.75a7.25 7.25 0 0 0 0 14.5c1 0 1.6-.65 1.6-1.45 0-.4-.15-.75-.4-1.05-.25-.3-.4-.65-.4-1.05 0-.85.7-1.5 1.5-1.5h1.6a3.35 3.35 0 0 0 3.35-3.35C17.25 5.7 14 2.75 10 2.75Z"
          />
          <circle cx="6.25" cy="10" r="1.1" />
          <circle cx="7.6" cy="6.4" r="1.1" />
          <circle cx="11.4" cy="5.75" r="1.1" />
          <circle cx="14.4" cy="8.25" r="1.1" />
        </svg>
      </button>
      {#if themesOpen}
        <div class="popover theme-menu" role="menu" use:menu={() => (themesOpen = false)}>
          <button
            class="item"
            role="menuitemcheckbox"
            aria-checked={appearance.scheme === 'dark'}
            onclick={toggleScheme}>{t.nav.darkMode}</button
          >
          <div class="sep"></div>
          <div class="theme-head" aria-hidden="true">
            <span>{t.nav.themes}</span>
            {#each SCHEMES as scheme (scheme)}<span class="scheme-name">{t.nav.modes[scheme]}</span
              >{/each}
          </div>
          <div role="group" aria-label={t.nav.themes}>
            {#each themes.list as theme (theme.id)}
              {@render themeRow(theme)}
            {/each}
          </div>
          {#if mine.length > 0}
            <div class="theme-head" aria-hidden="true">
              <span>{t.account.sections.themes}</span>
            </div>
            <div role="group" aria-label={t.account.sections.themes}>
              {#each mine as theme (theme.id)}
                {@render themeRow(theme)}
              {/each}
            </div>
          {/if}
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
      {#if createKey}<kbd>{formatShortcut(createKey, mac, t.editor.keys)}</kbd>{/if}
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

{#snippet themeRow(theme: ThemeManifest)}
  {@const skin = theme.id}
  {@const name = localized(theme.name, locale)}
  <div class="theme-row">
    <button
      class="item skin-name"
      title={name}
      tabindex="-1"
      role="menuitem"
      onclick={(e) => pickSkin(skin, e.currentTarget)}>{name}</button
    >
    {#each SCHEMES as scheme (scheme)}
      {@const sw = theme.preview[scheme]}
      {@const label = t.nav.skinMode(name, t.nav.modes[scheme])}
      <button
        class="scheme"
        tabindex="-1"
        role="menuitemradio"
        aria-checked={appearance.skin === skin && appearance.scheme === scheme}
        aria-label={label}
        title={label}
        style:--sw-bar={sw?.bar}
        style:--sw-paper={sw?.paper}
        style:--sw-ink={sw?.ink}
        style:--sw-accent={sw?.accent}
        onclick={(e) => pickLook(skin, scheme, e.currentTarget)}
      ></button>
    {/each}
  </div>
{/snippet}

<style>
  .instance-name {
    align-self: center;
    max-width: 18ch;
    overflow: hidden;
    padding-left: 8px;
    border-left: 1px solid currentColor;
    font: 500 0.78125rem/1.2 var(--font-body);
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
    font: 500 0.71875rem/1 var(--font-body);
    color: var(--muted);
  }

  .scheme-name {
    text-align: center;
  }

  /* Some languages write the mode names in lower case. */
  .scheme-name::first-letter {
    text-transform: uppercase;
  }

  /* The text lines up with the heading; the hover background reaches into the row's padding.
     Above the skins' [data-skin] .popover .item, which load later. */
  .theme-menu .theme-row .skin-name {
    display: block;
    width: calc(100% + 8px);
    margin-left: -8px;
    padding: 6px 8px;
    overflow: hidden;
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
    font-size: 0.75rem;
    color: var(--muted);
    white-space: nowrap;
  }
</style>
