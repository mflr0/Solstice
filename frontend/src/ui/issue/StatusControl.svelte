<script lang="ts">
  import type { IssueDetail, Transition } from '../../core/types.js';
  import { t } from '../../i18n/t.js';
  import type { IssueFields } from '../../lib/api.js';
  import { outside } from '../../lib/actions.js';
  import { session } from '../../lib/session.svelte.js';
  import Stamp from '../Stamp.svelte';

  /**
   * The status stamp. When the workflow allows moves, pressing it lists them
   * by name, each with the status it leads to; a move to a "done" status
   * asks for the resolution first.
   */
  interface Props {
    issue: IssueDetail;
    onchange: (fields: IssueFields) => Promise<void>;
  }
  const { issue, onchange }: Props = $props();

  let open = $state(false);
  let pending = $state<Transition | null>(null);
  let resolution = $state('');
  let busy = $state(false);

  const movable = $derived(issue.can.transition === true && issue.transitions.length > 0);

  const defaultResolution = $derived(session.meta?.defaultResolution ?? '');

  function close() {
    open = false;
    pending = null;
  }

  async function apply(tr: Transition) {
    if (tr.to.category === 'done' && !pending) {
      pending = tr;
      resolution = defaultResolution;
      return;
    }
    busy = true;
    try {
      const fields: IssueFields = { status: tr.to.id };
      if (tr.to.category === 'done' && resolution) fields.resolution = resolution;
      await onchange(fields);
      close();
    } finally {
      busy = false;
    }
  }
</script>

<div class="status" use:outside={close}>
  {#if movable}
    <button
      class="trigger status-trigger"
      aria-haspopup="menu"
      aria-expanded={open}
      aria-label={`${t.issue.changeStatus} (${issue.status.name})`}
      onclick={() => (open ? close() : (open = true))}
    >
      <Stamp status={issue.status} large />
      <span class="caret" aria-hidden="true">▾</span>
    </button>
  {:else}
    <Stamp status={issue.status} large />
  {/if}

  {#if open}
    <div class="popover" role="menu" aria-label={t.issue.changeStatus}>
      {#if pending}
        <form
          class="resolve"
          onsubmit={(e) => {
            e.preventDefault();
            if (pending) void apply(pending);
          }}
        >
          <p class="what">{pending.name}</p>
          <label class="field">
            <span>{t.issue.chooseResolution}</span>
            <select class="select" bind:value={resolution}>
              {#if !defaultResolution}<option value="">{t.issue.leaveUnresolved}</option>{/if}
              {#each session.meta?.resolutions ?? [] as r (r.id)}
                <option value={r.id}>{r.name}</option>
              {/each}
            </select>
          </label>
          <div class="row">
            <button type="button" class="btn btn-quiet btn-small" onclick={() => (pending = null)}
              >{t.common.cancel}</button
            >
            <button class="btn btn-primary btn-small" disabled={busy}
              >{t.issue.confirmTransition}</button
            >
          </div>
        </form>
      {:else}
        <div class="group-label">{t.issue.changeStatus}</div>
        {#each issue.transitions as tr (tr.id)}
          <button class="item" role="menuitem" disabled={busy} onclick={() => void apply(tr)}>
            <span class="name">{tr.name}</span>
            <span class="to"><Stamp status={tr.to} /></span>
          </button>
        {/each}
      {/if}
    </div>
  {/if}
</div>

<style>
  .status {
    position: relative;
    display: inline-flex;
  }

  .trigger {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    border: none;
    background: none;
    padding: 4px 2px;
    cursor: pointer;
  }

  .caret {
    color: var(--muted);
    font-size: 11px;
  }

  .trigger:hover .caret {
    color: var(--accent-text);
  }

  .popover {
    left: 0;
    top: calc(100% + 6px);
    min-width: 280px;
  }

  .item {
    justify-content: space-between;
  }

  .to {
    margin-left: 16px;
  }

  .resolve {
    display: grid;
    gap: 10px;
    padding: 8px 12px;
  }

  .what {
    margin: 0;
    font-weight: 600;
  }

  .row {
    justify-content: flex-end;
  }
</style>
