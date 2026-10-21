<script lang="ts">
  import { isoDay, jiraDuration, parseDuration, todayISO } from '../../core/dates.js';
  import { worklogEdit, worklogStart } from '../../core/estimates.js';
  import type { IssueDetail, Worklog } from '../../core/types.js';
  import { formatDateTime, formatDay, formatDuration, t } from '../../i18n/t.js';
  import { api, errorMessage } from '../../lib/api.js';
  import { notify } from '../../lib/ui.svelte.js';
  import Who from '../Who.svelte';
  import WikiText from '../WikiText.svelte';

  /** Time spent on the issue, and a line to log more. */
  interface Props {
    issue: IssueDetail;
    tick: number;
    onlogged: () => void;
  }
  const { issue, tick, onlogged }: Props = $props();

  let logs = $state<Worklog[] | null>(null);
  let error = $state('');
  let duration = $state('');
  let day = $state(todayISO());
  let body = $state('');
  let invalid = $state(false);
  let sending = $state(false);
  let editing = $state<number | null>(null);
  let editDuration = $state('');
  let editDay = $state('');
  let editBody = $state('');
  let editInvalid = $state(false);
  let armed = $state<number | null>(null);

  const total = $derived((logs ?? []).reduce((s, w) => s + w.seconds, 0));

  $effect(() => {
    void tick;
    const key = issue.key;
    api
      .worklogs(key)
      .then((l) => {
        if (key === issue.key) {
          logs = l;
          error = '';
        }
      })
      .catch((e: unknown) => (error = errorMessage(e)));
  });

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    const seconds = parseDuration(duration);
    invalid = seconds === null;
    if (seconds === null) return;
    sending = true;
    try {
      // Noon local time: the day stays the same in every time zone nearby.
      const started = worklogStart(day);
      await api.addWorklog(issue.key, { started, seconds, body });
      duration = '';
      body = '';
      onlogged();
    } catch (err) {
      notify(errorMessage(err), 'error');
    } finally {
      sending = false;
    }
  }

  function startEdit(w: Worklog) {
    editing = w.id;
    editDuration = jiraDuration(w.seconds);
    editDay = isoDay(new Date(w.started));
    editBody = w.body ?? '';
    editInvalid = false;
    armed = null;
  }

  async function saveEdit(w: Worklog) {
    const edit = worklogEdit(w, { duration: editDuration, day: editDay, body: editBody });
    editInvalid = !edit.ok;
    if (!edit.ok) return;
    if (Object.keys(edit.patch).length === 0) {
      editing = null;
      return;
    }
    sending = true;
    try {
      const saved = await api.editWorklog(w.id, edit.patch);
      logs = (logs ?? []).map((x) => (x.id === w.id ? saved : x));
      editing = null;
      onlogged();
    } catch (err) {
      notify(errorMessage(err), 'error');
    } finally {
      sending = false;
    }
  }

  async function remove(w: Worklog) {
    if (armed !== w.id) {
      armed = w.id;
      return;
    }
    armed = null;
    try {
      await api.deleteWorklog(w.id);
      logs = (logs ?? []).filter((x) => x.id !== w.id);
      onlogged();
    } catch (err) {
      notify(errorMessage(err), 'error');
    }
  }
</script>

{#if error}
  <p class="error-text" role="alert">{error}</p>
{:else if !logs}
  <p class="muted loading">{t.common.loading}</p>
{:else}
  {#if logs.length === 0}
    <p class="empty">{t.worklogs.empty}</p>
  {:else}
    <table class="ledger">
      <tbody>
        {#each logs as w (w.id)}
          {#if editing === w.id}
            <tr>
              <td colspan="5" class="c-edit">
                <form
                  class="log"
                  onsubmit={(e) => {
                    e.preventDefault();
                    void saveEdit(w);
                  }}
                >
                  <label class="field dur">
                    <span>{t.worklogs.duration}</span>
                    <input
                      class="input"
                      bind:value={editDuration}
                      placeholder={t.worklogs.durationHint}
                      aria-invalid={editInvalid}
                      required
                    />
                  </label>
                  <label class="field">
                    <span>{t.worklogs.started}</span>
                    <input class="input" type="date" bind:value={editDay} required />
                  </label>
                  <label class="field grow">
                    <span>{t.worklogs.body}</span>
                    <input class="input" bind:value={editBody} />
                  </label>
                  <button type="button" class="btn btn-quiet" onclick={() => (editing = null)}
                    >{t.common.cancel}</button
                  >
                  <button class="btn btn-primary" disabled={sending}>{t.common.save}</button>
                </form>
                {#if editInvalid}<p class="error-text">{t.worklogs.invalid}</p>{/if}
              </td>
            </tr>
          {:else}
            <tr>
              <td class="c-day" title={formatDateTime(w.started)}
                >{formatDay(isoDay(new Date(w.started)))}</td
              >
              <td class="c-who"><Who user={w.author} empty="—" /></td>
              <td class="c-body"
                >{#if w.body}<WikiText source={w.body} />{/if}</td
              >
              <td class="num c-dur">{formatDuration(w.seconds)}</td>
              <td class="c-tools">
                {#if w.canEdit}
                  <span class="tools">
                    <button class="link-btn" onclick={() => startEdit(w)}>{t.common.edit}</button>
                    <button
                      class="link-btn"
                      class:armed={armed === w.id}
                      onclick={() => void remove(w)}
                      onblur={() => (armed = null)}
                      >{armed === w.id ? t.common.confirmDelete : t.common.delete}</button
                    >
                  </span>
                {/if}
              </td>
            </tr>
          {/if}
        {/each}
      </tbody>
      <tfoot>
        <tr>
          <td colspan="5" class="num total">{t.worklogs.total(formatDuration(total))}</td>
        </tr>
      </tfoot>
    </table>
  {/if}

  {#if issue.can.worklog}
    <form class="log" onsubmit={submit}>
      <label class="field dur">
        <span>{t.worklogs.duration}</span>
        <input
          class="input"
          bind:value={duration}
          placeholder={t.worklogs.durationHint}
          aria-invalid={invalid}
          required
        />
      </label>
      <label class="field">
        <span>{t.worklogs.started}</span>
        <input class="input" type="date" bind:value={day} required />
      </label>
      <label class="field grow">
        <span>{t.worklogs.body}</span>
        <input class="input" bind:value={body} />
      </label>
      <button class="btn btn-primary" disabled={sending}>{t.worklogs.submit}</button>
    </form>
    {#if invalid}<p class="error-text">{t.worklogs.invalid}</p>{/if}
  {/if}
{/if}

<style>
  .c-day {
    width: 130px;
    white-space: nowrap;
  }

  .c-who {
    width: 190px;
  }

  .c-body :global(.wiki) {
    font-size: 13.5px;
  }

  .c-body :global(.wiki p) {
    margin: 0;
  }

  .c-dur {
    width: 80px;
  }

  .c-tools {
    width: 1%;
    white-space: nowrap;
  }

  .tools {
    display: flex;
    gap: 12px;
    font-size: 12.5px;
  }

  .tools .armed {
    color: var(--accent-text);
  }

  .c-edit {
    padding-top: 4px;
  }

  .c-edit .log {
    margin-top: 6px;
  }

  .total {
    border-bottom: none;
    color: var(--ink-soft);
  }

  tfoot td {
    border-top: 1px solid var(--rule-strong);
  }

  .log {
    display: flex;
    gap: 10px;
    align-items: flex-end;
    margin-top: 14px;
    flex-wrap: wrap;
  }

  .dur {
    width: 170px;
  }

  .grow {
    flex: 1;
    min-width: 180px;
  }
</style>
