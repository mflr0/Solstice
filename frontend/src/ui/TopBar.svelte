<script lang="ts">
  import { initials, projectOfKey } from '../core/marks.js';
  import { paths } from '../core/route.js';
  import { t } from '../i18n/fr.js';
  import { outside } from '../lib/actions.js';
  import { live } from '../lib/live.svelte.js';
  import { router } from '../lib/router.svelte.js';
  import { session } from '../lib/session.svelte.js';
  import { applyTheme, openCreate, theme, type Theme } from '../lib/ui.svelte.js';
  import Logo from './Logo.svelte';

  interface Props {
    /** The project the current page is about, to preselect it on creation. */
    project: string | undefined;
  }
  const { project }: Props = $props();

  let q = $state('');
  let menuOpen = $state(false);
  const section = $derived.by(() => {
    switch (router.route.name) {
      case 'home':
        return 'home';
      case 'projects':
      case 'project':
      case 'projectSettings':
        return 'projects';
      case 'search':
        return 'search';
      default:
        return '';
    }
  });
  const themes: Theme[] = ['system', 'light', 'dark'];

  function quickSearch(e: SubmitEvent) {
    e.preventDefault();
    const v = q.trim();
    if (!v) return;
    if (projectOfKey(v)) router.navigate(paths.issue(v.toUpperCase()));
    else router.navigate(paths.search(v));
    q = '';
    (document.activeElement as HTMLElement | null)?.blur();
  }
</script>

<header class="topbar">
  <div class="inner">
    <a class="brand" href={paths.home()}>
      <Logo />
      <span class="wordmark">{t.appName}</span>
    </a>

    <nav aria-label={t.common.mainNav}>
      <a href={paths.home()} aria-current={section === 'home' ? 'page' : undefined}>{t.nav.work}</a>
      <a href={paths.projects()} aria-current={section === 'projects' ? 'page' : undefined}
        >{t.nav.projects}</a
      >
      <a href={paths.search()} aria-current={section === 'search' ? 'page' : undefined}
        >{t.nav.search}</a
      >
    </nav>

    <form class="quick" role="search" onsubmit={quickSearch}>
      <label class="sr-only" for="quick-search">{t.nav.quickSearch}</label>
      <input
        id="quick-search"
        class="input"
        bind:value={q}
        placeholder={t.nav.quickSearchPlaceholder}
        autocomplete="off"
        spellcheck="false"
      />
      <kbd class="kbd" aria-hidden="true">/</kbd>
    </form>

    <span
      class="live"
      class:on={live.connected}
      title={live.connected ? t.nav.live : t.nav.offline}
      role="img"
      aria-label={live.connected ? t.nav.live : t.nav.offline}
    ></span>

    <button class="btn btn-primary" onclick={() => openCreate(project ? { project } : {})}>
      {t.nav.create} <kbd>c</kbd>
    </button>

    {#if session.me}
      <div class="account" use:outside={() => (menuOpen = false)}>
        <button
          class="me"
          aria-haspopup="menu"
          aria-expanded={menuOpen}
          aria-label={t.nav.account}
          onclick={() => (menuOpen = !menuOpen)}
        >
          <span class="initials">{initials(session.me.name)}</span>
        </button>
        {#if menuOpen}
          <div class="popover menu" role="menu">
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
            {#if session.me.admin}
              <a
                class="item"
                role="menuitem"
                href={paths.adminUsers()}
                onclick={() => (menuOpen = false)}>{t.nav.admin}</a
              >
            {/if}
            <div class="sep"></div>
            <div class="group-label">{t.nav.theme}</div>
            {#each themes as th (th)}
              <button
                class="item"
                role="menuitemradio"
                aria-checked={theme.value === th}
                onclick={() => applyTheme(th)}>{t.nav.themes[th]}</button
              >
            {/each}
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
  .topbar {
    position: sticky;
    top: 0;
    z-index: 30;
    background: var(--paper-raised);
    border-bottom: 1px solid var(--rule);
  }

  .inner {
    max-width: 1280px;
    margin: 0 auto;
    height: 54px;
    padding: 0 32px;
    display: flex;
    align-items: center;
    gap: 22px;
  }

  .brand {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    text-decoration: none;
  }

  .wordmark {
    font: 600 21px/1 var(--serif);
    letter-spacing: -0.01em;
  }

  nav {
    display: flex;
    gap: 18px;
  }

  nav a {
    text-decoration: none;
    font-weight: 500;
    color: var(--ink-soft);
    padding: 17px 0 15px;
    border-bottom: 2px solid transparent;
  }

  nav a:hover {
    color: var(--ink);
  }

  nav a[aria-current='page'] {
    color: var(--ink);
    border-bottom-color: var(--accent);
  }

  .quick {
    position: relative;
    margin-left: auto;
    width: min(300px, 28vw);
  }

  .quick .input {
    height: 30px;
    padding-right: 28px;
    font: 13px var(--mono);
    background: var(--paper);
  }

  .quick kbd {
    position: absolute;
    right: 8px;
    top: 50%;
    transform: translateY(-50%);
    pointer-events: none;
  }

  .live {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    border: 1.5px solid var(--muted);
    flex: none;
  }

  .live.on {
    border-color: var(--stamp-done);
    background: var(--stamp-done);
  }

  .account {
    position: relative;
  }

  .me {
    border: none;
    background: none;
    padding: 0;
    cursor: pointer;
  }

  .initials {
    display: grid;
    place-items: center;
    width: 30px;
    height: 30px;
    border: 1px solid var(--ink-soft);
    border-radius: 2px;
    font: 500 11px/1 var(--mono);
    background: var(--paper);
  }

  .menu {
    right: 0;
    top: calc(100% + 8px);
    min-width: 230px;
  }

  .menu .who {
    display: grid;
    gap: 2px;
    padding: 8px 12px 6px;
  }

  @media (max-width: 860px) {
    .inner {
      padding: 0 16px;
      gap: 12px;
    }

    nav a:nth-child(3),
    .quick,
    .btn kbd {
      display: none;
    }
  }
</style>
