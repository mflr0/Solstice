<script lang="ts">
  import { USERNAME_PATTERN } from '../../core/admin-forms.js';
  import { checkPassword, MIN_PASSWORD } from '../../core/setup.js';
  import type { SetupMessages } from '../../i18n/setup/index.js';
  import { setupApi, setupErrorMessage, type SetupFinished } from '../../lib/setup-api.js';

  /** The first administrator: a new account, Argon2id-hashed by the server. */
  interface Props {
    m: SetupMessages;
    afterImport: boolean;
    ondone: (result: SetupFinished) => void;
    onback: () => void;
    /** Shown in place of the back button, e.g. "choose an imported account instead". */
    backLabel?: string;
  }
  const { m, afterImport, ondone, onback, backLabel }: Props = $props();

  let form = $state({ username: '', name: '', email: '', password: '' });
  let confirm = $state('');
  let pending = $state(false);
  let error = $state('');

  const username = $derived(form.username.trim().toLowerCase());
  const usernameBad = $derived(username !== '' && !USERNAME_PATTERN.test(username));
  const check = $derived(checkPassword(form.password, form.username));
  const mismatch = $derived(confirm !== '' && confirm !== form.password);
  const ready = $derived(
    check.acceptable &&
      confirm === form.password &&
      USERNAME_PATTERN.test(username) &&
      form.email.trim() !== '',
  );
  const level = $derived(
    form.password === '' ? '' : check.acceptable ? m.admin.levels[check.score] : m.admin.tooWeak,
  );

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    if (!ready) return;
    pending = true;
    error = '';
    try {
      ondone(await setupApi.finish({ ...form, username }));
    } catch (err) {
      error = setupErrorMessage(err);
    } finally {
      pending = false;
    }
  }
</script>

<form onsubmit={submit} novalidate>
  <div>
    <h1 class="page-title" id="setup-step-title" tabindex="-1">{m.admin.title}</h1>
    <p class="lead">{afterImport ? m.admin.leadAfterImport : m.admin.lead}</p>
  </div>

  <div class="setup-row">
    <label class="field">
      <span>{m.admin.username}</span>
      <input
        class="input"
        bind:value={form.username}
        required
        autocomplete="username"
        autocapitalize="none"
        spellcheck="false"
        aria-invalid={usernameBad ? 'true' : undefined}
        aria-describedby="admin-username-hint"
      />
      <span class="hint" class:error-text={usernameBad} id="admin-username-hint"
        >{m.admin.usernameHint}</span
      >
    </label>
    <label class="field">
      <span>{m.admin.name}</span>
      <input class="input" bind:value={form.name} autocomplete="name" />
    </label>
  </div>
  <label class="field">
    <span>{m.admin.email}</span>
    <input class="input" type="email" bind:value={form.email} required autocomplete="email" />
  </label>
  <div class="setup-row">
    <label class="field">
      <span>{m.admin.password}</span>
      <input
        class="input"
        type="password"
        bind:value={form.password}
        required
        minlength={MIN_PASSWORD}
        autocomplete="new-password"
        aria-describedby="admin-strength admin-rules"
      />
    </label>
    <label class="field">
      <span>{m.admin.confirm}</span>
      <input
        class="input"
        type="password"
        bind:value={confirm}
        required
        autocomplete="new-password"
        aria-invalid={mismatch ? 'true' : undefined}
        aria-describedby={mismatch ? 'admin-mismatch' : undefined}
      />
      {#if mismatch}<span class="error-text hint" id="admin-mismatch">{m.admin.mismatch}</span>{/if}
    </label>
  </div>

  <div class="strength" id="admin-strength">
    <div class="meter" data-score={form.password === '' ? 0 : check.score} aria-hidden="true">
      {#each [1, 2, 3, 4] as i (i)}
        <span class:on={form.password !== '' && check.acceptable && i <= check.score}></span>
      {/each}
    </div>
    <span class="strength-text" aria-live="polite">
      <span class="muted">{m.admin.strength}</span>
      <strong>{level}</strong>
    </span>
  </div>
  <ul class="rules" id="admin-rules" aria-label={m.admin.rules}>
    <li class:met={check.longEnough}>
      <span aria-hidden="true">{check.longEnough ? '✓' : '·'}</span>
      {m.admin.ruleLength}
      <span class="sr-only">({check.longEnough ? m.admin.met : m.admin.unmet})</span>
    </li>
    <li class:met={check.notUsername && form.password !== ''}>
      <span aria-hidden="true">{check.notUsername && form.password !== '' ? '✓' : '·'}</span>
      {m.admin.ruleUsername}
      <span class="sr-only"
        >({check.notUsername && form.password !== '' ? m.admin.met : m.admin.unmet})</span
      >
    </li>
  </ul>
  <p class="muted advice">{m.admin.advice}</p>

  {#if error}<p class="error-text" role="alert">{error}</p>{/if}

  <div class="setup-actions">
    <button class="btn btn-quiet" type="button" onclick={onback}>{backLabel ?? m.back}</button>
    <button class="btn btn-primary" disabled={pending || !ready}
      >{pending ? m.admin.submitting : m.admin.submit}</button
    >
  </div>
</form>

<style>
  .field .hint.error-text {
    color: var(--accent-text);
  }

  .strength {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .meter {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: 4px;
    width: 180px;
  }

  .meter span {
    height: 6px;
    border-radius: var(--radius-xs);
    background: var(--paper-sunk);
    border: 1px solid var(--rule);
  }

  /* Weak in the accent, then the inks of work under way and work done. */
  .meter[data-score='1'] span.on {
    background: var(--accent);
    border-color: var(--accent);
  }

  .meter[data-score='2'] span.on {
    background: var(--stamp-doing);
    border-color: var(--stamp-doing);
  }

  .meter[data-score='3'] span.on,
  .meter[data-score='4'] span.on {
    background: var(--stamp-done);
    border-color: var(--stamp-done);
  }

  .strength-text {
    font-size: 0.8125rem;
    color: var(--ink-soft);
  }

  .rules {
    list-style: none;
    margin: -6px 0 0;
    padding: 0;
    display: flex;
    flex-wrap: wrap;
    gap: 4px 18px;
    font-size: 0.8125rem;
    color: var(--muted);
  }

  .rules li.met {
    color: var(--stamp-done);
  }

  .advice {
    margin: -6px 0 0;
    font-size: 0.8125rem;
  }
</style>
