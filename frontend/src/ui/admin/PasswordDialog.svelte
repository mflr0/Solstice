<script lang="ts">
  import { passwordModes, passwordTooShort, type PasswordMode } from '../../core/account-links.js';
  import type { UserRow } from '../../core/admin-types.js';
  import { t } from '../../i18n/t.js';
  import { adminApi } from '../../lib/admin-api.js';
  import { errorMessage } from '../../lib/api.js';
  import { publicSettings } from '../../lib/public-settings.svelte.js';
  import CopyField from '../CopyField.svelte';
  import Modal from '../Modal.svelte';

  /**
   * A new password for someone: generated (shown once, to pass on), typed,
   * or chosen by the person through a link sent by e-mail. Generated or
   * typed, their sessions end; with a link nothing changes until it is used.
   */
  interface Props {
    user: UserRow | null;
    onclose: () => void;
    onsaved: (message: string) => void;
  }
  const { user, onclose, onsaved }: Props = $props();

  let mode = $state<PasswordMode>('generate');
  let typed = $state('');
  let generated = $state('');
  let error = $state('');
  let busy = $state(false);

  const min = $derived(publicSettings.value.passwordMinLength);
  const modes = $derived(user ? passwordModes(user, publicSettings.value.emailLinks) : []);
  const short = $derived(mode === 'type' && passwordTooShort(typed, min));

  $effect(() => {
    if (!user) return;
    mode = 'generate';
    typed = '';
    generated = '';
    error = '';
  });

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    if (!user || short) return;
    busy = true;
    error = '';
    try {
      if (mode === 'generate') {
        generated = (await adminApi.resetPassword(user.id)).password;
      } else if (mode === 'link') {
        await adminApi.sendResetLink(user.id);
        onsaved(t.usersAdmin.linkSent(user.name));
        onclose();
      } else {
        await adminApi.updateUser(user.id, { password: typed });
        onsaved(t.usersAdmin.passwordSaved(user.name));
        onclose();
      }
    } catch (err) {
      error = errorMessage(err);
    } finally {
      busy = false;
    }
  }
</script>

<Modal
  open={user !== null}
  title={t.usersAdmin.passwordTitle(user?.name ?? '')}
  onclose={() => !busy && onclose()}
>
  {#if generated}
    <p class="warning">{t.usersAdmin.generatedOnce}</p>
    <CopyField value={generated} label={t.usersAdmin.password} />
  {:else}
    <form id="password-form" class="password-form" onsubmit={submit} novalidate>
      <fieldset class="choices">
        <legend class="sr-only">{t.usersAdmin.password}</legend>
        <label class="check">
          <input type="radio" name="pw-mode" value="generate" bind:group={mode} />
          <span>{t.usersAdmin.generate}</span>
        </label>
        <label class="check">
          <input type="radio" name="pw-mode" value="type" bind:group={mode} />
          <span>{t.usersAdmin.typePassword}</span>
        </label>
        {#if modes.includes('link')}
          <label class="check">
            <input type="radio" name="pw-mode" value="link" bind:group={mode} />
            <span>{t.usersAdmin.sendLink}</span>
          </label>
        {/if}
      </fieldset>
      {#if user && !modes.includes('link') && user.active && !user.email.trim() && publicSettings.value.emailLinks}
        <p class="form-hint">{t.usersAdmin.linkNeedsEmail}</p>
      {/if}
      {#if mode === 'type'}
        <label class="field">
          <span>{t.usersAdmin.newPassword}</span>
          <input
            class="input"
            type="password"
            bind:value={typed}
            autocomplete="new-password"
            aria-invalid={typed !== '' && short}
          />
          <span class="hint">{t.usersAdmin.passwordHint(min)}</span>
        </label>
      {/if}
      <p class="form-hint">{mode === 'link' ? t.usersAdmin.linkHint : t.usersAdmin.sessionsEnd}</p>
      {#if error}<p class="error-text" role="alert">{error}</p>{/if}
    </form>
  {/if}
  {#snippet footer()}
    {#if generated}
      <button
        type="button"
        class="btn btn-primary"
        onclick={() => {
          if (user) onsaved(t.usersAdmin.passwordSaved(user.name));
          onclose();
        }}>{t.usersAdmin.done}</button
      >
    {:else}
      <button type="button" class="btn btn-quiet" disabled={busy} onclick={onclose}
        >{t.common.cancel}</button
      >
      <button class="btn btn-primary" type="submit" form="password-form" disabled={busy || short}
        >{mode === 'generate'
          ? t.usersAdmin.generateButton
          : mode === 'link'
            ? t.usersAdmin.sendLinkButton
            : t.common.save}</button
      >
    {/if}
  {/snippet}
</Modal>

<style>
  .password-form {
    display: grid;
    gap: 14px;
  }

  .password-form .form-hint {
    margin: 0;
  }

  .warning {
    margin: 0;
    font-weight: 600;
  }
</style>
