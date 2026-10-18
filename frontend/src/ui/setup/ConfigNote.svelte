<script lang="ts">
  import type { SetupMessages } from '../../i18n/setup/index.js';
  import type { ConfigSave } from '../../lib/setup-api.js';

  /** The configuration file could not be written: the lines to add by hand. */
  interface Props {
    m: SetupMessages;
    config: ConfigSave;
  }
  const { m, config }: Props = $props();

  let copied = $state(false);

  async function copy() {
    try {
      await navigator.clipboard.writeText(config.lines ?? '');
      copied = true;
      setTimeout(() => (copied = false), 2000);
    } catch {
      // No clipboard (plain http): the lines stay selectable.
    }
  }
</script>

{#if config.written}
  <p class="setup-ok" role="status">{m.config.written(config.path)}</p>
{:else}
  <div class="setup-warn" role="alert">
    <p>
      {m.config.notWritten(config.path)}
      {#if config.lines?.includes('SOLSTICE_DB_URL=')}{m.config.passwordNote}{/if}
    </p>
    <pre class="setup-code">{config.lines}</pre>
    <div class="row">
      <button class="btn btn-small" type="button" onclick={copy}
        >{copied ? m.copied : m.copy}</button
      >
      {#if config.error}<span class="muted mono">{m.config.reason(config.error)}</span>{/if}
    </div>
  </div>
{/if}

<style>
  .setup-warn {
    display: grid;
    gap: 8px;
  }

  .setup-warn p {
    margin: 0;
  }
</style>
