<script lang="ts">
  import type { SetupMessages } from '../../i18n/setup/index.js';
  import type { SetupFinished, SetupStatus } from '../../lib/setup-api.js';
  import AdminForm from './AdminForm.svelte';
  import ImportPanel from './ImportPanel.svelte';

  /** Where the instance starts from: nothing (an administrator to create) or a Jira. */
  interface Props {
    m: SetupMessages;
    num: (n: number) => string;
    status: SetupStatus;
    ondone: (result: SetupFinished) => void;
    onback: () => void;
  }
  const { m, num, status, ondone, onback }: Props = $props();

  // svelte-ignore state_referenced_locally
  let mode = $state<'choose' | 'new' | 'import'>(status.import ? 'import' : 'choose');
</script>

{#if mode === 'new'}
  <AdminForm {m} afterImport={false} {ondone} onback={() => (mode = 'choose')} />
{:else if mode === 'import'}
  <ImportPanel {m} {num} {status} {ondone} onback={() => (mode = 'choose')} />
{:else}
  <div class="setup-body">
    <div>
      <h1 class="page-title" id="setup-step-title" tabindex="-1">{m.start.title}</h1>
      <p class="lead">{m.start.lead}</p>
    </div>
    <div class="choices" role="group" aria-label={m.start.choices}>
      <button class="choice" type="button" onclick={() => (mode = 'new')}>
        <span class="choice-title">{m.start.newTitle}</span>
        <span class="choice-text">{m.start.newText}</span>
      </button>
      <button class="choice" type="button" onclick={() => (mode = 'import')}>
        <span class="choice-title">{m.start.importTitle}</span>
        <span class="choice-text">{m.start.importText}</span>
      </button>
    </div>
    <div class="setup-actions">
      <button class="btn btn-quiet" type="button" onclick={onback}>{m.back}</button>
    </div>
  </div>
{/if}

<style>
  .choices {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
    gap: 12px;
  }

  .choice {
    display: grid;
    align-content: start;
    gap: 6px;
    padding: 16px 18px;
    text-align: left;
    background: var(--paper-raised);
    border: var(--frame) solid var(--rule-strong);
    border-radius: var(--radius-m);
    cursor: pointer;
  }

  .choice:hover {
    border-color: var(--accent);
    background: var(--accent-wash);
  }

  .choice-title {
    font: var(--title-weight) 18px/1.2 var(--font-title);
    color: var(--ink);
  }

  .choice-text {
    color: var(--ink-soft);
    line-height: 1.45;
  }
</style>
