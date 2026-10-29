<script lang="ts">
  import {
    dayTicks,
    flowBands,
    localDay,
    scale,
    valueTicks,
    type FlowReport,
  } from '../../core/board-report.js';
  import { t } from '../../i18n/t.js';

  /**
   * A board's cumulative flow: per day, its issues in each column as stacked
   * areas, the last column at the bottom. Hand-drawn SVG in the skin's
   * colours; a table gives the same data to screen readers.
   */
  interface Props {
    report: FlowReport;
    board: string;
  }
  const { report, board }: Props = $props();

  const HEIGHT = 260;
  const PAD = { top: 12, right: 16, bottom: 28, left: 44 };
  // First column: to do, last: done; the ones between take the other tones.
  const MIDDLE = ['--stamp-doing', '--tone-epic', '--tone-bug', '--tone-task', '--tone-subtask'];

  let width = $state(600);

  const bands = $derived(flowBands(report));
  const times = $derived(report.days.map((d) => localDay(d.date)));
  const domain = $derived<[number, number]>([
    times[0] ?? 0,
    Math.max(times[times.length - 1] ?? 1, (times[0] ?? 0) + 1),
  ]);
  const top = $derived(Math.max(0, ...(bands[0]?.hi ?? [])));
  const ticks = $derived(valueTicks(top));
  const x = $derived(scale(domain, [PAD.left, Math.max(PAD.left + 1, width - PAD.right)]));
  const y = $derived(scale([0, ticks[ticks.length - 1] ?? 1], [HEIGHT - PAD.bottom, PAD.top]));
  // dayTicks starts the day after its first argument.
  const days = $derived(dayTicks(domain[0] - 1, domain[1], Math.max(2, Math.floor(width / 72))));

  function colour(c: number): string {
    const last = report.columns.length - 1;
    if (c === last && last > 0) return 'var(--stamp-done)';
    if (c === 0) return 'var(--stamp-todo)';
    return `var(${MIDDLE[(c - 1) % MIDDLE.length]})`;
  }

  function area(lo: number[], hi: number[]): string {
    const up = hi.map((v, i) => `${x(times[i]!).toFixed(1)},${y(v).toFixed(1)}`);
    const down = lo.map((v, i) => `${x(times[i]!).toFixed(1)},${y(v).toFixed(1)}`).reverse();
    return `M${up.join('L')}L${down.join('L')}Z`;
  }
</script>

<figure class="flow" bind:clientWidth={width}>
  <svg
    {width}
    height={HEIGHT}
    viewBox="0 0 {width} {HEIGHT}"
    role="img"
    aria-label={t.reports.flowLabel(board)}
  >
    {#each ticks as v (v)}
      <line class="grid" x1={PAD.left} x2={width - PAD.right} y1={y(v)} y2={y(v)} />
      <text class="tick" x={PAD.left - 6} y={y(v)} text-anchor="end" dominant-baseline="middle"
        >{v}</text
      >
    {/each}
    {#each bands as b, c (report.columns[c]!.id)}
      <path class="band" d={area(b.lo, b.hi)} style:fill={colour(c)}
        ><title>{report.columns[c]!.name}</title></path
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
  </svg>
  <!-- A table ignores the 1px width of .sr-only; its box doesn't. -->
  <div class="sr-only">
    <table>
      <caption>{t.reports.flowLabel(board)}</caption>
      <thead>
        <tr>
          <th scope="col">{t.reports.day}</th>
          {#each report.columns as col (col.id)}<th scope="col">{col.name}</th>{/each}
        </tr>
      </thead>
      <tbody>
        {#each report.days as d (d.date)}
          <tr>
            <th scope="row">{t.boards.day(new Date(localDay(d.date)).toISOString())}</th>
            {#each d.counts as n, c (c)}<td>{n}</td>{/each}
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
  <figcaption class="legend">
    {#each report.columns as col, c (col.id)}
      <span class="key" style:--key={colour(c)}>{col.name}</span>
    {/each}
  </figcaption>
</figure>

<style>
  .flow {
    margin: 0;
    min-width: 0;
  }

  svg {
    display: block;
    overflow: visible;
  }

  .band {
    fill-opacity: 0.85;
    stroke: var(--paper-raised);
    stroke-width: 1;
    stroke-linejoin: round;
  }

  .grid {
    stroke: var(--rule);
    stroke-width: 1;
    stroke-opacity: 0.6;
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
    width: 12px;
    height: 12px;
    margin-right: 6px;
    vertical-align: -1px;
    border-radius: 2px;
    background: var(--key);
  }
</style>
