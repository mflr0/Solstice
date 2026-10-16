<script lang="ts">
  import { onMount } from 'svelte';
  import { paths, safeNext } from '../../core/route.js';
  import { t } from '../../i18n/t.js';
  import { api, errorMessage } from '../../lib/api.js';
  import { publicSettings } from '../../lib/public-settings.svelte.js';
  import { router } from '../../lib/router.svelte.js';
  import { session } from '../../lib/session.svelte.js';
  import AuthSheet from '../AuthSheet.svelte';

  let username = $state('');
  let password = $state('');
  let pending = $state(false);
  let error = $state('');

  $effect(() => {
    document.title = `${t.login.title} — ${t.appName}`;
  });

  // Whether "forgot your password" can send an e-mail is the instance's
  // current setting, not what was known at the last visit.
  onMount(() => void publicSettings.load());

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    pending = true;
    error = '';
    try {
      const me = await api.login(username.trim(), password);
      await session.signedIn(me);
      router.navigate(safeNext(router.param('next')), { replace: true });
    } catch (err) {
      error = errorMessage(err);
      password = '';
    } finally {
      pending = false;
    }
  }
</script>

<AuthSheet>
  <form onsubmit={submit}>
    <h1 class="sr-only">{t.login.title}</h1>
    <label class="field">
      <span>{t.login.username}</span>
      <!-- svelte-ignore a11y_autofocus -->
      <input class="input" bind:value={username} autocomplete="username" required autofocus />
    </label>
    <label class="field">
      <span>{t.login.password}</span>
      <input
        class="input"
        type="password"
        bind:value={password}
        autocomplete="current-password"
        required
      />
    </label>
    {#if error}<p class="error-text" role="alert">{error}</p>{/if}
    <button class="btn btn-primary submit" disabled={pending}
      >{pending ? t.login.pending : t.login.submit}</button
    >
  </form>
  {#if publicSettings.loaded}
    <p class="aside">
      {#if publicSettings.value.emailLinks}
        <a href={paths.forgot()}>{t.login.forgot}</a>
      {:else}
        {t.login.forgotAdmin}
      {/if}
    </p>
  {/if}
  <p class="note">{t.login.note}</p>
</AuthSheet>
