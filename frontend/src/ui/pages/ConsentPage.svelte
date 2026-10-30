<script lang="ts">
  import type { OAuthRequest, TokenScope } from '../../core/admin-types.js';
  import { t } from '../../i18n/t.js';
  import { accountApi } from '../../lib/admin-api.js';
  import { ApiError, errorMessage } from '../../lib/api.js';
  import { router } from '../../lib/router.svelte.js';
  import { session } from '../../lib/session.svelte.js';
  import AuthSheet from '../AuthSheet.svelte';

  /**
   * The consent page of OAuth (/oauth/authorize): an MCP client asks to use
   * the account. The user sees which application, where the browser goes
   * back, and the scopes asked for, unticks what it does not need, then
   * allows or refuses. Either way the server gives the address to go back to.
   */
  const query = router.search;

  let req = $state<OAuthRequest | null>(null);
  let ticked = $state<TokenScope[]>([]);
  let error = $state('');
  /** Where to go back with the error, when the client can be trusted with it. */
  let errorBack = $state<{ href: string; client: string } | null>(null);
  let pending = $state(false);
  let leaving = $state(false);

  $effect(() => {
    document.title = `${req ? t.oauth.title(req.client.name) : t.oauth.checking} — ${t.appName}`;
  });

  $effect(() => {
    accountApi
      .checkAuthorization(query)
      .then((r) => {
        req = r;
        ticked = [...r.scopes];
      })
      .catch((e: unknown) => {
        error = errorMessage(e);
        const data =
          e instanceof ApiError ? (e.data as { redirect?: string; client?: string }) : null;
        if (data?.redirect && data.client) errorBack = { href: data.redirect, client: data.client };
      });
  });

  function toggle(sc: TokenScope, on: boolean) {
    ticked = on ? [...ticked, sc] : ticked.filter((s) => s !== sc);
  }

  async function answer(allow: boolean) {
    pending = true;
    error = '';
    try {
      const { redirect } = await accountApi.consent(query, allow, ticked);
      leaving = true;
      window.location.assign(redirect);
    } catch (e) {
      error = errorMessage(e);
      pending = false;
    }
  }
</script>

<AuthSheet>
  {#if leaving}
    <p class="muted loading" role="status">{t.oauth.leaving}</p>
  {:else if req}
    <h1 class="auth-title">{t.oauth.title(req.client.name)}</h1>
    <p class="auth-lead">
      {t.oauth.identified(req.client.host)}
      {#if session.me}{t.oauth.signedIn(session.me.name)}{/if}
    </p>
    <form
      onsubmit={(e) => {
        e.preventDefault();
        void answer(true);
      }}
    >
      <fieldset class="choices scopes">
        <legend class="field-label">{t.oauth.scopes}</legend>
        {#each req.scopes as sc (sc)}
          <label class="check">
            <input
              class="checkbox"
              type="checkbox"
              checked={sc === 'read' || ticked.includes(sc)}
              disabled={sc === 'read' || pending}
              onchange={(e) => toggle(sc, e.currentTarget.checked)}
            />
            <span>
              <span class="key">{sc}</span>
              <span class="muted small">{t.tokensPage.scopeHints[sc]}</span>
            </span>
          </label>
        {/each}
        <span class="hint">{t.oauth.scopesHint}</span>
      </fieldset>
      <p class="back muted small">{t.oauth.back(req.redirectHost)}</p>
      {#if error}<p class="error-text" role="alert">{error}</p>{/if}
      <div class="actions">
        <button class="btn" type="button" disabled={pending} onclick={() => void answer(false)}
          >{t.oauth.deny}</button
        >
        <button class="btn btn-primary" type="submit" disabled={pending}>{t.oauth.allow}</button>
      </div>
    </form>
  {:else if error}
    <p class="error-text" role="alert">{error}</p>
    {#if errorBack}
      <p class="aside"><a href={errorBack.href}>{t.oauth.backTo(errorBack.client)}</a></p>
    {/if}
  {:else}
    <p class="muted loading">{t.oauth.checking}</p>
  {/if}
</AuthSheet>

<style>
  .scopes {
    display: grid;
    gap: 8px;
    min-width: 0;
  }

  .scopes .key {
    margin-right: 6px;
  }

  .back {
    margin: 0;
    overflow-wrap: anywhere;
  }

  .actions {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
  }
</style>
