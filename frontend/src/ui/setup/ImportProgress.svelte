<script lang="ts">
  import { IMPORT_PHASES } from '../../core/setup.js';
  import type { ImportView } from '../../core/admin-types.js';
  import type { SetupMessages } from '../../i18n/setup/index.js';

  /**
   * An import running: the progress bar, the rows copied and the phases
   * done, current and to come. Shared by the setup wizard and the
   * administration's import page.
   */
  interface Props {
    m: SetupMessages;
    num: (n: number) => string;
    job: Pick<ImportView, 'phase' | 'done' | 'total' | 'rows'>;
  }
  const { m, num, job }: Props = $props();

  const phaseName = (p: string) => (m.import.phases as Record<string, string>)[p] ?? p;
</script>

<div class="progress">
  <progress
    max={job.total || IMPORT_PHASES.length}
    value={job.done}
    aria-label={m.import.progressLabel}
  ></progress>
  <span class="rows mono">{m.import.rows(num(job.rows))}</span>
</div>
<ol class="phases">
  {#each IMPORT_PHASES as p, i (p)}
    {@const state = i < job.done ? 'done' : p === job.phase ? 'current' : 'todo'}
    <li class="phase {state}" aria-current={state === 'current' ? 'step' : undefined}>
      <span class="mark" aria-hidden="true"
        >{state === 'done' ? '✓' : state === 'current' ? '›' : ''}</span
      >
      {phaseName(p)}
      {#if state !== 'todo'}
        <span class="sr-only"
          >({state === 'done' ? m.import.phaseDone : m.import.phaseCurrent})</span
        >
      {/if}
    </li>
  {/each}
</ol>
<p class="sr-only" aria-live="polite">{job.phase ? phaseName(job.phase) : ''}</p>

<style>
  .progress {
    display: grid;
    gap: 8px;
  }

  progress {
    width: 100%;
    height: 8px;
    appearance: none;
    border: 1px solid var(--rule-strong);
    border-radius: var(--radius-xs);
    background: var(--paper-sunk);
    overflow: hidden;
  }

  progress::-webkit-progress-bar {
    background: var(--paper-sunk);
  }

  progress::-webkit-progress-value {
    background: var(--accent);
    transition: width 0.4s ease;
  }

  progress::-moz-progress-bar {
    background: var(--accent);
  }

  .rows {
    color: var(--ink-soft);
  }

  .phases {
    list-style: none;
    margin: 0;
    padding: 0;
    columns: 2 220px;
    column-gap: 24px;
  }

  .phase {
    display: flex;
    gap: 8px;
    padding: 3px 0;
    color: var(--muted);
    break-inside: avoid;
  }

  .phase .mark {
    width: 1em;
    flex: none;
    text-align: center;
  }

  .phase.done {
    color: var(--ink-soft);
  }

  .phase.done .mark {
    color: var(--stamp-done);
  }

  .phase.current {
    color: var(--ink);
    font-weight: 600;
  }

  .phase.current .mark {
    color: var(--accent-text);
  }
</style>
