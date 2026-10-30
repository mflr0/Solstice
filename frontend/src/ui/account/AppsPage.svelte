<script lang="ts">
  import type { OAuthGrant } from '../../core/admin-types.js';
  import { formatDateTime, relative, t } from '../../i18n/t.js';
  import { accountApi } from '../../lib/admin-api.js';
  import { errorMessage } from '../../lib/api.js';
  import { notify } from '../../lib/ui.svelte.js';
  import ConfirmDialog from '../ConfirmDialog.svelte';
  import EmptyState from '../EmptyState.svelte';

  /**
   * The applications (MCP clients) one authorized through OAuth, with what
   * they may do, to revoke the ones no longer wanted.
   */
  let grants = $state<OAuthGrant[] | null>(null);
  let loadError = $state('');
  let revoking = $state<OAuthGrant | null>(null);

  async function load() {
    try {
      grants = await accountApi.grants();
      loadError = '';
    } catch (e) {
      loadError = errorMessage(e);
    }
  }

  $effect(() => {
    void load();
  });
</script>

<header class="page-head">
  <div>
    <p class="eyebrow">{t.account.title}</p>
    <h1 class="page-title">{t.account.sections.apps}</h1>
  </div>
</header>
<p class="lead-text">{t.appsPage.intro}</p>

{#if loadError}
  <p class="error-text" role="alert">{loadError}</p>
{:else if !grants}
  <p class="muted loading">{t.common.loading}</p>
{:else if grants.length === 0}
  <EmptyState message={t.appsPage.empty} hint={t.appsPage.emptyHint} />
{:else}
  <div class="table-scroll">
    <table class="ledger">
      <thead>
        <tr>
          <th>{t.appsPage.colApp}</th>
          <th class="c-scopes">{t.appsPage.colScopes}</th>
          <th class="c-created">{t.appsPage.colAuthorized}</th>
          <th class="c-used">{t.appsPage.colUsed}</th>
          <th><span class="sr-only">{t.common.actions}</span></th>
        </tr>
      </thead>
      <tbody>
        {#snippet scopeList(g: OAuthGrant)}
          <span class="scope-list">
            {#each g.scopes as sc (sc)}
              <span class="chip key" title={t.tokensPage.scopeHints[sc]}>{sc}</span>
            {/each}
          </span>
        {/snippet}
        {#each grants as g (g.id)}
          <tr>
            <td>
              {g.clientName}
              <span class="key muted host" title={g.clientId}>{g.clientHost}</span>
              <span class="scopes-below">{@render scopeList(g)}</span>
            </td>
            <td class="c-scopes">{@render scopeList(g)}</td>
            <td class="c-created muted" title={formatDateTime(g.created)}>{relative(g.created)}</td>
            <td class="c-used muted" title={g.lastUsed ? formatDateTime(g.lastUsed) : undefined}>
              {g.lastUsed ? relative(g.lastUsed) : t.appsPage.neverUsed}
            </td>
            <td class="row-actions">
              <button class="link-btn" onclick={() => (revoking = g)}>{t.appsPage.revoke}</button>
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
{/if}

<ConfirmDialog
  open={revoking !== null}
  title={t.appsPage.revokeTitle(revoking?.clientName ?? '')}
  confirm={t.appsPage.revoke}
  onclose={() => (revoking = null)}
  onconfirm={async () => {
    if (!revoking) return;
    await accountApi.revokeGrant(revoking.id);
    notify(t.appsPage.revoked);
    void load();
  }}
>
  <p>{t.appsPage.revokeBody}</p>
</ConfirmDialog>

<style>
  .host {
    display: block;
    overflow-wrap: anywhere;
  }

  .scope-list {
    display: flex;
    flex-wrap: wrap;
    gap: 4px;
  }

  .scopes-below {
    display: none;
  }

  @media (max-width: 760px) {
    .c-created {
      display: none;
    }
  }

  @media (max-width: 600px) {
    .c-used,
    .c-scopes {
      display: none;
    }

    /* Below the name, so that the row still fits a phone. */
    .scopes-below {
      display: block;
      margin-top: 4px;
    }
  }
</style>
