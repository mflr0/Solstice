<script lang="ts">
  import { onMount } from 'svelte';
  import { legacyBoardPath } from '../core/board.js';
  import { projectOfKey } from '../core/marks.js';
  import { paths, safeNext } from '../core/route.js';
  import { t } from '../i18n/fr.js';
  import { live } from '../lib/live.svelte.js';
  import { interceptLinks, router } from '../lib/router.svelte.js';
  import { session } from '../lib/session.svelte.js';
  import { dialogs, initAppearance, isTyping, notify, openCreate } from '../lib/ui.svelte.js';
  import IssuePage from './issue/IssuePage.svelte';
  import NewIssueDialog from './NewIssueDialog.svelte';
  import Notes from './Notes.svelte';
  import AdminUsersPage from './pages/AdminUsersPage.svelte';
  import BacklogPage from './pages/BacklogPage.svelte';
  import BoardPage from './pages/BoardPage.svelte';
  import BoardSettingsPage from './pages/BoardSettingsPage.svelte';
  import BoardsPage from './pages/BoardsPage.svelte';
  import HomePage from './pages/HomePage.svelte';
  import LoginPage from './pages/LoginPage.svelte';
  import NotFoundPage from './pages/NotFoundPage.svelte';
  import PersonPage from './pages/PersonPage.svelte';
  import ProjectPage from './pages/ProjectPage.svelte';
  import ProjectSettingsPage from './pages/ProjectSettingsPage.svelte';
  import ProjectsPage from './pages/ProjectsPage.svelte';
  import SearchPage from './pages/SearchPage.svelte';
  import ThemesPage from './pages/ThemesPage.svelte';
  import TopBar from './TopBar.svelte';

  const route = $derived(router.route);

  /** The project the page is about, offered first when creating an issue. */
  const project = $derived.by(() => {
    switch (route.name) {
      case 'project':
      case 'projectSettings':
        return route.key;
      case 'issue':
        return projectOfKey(route.key) ?? undefined;
      default:
        return undefined;
    }
  });

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

  // Not signed in: every page leads to the sign-in form, remembering where to go back.
  $effect(() => {
    if (session.ready && !session.me && route.name !== 'login') {
      router.navigate(paths.login(router.here), { replace: true });
    }
  });

  function shortcuts(e: KeyboardEvent) {
    if (!session.me || dialogs.create || e.ctrlKey || e.metaKey || e.altKey || isTyping(e.target)) {
      return;
    }
    if (e.key === '/') {
      e.preventDefault();
      document.getElementById('quick-search')?.focus();
    } else if (e.key === 'c') {
      e.preventDefault();
      openCreate(project ? { project } : {});
    }
  }
</script>

{#if !session.ready}
  <p class="app-booting booting muted loading">{t.common.loading}</p>
{:else if !session.me || route.name === 'login'}
  <LoginPage />
{:else}
  <div class="app-shell">
    <TopBar {project} />
    <main class="app-main">
      {#if route.name === 'home'}
        <HomePage />
      {:else if route.name === 'projects'}
        <ProjectsPage />
      {:else if route.name === 'project'}
        <ProjectPage projectKey={route.key} />
      {:else if route.name === 'projectSettings'}
        <ProjectSettingsPage projectKey={route.key} />
      {:else if route.name === 'search'}
        <SearchPage />
      {:else if route.name === 'issue'}
        <IssuePage issueKey={route.key} />
      {:else if route.name === 'person'}
        {#key route.username}<PersonPage username={route.username} />{/key}
      {:else if route.name === 'adminUsers'}
        <AdminUsersPage />
      {:else if route.name === 'themes'}
        <ThemesPage />
      {:else if route.name === 'boards'}
        <BoardsPage />
      {:else if route.name === 'board'}
        {#key route.id}<BoardPage boardId={route.id} />{/key}
      {:else if route.name === 'backlog'}
        {#key route.id}<BacklogPage boardId={route.id} />{/key}
      {:else if route.name === 'boardSettings'}
        {#key route.id}<BoardSettingsPage boardId={route.id} />{/key}
      {:else if route.name === 'legacyBoard'}
        <div class="page"><p class="muted loading">{t.common.loading}</p></div>
      {:else}
        <NotFoundPage />
      {/if}
    </main>
    <footer class="app-statusbar">
      <span class="statusbar-cell">{t.statusbar.ready}</span>
      <span class="statusbar-cell" class:is-live={live.connected}
        >{live.connected ? t.nav.live : t.nav.offline}</span
      >
      <span class="statusbar-cell">{session.me.name}</span>
    </footer>
  </div>
  <NewIssueDialog />
{/if}
<Notes />

<style>
  .booting {
    padding: 40px;
    text-align: center;
  }
</style>
