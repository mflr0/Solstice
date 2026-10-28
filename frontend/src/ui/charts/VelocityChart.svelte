<script lang="ts">
  import {
    averageCompleted,
    bands,
    scale,
    shortLabels,
    valueTicks,
    type VelocityReport,
  } from '../../core/board-report.js';
  import { t } from '../../i18n/t.js';

  /**
   * A board's velocity: per closed sprint, oldest first, the work committed
   * at start beside the work completed, against the average completed.
   * Hand-drawn SVG in the skin's colours; a table gives the same data to
   * screen readers.
   */
  interface Props {
    report: VelocityReport;
  }
  const { report }: Props = $props();

  const HEIGHT = 240;
  const PAD = { top: 12, right: 16, bottom: 28, left: 44 };

  let width = $state(600);

  const average = $derived(averageCompleted(report));
  const ticks = $derived(
    valueTicks(Math.max(0, ...report.sprints.flatMap((s) => [s.committed, s.completed]))),
  );
  const y = $derived(scale([0, ticks[ticks.length - 1] ?? 1], [HEIGHT - PAD.bottom, PAD.top]));
  const slots = $derived(
    bands(report.sprints.length, [PAD.left, Math.max(PAD.left + 1, width - PAD.right)], 0.3, 160),
  );
  // About 6.5px a character at 11px.
  const maxChars = $derived(Math.max(3, Math.floor((slots[0]?.w ?? 0) / 6.5)));
  const labels = $derived(
    shortLabels(
      report.sprints.map((s) => s.sprint.name),
      maxChars,
    ),
  );
  const value = (n: number) =>
    report.estimate === '' ? String(n) : t.boards.estimateValue(n, report.estimate);
  const bar = (v: number) => ({ y: y(v), h: Math.max(0, y(0) - y(v)) });
</script>

<figure class="velocity" bind:clientWidth={width}>
  <svg
    {width}
    height={HEIGHT}
    viewBox="0 0 {width} {HEIGHT}"
    role="img"
    aria-label={t.reports.velocityLabel}
  >
    {#each ticks as v (v)}
      <line class="grid" x1={PAD.left} x2={width - PAD.right} y1={y(v)} y2={y(v)} />
      <text class="tick" x={PAD.left - 6} y={y(v)} text-anchor="end" dominant-baseline="middle"
        >{v}</text
      >
    {/each}
    {#each report.sprints as s, i (s.sprint.id)}
      {@const slot = slots[i]!}
      {@const half = slot.w / 2}
      {@const c = bar(s.committed)}
      {@const d = bar(s.completed)}
      <g class="sprint">
        <title>{t.reports.barTitle(s.sprint.name, value(s.committed), value(s.completed))}</title>
        <rect class="committed" x={slot.x} y={c.y} width={half} height={c.h} />
        <rect class="completed" x={slot.x + half} y={d.y} width={half} height={d.h} />
        <text class="tick" x={slot.x + half} y={HEIGHT - PAD.bottom + 16} text-anchor="middle"
          >{labels[i]}</text
        >
      </g>
    {/each}
    <line
      class="axis"
      x1={PAD.left}
      x2={width - PAD.right}
      y1={HEIGHT - PAD.bottom}
      y2={HEIGHT - PAD.bottom}
    />
    {#if average !== null}
      <line class="average" x1={PAD.left} x2={width - PAD.right} y1={y(average)} y2={y(average)} />
    {/if}
  </svg>
  <!-- A table ignores the 1px width of .sr-only; its box doesn't. -->
  <div class="sr-only">
    <table>
      <caption>{t.reports.velocityLabel}</caption>
      <thead>
        <tr
          ><th scope="col">{t.boards.sprint}</th><th scope="col">{t.reports.committed}</th><th
            scope="col">{t.reports.groups.completed}</th
          ></tr
        >
      </thead>
      <tbody>
        {#each report.sprints as s (s.sprint.id)}
          <tr><td>{s.sprint.name}</td><td>{value(s.committed)}</td><td>{value(s.completed)}</td></tr
          >
        {/each}
      </tbody>
    </table>
  </div>
  <figcaption class="legend">
    <span class="key committed-key">{t.reports.committed}</span>
    <span class="key completed-key">{t.reports.groups.completed}</span>
    <span class="key average-key">{t.reports.average}</span>
    {#if report.estimate === 'time'}<span class="unit">{t.reports.inHours}</span>{/if}
  </figcaption>
</figure>

<style>
  .velocity {
    margin: 0;
    min-width: 0;
  }

  svg {
    display: block;
    overflow: visible;
  }

  .grid {
    stroke: var(--rule);
    stroke-width: 1;
  }

  .axis {
    stroke: var(--rule-strong);
    stroke-width: 1;
  }

  .tick {
    fill: var(--muted);
    font-family: var(--font-body);
    font-size: 11px;
  }

  .committed {
    fill: var(--rule-strong);
  }

  .completed {
    fill: var(--accent);
  }

  .average {
    stroke: var(--ink-soft);
    stroke-width: 1.5;
    stroke-dasharray: 5 4;
  }

  .legend {
    display: flex;
    flex-wrap: wrap;
    gap: 4px 16px;
    margin-top: 8px;
    color: var(--ink-soft);
    font-size: 13px;
  }

  .key::before {
    content: '';
    display: inline-block;
    width: 12px;
    height: 12px;
    margin-right: 6px;
    vertical-align: middle;
  }

  .committed-key::before {
    background: var(--rule-strong);
  }

  .completed-key::before {
    background: var(--accent);
  }

  .average-key::before {
    width: 18px;
    height: 0;
    border-top: 2px dashed var(--ink-soft);
  }

  .unit {
    color: var(--muted);
  }
</style>
