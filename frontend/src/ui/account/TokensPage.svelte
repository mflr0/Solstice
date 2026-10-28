<script lang="ts">
  import { tokenExpiry, tokenProblems, type ExpiryChoice } from '../../core/admin-forms.js';
  import { tokenState, type TokenState } from '../../core/admin-integrations.js';
  import type { AccessToken } from '../../core/admin-types.js';
  import { isoDay } from '../../core/dates.js';
  import { formatDateTime, relative, t } from '../../i18n/t.js';
  import { accountApi } from '../../lib/admin-api.js';
  import { errorMessage } from '../../lib/api.js';
  import { notify } from '../../lib/ui.svelte.js';
  import ConfirmDialog from '../ConfirmDialog.svelte';
  import CopyField from '../CopyField.svelte';
  import Dropdown from '../Dropdown.svelte';
  import EmptyState from '../EmptyState.svelte';

  /**
   * One's personal access tokens: create one (a name and an expiry), copy
   * its secret, which is shown this once, and revoke the ones no longer
   * needed. The instance may limit who creates tokens (administration).
   */
  const EXPIRIES: ExpiryChoice[] = ['30', '90', '365', 'date', 'never'];
  const STAMPS: Record<TokenState, string> = { active: 'done', expiring: 'doing', expired: '' };

  let tokens = $state<AccessToken[] | null>(null);
  let canCreate = $state<boolean | null>(null);
  let loadError = $state('');
  let name = $state('');
  let choice = $state<ExpiryChoice>('90');
  let date = $state('');
  let tried = $state(false);
  let creating = $state(false);
  let createError = $state('');
  let created = $state<AccessToken | null>(null);
  let revoking = $state<AccessToken | null>(null);

  const now = $derived(new Date());
  const today = $derived(isoDay(now));
  const problems = $derived(tokenProblems(name, choice, date, now));
  const shown = $derived(tried ? problems : {});

  async function load() {
    try {
      tokens = await accountApi.tokens();
      loadError = '';
    } catch (e) {
      loadError = errorMessage(e);
    }
  }

  $effect(() => {
    void load();
    accountApi
      .tokenPolicy()
      .then((p) => (canCreate = p.canCreate))
      .catch(() => (canCreate = null));
  });

  async function create(e: SubmitEvent) {
    e.preventDefault();
    tried = true;
    createError = '';
    if (problems.name || problems.expires) return;
    creating = true;
    try {
      created = await accountApi.createToken(name.trim(), tokenExpiry(choice, date, now));
      name = '';
      date = '';
      tried = false;
      await load();
    } catch (err) {
      createError = errorMessage(err);
    } finally {
      creating = false;
    }
  }
</script>

<header class="page-head">
  <div>
    <p class="eyebrow">{t.account.title}</p>
    <h1 class="page-title">{t.account.sections.tokens}</h1>
  </div>
</header>
<p class="lead-text">{t.tokensPage.intro} <code class="mono">Authorization: Bearer …</code></p>

{#if created?.token}
  <section class="result secret" role="status" aria-labelledby="token-created">
    <strong id="token-created">{t.tokensPage.createdTitle(created.name)}</strong>
    <p>{t.tokensPage.copyNow}</p>
    <CopyField value={created.token} label={t.tokensPage.secret} />
    <div>
      <button class="btn" onclick={() => (created = null)}>{t.tokensPage.copied}</button>
    </div>
  </section>
{/if}

{#if canCreate === false}
  <p class="result failed" role="note">{t.tokensPage.restricted}</p>
{:else}
  <section class="card new" aria-labelledby="token-new">
    <h2 class="card-head" id="token-new">{t.tokensPage.newTitle}</h2>
    <form class="card-body" onsubmit={create} novalidate>
      <div class="form-grid wide-first">
        <label class="field">
          <span>{t.tokensPage.name}</span>
          <input
            class="input"
            bind:value={name}
            autocomplete="off"
            aria-invalid={!!shown.name}
            aria-describedby="token-name-hint"
          />
          {#if shown.name}
            <span class="field-error">{t.tokensPage.problems[shown.name]}</span>
          {:else}
            <span class="hint" id="token-name-hint">{t.tokensPage.nameHint}</span>
          {/if}
        </label>
        <div class="field">
          <span>{t.tokensPage.expires}</span>
          <Dropdown
            label={t.tokensPage.expires}
            value={choice}
            options={EXPIRIES.map((c) => ({ value: c, label: t.tokensPage.expiry[c] }))}
            onchange={(v) => (choice = v as ExpiryChoice)}
          />
        </div>
      </div>
      {#if choice === 'date'}
        <label class="field date">
          <span>{t.tokensPage.lastDay}</span>
          <input
            class="input"
            type="date"
            bind:value={date}
            min={today}
            aria-invalid={!!shown.expires}
          />
          {#if shown.expires}
            <span class="field-error">{t.tokensPage.problems[shown.expires]}</span>
          {/if}
        </label>
      {/if}
      {#if createError}<p class="error-text" role="alert">{createError}</p>{/if}
      <div>
        <button class="btn btn-primary" type="submit" disabled={creating}
          >{creating ? t.tokensPage.creating : t.tokensPage.create}</button
        >
      </div>
    </form>
  </section>
{/if}

<h2 class="section-title">{t.tokensPage.listTitle}</h2>
{#if loadError}
  <p class="error-text" role="alert">{loadError}</p>
{:else if !tokens}
  <p class="muted loading">{t.common.loading}</p>
{:else if tokens.length === 0}
  <EmptyState
    message={t.tokensPage.empty}
    hint={canCreate === false ? undefined : t.tokensPage.emptyHint}
  />
{:else}
  <div class="table-scroll">
    <table class="ledger">
      <thead>
        <tr>
          <th>{t.tokensPage.colName}</th>
          <th class="c-created">{t.tokensPage.colCreated}</th>
          <th class="c-used">{t.tokensPage.colUsed}</th>
          <th>{t.tokensPage.colExpires}</th>
          <th><span class="sr-only">{t.common.actions}</span></th>
        </tr>
      </thead>
      <tbody>
        {#each tokens as tok (tok.id)}
          {@const state = tokenState(tok, now)}
          <tr class:is-off={state === 'expired'}>
            <td>{tok.name}</td>
            <td class="c-created muted" title={formatDateTime(tok.created)}
              >{relative(tok.created)}</td
            >
            <td
              class="c-used muted"
              title={tok.lastUsed ? formatDateTime(tok.lastUsed) : undefined}
            >
              {tok.lastUsed ? relative(tok.lastUsed) : t.tokensPage.neverUsed}
            </td>
            <td>
              <span class="stamp {STAMPS[state]}">{t.tokensPage.states[state]}</span>
              <span
                class="muted small"
                title={tok.expires ? formatDateTime(tok.expires) : undefined}
              >
                {tok.expires ? relative(tok.expires) : t.tokensPage.never}
              </span>
            </td>
            <td class="row-actions">
              <button class="link-btn" onclick={() => (revoking = tok)}
                >{t.tokensPage.revoke}</button
              >
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
{/if}

<ConfirmDialog
  open={revoking !== null}
  title={t.tokensPage.revokeTitle(revoking?.name ?? '')}
  confirm={t.tokensPage.revoke}
  onclose={() => (revoking = null)}
  onconfirm={async () => {
    if (!revoking) return;
    await accountApi.revokeToken(revoking.id);
    notify(t.tokensPage.revoked);
    void load();
  }}
>
  <p>{t.tokensPage.revokeBody}</p>
</ConfirmDialog>

<style>
  .secret {
    display: grid;
    gap: 10px;
    max-width: 720px;
    margin-bottom: 22px;
  }

  .secret p {
    margin: 0;
  }

  .new {
    max-width: 720px;
    margin-bottom: 8px;
  }

  .date {
    max-width: 220px;
  }

  @media (max-width: 760px) {
    .c-created {
      display: none;
    }
  }

  @media (max-width: 600px) {
    .c-used {
      display: none;
    }
  }
</style>
