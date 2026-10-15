<script lang="ts">
  import type { SetupMessages } from '../../i18n/setup/index.js';
  import type { ConfigSave, SetupFinished, SetupStatus } from '../../lib/setup-api.js';

  interface Props {
    m: SetupMessages;
    status: SetupStatus;
    finished: SetupFinished | null;
    existing: boolean;
    config: ConfigSave | null;
  }
  const { m, status, finished, existing, config }: Props = $props();

  const server = $derived(finished?.server || status.database?.server || '');
</script>

<div class="setup-body">
  <div>
    <h1 class="page-title" id="setup-step-title" tabindex="-1">{m.done.title}</h1>
    <p class="lead">{existing ? m.done.existingLead : m.done.lead}</p>
  </div>

  {#if finished}
    <dl class="setup-facts" aria-label={m.done.summary}>
      <dt>{m.done.admin}</dt>
      <dd><span class="key">{finished.username}</span></dd>
      {#if finished.baseURL}
        <dt>{m.done.baseURL}</dt>
        <dd class="mono">{finished.baseURL}</dd>
      {/if}
      {#if finished.tz}
        <dt>{m.done.tz}</dt>
        <dd>{finished.tz}</dd>
      {/if}
      {#if server}
        <dt>{m.done.database}</dt>
        <dd>{server}</dd>
      {/if}
      <dt>{m.done.data}</dt>
      <dd>{finished.imported ? m.done.imported : m.done.fresh}</dd>
      {#if config}
        <dt>{m.done.config}</dt>
        <dd class="mono">{config.written ? config.path : m.done.configToComplete(config.path)}</dd>
      {/if}
    </dl>
  {/if}

  <div class="setup-actions">
    <span></span>
    <a class="btn btn-primary" href="/login">{m.done.signIn}</a>
  </div>
</div>
