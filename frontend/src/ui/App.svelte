<script lang="ts">
  import { onMount } from 'svelte';
  import { projectOfKey } from '../core/marks.js';
  import { paths, safeNext } from '../core/route.js';
  import { t } from '../i18n/fr.js';
  import { live } from '../lib/live.svelte.js';
  import { interceptLinks, router } from '../lib/router.svelte.js';
  import { session } from '../lib/session.svelte.js';
  import { applyTheme, dialogs, isTyping, notify, openCreate, theme } from '../lib/ui.svelte.js';
  import IssuePage from './issue/IssuePage.svelte';
  import NewIssueDialog from './NewIssueDialog.svelte';
  import Notes from './Notes.svelte';
  import AdminUsersPage from './pages/AdminUsersPage.svelte';
  import HomePage from './pages/HomePage.svelte';
  import LoginPage from './pages/LoginPage.svelte';
  import NotFoundPage from './pages/NotFoundPage.svelte';
  import PersonPage from './pages/PersonPage.svelte';
  import ProjectPage from './pages/ProjectPage.svelte';
  import ProjectSettingsPage from './pages/ProjectSettingsPage.svelte';
  import ProjectsPage from './pages/ProjectsPage.svelte';
  import SearchPage from './pages/SearchPage.svelte';
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
    applyTheme(theme.value);
    session.init().catch(() => notify(t.errors.network, 'error'));
    document.addEventListener('click', interceptLinks);
    window.addEventListener('keydown', shortcuts);
    return () => {
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
  <p class="booting muted">{t.common.loading}</p>
{:else if !session.me || route.name === 'login'}
  <LoginPage />
{:else}
  <TopBar {project} />
  <main>
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
    {:else}
      <NotFoundPage />
    {/if}
  </main>
  <NewIssueDialog />
{/if}
<Notes />

<style>
  .booting {
    padding: 40px;
    text-align: center;
  }
</style>
