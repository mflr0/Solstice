<script lang="ts">
  import { typedMatches } from '../../core/admin-forms.js';
  import type { KeyCheck } from '../../core/admin-types.js';
  import { keyProblem, normalizeKey } from '../../core/project-admin.js';
  import { t } from '../../i18n/t.js';
  import { autofocus } from '../../lib/actions.js';
  import { adminApi } from '../../lib/admin-api.js';
  import { errorMessage } from '../../lib/api.js';
  import Modal from '../Modal.svelte';

  /**
   * Changing a project's key: the new key is checked as it is typed (the
   * creation form's check), and the current key must be typed to confirm.
   * The action runs from the dialog, which stays open and shows the
   * server's refusal in place.
   */
  interface Props {
    open: boolean;
    project: { key: string; name: string } | null;
    onrenamed: (oldKey: string, newKey: string, name: string) => void;
    onclose: () => void;
  }
  const { open, project, onrenamed, onclose }: Props = $props();

  let newKey = $state('');
  let typed = $state('');
  let busy = $state(false);
  let error = $state('');
  let keyState = $state<{ key: string; check: KeyCheck | null } | null>(null);
  let timer: ReturnType<typeof setTimeout> | undefined;
  let seq = 0;
  const formId = 'rename-key-form';

  const current = $derived(project?.key ?? '');
  const same = $derived(newKey !== '' && newKey === current);
  const problem = $derived(newKey && !same ? keyProblem(newKey) : null);
  /** The server's word on the key, once it answers for the key now typed. */
  const taken = $derived(
    keyState && keyState.key === newKey && keyState.check && !keyState.check.available
      ? (keyState.check.reason ?? '')
      : '',
  );
  const free = $derived(
    !same && !keyProblem(newKey) && keyState?.key === newKey && keyState.check?.available === true,
  );
  const checking = $derived(!same && !!newKey && !keyProblem(newKey) && keyState?.key !== newKey);
  const ready = $derived(free && typedMatches(typed, current));

  $effect(() => {
    if (!open) return;
    newKey = '';
    typed = '';
    error = '';
    keyState = null;
    clearTimeout(timer);
  });

  function keyTyped(e: Event & { currentTarget: HTMLInputElement }) {
    newKey = normalizeKey(e.currentTarget.value);
    e.currentTarget.value = newKey;
    clearTimeout(timer);
    const key = newKey;
    if (key === current || keyProblem(key)) return;
    const mine = ++seq;
    timer = setTimeout(async () => {
      try {
        const check = await adminApi.checkKey(key, current);
        if (mine === seq) keyState = { key, check };
      } catch {
        // The server has the last word when the change is sent.
        if (mine === seq) keyState = { key, check: null };
      }
    }, 300);
  }

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    if (!ready || busy || !project) return;
    busy = true;
    error = '';
    try {
      const renamed = await adminApi.renameProject(project.key, newKey, typed.trim());
      onrenamed(project.key, renamed.key, project.name);
      onclose();
    } catch (err) {
      error = errorMessage(err);
    } finally {
      busy = false;
    }
  }
</script>

<Modal
  {open}
  title={t.projectsAdmin.changeKeyTitle(project?.name ?? '')}
  onclose={() => !busy && onclose()}
>
  <form id={formId} class="rename-form" onsubmit={submit} novalidate>
    <p class="body">{t.projectsAdmin.changeKeyBody(current)}</p>
    <label class="field">
      <span>{t.projectsAdmin.newKey}</span>
      <input
        class="input mono"
        value={newKey}
        oninput={keyTyped}
        autocomplete="off"
        spellcheck="false"
        maxlength="10"
        aria-invalid={!!(problem || taken || same)}
        aria-describedby="rename-key-note"
        use:autofocus
      />
    </label>
    <p class="note" id="rename-key-note" aria-live="polite">
      {#if same}
        <span class="field-error">{t.projectsAdmin.keyUnchanged}</span>
      {:else if problem}
        <span class="field-error">{t.projectsAdmin.problems[problem]}</span>
      {:else if taken}
        <span class="field-error">{taken}</span>
      {:else if free}
        <span class="ok">{t.projectsAdmin.keyFree(newKey)}</span>
      {:else if checking}
        <span class="muted">{t.projectsAdmin.keyChecking}</span>
      {:else}
        <span class="muted">{t.projectsAdmin.keyHint}</span>
      {/if}
    </p>
    <label class="field">
      <span>{t.projectsAdmin.typeToConfirm(current)}</span>
      <input
        class="input mono"
        bind:value={typed}
        autocomplete="off"
        spellcheck="false"
        autocapitalize="off"
      />
    </label>
  </form>
  {#if error}<p class="error-text" role="alert">{error}</p>{/if}
  {#snippet footer()}
    <button type="button" class="btn btn-quiet" disabled={busy} onclick={onclose}
      >{t.common.cancel}</button
    >
    <button class="btn btn-primary" type="submit" form={formId} disabled={busy || !ready}
      >{busy ? t.common.saving : t.projectsAdmin.changeKey}</button
    >
  {/snippet}
</Modal>

<style>
  .rename-form {
    display: grid;
    gap: 14px;
  }

  .body {
    margin: 0;
  }

  .mono {
    font-family: var(--font-mono);
    font-size: 13px;
    letter-spacing: 0.04em;
  }

  /* A line of explanation under a field, close to it. */
  .note {
    margin: -8px 0 0;
    font-size: 12.5px;
    line-height: 1.45;
  }

  .ok {
    color: var(--ink-soft);
  }

  .field-error {
    font-size: 12.5px;
    color: var(--accent-text);
  }

  .input[aria-invalid='true'] {
    border-color: var(--accent);
  }
</style>
