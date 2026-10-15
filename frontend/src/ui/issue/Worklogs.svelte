<script lang="ts">
  import { parseDuration, todayISO } from '../../core/dates.js';
  import type { IssueDetail, Worklog } from '../../core/types.js';
  import { formatDateTime, formatDay, formatDuration, t } from '../../i18n/fr.js';
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

  const total = $derived((logs ?? []).reduce((s, w) => s + w.seconds, 0));

  $effect(() => {
    void tick;
    const key = issue.key;
    api
      .worklogs(key)
      .then((l) => {
        if (key === issue.key) logs = l;
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
      const started = new Date(`${day}T12:00:00`).toISOString();
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
          <tr>
            <td class="c-day" title={formatDateTime(w.started)}
              >{formatDay(todayISO(new Date(w.started)))}</td
            >
            <td class="c-who"><Who user={w.author} empty="—" /></td>
            <td class="c-body"
              >{#if w.body}<WikiText source={w.body} />{/if}</td
            >
            <td class="num c-dur">{formatDuration(w.seconds)}</td>
          </tr>
        {/each}
      </tbody>
      <tfoot>
        <tr>
          <td colspan="4" class="num total">{t.worklogs.total(formatDuration(total))}</td>
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
