<script lang="ts">
  import type { StatusRef } from '../../core/types.js';
  import { byCategory, effectiveName, type DraftTransition } from '../../core/workflow.js';
  import {
    diagramEdges,
    diagramOrder,
    layout,
    margins,
    type Box,
  } from '../../core/workflow-diagram.js';
  import { categoryLabels, t } from '../../i18n/t.js';
  import Stamp from '../Stamp.svelte';

  /**
   * The workflow drawn, read-only: the statuses as stamps on one row, by
   * category, with arrows for the transitions (geometry in
   * core/workflow-diagram.ts). The stamps are real ones, so every skin draws
   * them its way; the arrows are measured onto them.
   */
  interface Props {
    statuses: StatusRef[];
    transitions: DraftTransition[];
    initial: string;
  }
  const { statuses, transitions, initial }: Props = $props();

  const groups = $derived(byCategory(statuses));
  const order = $derived(diagramOrder(statuses).map((s) => s.id));
  const edges = $derived(
    diagramEdges(
      transitions.map((tr) => ({ from: tr.from, to: tr.to, name: effectiveName(tr, statuses) })),
      order,
    ),
  );
  const room = $derived(margins(order, edges));
  const category = $derived(new Map(statuses.map((s) => [s.id, s.category])));
  const nameOf = $derived(new Map(statuses.map((s) => [s.id, s.name])));

  let figure: HTMLElement | undefined = $state();
  let lane: HTMLElement | undefined = $state();
  let boxes = $state.raw<Record<string, Box>>({});
  // The lane's laid-out size, and the scale that fits its width to the figure.
  let fit = $state.raw({ w: 0, h: 0, scale: 1 });
  const drawn = $derived(layout(order, boxes, edges, initial || null));

  function measure() {
    if (!lane || !figure) return;
    const style = getComputedStyle(figure);
    const space =
      figure.getBoundingClientRect().width -
      parseFloat(style.paddingLeft) -
      parseFloat(style.paddingRight);
    const own = getComputedStyle(lane);
    const w = parseFloat(own.width);
    const h = parseFloat(own.height);
    fit = { w, h, scale: w > space && space > 0 ? space / w : 1 };
    // Rects come back scaled; the wires are drawn in the lane's own pixels.
    const base = lane.getBoundingClientRect();
    const k = base.width / w || 1;
    const next: Record<string, Box> = {};
    for (const el of lane.querySelectorAll<HTMLElement>('[data-node]')) {
      const r = el.getBoundingClientRect();
      next[el.dataset.node ?? ''] = {
        x: (r.left - base.left) / k,
        y: (r.top - base.top) / k,
        w: r.width / k,
        h: r.height / k,
      };
    }
    boxes = next;
  }

  // Measure after each change of the row, and whenever the figure or a stamp
  // resizes (fonts arriving, another skin, a narrower window).
  $effect(() => {
    void order;
    void room;
    if (!lane || !figure) return;
    measure();
    const watch = new ResizeObserver(measure);
    watch.observe(figure);
    watch.observe(lane);
    for (const el of lane.querySelectorAll('[data-node]')) watch.observe(el);
    return () => watch.disconnect();
  });

  function tip(from: string | null, to: string, names: string[]): string {
    const target = nameOf.get(to) ?? to;
    const source = from === null ? t.workflows.anyStatus : (nameOf.get(from) ?? from);
    return `${names.join(' · ')}${t.common.labelSep}${source} → ${target}`;
  }
</script>

<figure
  bind:this={figure}
  class="diagram"
  role="img"
  aria-label={t.workflows.diagramLabel(statuses.length, transitions.length)}
>
  <div
    class="lane"
    bind:this={lane}
    style:transform={fit.scale < 1 ? `scale(${fit.scale})` : null}
    style:margin-right={fit.scale < 1 ? `${fit.w * (fit.scale - 1)}px` : null}
    style:margin-bottom={fit.scale < 1 ? `${fit.h * (fit.scale - 1)}px` : null}
  >
    <svg class="wires">
      {#if drawn.bus}
        <g class="bus">
          <text x={drawn.bus.x1} y={drawn.bus.y - 6}>{t.workflows.anyStatus}</text>
          <path d="M{drawn.bus.x1},{drawn.bus.y}H{drawn.bus.x2}" />
        </g>
      {/if}
      {#each drawn.arcs as a (a.key)}
        <g class="wire {a.kind} to-{category.get(a.edge.to)}">
          <title>{tip(a.edge.from, a.edge.to, a.edge.names)}</title>
          <path class="hit" d={a.d} />
          <path class="line" d={a.d} />
          <polygon class="head" points={a.head} />
        </g>
      {/each}
      {#if drawn.start}
        <g class="start">
          <title>{t.workflows.initial}</title>
          <circle cx={drawn.start.cx} cy={drawn.start.cy} r="3.5" />
          <path d={drawn.start.d} />
          <polygon points={drawn.start.head} />
        </g>
      {/if}
    </svg>
    {#each groups as g (g.category)}
      <div class="group">
        <span class="caption">{categoryLabels[g.category]}</span>
        <ol class="nodes" style:padding-top="{room.above}px" style:padding-bottom="{room.below}px">
          {#each g.statuses as s (s.id)}
            <li data-node={s.id}><Stamp status={s} /></li>
          {/each}
        </ol>
      </div>
    {/each}
  </div>
</figure>

<style>
  .diagram {
    margin: 0;
    padding: 8px;
  }

  /* Scaled down from its top left corner when wider than the figure; the
     negative margins give back the room the scaling frees. */
  .lane {
    position: relative;
    display: flex;
    width: max-content;
    min-width: 100%;
    transform-origin: 0 0;
  }

  .group {
    display: flex;
    flex-direction: column;
    padding: 0 28px 0 34px;
  }

  .group + .group {
    border-left: 1px dashed var(--rule-strong);
  }

  .caption {
    padding-top: 2px;
    font: 500 0.6875rem/1.2 var(--font-body);
    text-transform: uppercase;
    letter-spacing: 0.12em;
    color: var(--muted);
    white-space: nowrap;
  }

  .nodes {
    display: flex;
    align-items: center;
    gap: 56px;
    margin: 0;
    padding-left: 0;
    padding-right: 0;
    list-style: none;
  }

  .nodes li {
    display: flex;
    position: relative;
    z-index: 1;
  }

  .wires {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    overflow: visible;
    pointer-events: none;
    fill: none;
  }

  .wire {
    --c: var(--stamp-todo);
    color: var(--c);
  }

  .wire.to-doing {
    --c: var(--stamp-doing);
  }

  .wire.to-done {
    --c: var(--stamp-done);
  }

  .line {
    stroke: currentColor;
    stroke-width: 1.4;
  }

  .any .line {
    stroke-dasharray: 2 3;
  }

  .head {
    fill: currentColor;
  }

  .hit {
    stroke: transparent;
    stroke-width: 10;
    pointer-events: stroke;
  }

  .wire:hover .line {
    stroke-width: 2.4;
  }

  .bus path {
    stroke: var(--muted);
    stroke-width: 1.2;
    stroke-dasharray: 2 3;
  }

  .bus text {
    fill: var(--muted);
    font: italic 400 0.71875rem var(--font-body);
  }

  .start {
    color: var(--ink);
    pointer-events: visiblePainted;
  }

  .start circle,
  .start polygon {
    fill: currentColor;
  }

  .start path {
    stroke: currentColor;
    stroke-width: 1.4;
  }
</style>
