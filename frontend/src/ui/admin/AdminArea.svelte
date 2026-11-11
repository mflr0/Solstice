<script lang="ts">
  import { untrack } from 'svelte';
  import { ADMIN_NAV, adminPath, type AdminSection } from '../../core/areas.js';
  import { t } from '../../i18n/t.js';
  import { router } from '../../lib/router.svelte.js';
  import { session } from '../../lib/session.svelte.js';
  import TypeLayoutPage from '../layouts/TypeLayoutPage.svelte';
  import Modal from '../Modal.svelte';
  import NotFoundPage from '../pages/NotFoundPage.svelte';
  import SettingsLayout from '../SettingsLayout.svelte';
  import AdminOverview from './AdminOverview.svelte';
  import AdminProjectsPage from './AdminProjectsPage.svelte';
  import AdminWorkflowsPage from './AdminWorkflowsPage.svelte';
  import AuditPage from './AuditPage.svelte';
  import BannerPage from './BannerPage.svelte';
  import CatalogPage from './CatalogPage.svelte';
  import FieldsPage from './FieldsPage.svelte';
  import GroupsPage from './GroupsPage.svelte';
  import ImportPage from './ImportPage.svelte';
  import MailPage from './MailPage.svelte';
  import SecurityLevelsPage from './SecurityLevelsPage.svelte';
  import SettingsPage from './SettingsPage.svelte';
  import StatusesPage from './StatusesPage.svelte';
  import SystemPage from './SystemPage.svelte';
  import ThemesPage from './ThemesPage.svelte';
  import TokensPage from './TokensPage.svelte';
  import UpdatesPage from './UpdatesPage.svelte';
  import UsersPage from './UsersPage.svelte';
  import WebhooksPage from './WebhooksPage.svelte';

  /**
   * The administration (/admin): global administrators only. One frame, a
   * side navigation grouped by theme, then the section's page.
   */
  interface Props {
    section: AdminSection;
    /** A page below the section (a webhook's or a workflow's id, "new"). */
    item: string | null;
  }
  const props: Props = $props();

  /** A page has unsaved changes: leaving it asks first. */
  let dirty = $state(false);
  let leaving = $state<(() => void) | null>(null);

  // Another page, reached while this one has unsaved changes (a link, the
  // back button): the address goes back to where the page was and the switch
  // waits for the answer.
  let shown = $state(untrack(() => ({ section: props.section, item: props.item })));
  let shownAt = untrack(() => router.here);
  $effect.pre(() => {
    const next = { section: props.section, item: props.item };
    const here = router.here;
    if ((next.section === shown.section && next.item === shown.item) || !untrack(() => dirty)) {
      if (next.section !== shown.section || next.item !== shown.item) shown = next;
      shownAt = here;
      return;
    }
    leaving = () => router.navigate(here, { replace: true });
    router.navigate(shownAt, { replace: true });
  });

  const section = $derived(shown.section);
  const item = $derived(shown.item);
  const label = $derived(t.adminArea.sections[section]);
  const groups = $derived(
    ADMIN_NAV.map((g) => ({
      label: t.adminArea.groups[g.group],
      links: g.items.map((i) => ({
        href: adminPath(i),
        label: t.adminArea.sections[i],
        current: i === section,
      })),
    })),
  );

  $effect(() => {
    if (section === 'issueTypes' && item !== null) return;
    document.title =
      section === 'overview'
        ? `${t.adminArea.title} — ${t.appName}`
        : `${label} — ${t.adminArea.title} — ${t.appName}`;
  });
</script>

{#if !session.me?.admin}
  <NotFoundPage message={t.adminArea.forbidden} />
{:else}
  <SettingsLayout area={t.adminArea.title} title={`${t.adminArea.title} — ${label}`} {groups}>
    {#key `${section}/${item ?? ''}`}
      {#if section === 'overview'}
        <AdminOverview />
      {:else if section === 'settings'}
        <SettingsPage />
      {:else if section === 'banner'}
        <BannerPage />
      {:else if section === 'themes'}
        <ThemesPage />
      {:else if section === 'users'}
        <UsersPage />
      {:else if section === 'groups'}
        <GroupsPage />
      {:else if section === 'projects'}
        <AdminProjectsPage />
      {:else if section === 'workflows'}
        <AdminWorkflowsPage {item} ondirty={(d) => (dirty = d)} />
      {:else if section === 'securityLevels'}
        <SecurityLevelsPage />
      {:else if section === 'issueTypes' && item !== null}
        <TypeLayoutPage typeId={item} ondirty={(d) => (dirty = d)} />
      {:else if section === 'issueTypes' || section === 'priorities' || section === 'resolutions' || section === 'linkTypes'}
        <CatalogPage kind={section} />
      {:else if section === 'statuses'}
        <StatusesPage />
      {:else if section === 'fields'}
        <FieldsPage />
      {:else if section === 'mail'}
        <MailPage />
      {:else if section === 'webhooks'}
        <WebhooksPage {item} />
      {:else if section === 'tokens'}
        <TokensPage />
      {:else if section === 'audit'}
        <AuditPage />
      {:else if section === 'import'}
        <ImportPage />
      {:else if section === 'updates'}
        <UpdatesPage />
      {:else}
        <SystemPage />
      {/if}
    {/key}
  </SettingsLayout>

  <Modal open={leaving !== null} title={t.workflows.leaveTitle} onclose={() => (leaving = null)}>
    <p>{t.layouts.leaveBody}</p>
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
