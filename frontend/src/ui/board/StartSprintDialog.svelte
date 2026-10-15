<script lang="ts">
  import { dayAfter, sprintEnd } from '../../core/board.js';
  import type { Sprint } from '../../core/board-types.js';
  import { t } from '../../i18n/t.js';
  import { api, errorMessage } from '../../lib/api.js';
  import { notify } from '../../lib/ui.svelte.js';
  import Modal from '../Modal.svelte';

  /** Starting a sprint: its name, goal and last day (two weeks by default). */
  interface Props {
    sprint: Sprint | null;
    onclose: () => void;
    onstarted: () => void;
  }
  const { sprint, onclose, onstarted }: Props = $props();

  let name = $state('');
  let goal = $state('');
  let end = $state(dayAfter(14));
  let error = $state('');
  let busy = $state(false);

  $effect(() => {
    if (!sprint) return;
    name = sprint.name;
    goal = sprint.goal ?? '';
    end = dayAfter(14);
    error = '';
  });

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    if (!sprint || busy) return;
    busy = true;
    error = '';
    try {
      const last = sprintEnd(end);
      await api.startSprint(sprint.id, {
        name: name.trim(),
        goal: goal.trim(),
        ...(last ? { end: last } : {}),
      });
      notify(t.boards.started(name.trim() || sprint.name));
      onstarted();
    } catch (err) {
      error = errorMessage(err);
    } finally {
      busy = false;
    }
  }
</script>

<Modal open={sprint !== null} title={t.boards.startTitle(sprint?.name ?? '')} {onclose}>
  <form id="start-sprint" class="form" onsubmit={submit}>
    <label class="field">
      <span>{t.boards.startName}</span>
      <input class="input" bind:value={name} required maxlength="255" />
    </label>
    <label class="field">
      <span>{t.boards.startGoal}</span>
      <textarea class="textarea goal" bind:value={goal} rows="3"></textarea>
    </label>
    <label class="field end">
      <span>{t.boards.startEnd}</span>
      <input class="input" type="date" bind:value={end} min={dayAfter(1)} required />
      <span class="hint">{t.boards.startEndHint}</span>
    </label>
    {#if error}<p class="error-text" role="alert">{error}</p>{/if}
  </form>
  {#snippet footer()}
    <button type="button" class="btn btn-quiet" onclick={onclose}>{t.common.cancel}</button>
    <button type="submit" class="btn btn-primary" form="start-sprint" disabled={busy}
      >{busy ? t.boards.starting : t.boards.startSprint}</button
    >
  {/snippet}
</Modal>

<style>
  .form {
    display: grid;
    gap: 12px;
  }

  .goal {
    font: inherit;
    min-height: 70px;
  }

  .end {
    max-width: 240px;
  }
</style>
