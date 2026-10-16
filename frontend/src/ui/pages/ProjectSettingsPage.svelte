<script lang="ts">
  import {
    nextTab,
    parseTab,
    PROJECT_TABS,
    type ProjectSettings,
    type ProjectTab,
  } from '../../core/project-admin.js';
  import { paths } from '../../core/route.js';
  import { t } from '../../i18n/t.js';
  import { projectAdminApi } from '../../lib/admin-api.js';
  import { ApiError, errorMessage } from '../../lib/api.js';
  import { router } from '../../lib/router.svelte.js';
  import Modal from '../Modal.svelte';
  import ComponentsTab from '../project/ComponentsTab.svelte';
  import GeneralTab from '../project/GeneralTab.svelte';
  import NotificationsTab from '../project/NotificationsTab.svelte';
  import PeopleTab from '../project/PeopleTab.svelte';
  import RightsTab from '../project/RightsTab.svelte';
  import VersionsTab from '../project/VersionsTab.svelte';
  import WorkflowsPanel from '../workflow/WorkflowsPanel.svelte';
  import NotFoundPage from './NotFoundPage.svelte';

  /**
   * A project's settings, one tab at a time (the tab is in the address:
   * /projects/KEY/settings/versions). Project administrators see them all;
   * anyone else who can see the project only gets the workflows, read-only.
   * Nothing here is shared with another project: the "no scheme" model.
   */
  interface Props {
    projectKey: string;
    tab?: ProjectTab | undefined;
  }
  const { projectKey, tab: routeTab }: Props = $props();

  let settings = $state<ProjectSettings | null>(null);
  let error = $state('');
  let forbidden = $state(false);
  /** The open tab has changes not saved yet. */
  let dirty = $state(false);
  let leaving = $state<(() => void) | null>(null);

  const configure = $derived(!forbidden);
  const tabs = $derived<ProjectTab[]>(configure ? [...PROJECT_TABS] : ['workflows']);
  const tab = $derived(parseTab(routeTab ?? null, configure));
  const loaded = $derived(settings !== null || forbidden);

  const labels = $derived<Record<ProjectTab, string>>({
    general: t.projectConfig.tabGeneral,
    components: t.projects.components,
    versions: t.projects.versions,
    people: t.projectConfig.tabPeople,
    rights: t.workflows.rightsTab,
    workflows: t.workflows.tab,
    notifications: t.projectConfig.tabNotifications,
  });

  /** Runs `go`, after asking when the open tab has unsaved changes. */
  function guard(go: () => void) {
    if (dirty) leaving = go;
    else go();
  }

  function showTab(next: ProjectTab) {
    if (next === tab) return;
    guard(() => router.navigate(paths.projectSettings(projectKey, next)));
  }

  function tabKey(e: KeyboardEvent) {
    const step = e.key === 'ArrowRight' ? 1 : e.key === 'ArrowLeft' ? -1 : 0;
    const edge = e.key === 'Home' ? tabs[0] : e.key === 'End' ? tabs[tabs.length - 1] : undefined;
    if (!step && !edge) return;
    e.preventDefault();
    const next = edge ?? nextTab(tabs, tab, step === 1 ? 1 : -1);
    showTab(next);
    queueMicrotask(() => document.getElementById(`tab-${next}`)?.focus());
  }

  // Addresses written before the tabs had their own (?tab=workflows) still work.
  $effect(() => {
    const old = router.param('tab');
    const found = PROJECT_TABS.find((x) => x === old);
    if (!found) return;
    // eslint-disable-next-line svelte/prefer-svelte-reactivity -- a throwaway value, not state
    const rest = new URLSearchParams(router.search);
    rest.delete('tab');
    const query = rest.toString();
    router.navigate(paths.projectSettings(projectKey, found) + (query ? `?${query}` : ''), {
      replace: true,
    });
  });

  $effect(() => {
    const key = projectKey;
    document.title = `${t.settings.title} ${key} — ${t.appName}`;
    settings = null;
    forbidden = false;
    error = '';
    projectAdminApi
      .settings(key)
      .then((s) => (settings = s))
      .catch((e: unknown) => {
        if (e instanceof ApiError && e.status === 403) forbidden = true;
        else error = errorMessage(e);
      });
  });
</script>

{#if error}
  <NotFoundPage message={error} />
{:else if !loaded}
  <div class="page"><p class="muted loading">{t.common.loading}</p></div>
{:else}
  <div class="page settings-area" data-title={`${t.settings.title} ${projectKey}`}>
    <header class="page-head">
      <div>
        <p class="eyebrow">
          <a href={paths.project(projectKey)}>{projectKey}</a> · {t.projects.settings}
        </p>
        <h1 class="page-title">{t.settings.title}</h1>
      </div>
      <a class="btn" href={paths.project(projectKey)}>{t.settings.backToProject}</a>
    </header>
    <p class="intro">{t.settings.intro}</p>

    {#if configure}
      <div class="tabs" role="tablist" aria-label={t.workflows.tabsLabel}>
        {#each tabs as k (k)}
          <button
            type="button"
            class="tab"
            role="tab"
            id="tab-{k}"
            aria-controls="panel-{k}"
            aria-selected={tab === k}
            tabindex={tab === k ? 0 : -1}
            onclick={() => showTab(k)}
            onkeydown={tabKey}>{labels[k]}</button
          >
        {/each}
      </div>
    {/if}

    <div
      class="panel"
      role="tabpanel"
      id="panel-{tab}"
      aria-labelledby={configure ? `tab-${tab}` : undefined}
    >
      {#if tab === 'general' && settings}
        <GeneralTab {settings} onchange={(s) => (settings = s)} ondirty={(d) => (dirty = d)} />
      {:else if tab === 'components'}
        <ComponentsTab {projectKey} />
      {:else if tab === 'versions'}
        <VersionsTab {projectKey} />
      {:else if tab === 'workflows'}
        <WorkflowsPanel {projectKey} {guard} ondirty={(d) => (dirty = d)} />
      {:else if tab === 'people'}
        <PeopleTab {projectKey} />
      {:else if tab === 'notifications'}
        <NotificationsTab {projectKey} ondirty={(d) => (dirty = d)} />
      {:else if tab === 'rights'}
        <RightsTab {projectKey} ondirty={(d) => (dirty = d)} />
      {/if}
    </div>
  </div>

  <Modal open={leaving !== null} title={t.workflows.leaveTitle} onclose={() => (leaving = null)}>
    <p class="leave">{t.projectConfig.leaveBody}</p>
    {#snippet footer()}
      <button type="button" class="btn btn-quiet" onclick={() => (leaving = null)}
        >{t.workflows.stay}</button
      >
      <button
        type="button"
        class="btn btn-danger"
        onclick={() => {
          const go = leaving;
          leaving = null;
          dirty = false;
          go?.();
        }}>{t.workflows.leave}</button
      >
    {/snippet}
  </Modal>
{/if}

<style>
  .intro {
    max-width: 70ch;
    margin: 0 0 6px;
    font: var(--title-italic) 15px/1.5 var(--font-title);
    color: var(--ink-soft);
  }

  /* Seven tabs do not fit a phone: the strip scrolls inside its own box. */
  .tabs {
    margin: 18px 0 4px;
    overflow-x: auto;
    max-width: 100%;
  }

  .tab {
    flex: none;
    white-space: nowrap;
  }

  .leave {
    margin: 0;
  }
</style>
