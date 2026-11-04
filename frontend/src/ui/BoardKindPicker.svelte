<script lang="ts">
  import type { BoardKind } from '../core/board-types.js';
  import { t } from '../i18n/t.js';

  /** The board type as two radio cards, each with a picture of that board. */
  interface Props {
    value: BoardKind;
    onchange: (kind: BoardKind) => void;
    name?: string | undefined;
  }
  const { value, onchange, name = 'kind' }: Props = $props();
</script>

<fieldset class="board-kinds">
  <legend class="field-label">{t.boards.kind}</legend>
  {#each ['kanban', 'scrum'] as const as kind (kind)}
    <label class="board-kind" data-kind={kind}>
      <svg class="board-kind-picture" viewBox="0 0 160 96" aria-hidden="true">
        {#if kind === 'kanban'}
          {#each [6, 44, 82, 120] as x (x)}
            <rect class="pic-lane" {x} y="6" width="34" height="84" rx="3" />
            <rect class="pic-head" x={x + 5} y="11" width="16" height="4" rx="2" />
          {/each}
          <rect class="pic-card" x="10" y="20" width="26" height="14" rx="2" />
          <rect class="pic-card" x="10" y="38" width="26" height="14" rx="2" />
          <rect class="pic-card" x="10" y="56" width="26" height="14" rx="2" />
          <rect class="pic-card" x="48" y="20" width="26" height="14" rx="2" />
          <rect class="pic-slot" x="86" y="38" width="26" height="14" rx="2" />
          <rect class="pic-card" x="86" y="20" width="26" height="14" rx="2" />
          <rect class="pic-card" x="124" y="20" width="26" height="14" rx="2" />
          <rect class="pic-card" x="124" y="38" width="26" height="14" rx="2" />
          <path class="pic-move" d="M66 47 Q76 40 82 44" />
          <path class="pic-move" d="M78 40 L83 44 L77 47" />
          <rect class="pic-card pic-moving" x="54" y="44" width="26" height="14" rx="2" />
        {:else}
          <rect class="pic-lane" x="6" y="6" width="38" height="84" rx="3" />
          <rect class="pic-head" x="11" y="11" width="18" height="4" rx="2" />
          {#each [20, 36, 52, 68] as y (y)}
            <rect class="pic-card" x="10" {y} width="30" height="12" rx="2" />
          {/each}
          <path class="pic-move" d="M47 42 H58" />
          <path class="pic-move" d="M54 38 L59 42 L54 46" />
          <rect class="pic-sprint" x="62" y="6" width="92" height="62" rx="4" />
          <rect class="pic-head" x="68" y="11" width="22" height="4" rx="2" />
          <rect class="pic-card pic-moving" x="68" y="20" width="38" height="12" rx="2" />
          <rect class="pic-card" x="110" y="20" width="38" height="12" rx="2" />
          <rect class="pic-card" x="68" y="36" width="38" height="12" rx="2" />
          <rect class="pic-card" x="110" y="36" width="38" height="12" rx="2" />
          <path class="pic-timeline" d="M62 80 H144" />
          <path class="pic-tick" d="M62 76 V84 M89 77 V83 M116 77 V83" />
          <path class="pic-pole" d="M144 90 V70" />
          <path class="pic-flag" d="M144 70 L156 74 L144 78 Z" />
        {/if}
      </svg>
      <span class="board-kind-name">
        <input
          type="radio"
          {name}
          value={kind}
          checked={value === kind}
          onchange={() => onchange(kind)}
        />
        <strong>{t.boards.kinds[kind]}</strong>
      </span>
      <span class="hint">{t.boards.kindHints[kind]}</span>
    </label>
  {/each}
</fieldset>

<style>
  .board-kinds {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(11rem, 1fr));
    gap: 10px;
    margin: 0;
    padding: 0;
    border: none;
    min-width: 0;
  }

  .board-kinds legend {
    padding: 0;
    margin-bottom: 6px;
  }

  .board-kind {
    display: grid;
    grid-template-rows: auto auto 1fr;
    gap: 6px;
    padding: 10px;
    min-width: 0;
    border: var(--frame, 1px) solid var(--rule-strong);
    border-radius: var(--radius-m);
    background: var(--paper);
    cursor: pointer;
  }

  .board-kind:hover {
    background: var(--accent-wash);
  }

  .board-kind:has(input:checked) {
    border-color: var(--accent);
    box-shadow: inset 0 0 0 1px var(--accent);
    background: var(--accent-wash);
  }

  .board-kind:has(input:focus-visible) {
    outline: 2px solid var(--focus);
    outline-offset: 2px;
  }

  .board-kind-picture {
    display: block;
    width: 100%;
    height: auto;
    border-radius: var(--radius-s);
    background: var(--paper-sunk);
  }

  .board-kind-name {
    display: flex;
    gap: 8px;
    align-items: center;
  }

  .board-kind-name input {
    margin: 0;
    accent-color: var(--accent);
  }

  .hint {
    font-size: 0.78125rem;
    color: var(--muted);
  }

  .pic-lane {
    fill: var(--paper);
    stroke: var(--rule);
  }

  .pic-head {
    fill: var(--rule-strong);
  }

  .pic-card {
    fill: var(--paper-raised);
    stroke: var(--rule-strong);
  }

  .pic-card.pic-moving {
    fill: var(--accent);
    stroke: var(--accent);
  }

  .pic-slot,
  .pic-sprint {
    fill: none;
    stroke: var(--accent);
    stroke-dasharray: 3 2;
  }

  .pic-sprint {
    fill: var(--paper);
  }

  .pic-move,
  .pic-timeline,
  .pic-tick,
  .pic-pole {
    fill: none;
    stroke: var(--ink-soft);
    stroke-width: 1.5;
    stroke-linecap: round;
    stroke-linejoin: round;
  }

  .pic-move {
    stroke: var(--accent);
  }

  .pic-flag {
    fill: var(--accent);
  }
</style>
