<script lang="ts">
  import {
    byOutcome,
    flowPeriods,
    type FlowReport,
    type Outcome,
    type SprintReport,
  } from '../../core/board-report.js';
  import type { BoardDetail, Sprint } from '../../core/board-types.js';
  import { paths } from '../../core/route.js';
  import { t } from '../../i18n/t.js';
  import { api, errorMessage } from '../../lib/api.js';
  import { router } from '../../lib/router.svelte.js';
  import BoardHeader from '../board/BoardHeader.svelte';
  import SprintHead from '../board/SprintHead.svelte';
  import BurndownChart from '../charts/BurndownChart.svelte';
  import FlowChart from '../charts/FlowChart.svelte';
  import Dropdown from '../Dropdown.svelte';
  import Illustration from '../Illustration.svelte';
  import Stamp from '../Stamp.svelte';
  import TypeMark from '../TypeMark.svelte';
  import NotFoundPage from './NotFoundPage.svelte';

  /**
   * A board's reports (?report=). A Scrum board's sprint report: one started
   * sprint (?sprint=id, else the active or the last one), its burndown, and
   * its issues by where they ended up. Any board's cumulative flow over the
   * last ?days= days.
   */
  interface Props {
    boardId: number;
  }
  const { boardId }: Props = $props();

  let detail = $state<BoardDetail | null>(null);
  let sprints = $state<Sprint[] | null>(null);
  let error = $state('');
  let report = $state<SprintReport | null>(null);
  let reportError = $state('');

  $effect(() => {
    Promise.all([api.board(boardId), api.reportSprints(boardId)])
      .then(([d, s]) => {
        detail = d;
        sprints = s;
      })
      .catch((e: unknown) => (error = errorMessage(e)));
  });

  const kind = $derived<'sprint' | 'flow'>(
    detail?.board.kind === 'scrum' && router.param('report') !== 'flow' ? 'sprint' : 'flow',
  );

  $effect(() => {
    if (detail) document.title = `${t.reports.kinds[kind]} — ${detail.board.name} — ${t.appName}`;
  });

  const period = $derived.by(() => {
    const n = Number(router.param('days'));
    return flowPeriods.find((p) => p === n) ?? 30;
  });

  let flow = $state<{ days: number; report: FlowReport } | null>(null);
  let flowError = $state('');
  let flowSeq = 0;
  $effect(() => {
    if (!detail || kind !== 'flow') return;
    const mine = ++flowSeq;
    const days = period;
    flowError = '';
    api
      .flowReport(boardId, days)
      .then((r) => {
        if (mine === flowSeq) flow = { days, report: r };
      })
      .catch((e: unknown) => {
        if (mine === flowSeq) flowError = errorMessage(e);
      });
  });
  const shownFlow = $derived(flow && flow.days === period ? flow.report : null);

  const chosen = $derived.by(() => {
    if (!sprints?.length) return null;
    const id = Number(router.param('sprint'));
    return sprints.find((s) => s.id === id) ?? sprints[0]!;
  });

  let seq = 0;
  $effect(() => {
    if (!chosen || kind !== 'sprint') return;
    const mine = ++seq;
    reportError = '';
    api
      .sprintReport(boardId, chosen.id)
      .then((r) => {
        if (mine === seq) report = r;
      })
      .catch((e: unknown) => {
        if (mine === seq) reportError = errorMessage(e);
      });
  });

  const shown = $derived(report && chosen && report.sprint.id === chosen.id ? report : null);
  const groups = $derived(shown ? byOutcome(shown) : null);
  const order: Outcome[] = ['completed', 'notCompleted', 'removed'];

  function work(n: number, r: SprintReport): string {
    return r.estimate === '' ? t.boards.cards(n) : t.boards.estimateValue(n, r.estimate);
  }

  function estimate(n: number | null, r: SprintReport): string {
    return n === null ? t.reports.unestimated : t.boards.estimateValue(n, r.estimate);
  }

  function groupTitle(o: Outcome, r: SprintReport): string {
    return o === 'notCompleted' && r.sprint.state === 'active'
      ? t.reports.notCompletedYet
      : t.reports.groups[o];
  }
</script>

{#if error}
  <NotFoundPage message={error} />
{:else if !detail || !sprints}
  <div class="page"><p class="muted loading">{t.common.loading}</p></div>
{:else}
  {@const d = detail}
  <div class="page reports-page" data-title={d.board.name}>
    <BoardHeader detail={d} view="reports" />

    <div class="pickers">
      {#if d.board.kind === 'scrum'}
        <div class="picker">
          <Dropdown
            label={t.reports.report}
            value={kind}
            options={(['sprint', 'flow'] as const).map((k) => ({
              value: k,
              label: t.reports.kinds[k],
            }))}
            onchange={(v) => router.setParams({ report: v === 'flow' ? v : null })}
          />
        </div>
      {/if}
      {#if kind === 'flow'}
        <div class="picker">
          <Dropdown
            label={t.reports.period}
            value={String(period)}
            options={flowPeriods.map((n) => ({ value: String(n), label: t.reports.lastDays(n) }))}
            onchange={(v) => router.setParams({ days: v === '30' ? null : v })}
          />
        </div>
      {:else if chosen}
        <div class="picker">
          <Dropdown
            label={t.boards.sprint}
            value={String(chosen.id)}
            options={sprints.map((s) => ({
              value: String(s.id),
              label: `${s.name} (${t.boards.sprintStates[s.state]})`,
            }))}
            onchange={(v) => router.setParams({ sprint: v })}
          />
        </div>
      {/if}
    </div>

    {#if kind === 'flow'}
      {#if flowError}
        <p class="error-text" role="alert">{flowError}</p>
      {:else if !shownFlow}
        <p class="muted loading">{t.common.loading}</p>
      {:else}
        <p class="notice">{t.reports.flowNote}</p>
        <FlowChart report={shownFlow} board={d.board.name} />
      {/if}
    {:else if !chosen}
      <Illustration kind="empty" />
      <p class="empty">{t.reports.none} {t.reports.noneHint}</p>
      <p><a class="btn" href={paths.backlog(d.board.id)}>{t.boards.openBacklog}</a></p>
    {:else if reportError}
      <p class="error-text" role="alert">{reportError}</p>
    {:else if !shown || !groups}
      <p class="muted loading">{t.common.loading}</p>
    {:else}
      {@const r = shown}
      <SprintHead
        title={r.sprint.name}
        sprint={r.sprint}
        meta={r.sprint.state === 'active'
          ? t.reports.untilNow
          : t.boards.dates(t.boards.day(r.sprint.start ?? r.end), t.boards.day(r.end))}
      />
      {#if r.estimate === ''}<p class="notice">{t.reports.counted}</p>{/if}

      <dl class="totals">
        {#each ['committed', 'added', 'removed', 'completed', 'notCompleted'] as const as k (k)}
          <div class="total">
            <dt>
              {k === 'committed'
                ? t.reports.committed
                : k === 'added'
                  ? t.reports.added
                  : groupTitle(k, r)}
            </dt>
            <dd>{work(r.totals[k], r)}</dd>
          </div>
        {/each}
      </dl>

      <section class="report-section" aria-labelledby="burndown-title">
        <h2 class="section-title" id="burndown-title">{t.reports.burndown}</h2>
        <BurndownChart report={r} />
      </section>

      {#each order as o (o)}
        {@const list = groups[o]}
        <section class="report-section" aria-labelledby="group-{o}">
          <h2 class="section-title" id="group-{o}">
            {groupTitle(o, r)} <span class="count">{t.boards.sectionCount(list.length)}</span>
          </h2>
          {#if list.length === 0}
            <p class="muted">{t.reports.emptyGroup}</p>
          {:else}
            <ul class="report-issues">
              {#each list as is (is.key)}
                <li class="report-issue">
                  <TypeMark type={is.type} />
                  <a class="key" href={paths.issue(is.key)}>{is.key}</a>
                  <span class="summary">{is.summary}</span>
                  {#if !is.committed}<span class="added">{t.reports.addedMark}</span>{/if}
                  <Stamp status={is.status} />
                  <span class="estimate">
                    {#if is.startEstimate !== is.endEstimate}
                      {t.reports.changed(
                        estimate(is.startEstimate, r),
                        estimate(is.endEstimate, r),
                      )}
                    {:else}
                      {estimate(is.endEstimate, r)}
                    {/if}
                  </span>
                </li>
              {/each}
            </ul>
          {/if}
        </section>
      {/each}
    {/if}
  </div>
{/if}

<style>
  .reports-page {
    max-width: 1100px;
  }

  .pickers {
    display: flex;
    flex-wrap: wrap;
    gap: 8px 16px;
    margin: 0 0 16px;
  }

  .picker {
    flex: 1 1 200px;
    max-width: 360px;
  }

  .notice {
    margin: 0 0 12px;
    color: var(--ink-soft);
    font-size: 13px;
  }

  .totals {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(130px, 1fr));
    gap: 8px;
    margin: 16px 0 24px;
  }

  .total {
    padding: 10px 12px;
    border: var(--frame) solid var(--rule);
    border-radius: var(--radius-s);
    background: var(--paper-raised);
  }

  .total dt {
    color: var(--ink-soft);
    font-size: 12px;
  }

  .total dd {
    margin: 4px 0 0;
    color: var(--ink);
    font-size: 18px;
    font-variant-numeric: tabular-nums;
  }

  .report-section {
    margin: 0 0 24px;
  }

  .section-title .count {
    color: var(--muted);
    font-size: 13px;
    font-weight: normal;
  }

  .report-issues {
    margin: 0;
    padding: 0;
    list-style: none;
  }

  .report-issue {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 4px 8px;
    padding: 6px 0;
    border-bottom: 1px solid var(--rule);
  }

  .report-issue .summary {
    flex: 1 1 200px;
    min-width: 0;
    overflow-wrap: anywhere;
  }

  .added {
    color: var(--accent-text);
    font-size: 12px;
  }

  .estimate {
    min-width: 4em;
    margin-left: auto;
    color: var(--ink-soft);
    font-size: 13px;
    font-variant-numeric: tabular-nums;
    text-align: right;
  }
</style>
