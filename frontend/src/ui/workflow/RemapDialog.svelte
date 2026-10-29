<script lang="ts">
  import type { StatusRef, StrandedDefault, StrandedStatus } from '../../core/types.js';
  import { t } from '../../i18n/t.js';
  import Dropdown from '../Dropdown.svelte';
  import Modal from '../Modal.svelte';
  import Stamp from '../Stamp.svelte';

  /**
   * Issues sit in statuses the workflow is about to lose (stranded), or the
   * default workflow lacks for the issue types going back to it
   * (strandedDefault): for each such status, where its issues go. Confirming
   * sends the choices.
   */
  interface Props {
    stranded: StrandedStatus[] | null;
    strandedDefault?: StrandedDefault | null;
    message: string;
    statuses: StatusRef[];
    choice: Record<string, string>;
    choiceDefault?: Record<string, string>;
    busy: boolean;
    confirmLabel?: string;
    onchoose: (from: string, to: string) => void;
    onchooseDefault?: (from: string, to: string) => void;
    onconfirm: () => void;
    onclose: () => void;
  }
  const {
    stranded,
    strandedDefault = null,
    message,
    statuses,
    choice,
    choiceDefault = {},
    busy,
    confirmLabel = t.workflows.remapConfirm,
    onchoose,
    onchooseDefault,
    onconfirm,
    onclose,
  }: Props = $props();
</script>

{#snippet moves(
  list: StrandedStatus[],
  targets: StatusRef[],
  chosen: Record<string, string>,
  pick: (from: string, to: string) => void,
)}
  <table class="ledger moves">
    <thead>
      <tr>
        <th scope="col">{t.workflows.statuses}</th>
        <th scope="col" class="num">{t.workflows.issuesCol}</th>
        <th scope="col">{t.workflows.moveTo}</th>
      </tr>
    </thead>
    <tbody>
      {#each list as s (s.status.id)}
        <tr>
          <th scope="row"><Stamp status={s.status} /></th>
          <td class="num">{t.workflows.issues(s.count)}</td>
          <td>
            <Dropdown
              label={`${t.workflows.moveTo} (${s.status.name})`}
              value={chosen[s.status.id] ?? ''}
              options={targets.map((st) => ({ value: st.id, label: st.name }))}
              onchange={(v) => pick(s.status.id, v)}
            />
          </td>
        </tr>
      {/each}
    </tbody>
  </table>
{/snippet}

<Modal
  open={stranded !== null || strandedDefault !== null}
  title={t.workflows.remapTitle}
  {onclose}
>
  <p class="lead">{message}</p>
  {#if stranded && stranded.length > 0}
    {@render moves(stranded, statuses, choice, onchoose)}
  {/if}
  {#if strandedDefault}
    <h3 class="group">{t.workflows.remapDefault(strandedDefault.workflow.name)}</h3>
    {@render moves(
      strandedDefault.stranded,
      strandedDefault.workflow.statuses,
      choiceDefault,
      onchooseDefault ?? (() => {}),
    )}
  {/if}
  <p class="hint">{t.workflows.remapNote}</p>
  {#snippet footer()}
    <button class="btn btn-quiet" onclick={onclose}>{t.common.cancel}</button>
    <button class="btn btn-primary" disabled={busy} onclick={onconfirm}
      >{busy ? t.common.saving : confirmLabel}</button
    >
  {/snippet}
</Modal>

<style>
  .lead {
    margin: 0;
  }

  .group {
    margin: 6px 0 0;
    font: 600 0.8125rem/1.4 var(--font-body);
    color: var(--ink-soft);
  }

  .moves th[scope='row'] {
    padding: 8px 10px;
    text-align: left;
    border-bottom: 1px solid var(--rule);
  }

  .moves tbody tr:hover td {
    background: none;
  }

  .moves :global(.select) {
    min-width: 10rem;
  }

  .hint {
    margin: 0;
    font-size: 0.78125rem;
    color: var(--muted);
  }
</style>
