<script lang="ts">
  import type { ImportView } from '../../core/admin-types.js';
  import type { SetupMessages } from '../../i18n/setup/index.js';

  /**
   * What a finished import copied, step by step, and what to check
   * (Jira's own warnings, in English as the importer wrote them). Shared by
   * the setup wizard and the administration's import page.
   */
  interface Props {
    m: SetupMessages;
    num: (n: number) => string;
    job: Pick<ImportView, 'steps' | 'warnings'>;
  }
  const { m, num, job }: Props = $props();

  const stepName = (s: string) => (m.import.report as Record<string, string>)[s] ?? s;
  const anySkipped = $derived(job.steps.some((s) => s.skipped > 0));
</script>

<details>
  <summary>{m.import.reportCaption}</summary>
  <div class="table-scroll">
    <table class="ledger report">
      <thead>
        <tr>
          <th scope="col">{m.import.reportItem}</th>
          <th scope="col" class="num">{m.import.reportImported}</th>
          {#if anySkipped}<th scope="col" class="num">{m.import.reportSkipped}</th>{/if}
        </tr>
      </thead>
      <tbody>
        {#each job.steps as s (s.name)}
          <tr>
            <td>{stepName(s.name)}</td>
            <td class="num">{num(s.imported)}</td>
            {#if anySkipped}<td class="num">{s.skipped ? num(s.skipped) : ''}</td>{/if}
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
  {#if anySkipped}<p class="muted">{m.import.skippedHint}</p>{/if}
</details>

{#if job.warnings.length > 0}
  <section class="warnings" aria-labelledby="import-warnings">
    <h2 class="section-title" id="import-warnings">{m.import.warnings}</h2>
    {#if m.import.warningsNote}<p class="muted">{m.import.warningsNote}</p>{/if}
    <ul lang="en">
      {#each job.warnings as w, i (i)}<li>{w}</li>{/each}
    </ul>
  </section>
{/if}

<style>
  .report tbody tr {
    cursor: default;
  }

  .warnings ul {
    margin: 0;
    padding-left: 20px;
    display: grid;
    gap: 4px;
  }

  .section-title {
    margin-top: 6px;
  }
</style>
