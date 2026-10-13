<script lang="ts">
  import { safeNext } from '../../core/route.js';
  import { t } from '../../i18n/fr.js';
  import { api, errorMessage } from '../../lib/api.js';
  import { router } from '../../lib/router.svelte.js';
  import { session } from '../../lib/session.svelte.js';
  import Logo from '../Logo.svelte';

  let username = $state('');
  let password = $state('');
  let pending = $state(false);
  let error = $state('');

  $effect(() => {
    document.title = `${t.login.title} — ${t.appName}`;
  });

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

<main class="login">
  <div class="sheet">
    <div class="brand">
      <Logo size={34} />
      <span class="wordmark">{t.appName}</span>
    </div>
    <p class="tagline">{t.tagline}</p>

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
    <p class="note">{t.login.note}</p>
  </div>
</main>

<style>
  .login {
    min-height: 100vh;
    display: grid;
    place-items: center;
    padding: 24px;
  }

  .sheet {
    width: min(360px, 100%);
    background: var(--paper-raised);
    border: 1px solid var(--rule);
    border-radius: 6px;
    box-shadow: var(--shadow-pop);
    padding: 30px 30px 22px;
    border-top: 2px solid var(--accent);
  }

  .brand {
    display: flex;
    align-items: center;
    gap: 10px;
  }

  .wordmark {
    font: 600 34px/1 var(--serif);
    letter-spacing: -0.015em;
  }

  .tagline {
    margin: 6px 0 24px;
    color: var(--muted);
    font: italic 15px var(--serif);
  }

  form {
    display: grid;
    gap: 14px;
  }

  .submit {
    justify-content: center;
    height: 36px;
    margin-top: 4px;
  }

  .note {
    margin: 18px 0 0;
    padding-top: 12px;
    border-top: 1px solid var(--rule);
    font-size: 12px;
    color: var(--muted);
  }
</style>
