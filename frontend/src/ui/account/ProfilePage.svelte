<script lang="ts">
  import {
    changesEmail,
    profileForm,
    profilePatch,
    profileProblems,
    type ProfileField,
    type ProfileForm,
  } from '../../core/account-settings.js';
  import { hasProblems } from '../../core/admin-forms.js';
  import { AVATAR_SIZE, squareCrop } from '../../core/avatar.js';
  import { initials } from '../../core/marks.js';
  import { paths } from '../../core/route.js';
  import { t } from '../../i18n/t.js';
  import { accountApi } from '../../lib/admin-api.js';
  import { api, errorMessage, hasPicture, pictureChanged, pictureUrl } from '../../lib/api.js';
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
  let currentInput = $state<HTMLInputElement>();

  const min = $derived(publicSettings.value.passwordMinLength);
  const problems = $derived(profileProblems(form, min, session.me));
  const dirty = $derived(
    session.me !== null &&
      (form.name.trim() !== session.me.name ||
        form.email.trim() !== session.me.email ||
        form.newPassword !== ''),
  );

  function shown(f: ProfileField): string {
    const p = problems[f];
    if (!p || !(tried || touched[f])) return '';
    return p === 'newShort' ? t.account.passwordShort(min) : t.account.problems[p];
  }

  $effect(() => {
    if (!publicSettings.loaded) void publicSettings.load();
  });

  /** Address of one's picture, null while the initials stand in. */
  let picture = $state<string | null>(null);
  let pictureBusy = $state(false);
  let pictureError = $state('');
  let fileInput = $state<HTMLInputElement>();

  async function loadPicture() {
    const me = session.me;
    if (!me) return;
    picture = (await hasPicture(me.id)) ? pictureUrl(me.id) : null;
  }

  $effect(() => {
    void loadPicture();
  });

  /** The middle of an image as a square PNG, scaled to what the server keeps; null if it cannot be read. */
  async function squarePng(file: File): Promise<Blob | null> {
    let bitmap: ImageBitmap;
    try {
      bitmap = await createImageBitmap(file);
    } catch {
      return null;
    }
    const crop = squareCrop(bitmap.width, bitmap.height);
    const canvas = document.createElement('canvas');
    canvas.width = AVATAR_SIZE;
    canvas.height = AVATAR_SIZE;
    const context = canvas.getContext('2d');
    if (!crop || !context) {
      bitmap.close();
      return null;
    }
    context.imageSmoothingQuality = 'high';
    context.drawImage(
      bitmap,
      crop.sx,
      crop.sy,
      crop.side,
      crop.side,
      0,
      0,
      AVATAR_SIZE,
      AVATAR_SIZE,
    );
    bitmap.close();
    return new Promise((resolve) => canvas.toBlob(resolve, 'image/png'));
  }

  async function changePicture(e: Event) {
    const input = e.currentTarget as HTMLInputElement;
    const file = input.files?.[0];
    input.value = '';
    const me = session.me;
    if (!file || !me) return;
    pictureBusy = true;
    pictureError = '';
    try {
      const png = await squarePng(file);
      if (!png) {
        pictureError = t.account.pictureUnreadable;
        return;
      }
      await api.setMyAvatar(png);
      pictureChanged(me.id);
      await loadPicture();
      notify(t.account.pictureSaved);
    } catch (err) {
      pictureError = errorMessage(err);
    } finally {
      pictureBusy = false;
    }
  }

  async function removePicture() {
    const me = session.me;
    if (!me) return;
    pictureBusy = true;
    pictureError = '';
    try {
      await api.removeMyAvatar();
      pictureChanged(me.id);
      picture = null;
      notify(t.account.pictureRemoved);
    } catch (err) {
      pictureError = errorMessage(err);
    } finally {
      pictureBusy = false;
    }
  }

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
    if (hasProblems(problems)) {
      // The current password sits in the section below the address.
      if (problems.currentPassword) currentInput?.focus();
      return;
    }
    const patch = profilePatch(form, me);
    if (Object.keys(patch).length === 0) return;
    saving = true;
    try {
      const updated = await accountApi.updateProfile(patch);
      session.me = updated;
      notify(patch.newPassword ? t.account.passwordChanged : t.account.saved);
      reset();
    } catch (err) {
      error = errorMessage(err);
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

  <section class="settings-form">
    <h2 class="section-title">{t.account.picture}</h2>
    <div class="picture">
      <span class="preview" aria-hidden="true">
        {#if picture}
          <img src={picture} alt="" onerror={() => (picture = null)} />
        {:else}
          {initials(session.me.name)}
        {/if}
      </span>
      <div class="picture-actions">
        <p class="form-hint">{t.account.pictureHint}</p>
        <input bind:this={fileInput} type="file" accept="image/*" hidden onchange={changePicture} />
        <button type="button" class="btn" disabled={pictureBusy} onclick={() => fileInput?.click()}
          >{t.account.pictureChange}</button
        >
        {#if picture}
          <button type="button" class="btn btn-quiet" disabled={pictureBusy} onclick={removePicture}
            >{t.account.pictureRemove}</button
          >
        {/if}
        {#if pictureError}<p class="error-text" role="alert">{pictureError}</p>{/if}
      </div>
    </div>
  </section>

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
        {:else if changesEmail(form, session.me)}
          <span class="hint">{t.account.emailChangeHint}</span>
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
        bind:this={currentInput}
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

  .picture {
    display: flex;
    gap: 16px;
    align-items: flex-start;
  }

  .preview {
    display: inline-grid;
    place-items: center;
    width: 64px;
    height: 64px;
    flex: none;
    overflow: hidden;
    border: 1px solid var(--rule-strong);
    border-radius: var(--avatar-radius);
    background: var(--paper-raised);
    color: var(--ink-soft);
    font: 500 20px/1 var(--font-mono);
  }

  .preview img {
    width: 100%;
    height: 100%;
    object-fit: cover;
    display: block;
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
