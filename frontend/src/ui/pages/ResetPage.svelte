<script lang="ts">
  import { onMount } from 'svelte';
  import {
    choosePasswordProblem,
    isLinkPurpose,
    type LinkPurpose,
  } from '../../core/account-links.js';
  import { paths } from '../../core/route.js';
  import { t } from '../../i18n/t.js';
  import { api, ApiError, errorMessage } from '../../lib/api.js';
  import { publicSettings } from '../../lib/public-settings.svelte.js';
  import { router } from '../../lib/router.svelte.js';
  import { session } from '../../lib/session.svelte.js';
  import { notify } from '../../lib/ui.svelte.js';
  import AuthSheet from '../AuthSheet.svelte';

  /**
   * The page a link from an e-mail leads to: choose a password (a forgotten
   * one, or the first one of an invited account). The link is checked first,
   * so that one that no longer works says so before anything is typed.
   */
  interface Props {
    token: string;
  }
  const { token }: Props = $props();

  type Link = { purpose: LinkPurpose; name: string };
  let link = $state<Link | null>(null);
  let stage = $state<'checking' | 'ok' | 'invalid' | 'error'>('checking');
  let loadError = $state('');
  let password = $state('');
  let confirm = $state('');
  let tried = $state(false);
  let pending = $state(false);
  let error = $state('');

  const min = $derived(publicSettings.value.passwordMinLength);
  const problem = $derived(choosePasswordProblem(password, confirm, min));
  const invite = $derived(link?.purpose === 'invite');

  $effect(() => {
    document.title = `${invite ? t.accountLinks.inviteTitle : t.accountLinks.resetTitle} — ${t.appName}`;
  });

  onMount(() => {
    void publicSettings.load();
    void check();
  });

  async function check() {
    try {
      const info = await api.checkLink(token);
      link = { purpose: isLinkPurpose(info.purpose) ? info.purpose : 'reset', name: info.name };
      stage = 'ok';
    } catch (e) {
      // 400: unknown, used, expired or replaced. Anything else (too many
      // attempts, no network) is told as it is.
      if (e instanceof ApiError && e.status === 400) {
        stage = 'invalid';
      } else {
        stage = 'error';
        loadError = errorMessage(e);
      }
    }
  }

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    tried = true;
    error = '';
    if (problem) return;
    pending = true;
    try {
      const me = await api.useLink(token, password);
      await session.signedIn(me);
      notify(t.accountLinks.saved);
      router.navigate(paths.home(), { replace: true });
    } catch (err) {
      error = errorMessage(err);
      // The link may have just been used up elsewhere: if so the page says it.
      void check();
    } finally {
      pending = false;
    }
  }
</script>

<AuthSheet>
  {#if stage === 'checking'}
    <p class="auth-lead loading">{t.accountLinks.checking}</p>
  {:else if stage === 'invalid'}
    <h1 class="auth-title">{t.accountLinks.invalidTitle}</h1>
    <p class="auth-lead">{t.accountLinks.invalid}</p>
    {#if publicSettings.value.emailLinks}
      <p class="aside"><a href={paths.forgot()}>{t.accountLinks.askAgain}</a></p>
    {/if}
    <p class="aside"><a href={paths.login()}>{t.accountLinks.backToSignIn}</a></p>
  {:else if stage === 'error'}
    <p class="error-text" role="alert">{loadError}</p>
    <p class="aside"><a href={paths.login()}>{t.accountLinks.backToSignIn}</a></p>
  {:else if link}
    <h1 class="auth-title">
      {invite ? t.accountLinks.inviteTitle : t.accountLinks.resetTitle}
    </h1>
    <p class="auth-lead">
      {invite ? t.accountLinks.inviteIntro(link.name) : t.accountLinks.resetIntro(link.name)}
    </p>
    <form onsubmit={submit} novalidate>
      <label class="field">
        <span>{t.accountLinks.newPassword}</span>
        <!-- svelte-ignore a11y_autofocus -->
        <input
          class="input"
          type="password"
          bind:value={password}
          autocomplete="new-password"
          aria-invalid={tried && problem === 'tooShort'}
          required
          autofocus
        />
        {#if tried && problem === 'tooShort'}
          <span class="field-error">{t.accountLinks.tooShort(min)}</span>
        {:else}
          <span class="hint">{t.usersAdmin.passwordHint(min)}</span>
        {/if}
      </label>
      <label class="field">
        <span>{t.accountLinks.confirm}</span>
        <input
          class="input"
          type="password"
          bind:value={confirm}
          autocomplete="new-password"
          aria-invalid={tried && problem === 'mismatch'}
          required
        />
        {#if tried && problem === 'mismatch'}
          <span class="field-error">{t.accountLinks.mismatch}</span>
        {/if}
      </label>
      {#if error}<p class="error-text" role="alert">{error}</p>{/if}
      <button class="btn btn-primary submit" disabled={pending}
        >{pending ? t.accountLinks.saving : t.accountLinks.save}</button
      >
    </form>
    {#if !invite}<p class="note">{t.accountLinks.sessionsEnd}</p>{/if}
  {/if}
</AuthSheet>
