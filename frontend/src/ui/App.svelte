<script lang="ts">
  import { onMount } from 'svelte';
  import { legacyBoardPath } from '../core/board.js';
  import { isPublicRoute, legacySearchPath, paths, safeNext } from '../core/route.js';
  import { createProject, onBoard } from '../core/scope.js';
  import { shortcutFor } from '../core/shortcuts.js';
  import { t } from '../i18n/t.js';
  import { pinnedHeight } from '../lib/actions.js';
  import { live } from '../lib/live.svelte.js';
  import { focusTransient, interceptLinks, router } from '../lib/router.svelte.js';
  import { session } from '../lib/session.svelte.js';
  import {
    appearance,
    dialogs,
    ignoreShortcut,
    initAppearance,
    mac,
    notify,
    openCreate,
    shownBoard,
    toggleScheme,
  } from '../lib/ui.svelte.js';
  import IssueModal from './issue/IssueModal.svelte';
  import IssuePage from './issue/IssuePage.svelte';
  import NewIssueDialog from './NewIssueDialog.svelte';
  import Notes from './Notes.svelte';
  import ForgotPage from './pages/ForgotPage.svelte';
  import HomePage from './pages/HomePage.svelte';
  import LoginPage from './pages/LoginPage.svelte';
  import NotFoundPage from './pages/NotFoundPage.svelte';
  import ProjectPage from './pages/ProjectPage.svelte';
  import ProjectsPage from './pages/ProjectsPage.svelte';
  import ResetPage from './pages/ResetPage.svelte';
  import ViewPage from './pages/ViewPage.svelte';
  import ShellBanner from './ShellBanner.svelte';
  import ShortcutsHelp from './ShortcutsHelp.svelte';
  import TopBar from './TopBar.svelte';

  const route = $derived(router.route);

  /** The project offered first when creating an issue: the page's, else the scoped one. */
  const project = $derived(
    createProject(
      route,
      session.scope,
      onBoard(route) && shownBoard.id === route.id ? shownBoard.projects : null,
    ),
  );

  onMount(() => {
    const stopAppearance = initAppearance();
    session.init().catch(() => notify(t.errors.network, 'error'));
    document.addEventListener('click', interceptLinks);
    window.addEventListener('keydown', shortcuts);
    return () => {
      stopAppearance();
      document.removeEventListener('click', interceptLinks);
      window.removeEventListener('keydown', shortcuts);
    };
  });

  $effect(() => {
    if (session.me) live.start();
    else live.stop();
  });

  // Already signed in: the sign-in page has nothing to offer.
  $effect(() => {
    if (session.me && route.name === 'login') {
      router.navigate(safeNext(router.param('next')), { replace: true });
    }
  });

  // Old board addresses (/secure/RapidBoard.jspa?rapidView=…) lead to the board.
  $effect(() => {
    if (session.me && route.name === 'legacyBoard') {
      const to = legacyBoardPath(router.path, router.search) ?? paths.boards();
      router.navigate(to, { replace: true });
    }
  });

  // Old search addresses (/issues?filter=12&…) lead to the view.
  $effect(() => {
    if (session.me && route.name === 'legacySearch') {
      router.navigate(legacySearchPath(router.search), { replace: true });
    }
  });

  // Not signed in: every page leads to the sign-in form, remembering where to go back.
  $effect(() => {
    if (session.ready && !session.me && !isPublicRoute(route)) {
      router.navigate(paths.login(router.here), { replace: true });
    }
  });

  let main: HTMLElement | undefined = $state();

  function skipToContent(e: MouseEvent) {
    e.preventDefault();
    if (main) focusTransient(main);
  }

  function shortcuts(e: KeyboardEvent) {
    if (!session.me || ignoreShortcut(e)) return;
    const action = shortcutFor(e, 'global', appearance.shortcuts, appearance.bindings, mac);
    if (action === 'search') {
      e.preventDefault();
      document.getElementById('quick-search')?.focus();
    } else if (action === 'create') {
      e.preventDefault();
      openCreate(project ? { project } : {});
    } else if (action === 'help') {
      e.preventDefault();
      dialogs.shortcuts = true;
    } else if (action === 'toggleScheme') {
      e.preventDefault();
      toggleScheme();
    }
  }
</script>

{#snippet pending()}
  <div class="page"><p class="muted loading">{t.common.loading}</p></div>
{/snippet}

{#snippet failed()}
  <div class="page"><p class="error-text" role="alert">{t.errors.pageLoad}</p></div>
{/snippet}

{#if !session.ready}
  <p class="app-booting booting muted loading">{t.common.loading}</p>
{:else if route.name === 'forgot'}
  <ForgotPage />
{:else if route.name === 'reset'}
  {#key route.token}<ResetPage token={route.token} />{/key}
{:else if !session.me || route.name === 'login'}
  <LoginPage />
{:else if route.name === 'oauthAuthorize'}
  {#await import('./pages/ConsentPage.svelte')}
    {@render pending()}
  {:then { default: ConsentPage }}
    <ConsentPage />
  {:catch}
    {@render failed()}
  {/await}
{:else}
  <div class="app-shell">
    <a class="skip-link" href="#main-content" onclick={skipToContent}>{t.nav.skipToContent}</a>
    <TopBar {project} />
    <ShellBanner />
    <main id="main-content" class="app-main" bind:this={main}>
      {#if route.name === 'home'}
        <HomePage />
      {:else if route.name === 'projects'}
        <ProjectsPage />
      {:else if route.name === 'project'}
        <ProjectPage projectKey={route.key} />
      {:else if route.name === 'projectSettings'}
        {#await import('./pages/ProjectSettingsPage.svelte')}
          {@render pending()}
        {:then { default: ProjectSettingsPage }}
          <ProjectSettingsPage projectKey={route.key} />
        {:catch}
          {@render failed()}
        {/await}
      {:else if route.name === 'view'}
        {#key route.id}<ViewPage viewId={route.id} />{/key}
      {:else if route.name === 'issue'}
        <IssuePage issueKey={route.key} />
      {:else if route.name === 'person'}
        {#key route.username}{#await import('./pages/PersonPage.svelte')}
            {@render pending()}
          {:then { default: PersonPage }}
            <PersonPage username={route.username} />
          {:catch}
            {@render failed()}
          {/await}{/key}
      {:else if route.name === 'admin'}
        {#await import('./admin/AdminArea.svelte')}
          {@render pending()}
        {:then { default: AdminArea }}
          <AdminArea section={route.section} item={route.item} />
        {:catch}
          {@render failed()}
        {/await}
      {:else if route.name === 'account'}
        {#await import('./account/AccountArea.svelte')}
          {@render pending()}
        {:then { default: AccountArea }}
          <AccountArea section={route.section} />
        {:catch}
          {@render failed()}
        {/await}
      {:else if route.name === 'themes'}
        {#await import('./pages/ThemesPage.svelte')}
          {@render pending()}
        {:then { default: ThemesPage }}
          <ThemesPage />
        {:catch}
          {@render failed()}
        {/await}
      {:else if route.name === 'boards'}
        {#await import('./pages/BoardsPage.svelte')}
          {@render pending()}
        {:then { default: BoardsPage }}
          <BoardsPage />
        {:catch}
          {@render failed()}
        {/await}
      {:else if route.name === 'board'}
        {#key route.id}{#await import('./pages/BoardPage.svelte')}
            {@render pending()}
          {:then { default: BoardPage }}
            <BoardPage boardId={route.id} />
          {:catch}
            {@render failed()}
          {/await}{/key}
      {:else if route.name === 'backlog'}
        {#key route.id}{#await import('./pages/BacklogPage.svelte')}
            {@render pending()}
          {:then { default: BacklogPage }}
            <BacklogPage boardId={route.id} />
          {:catch}
            {@render failed()}
          {/await}{/key}
      {:else if route.name === 'boardReports'}
        {#key route.id}{#await import('./pages/BoardReportsPage.svelte')}
            {@render pending()}
          {:then { default: BoardReportsPage }}
            <BoardReportsPage boardId={route.id} />
          {:catch}
            {@render failed()}
          {/await}{/key}
      {:else if route.name === 'boardSettings'}
        {#key route.id}{#await import('./pages/BoardSettingsPage.svelte')}
            {@render pending()}
          {:then { default: BoardSettingsPage }}
            <BoardSettingsPage boardId={route.id} />
          {:catch}
            {@render failed()}
          {/await}{/key}
      {:else if route.name === 'legacyBoard' || route.name === 'legacySearch'}
        <div class="page"><p class="muted loading">{t.common.loading}</p></div>
      {:else}
        <NotFoundPage />
      {/if}
    </main>
    <footer class="app-statusbar" use:pinnedHeight={'--statusbar-height'}>
      <span class="statusbar-cell">{t.statusbar.ready}</span>
      <span class="statusbar-cell" class:is-live={live.connected}
        >{live.connected ? t.nav.live : t.nav.offline}</span
      >
      <span class="statusbar-cell">{session.me.name}</span>
    </footer>
  </div>
  <NewIssueDialog />
  <ShortcutsHelp />
  <IssueModal />
{/if}
<Notes />

<style>
  .booting {
    padding: 40px;
    text-align: center;
  }
</style>
