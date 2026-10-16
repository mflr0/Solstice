<script lang="ts">
  import type { Snippet } from 'svelte';
  import { t } from '../i18n/t.js';
  import Logo from './Logo.svelte';

  /**
   * The sheet the pages outside the application sit on: the sign-in form and
   * the pages of the forgotten-password links. Forms inside take the sheet's
   * spacing; `.submit` is the main button, `.aside` a line under it, `.note` a quiet
   * line at the end.
   */
  interface Props {
    children: Snippet;
  }
  const { children }: Props = $props();
</script>

<main class="login">
  <div class="sheet card login-card">
    <div class="brand">
      <Logo size={34} />
      <span class="wordmark">{t.appName}</span>
    </div>
    <p class="tagline">{t.tagline}</p>
    {@render children()}
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
    border: var(--frame) solid var(--rule);
    border-radius: var(--radius-l);
    box-shadow: var(--shadow-pop);
    padding: 30px 30px 22px;
    border-top: var(--card-top);
  }

  .brand {
    display: flex;
    align-items: center;
    gap: 10px;
  }

  .wordmark {
    font: var(--title-weight) 34px/1 var(--font-title);
    letter-spacing: -0.015em;
  }

  .tagline {
    margin: 6px 0 24px;
    color: var(--muted);
    font: var(--title-italic) 15px var(--font-title);
  }

  .sheet :global(.auth-title) {
    margin: 0 0 8px;
    font: var(--title-weight) 20px/1.2 var(--font-title);
  }

  .sheet :global(.auth-lead) {
    margin: 0 0 16px;
    font-size: 13px;
    line-height: 1.5;
    color: var(--muted);
  }

  .sheet :global(form) {
    display: grid;
    gap: 14px;
  }

  .sheet :global(.submit) {
    justify-content: center;
    height: 36px;
    margin-top: 4px;
  }

  .sheet :global(.aside) {
    margin: 14px 0 0;
    font-size: 13px;
    color: var(--muted);
  }

  .sheet :global(.note) {
    margin: 18px 0 0;
    padding-top: 12px;
    border-top: 1px solid var(--rule);
    font-size: 12px;
    color: var(--muted);
  }
</style>
