<script lang="ts">
  import { tokenState, type TokenState } from '../../core/admin-integrations.js';
  import {
    TOKEN_SHOWS,
    creatorsProblem,
    filterTokens,
    refreshDaysValid,
    REFRESH_MAX_DAYS,
    sameCreators,
    type TokenShow,
  } from '../../core/admin-ops.js';
  import type {
    AccessToken,
    AdminOAuthGrant,
    Group,
    TokenScope,
    TokenSettings,
  } from '../../core/admin-types.js';
  import { formatNumber } from '../../i18n/format.js';
  import { formatDateTime, relative, t } from '../../i18n/t.js';
  import { adminApi } from '../../lib/admin-api.js';
  import { errorMessage } from '../../lib/api.js';
  import { notify } from '../../lib/ui.svelte.js';
  import ConfirmDialog from '../ConfirmDialog.svelte';
  import Dropdown from '../Dropdown.svelte';
  import EmptyState from '../EmptyState.svelte';

  /**
   * Every account's personal access tokens and OAuth grants, to revoke, and
   * who may create tokens: administrators only, the members of a group, or
   * everyone; with how long OAuth applications stay signed in and whether
   * they may register themselves.
   */
  const WHO: TokenSettings['who'][] = ['admins', 'group', 'everyone'];
  const STAMPS: Record<TokenState, string> = { active: 'done', expiring: 'doing', expired: '' };

  let tokens = $state<AccessToken[] | null>(null);
  let error = $state('');
  let q = $state('');
  let show = $state<TokenShow>('all');
  let revoking = $state<AccessToken | null>(null);

  let policy = $state<TokenSettings | null>(null);
  let groups = $state<Group[]>([]);
  let who = $state<TokenSettings['who']>('admins');
  let group = $state<number | null>(null);
  let refreshDays = $state('');
  let registration = $state(true);
  let policyError = $state('');
  let saving = $state(false);

  const now = $derived(new Date());
  const list = $derived(filterTokens(tokens ?? [], q, show, now));
  const groupProblem = $derived(creatorsProblem(who, group));
  const refreshProblem = $derived(!refreshDaysValid(refreshDays));
  const dirty = $derived(
    policy !== null &&
      (!sameCreators(policy, who, group) ||
        refreshDays.trim() !== String(policy.refreshDays) ||
        registration !== policy.registration),
  );

  let grants = $state<AdminOAuthGrant[] | null>(null);
  let grantsError = $state('');
  let revokingGrant = $state<AdminOAuthGrant | null>(null);

  function adopt(s: TokenSettings) {
    policy = s;
    who = s.who;
    group = s.group?.id ?? null;
    refreshDays = String(s.refreshDays);
    registration = s.registration;
  }

  async function load() {
    try {
      tokens = await adminApi.tokens('');
      error = '';
    } catch (e) {
      error = errorMessage(e);
    }
  }

  async function loadGrants() {
    try {
      grants = await adminApi.grants();
      grantsError = '';
    } catch (e) {
      grantsError = errorMessage(e);
    }
  }

  $effect(() => {
    void load();
    void loadGrants();
    adminApi
      .tokenSettings()
      .then(adopt)
      .catch((e: unknown) => (policyError = errorMessage(e)));
    adminApi
      .groups()
      .then((g) => (groups = g))
      .catch(() => (groups = []));
  });

  async function save(e: SubmitEvent) {
    e.preventDefault();
    if (groupProblem || refreshProblem) return;
    saving = true;
    policyError = '';
    try {
      adopt(
        await adminApi.saveTokenSettings(
          who,
          who === 'group' ? (group ?? undefined) : undefined,
          Number(refreshDays.trim()),
          registration,
        ),
      );
      notify(t.tokensAdmin.policySaved);
    } catch (err) {
      policyError = errorMessage(err);
    } finally {
      saving = false;
    }
  }

  function clear() {
    q = '';
    show = 'all';
  }
</script>

<header class="page-head">
  <div>
    <p class="eyebrow">{t.adminArea.groups.integrations}</p>
    <h1 class="page-title">
      {t.adminArea.sections.tokens}
      {#if tokens}<span class="count muted">{formatNumber(tokens.length)}</span>{/if}
    </h1>
  </div>
</header>
<p class="lead-text">{t.tokensAdmin.intro}</p>

<section class="card policy" aria-labelledby="token-policy">
  <h2 class="card-head" id="token-policy">{t.tokensAdmin.policyTitle}</h2>
  <div class="card-body">
    <p class="muted small">{t.tokensAdmin.policyIntro}</p>
    {#if policyError}<p class="error-text" role="alert">{policyError}</p>{/if}
    {#if policy}
      <form class="policy-form" onsubmit={save} novalidate>
        <fieldset class="choices">
          <legend class="sr-only">{t.tokensAdmin.policyTitle}</legend>
          {#each WHO as w (w)}
            <label class="check">
              <input type="radio" name="token-who" value={w} bind:group={who} />
              <span>{t.tokensAdmin.who[w]}</span>
            </label>
          {/each}
        </fieldset>
        {#if who === 'group'}
          <div class="field">
            <span>{t.tokensAdmin.group}</span>
            <Dropdown
              label={t.tokensAdmin.group}
              value={group === null ? '' : String(group)}
              options={[
                { value: '', label: t.tokensAdmin.groupPlaceholder },
                ...groups.map((g) => ({ value: String(g.id), label: g.name })),
              ]}
              invalid={!!groupProblem}
              onchange={(v) => (group = v ? Number(v) : null)}
            />
            {#if groups.length === 0}
              <span class="hint">{t.tokensAdmin.noGroups}</span>
            {:else if policy.who === 'group' && !policy.group}
              <span class="field-error">{t.tokensAdmin.groupGone}</span>
            {/if}
          </div>
          {#if groupProblem && dirty}
            <p class="field-error">{t.tokensAdmin.problems[groupProblem]}</p>
          {/if}
        {/if}
        <label class="field refresh">
          <span>{t.tokensAdmin.refreshDays}</span>
          <input
            class="input"
            inputmode="numeric"
            bind:value={refreshDays}
            aria-invalid={refreshProblem}
            aria-describedby="token-refresh-hint"
          />
          {#if refreshProblem}
            <span class="field-error" id="token-refresh-hint"
              >{t.tokensAdmin.refreshDaysInvalid(REFRESH_MAX_DAYS)}</span
            >
          {:else}
            <span class="hint" id="token-refresh-hint">{t.tokensAdmin.refreshDaysHint}</span>
          {/if}
        </label>
        <div class="field">
          <label class="check">
            <input
              class="checkbox"
              type="checkbox"
              bind:checked={registration}
              aria-describedby="token-registration-hint"
            />
            <span>{t.tokensAdmin.registration}</span>
          </label>
          <span class="hint" id="token-registration-hint">{t.tokensAdmin.registrationHint}</span>
        </div>
        <div>
          <button
            class="btn btn-primary"
            type="submit"
            disabled={saving || !dirty || !!groupProblem || refreshProblem}
            >{saving ? t.common.saving : t.common.save}</button
          >
        </div>
      </form>
    {:else if !policyError}
      <p class="muted loading">{t.common.loading}</p>
    {/if}
  </div>
</section>

{#snippet scopeList(tok: { scopes: TokenScope[] })}
  <span class="scope-list">
    {#each tok.scopes as sc (sc)}
      <span class="chip key" title={t.tokensPage.scopeHints[sc]}>{sc}</span>
    {/each}
  </span>
{/snippet}

<div class="list-tools">
  <input
    class="input search"
    type="search"
    bind:value={q}
    placeholder={t.tokensAdmin.search}
    aria-label={t.tokensAdmin.search}
  />
  <Dropdown
    label={t.tokensAdmin.show}
    value={show}
    options={TOKEN_SHOWS.map((s) => ({ value: s, label: t.tokensAdmin.shows[s] }))}
    onchange={(v) => (show = v as (typeof TOKEN_SHOWS)[number])}
  />
</div>

{#if error}
  <p class="error-text" role="alert">{error}</p>
{:else if !tokens}
  <p class="muted loading">{t.common.loading}</p>
{:else if tokens.length === 0}
  <EmptyState message={t.tokensAdmin.empty} hint={t.tokensAdmin.emptyHint} />
{:else if list.length === 0}
  <EmptyState kind="search" message={t.adminArea.noMatch}>
    <button class="btn" onclick={clear}>{t.adminArea.clearSearch}</button>
  </EmptyState>
{:else}
  <div class="table-scroll">
    <table class="ledger tokens">
      <thead>
        <tr>
          <th>{t.tokensAdmin.colOwner}</th>
          <th>{t.tokensAdmin.colName}</th>
          <th class="c-scopes">{t.tokensAdmin.colScopes}</th>
          <th class="c-created">{t.tokensAdmin.colCreated}</th>
          <th class="c-used">{t.tokensAdmin.colUsed}</th>
          <th>{t.tokensAdmin.colExpires}</th>
          <th><span class="sr-only">{t.common.actions}</span></th>
        </tr>
      </thead>
      <tbody>
        {#each list as tok (tok.id)}
          {@const state = tokenState(tok, now)}
          <tr class:is-off={state === 'expired' || tok.user?.active === false}>
            <td>
              {#if tok.user}
                {tok.user.name}
                <span class="key muted">{tok.user.username}</span>
                {#if !tok.user.active}<span class="stamp">{t.tokensAdmin.inactiveOwner}</span>{/if}
              {/if}
            </td>
            <td>
              {tok.name}
              <span class="scopes-below">{@render scopeList(tok)}</span>
            </td>
            <td class="c-scopes">{@render scopeList(tok)}</td>
            <td class="c-created muted" title={formatDateTime(tok.created)}
              >{relative(tok.created)}</td
            >
            <td
              class="c-used muted"
              title={tok.lastUsed ? formatDateTime(tok.lastUsed) : undefined}
            >
              {tok.lastUsed ? relative(tok.lastUsed) : t.tokensAdmin.neverUsed}
            </td>
            <td>
              <span class="stamp {STAMPS[state]}">{t.tokensAdmin.states[state]}</span>
              <span
                class="muted small"
                title={tok.expires ? formatDateTime(tok.expires) : undefined}
              >
                {tok.expires ? relative(tok.expires) : t.tokensAdmin.never}
              </span>
            </td>
            <td class="row-actions">
              <button class="link-btn" onclick={() => (revoking = tok)}
                >{t.tokensAdmin.revoke}</button
              >
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
{/if}

<section class="grants" aria-labelledby="oauth-grants">
  <h2 class="section-title" id="oauth-grants">
    {t.tokensAdmin.grantsTitle}
    {#if grants}<span class="count muted">{formatNumber(grants.length)}</span>{/if}
  </h2>
  <p class="muted small">{t.tokensAdmin.grantsIntro}</p>
  {#if grantsError}
    <p class="error-text" role="alert">{grantsError}</p>
  {:else if !grants}
    <p class="muted loading">{t.common.loading}</p>
  {:else if grants.length === 0}
    <EmptyState message={t.tokensAdmin.grantsEmpty} hint={t.tokensAdmin.grantsEmptyHint} />
  {:else}
    <div class="table-scroll">
      <table class="ledger tokens">
        <thead>
          <tr>
            <th>{t.tokensAdmin.colOwner}</th>
            <th>{t.tokensAdmin.colApp}</th>
            <th class="c-scopes">{t.tokensAdmin.colScopes}</th>
            <th class="c-created">{t.tokensAdmin.colAuthorized}</th>
            <th class="c-used">{t.tokensAdmin.colUsed}</th>
            <th><span class="sr-only">{t.common.actions}</span></th>
          </tr>
        </thead>
        <tbody>
          {#each grants as g (g.id)}
            <tr class:is-off={!g.user.active}>
              <td>
                {g.user.name}
                <span class="key muted">{g.user.username}</span>
                {#if !g.user.active}<span class="stamp">{t.tokensAdmin.inactiveOwner}</span>{/if}
              </td>
              <td class="app">
                {g.clientName}
                {#if g.registered}
                  <span class="stamp" title={t.tokensAdmin.registeredHint}
                    >{t.tokensAdmin.registered}</span
                  >
                {:else}
                  <span class="key muted host" title={g.clientId}>{g.clientHost}</span>
                {/if}
                <span class="scopes-below">{@render scopeList(g)}</span>
              </td>
              <td class="c-scopes">{@render scopeList(g)}</td>
              <td class="c-created muted" title={formatDateTime(g.created)}
                >{relative(g.created)}</td
              >
              <td class="c-used muted" title={g.lastUsed ? formatDateTime(g.lastUsed) : undefined}>
                {g.lastUsed ? relative(g.lastUsed) : t.tokensAdmin.neverUsed}
              </td>
              <td class="row-actions">
                <button class="link-btn" onclick={() => (revokingGrant = g)}
                  >{t.tokensAdmin.revoke}</button
                >
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
</section>

<ConfirmDialog
  open={revokingGrant !== null}
  title={t.tokensAdmin.revokeGrantTitle(revokingGrant?.clientName ?? '')}
  confirm={t.tokensAdmin.revoke}
  onclose={() => (revokingGrant = null)}
  onconfirm={async () => {
    if (!revokingGrant) return;
    await adminApi.revokeGrant(revokingGrant.id);
    notify(t.tokensAdmin.grantRevoked);
    void loadGrants();
  }}
>
  <p>{t.tokensAdmin.revokeGrantBody(revokingGrant?.user.name ?? '')}</p>
</ConfirmDialog>

<ConfirmDialog
  open={revoking !== null}
  title={t.tokensAdmin.revokeTitle(revoking?.name ?? '')}
  confirm={t.tokensAdmin.revoke}
  onclose={() => (revoking = null)}
  onconfirm={async () => {
    if (!revoking) return;
    await adminApi.revokeToken(revoking.id);
    notify(t.tokensAdmin.revoked);
    void load();
  }}
>
  <p>{t.tokensAdmin.revokeBody(revoking?.user?.name ?? '')}</p>
</ConfirmDialog>

<style>
  .count {
    margin-left: 6px;
    font: 500 0.875rem/1 var(--font-mono);
  }

  .policy {
    max-width: 45rem;
    margin-bottom: 26px;
  }

  .scope-list {
    display: flex;
    flex-wrap: wrap;
    gap: 4px;
  }

  .policy-form {
    display: grid;
    gap: 12px;
  }

  .refresh {
    max-width: 22rem;
  }

  .grants {
    margin-top: 32px;
  }

  .app .stamp,
  .app .host {
    margin-left: 4px;
    overflow-wrap: anywhere;
  }

  @media (max-width: 760px) {
    .c-created {
      display: none;
    }
  }

  .scopes-below {
    display: none;
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
