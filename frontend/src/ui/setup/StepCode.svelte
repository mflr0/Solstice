<script lang="ts">
  import { codeComplete, formatCode } from '../../core/setup.js';
  import type { SetupMessages } from '../../i18n/setup/index.js';
  import { setupApi, setupErrorMessage } from '../../lib/setup-api.js';

  interface Props {
    m: SetupMessages;
    onunlocked: () => Promise<void>;
    onback: () => void;
  }
  const { m, onunlocked, onback }: Props = $props();

  let code = $state('');
  let pending = $state(false);
  let error = $state('');

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    if (!codeComplete(code)) return;
    pending = true;
    error = '';
    try {
      await setupApi.unlock(code);
      await onunlocked();
    } catch (err) {
      error = setupErrorMessage(err, m.errors.generic);
    } finally {
      pending = false;
    }
  }
</script>

<form onsubmit={submit} novalidate>
  <div>
    <h1 class="page-title" id="setup-step-title" tabindex="-1">{m.code.title}</h1>
    <p class="lead">{m.code.lead}</p>
  </div>
  <label class="field code-field">
    <span>{m.code.label}</span>
    <input
      class="input code-input"
      bind:value={code}
      oninput={() => (code = formatCode(code))}
      autocomplete="one-time-code"
      autocapitalize="characters"
      spellcheck="false"
      maxlength="9"
      placeholder="XXXX-XXXX"
      aria-invalid={error ? 'true' : undefined}
      aria-describedby={error ? 'code-error' : 'code-where'}
    />
  </label>
  {#if error}<p id="code-error" class="error-text" role="alert">{error}</p>{/if}

  <details id="code-where">
    <summary>{m.code.whereTitle}</summary>
    <p>{m.code.whereLog}</p>
    <pre class="setup-code">journalctl -u solstice
docker compose logs solstice</pre>
    <p>{m.code.whereFile}</p>
    <pre class="setup-code">/var/lib/solstice/setup-code</pre>
    <p class="muted">{m.code.renewed}</p>
  </details>

  <div class="setup-actions">
    <button class="btn btn-quiet" type="button" onclick={onback}>{m.back}</button>
    <button class="btn btn-primary" disabled={pending || !codeComplete(code)}
      >{pending ? m.code.checking : m.code.submit}</button
    >
  </div>
</form>

<style>
  .code-field {
    max-width: 260px;
  }

  .code-input {
    font: 500 22px/1.2 var(--font-mono);
    letter-spacing: 0.12em;
    text-transform: uppercase;
    padding: 8px 12px;
  }
</style>
