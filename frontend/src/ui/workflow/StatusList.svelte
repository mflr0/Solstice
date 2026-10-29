<script lang="ts">
  import type { StatusRef } from '../../core/types.js';
  import { moveStatus } from '../../core/workflow.js';
  import { categoryLabels, t } from '../../i18n/t.js';
  import Stamp from '../Stamp.svelte';
  import StatusPicker from './StatusPicker.svelte';

  /**
   * The workflow's statuses in order, with how many issues sit in each.
   * Editable: reorder (drag, or the arrow buttons), remove, add.
   */
  interface Props {
    projectKey: string;
    statuses: StatusRef[];
    initial: string;
    usage: Record<string, number>;
    catalogue: StatusRef[];
    readonly: boolean;
    onreorder: (statuses: StatusRef[]) => void;
    onremove: (id: string) => void;
    onadd: (s: StatusRef) => void;
  }
  const {
    projectKey,
    statuses,
    initial,
    usage,
    catalogue,
    readonly,
    onreorder,
    onremove,
    onadd,
  }: Props = $props();

  let dragged = $state<number | null>(null);
  let over = $state<number | null>(null);

  function move(from: number, to: number) {
    onreorder(moveStatus(statuses, from, to));
  }

  function drop(e: DragEvent, i: number) {
    e.preventDefault();
    if (dragged !== null) move(dragged, i);
    dragged = null;
    over = null;
  }
</script>

<ol class="statuses">
  {#each statuses as s, i (s.id)}
    <li
      class:is-over={over === i && dragged !== null && dragged !== i}
      class:is-dragging={dragged === i}
      draggable={!readonly}
      ondragstart={(e) => {
        dragged = i;
        e.dataTransfer?.setData('text/plain', s.name);
        if (e.dataTransfer) e.dataTransfer.effectAllowed = 'move';
      }}
      ondragover={(e) => {
        if (dragged === null) return;
        e.preventDefault();
        over = i;
      }}
      ondragleave={() => over === i && (over = null)}
      ondrop={(e) => drop(e, i)}
      ondragend={() => ((dragged = null), (over = null))}
    >
      {#if !readonly}<span class="grip" aria-hidden="true"></span>{/if}
      <span class="stamp-cell"><Stamp status={s} /></span>
      <span class="meta">
        <span class="cat">{categoryLabels[s.category]}</span>
        <span class="num">{t.workflows.issues(usage[s.id] ?? 0)}</span>
        {#if s.id === initial}<span class="chip start">{t.workflows.start}</span>{/if}
      </span>
      {#if !readonly}
        <span class="tools">
          <button
            type="button"
            class="btn btn-quiet btn-small"
            aria-label={t.workflows.moveUp(s.name)}
            title={t.workflows.moveUp(s.name)}
            disabled={i === 0}
            onclick={() => move(i, i - 1)}>↑</button
          >
          <button
            type="button"
            class="btn btn-quiet btn-small"
            aria-label={t.workflows.moveDown(s.name)}
            title={t.workflows.moveDown(s.name)}
            disabled={i === statuses.length - 1}
            onclick={() => move(i, i + 1)}>↓</button
          >
          <button
            type="button"
            class="btn btn-quiet btn-small remove"
            aria-label={t.workflows.removeStatus(s.name)}
            title={statuses.length === 1
              ? t.workflows.lastStatus
              : t.workflows.removeStatus(s.name)}
            disabled={statuses.length === 1}
            onclick={() => onremove(s.id)}>×</button
          >
        </span>
      {/if}
    </li>
  {/each}
</ol>

{#if !readonly}
  <div class="foot">
    <StatusPicker {projectKey} {catalogue} taken={statuses} onpick={onadd} />
    <span class="hint">{t.workflows.reorderHint}</span>
  </div>
{/if}

<style>
  .statuses {
    list-style: none;
    margin: 0;
    padding: 0;
    border-top: 1px solid var(--rule);
  }

  li {
    display: flex;
    align-items: center;
    gap: 10px;
    min-height: 2.5rem;
    padding: 5px 2px;
    border-bottom: 1px solid var(--rule);
    background: var(--paper);
  }

  li[draggable='true'] {
    cursor: grab;
  }

  li.is-dragging {
    opacity: 0.5;
  }

  li.is-over {
    box-shadow: inset 0 2px 0 var(--accent);
  }

  .grip {
    flex: none;
    width: 8px;
    height: 14px;
    color: var(--muted);
    background: radial-gradient(circle, currentColor 1px, transparent 1.4px) 0 0 / 4px 4px;
  }

  .stamp-cell {
    flex: none;
    min-width: 0;
  }

  .meta {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 4px 10px;
    min-width: 0;
    font-size: 0.78125rem;
    color: var(--muted);
  }

  .num {
    font-family: var(--font-mono);
    font-size: 0.75rem;
    font-variant-numeric: tabular-nums;
  }

  .start {
    font-family: var(--font-body);
    font-size: 0.71875rem;
    color: var(--ink-soft);
    border-color: var(--rule-strong);
  }

  .tools {
    display: flex;
    gap: 2px;
    margin-left: auto;
    flex: none;
  }

  .tools .btn {
    min-width: 1.625rem;
    justify-content: center;
  }

  .remove:hover:not([disabled]) {
    color: var(--accent-text);
  }

  .foot {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    gap: 6px 16px;
    padding-top: 10px;
  }

  .hint {
    font-size: 0.75rem;
    color: var(--muted);
  }
</style>
