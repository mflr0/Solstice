<script lang="ts">
  import { assignColumn } from '../../core/project-admin.js';
  import { paths } from '../../core/route.js';
  import { t } from '../../i18n/t.js';
  import { api, ApiError, errorMessage, type ImportReport } from '../../lib/api.js';
  import { notify } from '../../lib/ui.svelte.js';
  import Dropdown from '../Dropdown.svelte';

  /**
   * The Import tab: issues from a CSV file. Choosing the file checks it
   * (nothing is written): the columns and the field each goes to, which can
   * be changed, and every row with its errors. « Import » then creates every
   * row, or none.
   */
  interface Props {
    projectKey: string;
  }
  const { projectKey }: Props = $props();

  // The server's caps (issue_import.go).
  const MAX_ROWS = 1000;
  const MAX_MIB = 4;

  let file = $state<File | null>(null);
  let report = $state<ImportReport | null>(null);
  let error = $state('');
  let busy = $state<'checking' | 'importing' | null>(null);
  let created = $state<string[] | null>(null);
  let input = $state<HTMLInputElement | null>(null);

  const failed = $derived(report ? report.rows.filter((r) => r.errors?.length).length : 0);
  const ready = $derived(report ? report.rows.length - failed : 0);
  const canImport = $derived(!!report && !busy && report.rows.length > 0 && failed === 0);
  /** The new keys, lowest to highest: the file's order is not the creation order. */
  const createdRange = $derived.by(() => {
    const nums = (created ?? [])
      .map((k) => Number(k.slice(k.lastIndexOf('-') + 1)))
      .sort((a, b) => a - b);
    if (nums.length === 0) return '';
    const first = `${projectKey}-${nums[0]}`;
    return nums.length > 1 ? `${first} – ${projectKey}-${nums[nums.length - 1]}` : first;
  });
  const mapping = () => report?.columns.map((c) => c.field) ?? null;

  /** Checks the file, with the mapping given (null: the server guesses). */
  async function check(f: File, map: string[] | null) {
    busy = 'checking';
    error = '';
    try {
      const r = await api.importIssues(projectKey, f, map, true);
      if (file === f) report = r;
    } catch (e) {
      if (file === f) {
        error = errorMessage(e);
        if (!map) report = null;
      }
    } finally {
      if (file === f) busy = null;
    }
  }

  function choose(e: Event & { currentTarget: HTMLInputElement }) {
    const f = e.currentTarget.files?.[0] ?? null;
    file = f;
    report = null;
    created = null;
    error = '';
    if (f) void check(f, null);
  }

  function remap(index: number, field: string) {
    if (!report || !file) return;
    report = { ...report, columns: assignColumn(report.columns, index, field) };
    void check(file, mapping());
  }

  async function run() {
    if (!file || !report) return;
    const f = file;
    busy = 'importing';
    error = '';
    try {
      const r = await api.importIssues(projectKey, f, mapping(), false);
      created = r.created ?? [];
      notify(t.projectConfig.import.done(created.length));
      file = null;
      report = null;
      if (input) input.value = '';
    } catch (e) {
      const data =
        e instanceof ApiError ? (e.data as { import?: ImportReport } | undefined) : undefined;
      if (data?.import) report = data.import;
      error = errorMessage(e);
    } finally {
      busy = null;
    }
  }

  function again() {
    created = null;
    input?.click();
  }
</script>

<h2 class="section-title">{t.projectConfig.import.title}</h2>
<p class="lead-text">{t.projectConfig.import.intro}</p>

<div class="settings-form">
  <label class="field">
    <span>{t.projectConfig.import.file}</span>
    <input
      bind:this={input}
      class="input"
      type="file"
      accept=".csv,text/csv"
      disabled={busy === 'importing'}
      onchange={choose}
    />
    <span class="hint">{t.projectConfig.import.limits(MAX_ROWS, MAX_MIB)}</span>
  </label>
</div>

{#if created}
  <p class="done" role="status">
    {t.projectConfig.import.done(created.length)}
    {#if created.length > 0}
      <a href={paths.search(`project = ${projectKey} ORDER BY created DESC`)}>{createdRange}</a>
    {/if}
  </p>
  <button type="button" class="btn btn-quiet" onclick={again}>{t.projectConfig.import.again}</button
  >
{/if}

{#if busy === 'checking' && !report}
  <p class="muted loading" role="status">{t.projectConfig.import.checking}</p>
{/if}
{#if error}
  <p class="error-text" role="alert">{error}</p>
{/if}

{#if report}
  <h2 class="section-title">{t.projectConfig.import.columnsTitle}</h2>
  <div class="table-scroll">
    <table class="ledger">
      <thead>
        <tr>
          <th scope="col">{t.projectConfig.import.column}</th>
          <th scope="col">{t.projectConfig.import.field}</th>
        </tr>
      </thead>
      <tbody>
        {#each report.columns as c, i (i)}
          <tr>
            <th scope="row" class="header">{c.header}</th>
            <td>
              {#if c.readOnly}
                <span class="muted">{t.projectConfig.import.readOnly}</span>
              {:else}
                <Dropdown
                  label={t.projectConfig.import.fieldFor(c.header)}
                  value={c.field}
                  options={[
                    { value: '', label: t.projectConfig.import.skip },
                    ...report.targets.map((f) => ({ value: f.id, label: f.name })),
                  ]}
                  disabled={!!busy}
                  onchange={(v) => remap(i, v)}
                />
                {#if !c.known && !c.field}
                  <span class="hint">{t.projectConfig.import.unknown}</span>
                {/if}
              {/if}
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>

  <h2 class="section-title">{t.projectConfig.import.rowsTitle}</h2>
  {#if report.rows.length === 0}
    <p class="muted">{t.projectConfig.import.noRows}</p>
  {:else}
    <p class={failed ? 'error-text' : 'muted'} role="status">
      {failed ? t.projectConfig.import.failed(failed) : t.projectConfig.import.ready(ready)}
    </p>
    <div class="table-scroll">
      <table class="ledger">
        <thead>
          <tr>
            <th scope="col">{t.projectConfig.import.line}</th>
            <th scope="col">{t.projectConfig.import.key}</th>
            <th scope="col">{t.projectConfig.import.summary}</th>
            <th scope="col">{t.projectConfig.import.type}</th>
            <th scope="col">{t.projectConfig.import.status}</th>
            <th scope="col">{t.projectConfig.import.problems}</th>
          </tr>
        </thead>
        <tbody>
          {#each report.rows as r (r.line)}
            <tr class:bad={!!r.errors?.length}>
              <td class="num">{r.line}</td>
              <td class="mono">{r.key ?? ''}</td>
              <td class="summary">{r.summary}</td>
              <td>{r.type ?? ''}</td>
              <td>{r.status ?? ''}</td>
              <td>
                {#if r.errors?.length}
                  <ul class="errors">
                    {#each r.errors as m, j (j)}<li>{m}</li>{/each}
                  </ul>
                {/if}
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}

  <div class="actions">
    <button type="button" class="btn btn-primary" disabled={!canImport} onclick={() => void run()}
      >{busy === 'importing'
        ? t.projectConfig.import.importing
        : t.projectConfig.import.submit(ready)}</button
    >
  </div>
{/if}

<style>
  th.header {
    text-align: left;
    text-transform: none;
    letter-spacing: 0;
    font-weight: 600;
    overflow-wrap: anywhere;
  }

  .hint {
    display: block;
    margin-top: 4px;
  }

  td.summary {
    min-width: 200px;
    overflow-wrap: anywhere;
  }

  td.num {
    font-variant-numeric: tabular-nums;
  }

  tr.bad td {
    color: var(--accent-text);
  }

  .errors {
    margin: 0;
    padding-left: 18px;
  }

  .actions {
    margin-top: 16px;
  }

  .done {
    margin: 12px 0;
  }
</style>
