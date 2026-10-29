<script lang="ts">
  import {
    burndownLine,
    dayTicks,
    guideline,
    scale,
    timeDomain,
    valueTicks,
    type SprintReport,
  } from '../../core/board-report.js';
  import { t } from '../../i18n/t.js';

  /**
   * A sprint's burndown: the remaining work as a staircase over the sprint,
   * against the guideline from the work at start to nothing on the last day.
   * Hand-drawn SVG in the skin's colours; a table gives the same data to
   * screen readers.
   */
  interface Props {
    report: SprintReport;
  }
  const { report }: Props = $props();

  const HEIGHT = 240;
  const PAD = { top: 12, right: 16, bottom: 28, left: 44 };

  let width = $state(600);

  const line = $derived(burndownLine(report));
  const guide = $derived(guideline(report));
  const domain = $derived(timeDomain(report));
  const ticks = $derived(valueTicks(Math.max(report.startRemaining, ...line.map((p) => p.v))));
  const x = $derived(scale(domain, [PAD.left, Math.max(PAD.left + 1, width - PAD.right)]));
  const y = $derived(scale([0, ticks[ticks.length - 1] ?? 1], [HEIGHT - PAD.bottom, PAD.top]));
  const days = $derived(dayTicks(domain[0], domain[1], Math.max(2, Math.floor(width / 72))));
  const path = (pts: { t: number; v: number }[]) =>
    pts.map((p, i) => `${i ? 'L' : 'M'}${x(p.t).toFixed(1)},${y(p.v).toFixed(1)}`).join('');
  const value = (n: number) =>
    report.estimate === '' ? String(n) : t.boards.estimateValue(n, report.estimate);
  const now = $derived(report.sprint.state === 'active' ? Date.parse(report.end) : null);
</script>

<figure class="burndown" bind:clientWidth={width}>
  <svg
    {width}
    height={HEIGHT}
    viewBox="0 0 {width} {HEIGHT}"
    role="img"
    aria-label={t.reports.burndownLabel(report.sprint.name)}
  >
    {#each ticks as v (v)}
      <line class="grid" x1={PAD.left} x2={width - PAD.right} y1={y(v)} y2={y(v)} />
      <text class="tick" x={PAD.left - 6} y={y(v)} text-anchor="end" dominant-baseline="middle"
        >{v}</text
      >
    {/each}
    {#each days as d (d)}
      <line class="day" x1={x(d)} x2={x(d)} y1={HEIGHT - PAD.bottom} y2={HEIGHT - PAD.bottom + 4} />
      <text class="tick" x={x(d)} y={HEIGHT - PAD.bottom + 16} text-anchor="middle"
        >{t.boards.day(new Date(d).toISOString())}</text
      >
    {/each}
    <line
      class="axis"
      x1={PAD.left}
      x2={width - PAD.right}
      y1={HEIGHT - PAD.bottom}
      y2={HEIGHT - PAD.bottom}
    />
    {#if now !== null}
      <line class="now" x1={x(now)} x2={x(now)} y1={PAD.top} y2={HEIGHT - PAD.bottom} />
    {/if}
    <path class="guide" d={path(guide)} />
    <path class="remaining" d={path(line)} />
    {#each report.steps as s, i (i)}
      <circle class="step" cx={x(Date.parse(s.at))} cy={y(s.remaining)} r="3"
        ><title>{t.reports.stepTitle(s.issue, value(s.remaining))}</title></circle
      >
    {/each}
  </svg>
  <figcaption class="legend">
    <span class="key remaining-key">{t.reports.remaining}</span>
    <span class="key guide-key">{t.reports.guideline}</span>
    {#if report.estimate === 'time'}<span class="unit">{t.reports.inHours}</span>{/if}
  </figcaption>
  <!-- A table ignores the 1px width of .sr-only; its box doesn't. -->
  <div class="sr-only">
    <table>
      <caption>{t.reports.burndownLabel(report.sprint.name)}</caption>
      <thead>
        <tr
          ><th scope="col">{t.reports.when}</th><th scope="col">{t.reports.issue}</th><th
            scope="col">{t.reports.remaining}</th
          ></tr
        >
      </thead>
      <tbody>
        <tr><td>{t.reports.atStart}</td><td></td><td>{value(report.startRemaining)}</td></tr>
        {#each report.steps as s, i (i)}
          <tr><td>{t.boards.day(s.at)}</td><td>{s.issue}</td><td>{value(s.remaining)}</td></tr>
        {/each}
      </tbody>
    </table>
  </div>
</figure>

<style>
  .burndown {
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

  .axis,
  .day {
    stroke: var(--rule-strong);
    stroke-width: 1;
  }

  .tick {
    fill: var(--muted);
    font-family: var(--font-body);
    font-size: 0.6875rem;
  }

  .now {
    stroke: var(--rule-strong);
    stroke-dasharray: 2 3;
  }

  .guide {
    fill: none;
    stroke: var(--muted);
    stroke-width: 1.5;
    stroke-dasharray: 5 4;
  }

  .remaining {
    fill: none;
    stroke: var(--accent);
    stroke-width: 2;
    stroke-linejoin: round;
  }

  .step {
    fill: var(--paper-raised);
    stroke: var(--accent);
    stroke-width: 1.5;
  }

  .legend {
    display: flex;
    flex-wrap: wrap;
    gap: 4px 16px;
    margin-top: 8px;
    color: var(--ink-soft);
    font-size: 0.8125rem;
  }

  .key::before {
    content: '';
    display: inline-block;
    width: 18px;
    margin-right: 6px;
    vertical-align: middle;
    border-top: 2px solid var(--accent);
  }

  .guide-key::before {
    border-top: 2px dashed var(--muted);
  }

  .unit {
    color: var(--muted);
  }
</style>
