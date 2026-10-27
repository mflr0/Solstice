<script lang="ts">
  import { ADMIN_NAV, adminPath, type AdminSection } from '../../core/areas.js';
  import { t } from '../../i18n/t.js';
  import { session } from '../../lib/session.svelte.js';
  import NotFoundPage from '../pages/NotFoundPage.svelte';
  import SettingsLayout from '../SettingsLayout.svelte';
  import AdminOverview from './AdminOverview.svelte';
  import AdminProjectsPage from './AdminProjectsPage.svelte';
  import AuditPage from './AuditPage.svelte';
  import BannerPage from './BannerPage.svelte';
  import CatalogPage from './CatalogPage.svelte';
  import FieldsPage from './FieldsPage.svelte';
  import GroupsPage from './GroupsPage.svelte';
  import ImportPage from './ImportPage.svelte';
  import MailPage from './MailPage.svelte';
  import RolesPage from './RolesPage.svelte';
  import SecurityLevelsPage from './SecurityLevelsPage.svelte';
  import SettingsPage from './SettingsPage.svelte';
  import StatusesPage from './StatusesPage.svelte';
  import SystemPage from './SystemPage.svelte';
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
    /** A page below the section (a webhook's id, "new"). */
    item: string | null;
  }
  const { section, item }: Props = $props();

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
      {:else if section === 'users'}
        <UsersPage />
      {:else if section === 'groups'}
        <GroupsPage />
      {:else if section === 'roles'}
        <RolesPage />
      {:else if section === 'projects'}
        <AdminProjectsPage />
      {:else if section === 'securityLevels'}
        <SecurityLevelsPage />
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
{/if}
