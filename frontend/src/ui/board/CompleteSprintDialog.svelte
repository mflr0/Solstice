<script lang="ts">
  import type { Sprint } from '../../core/board-types.js';
  import { t } from '../../i18n/t.js';
  import { api, errorMessage } from '../../lib/api.js';
  import { notify } from '../../lib/ui.svelte.js';
  import Modal from '../Modal.svelte';

  /**
   * Completing the active sprint: how much got done, and where the
   * unfinished issues go (a future sprint, a new one made on the way, or
   * back to the backlog).
   */
  interface Props {
    boardId: number;
    sprint: Sprint | null;
    /** The board's sprints not yet started. */
    future: Sprint[];
    onclose: () => void;
    oncompleted: () => void;
  }
  const { boardId, sprint, future, onclose, oncompleted }: Props = $props();

  /** The choice that makes a sprint on the way. */
  const NEW_SPRINT = -1;

  let counts = $state<{ done: number; open: number } | null>(null);
  let target = $state(0);
  /** A sprint made for this completion that failed after it: used again, not made twice. */
  let made = 0;
  let error = $state('');
  let busy = $state(false);

  $effect(() => {
    const s = sprint;
    if (!s) return;
    counts = null;
    target = 0;
    made = 0;
    error = '';
    api
      .boardCards(boardId, { sprint: s.id })
      .then((res) => {
        if (sprint?.id !== s.id) return;
        const issues = res.cards.filter((c) => !c.type.subtask);
        const done = issues.filter((c) => c.status.category === 'done').length;
        counts = { done, open: issues.length - done };
      })
      .catch((e: unknown) => (error = errorMessage(e)));
  });

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    if (!sprint || busy) return;
    busy = true;
    error = '';
    try {
      if (target === NEW_SPRINT && made === 0) made = (await api.createSprint(boardId)).id;
      await api.completeSprint(sprint.id, (target === NEW_SPRINT ? made : target) || undefined);
      notify(t.boards.completed(sprint.name));
      oncompleted();
    } catch (err) {
      error = errorMessage(err);
    } finally {
      busy = false;
    }
  }
</script>

<Modal open={sprint !== null} title={t.boards.completeTitle(sprint?.name ?? '')} {onclose}>
  <form id="complete-sprint" class="form" onsubmit={submit}>
    {#if counts}
      <p class="summary">{t.boards.completeSummary(counts.done, counts.open)}</p>
    {:else if !error}
      <p class="muted loading">{t.common.loading}</p>
    {/if}
    {#if !counts || counts.open > 0}
      <label class="field target">
        <span>{t.boards.completeMoveTo}</span>
        <select class="select" bind:value={target}>
          <option value={0}>{t.boards.toBacklog}</option>
          <option value={NEW_SPRINT}>{t.boards.toNewSprint}</option>
          {#each future as f (f.id)}
            <option value={f.id}>{f.name}</option>
          {/each}
        </select>
      </label>
    {/if}
    {#if error}<p class="error-text" role="alert">{error}</p>{/if}
  </form>
  {#snippet footer()}
    <button type="button" class="btn btn-quiet" onclick={onclose}>{t.common.cancel}</button>
    <button type="submit" class="btn btn-primary" form="complete-sprint" disabled={busy}
      >{busy ? t.boards.completing : t.boards.completeSprint}</button
    >
  {/snippet}
</Modal>

<style>
  .form {
    display: grid;
    gap: 12px;
  }

  .summary {
    margin: 0;
  }

  .target {
    max-width: 360px;
  }
</style>
