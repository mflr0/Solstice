<script lang="ts">
  import { t } from '../i18n/t.js';

  /** A value to copy (a new access token): read-only, selected on focus, with a copy button. */
  interface Props {
    value: string;
    label: string;
  }
  const { value, label }: Props = $props();

  let copied = $state(false);
  let input: HTMLInputElement | undefined = $state();

  async function copy() {
    try {
      await navigator.clipboard.writeText(value);
    } catch {
      // No clipboard access (plain http): select it for Ctrl+C instead.
      input?.select();
      document.execCommand('copy');
    }
    copied = true;
    setTimeout(() => (copied = false), 2500);
  }
</script>

<div class="copy-field">
  <input
    bind:this={input}
    class="input key"
    readonly
    {value}
    aria-label={label}
    onfocus={(e) => e.currentTarget.select()}
  />
  <button type="button" class="btn btn-primary" onclick={() => void copy()}
    >{copied ? t.adminArea.copied : t.adminArea.copy}</button
  >
</div>

<style>
  .copy-field {
    display: flex;
    gap: 8px;
  }

  .copy-field .input {
    flex: 1;
    min-width: 0;
    font-family: var(--font-mono);
    font-size: 0.8125rem;
  }
</style>
