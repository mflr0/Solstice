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
  import { categoryLabels, t } from '../../i18n/fr.js';
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

  let lane: HTMLElement | undefined = $state();
  let boxes = $state.raw<Record<string, Box>>({});
  let size = $state.raw({ w: 0, h: 0 });
  const drawn = $derived(layout(order, boxes, edges, initial || null));

  function measure() {
    if (!lane) return;
    const base = lane.getBoundingClientRect();
    const next: Record<string, Box> = {};
    for (const el of lane.querySelectorAll<HTMLElement>('[data-node]')) {
      const r = el.getBoundingClientRect();
      next[el.dataset.node ?? ''] = {
        x: r.left - base.left,
        y: r.top - base.top,
        w: r.width,
        h: r.height,
      };
    }
    boxes = next;
    size = { w: lane.scrollWidth, h: lane.scrollHeight };
  }

  // Measure after each change of the row, and whenever a stamp resizes
  // (fonts arriving, another skin).
  $effect(() => {
    void order;
    void room;
    if (!lane) return;
    measure();
    const watch = new ResizeObserver(measure);
    watch.observe(lane);
    for (const el of lane.querySelectorAll('[data-node]')) watch.observe(el);
    return () => watch.disconnect();
  });

  function tip(from: string | null, to: string, names: string[]): string {
    const target = nameOf.get(to) ?? to;
    const source = from === null ? t.workflows.anyStatus : (nameOf.get(from) ?? from);
    return `${names.join(' · ')} : ${source} → ${target}`;
  }
</script>

<figure
  class="diagram"
  role="img"
  aria-label={t.workflows.diagramLabel(statuses.length, transitions.length)}
>
  <div class="lane" bind:this={lane}>
    <svg class="wires" width={size.w} height={size.h} viewBox="0 0 {size.w} {size.h}">
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
    overflow-x: auto;
    padding-bottom: 4px;
  }

  .lane {
    position: relative;
    display: inline-flex;
    min-width: 100%;
    padding: 0 8px;
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
    font: 500 11px/1.2 var(--font-body);
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
    left: 0;
    top: 0;
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
    font: italic 400 11.5px var(--font-body);
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
