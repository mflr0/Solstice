<script lang="ts">
  import {
    changesPassword,
    profileApplied,
    profileForm,
    profilePatch,
    profileProblems,
    type ProfileField,
    type ProfileForm,
  } from '../../core/account-settings.js';
  import { hasProblems } from '../../core/admin-forms.js';
  import { paths } from '../../core/route.js';
  import { t } from '../../i18n/fr.js';
  import { accountApi } from '../../lib/admin-api.js';
  import { ApiError, errorMessage } from '../../lib/api.js';
  import { publicSettings } from '../../lib/public-settings.svelte.js';
  import { session } from '../../lib/session.svelte.js';
  import { notify } from '../../lib/ui.svelte.js';

  /** One's identity (name, e-mail) and password. */

  let form = $state<ProfileForm>(
    session.me
      ? profileForm(session.me)
      : { name: '', email: '', currentPassword: '', newPassword: '', confirmPassword: '' },
  );
  let tried = $state(false);
  let touched = $state<Partial<Record<ProfileField, boolean>>>({});
  let saving = $state(false);
  let error = $state('');
  /** The server is too old to change the name or e-mail. */
  let unsupported = $state(false);

  const min = $derived(publicSettings.value.passwordMinLength);
  const problems = $derived(profileProblems(form, min));
  const dirty = $derived(
    session.me !== null &&
      (form.name.trim() !== session.me.name ||
        form.email.trim() !== session.me.email ||
        changesPassword(form)),
  );

  function shown(f: ProfileField): string {
    const p = problems[f];
    if (!p || !(tried || touched[f])) return '';
    return p === 'newShort' ? t.account.passwordShort(min) : t.account.problems[p];
  }

  $effect(() => {
    if (!publicSettings.loaded) void publicSettings.load();
  });

  function reset() {
    if (session.me) form = profileForm(session.me);
    tried = false;
    touched = {};
    error = '';
  }

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    const me = session.me;
    if (!me) return;
    tried = true;
    error = '';
    if (hasProblems(problems)) return;
    const patch = profilePatch(form, me);
    if (Object.keys(patch).length === 0) return;
    saving = true;
    try {
      const updated = await accountApi.updateProfile(patch);
      session.me = updated;
      if (!profileApplied(patch, updated)) {
        unsupported = true;
        return;
      }
      notify(patch.newPassword ? t.account.passwordChanged : t.account.saved);
      reset();
    } catch (err) {
      // An older server only knows the language: it refuses the other fields.
      if (err instanceof ApiError && err.status === 400 && err.message.includes('unknown field')) {
        unsupported = true;
      } else {
        error = errorMessage(err);
      }
    } finally {
      saving = false;
    }
  }
</script>

<header class="page-head">
  <div>
    <p class="eyebrow">{t.account.title}</p>
    <h1 class="page-title">{t.account.sections.profile}</h1>
  </div>
  {#if session.me}
    <a class="btn" href={paths.person(session.me.username)}>{t.account.myPage}</a>
  {/if}
</header>
<p class="lead-text">{t.account.profileIntro}</p>

{#if session.me}
  <dl class="who">
    <div>
      <dt>{t.account.username}</dt>
      <dd>
        <span class="key">{session.me.username}</span>
        {#if session.me.admin}<span class="stamp done">{t.account.adminBadge}</span>{/if}
      </dd>
    </div>
  </dl>
  <p class="form-hint who-hint">{t.account.usernameHint}</p>

  {#if unsupported}
    <p class="result failed" role="alert">{t.account.profileUnsupported}</p>
  {/if}

  <form class="settings-form" onsubmit={submit} novalidate>
    <h2 class="section-title">{t.account.identity}</h2>
    <div class="form-grid">
      <label class="field">
        <span>{t.account.name}</span>
        <input
          class="input"
          bind:value={form.name}
          autocomplete="name"
          aria-invalid={!!shown('name')}
          onblur={() => (touched.name = true)}
        />
        {#if shown('name')}<span class="field-error">{shown('name')}</span>{/if}
      </label>
      <label class="field">
        <span>{t.account.email}</span>
        <input
          class="input"
          type="email"
          bind:value={form.email}
          autocomplete="email"
          aria-invalid={!!shown('email')}
          onblur={() => (touched.email = true)}
        />
        {#if shown('email')}
          <span class="field-error">{shown('email')}</span>
        {:else}
          <span class="hint">{t.account.emailHint}</span>
        {/if}
      </label>
    </div>

    <h2 class="section-title">{t.account.passwordTitle}</h2>
    <p class="form-hint">{t.account.passwordIntro}</p>
    <!-- Lets password managers match the account. -->
    <input
      class="sr-only"
      type="text"
      name="username"
      value={session.me.username}
      autocomplete="username"
      readonly
      tabindex="-1"
      aria-hidden="true"
    />
    <label class="field half">
      <span>{t.account.currentPassword}</span>
      <input
        class="input"
        type="password"
        bind:value={form.currentPassword}
        autocomplete="current-password"
        aria-invalid={!!shown('currentPassword')}
        onblur={() => (touched.currentPassword = true)}
      />
      {#if shown('currentPassword')}
        <span class="field-error">{shown('currentPassword')}</span>
      {/if}
    </label>
    <div class="form-grid">
      <label class="field">
        <span>{t.account.newPassword}</span>
        <input
          class="input"
          type="password"
          bind:value={form.newPassword}
          autocomplete="new-password"
          aria-invalid={!!shown('newPassword')}
          onblur={() => (touched.newPassword = true)}
        />
        {#if shown('newPassword')}
          <span class="field-error">{shown('newPassword')}</span>
        {:else}
          <span class="hint">{t.account.passwordHint(min)}</span>
        {/if}
      </label>
      <label class="field">
        <span>{t.account.confirmPassword}</span>
        <input
          class="input"
          type="password"
          bind:value={form.confirmPassword}
          autocomplete="new-password"
          aria-invalid={!!shown('confirmPassword')}
          onblur={() => (touched.confirmPassword = true)}
        />
        {#if shown('confirmPassword')}
          <span class="field-error">{shown('confirmPassword')}</span>
        {/if}
      </label>
    </div>

    {#if error}<p class="error-text" role="alert">{error}</p>{/if}
    {#if dirty}
      <div class="savebar" role="region" aria-label={t.settings.unsaved}>
        <span>{t.settings.unsaved}</span>
        <button type="button" class="btn btn-quiet" onclick={reset}>{t.settings.discard}</button>
        <button class="btn btn-primary" type="submit" disabled={saving}
          >{saving ? t.common.saving : t.common.save}</button
        >
      </div>
    {/if}
  </form>
{/if}

<style>
  .who {
    margin: 0 0 4px;
  }

  .who div {
    display: flex;
    flex-wrap: wrap;
    gap: 6px 14px;
    align-items: baseline;
  }

  .who dt {
    font: 500 12px/1.2 var(--font-body);
    color: var(--ink-soft);
  }

  .who dd {
    margin: 0;
    display: flex;
    gap: 10px;
    align-items: center;
  }

  .who-hint {
    margin: 0 0 18px;
  }

  .result {
    max-width: 720px;
    margin-bottom: 16px;
  }

  .half {
    max-width: calc(50% - 7px);
  }

  @media (max-width: 600px) {
    .half {
      max-width: none;
    }
  }
</style>
