<script lang="ts">
  import { dayAfter, localDay, sprintEdit } from '../../core/board.js';
  import type { Sprint } from '../../core/board-types.js';
  import { t } from '../../i18n/t.js';
  import { api, errorMessage } from '../../lib/api.js';
  import { notify } from '../../lib/ui.svelte.js';
  import Modal from '../Modal.svelte';

  /**
   * Changing a sprint: its name, goal and dates. A sprint not started yet
   * may have no dates; one under way keeps both, and began no later than today.
   */
  interface Props {
    sprint: Sprint | null;
    onclose: () => void;
    onsaved: () => void;
  }
  const { sprint, onclose, onsaved }: Props = $props();

  let name = $state('');
  let goal = $state('');
  let start = $state('');
  let end = $state('');
  let error = $state('');
  let busy = $state(false);

  const running = $derived(sprint?.state === 'active');

  $effect(() => {
    if (!sprint) return;
    name = sprint.name;
    goal = sprint.goal ?? '';
    start = localDay(sprint.start);
    end = localDay(sprint.end);
    error = '';
  });

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    if (!sprint || busy) return;
    const edit = sprintEdit(sprint, { name, goal, start, end });
    if (!edit.ok) {
      error = t.boards.sprintErrors[edit.error];
      return;
    }
    if (Object.keys(edit.patch).length === 0) {
      onclose();
      return;
    }
    busy = true;
    error = '';
    try {
      await api.updateSprint(sprint.id, edit.patch);
      notify(t.boards.sprintUpdated(name.trim()));
      onsaved();
    } catch (err) {
      error = errorMessage(err);
    } finally {
      busy = false;
    }
  }
</script>

<Modal open={sprint !== null} title={t.boards.editTitle(sprint?.name ?? '')} {onclose}>
  <form id="edit-sprint" class="form" onsubmit={submit}>
    <label class="field">
      <span>{t.boards.startName}</span>
      <input class="input" bind:value={name} required maxlength="255" />
    </label>
    <label class="field">
      <span>{t.boards.startGoal}</span>
      <textarea class="textarea goal" bind:value={goal} rows="3"></textarea>
    </label>
    <div class="days">
      <label class="field">
        <span>{t.boards.editStart}</span>
        <input
          class="input"
          type="date"
          bind:value={start}
          max={running ? dayAfter(0) : undefined}
          required={running}
        />
      </label>
      <label class="field">
        <span>{t.boards.startEnd}</span>
        <input class="input" type="date" bind:value={end} required={running} />
      </label>
    </div>
    {#if !running}<p class="hint">{t.boards.editDatesHint}</p>{/if}
    {#if error}<p class="error-text" role="alert">{error}</p>{/if}
  </form>
  {#snippet footer()}
    <button type="button" class="btn btn-quiet" onclick={onclose}>{t.common.cancel}</button>
    <button type="submit" class="btn btn-primary" form="edit-sprint" disabled={busy}
      >{busy ? t.common.saving : t.common.save}</button
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

  .days {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 240px));
    gap: 12px;
  }

  .hint {
    margin: 0;
    color: var(--muted);
    font-size: 13px;
  }

  @media (max-width: 640px) {
    .days {
      grid-template-columns: 1fr;
    }
  }
</style>
