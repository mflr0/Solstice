<script lang="ts">
  import { ACCOUNT_SECTIONS, accountPath, type AccountSection } from '../../core/areas.js';
  import { t } from '../../i18n/fr.js';
  import SettingsLayout from '../SettingsLayout.svelte';
  import AppearancePage from './AppearancePage.svelte';
  import NotificationsPage from './NotificationsPage.svelte';
  import ProfilePage from './ProfilePage.svelte';
  import SubscriptionsPage from './SubscriptionsPage.svelte';
  import TokensPage from './TokensPage.svelte';

  /** One's own settings (/settings): profile, language and look, e-mails, tokens, subscriptions. */
  interface Props {
    section: AccountSection;
  }
  const { section }: Props = $props();

  const label = $derived(t.account.sections[section]);
  const groups = $derived([
    {
      label: null,
      links: ACCOUNT_SECTIONS.map((s) => ({
        href: accountPath(s),
        label: t.account.sections[s],
        current: s === section,
      })),
    },
  ]);

  $effect(() => {
    document.title = `${label} — ${t.account.title} — ${t.appName}`;
  });
</script>

<SettingsLayout area={t.account.title} title={`${t.account.title} — ${label}`} {groups}>
  {#key section}
    {#if section === 'profile'}
      <ProfilePage />
    {:else if section === 'appearance'}
      <AppearancePage />
    {:else if section === 'notifications'}
      <NotificationsPage />
    {:else if section === 'tokens'}
      <TokensPage />
    {:else}
      <SubscriptionsPage />
    {/if}
  {/key}
</SettingsLayout>
