<script lang="ts">
  import { onMount } from 'svelte';
  import { canAskForLink } from '../../core/account-links.js';
  import { paths } from '../../core/route.js';
  import { t } from '../../i18n/t.js';
  import { api, errorMessage } from '../../lib/api.js';
  import { publicSettings } from '../../lib/public-settings.svelte.js';
  import AuthSheet from '../AuthSheet.svelte';

  /**
   * "Forgot your password?": a user name or an address, and always the same
   * answer, whether or not an account matches.
   */
  let identifier = $state('');
  let pending = $state(false);
  let error = $state('');
  let sent = $state(false);

  $effect(() => {
    document.title = `${t.accountLinks.forgotTitle} — ${t.appName}`;
  });

  onMount(() => void publicSettings.load());

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    if (!canAskForLink(identifier)) return;
    pending = true;
    error = '';
    try {
      await api.forgotPassword(identifier.trim());
      sent = true;
    } catch (err) {
      error = errorMessage(err);
    } finally {
      pending = false;
    }
  }
</script>

<AuthSheet>
  {#if !publicSettings.loaded}
    <p class="auth-lead loading">{t.common.loading}</p>
  {:else if !publicSettings.value.emailLinks}
    <h1 class="auth-title">{t.accountLinks.forgotTitle}</h1>
    <p class="auth-lead">{t.accountLinks.unavailable}</p>
    <p class="aside"><a href={paths.login()}>{t.accountLinks.backToSignIn}</a></p>
  {:else if sent}
    <h1 class="auth-title">{t.accountLinks.sentTitle}</h1>
    <p class="auth-lead" role="status">{t.accountLinks.sent}</p>
    <p class="aside"><a href={paths.login()}>{t.accountLinks.backToSignIn}</a></p>
  {:else}
    <h1 class="auth-title">{t.accountLinks.forgotTitle}</h1>
    <p class="auth-lead">{t.accountLinks.forgotIntro}</p>
    <form onsubmit={submit}>
      <label class="field">
        <span>{t.accountLinks.identifier}</span>
        <!-- svelte-ignore a11y_autofocus -->
        <input
          class="input"
          bind:value={identifier}
          autocomplete="username"
          autocapitalize="none"
          spellcheck="false"
          required
          autofocus
        />
      </label>
      {#if error}<p class="error-text" role="alert">{error}</p>{/if}
      <button class="btn btn-primary submit" disabled={pending || !canAskForLink(identifier)}
        >{pending ? t.accountLinks.sending : t.accountLinks.send}</button
      >
    </form>
    <p class="aside"><a href={paths.login()}>{t.accountLinks.backToSignIn}</a></p>
  {/if}
</AuthSheet>
