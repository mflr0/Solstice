<script lang="ts">
  import { auditFiltered, auditParams, auditQuery } from '../../core/admin-forms.js';
  import {
    AUDIT_FAMILIES,
    detailLines,
    plainValue,
    retentionShortens,
  } from '../../core/admin-integrations.js';
  import { auditActor, AUDIT_MIN_RETENTION, retentionProblem } from '../../core/admin-ops.js';
  import type { AuditPage as AuditList, AuditQuery } from '../../core/admin-types.js';
  import { formatDateTime, relative, t } from '../../i18n/t.js';
  import { adminApi } from '../../lib/admin-api.js';
  import { errorMessage } from '../../lib/api.js';
  import { router } from '../../lib/router.svelte.js';
  import { notify } from '../../lib/ui.svelte.js';
  import ConfirmDialog from '../ConfirmDialog.svelte';
  import EmptyState from '../EmptyState.svelte';
  import Pager from '../Pager.svelte';

  /**
   * The audit log: who signed in and who changed what, newest first, with
   * filters kept in the address, entries that open to show their details,
   * and how long entries are kept.
   */
  let query = $state<AuditQuery>(auditQuery(router.search));
  let list = $state<AuditList | null>(null);
  let error = $state('');
  let open = $state<Record<number, boolean>>({});
  let timer: ReturnType<typeof setTimeout> | undefined;
  let seq = 0;

  let stored = $state<number | null>(null);
  let days = $state('');
  let retentionError = $state('');
  let tried = $state(false);
  let saving = $state(false);
  let shortening = $state(false);

  const filtered = $derived(auditFiltered(query));
  /** The action as the family select shows it: a family, or none while one exact action is chosen. */
  const family = $derived(
    (AUDIT_FAMILIES as readonly string[]).includes(query.action) ? query.action : '',
  );
  const exact = $derived(query.action.includes('.') ? query.action : '');
  const problem = $derived(retentionProblem(days));
  const shownProblem = $derived(
    problem === 'tooShort'
      ? t.auditAdmin.retention.problems.tooShort(AUDIT_MIN_RETENTION)
      : problem && tried
        ? t.auditAdmin.retention.problems.invalid
        : '',
  );
  const dirty = $derived(stored !== null && days.trim() !== String(stored));

  function actionLabel(action: string): string {
    return (t.auditAdmin.actions as Record<string, string>)[action] ?? action;
  }

  function targetLabel(type: string): string {
    return (t.auditAdmin.targets as Record<string, string>)[type] ?? type;
  }

  function detailKey(key: string): string {
    if (!key) return '';
    return (t.auditAdmin.detailKeys as Record<string, string>)[key] ?? key;
  }

  function value(v: unknown): string {
    return plainValue(v, t.common.yes, t.common.no);
  }

  async function load() {
    const mine = ++seq;
    try {
      const found = await adminApi.audit(query);
      if (mine === seq) {
        list = found;
        error = '';
      }
    } catch (e) {
      if (mine === seq) error = errorMessage(e);
    }
  }

  async function loadRetention() {
    try {
      const s = await adminApi.auditSettings();
      stored = s.retentionDays;
      days = String(s.retentionDays);
    } catch (e) {
      retentionError = errorMessage(e);
    }
  }

  $effect(() => {
    void load();
    void loadRetention();
    return () => clearTimeout(timer);
  });

  /** Filters changed: back to the first page, kept in the address. */
  function changed(delay = 0) {
    clearTimeout(timer);
    query.page = 1;
    timer = setTimeout(() => {
      router.setParams(auditParams(query));
      void load();
    }, delay);
  }

  function goto(page: number) {
    query.page = page;
    router.setParams(auditParams(query));
    void load();
    window.scrollTo(0, 0);
  }

  function clear() {
    query = { user: '', action: '', from: '', to: '', q: '', page: 1 };
    changed();
  }

  function filterAction(action: string) {
    query.action = action;
    changed();
  }

  async function save() {
    saving = true;
    retentionError = '';
    try {
      const s = await adminApi.saveAuditSettings(Number(days.trim()));
      stored = s.retentionDays;
      days = String(s.retentionDays);
      notify(t.auditAdmin.retention.saved);
      void load();
    } catch (e) {
      retentionError = errorMessage(e);
    } finally {
      saving = false;
    }
  }

  function submit(e: SubmitEvent) {
    e.preventDefault();
    tried = true;
    if (problem || stored === null) return;
    if (retentionShortens(stored, Number(days.trim()))) shortening = true;
    else void save();
  }
</script>

<header class="page-head">
  <div>
    <p class="eyebrow">{t.adminArea.groups.system}</p>
    <h1 class="page-title">
      {t.adminArea.sections.audit}
      {#if list}<span class="count muted">{t.auditAdmin.count(list.total)}</span>{/if}
    </h1>
  </div>
</header>
<p class="lead-text">{t.auditAdmin.intro}</p>

<form
  class="list-tools filters"
  onsubmit={(e) => e.preventDefault()}
  aria-label={t.auditAdmin.filters}
>
  <label class="field">
    <span>{t.auditAdmin.family}</span>
    <select class="select" value={family} onchange={(e) => filterAction(e.currentTarget.value)}>
      <option value="">{t.auditAdmin.allFamilies}</option>
      {#each AUDIT_FAMILIES as f (f)}
        <option value={f}>{t.auditAdmin.families[f]}</option>
      {/each}
    </select>
  </label>
  <label class="field">
    <span>{t.auditAdmin.actor}</span>
    <input
      class="input"
      type="search"
      bind:value={query.user}
      oninput={() => changed(300)}
      placeholder={t.auditAdmin.actorPlaceholder}
      autocomplete="off"
      spellcheck="false"
    />
  </label>
  <label class="field grow">
    <span>{t.auditAdmin.target}</span>
    <input
      class="input"
      type="search"
      bind:value={query.q}
      oninput={() => changed(300)}
      placeholder={t.auditAdmin.targetPlaceholder}
      autocomplete="off"
    />
  </label>
  <label class="field">
    <span>{t.auditAdmin.from}</span>
    <input
      class="input"
      type="date"
      bind:value={query.from}
      onchange={() => changed()}
      max={query.to || undefined}
    />
  </label>
  <label class="field">
    <span>{t.auditAdmin.to}</span>
    <input
      class="input"
      type="date"
      bind:value={query.to}
      onchange={() => changed()}
      min={query.from || undefined}
    />
  </label>
  {#if filtered}
    <button type="button" class="btn btn-quiet" onclick={clear}>{t.auditAdmin.clear}</button>
  {/if}
</form>
{#if exact}
  <p class="chosen">
    <span class="chip">{actionLabel(exact)}</span>
    <button type="button" class="link-btn" onclick={() => filterAction('')}
      >{t.auditAdmin.clear}</button
    >
  </p>
{/if}

{#if error}
  <p class="error-text" role="alert">{error}</p>
{:else if !list}
  <p class="muted loading">{t.common.loading}</p>
{:else if list.entries.length === 0}
  {#if filtered}
    <EmptyState kind="search" message={t.auditAdmin.noMatch}>
      <button class="btn" onclick={clear}>{t.auditAdmin.clear}</button>
    </EmptyState>
  {:else}
    <EmptyState message={t.auditAdmin.empty} hint={t.auditAdmin.emptyHint} />
  {/if}
{:else}
  <div class="table-scroll">
    <table class="ledger audit">
      <thead>
        <tr>
          <th class="c-when">{t.auditAdmin.colWhen}</th>
          <th class="c-who">{t.auditAdmin.colWho}</th>
          <th>{t.auditAdmin.colAction}</th>
          <th class="c-target">{t.auditAdmin.colTarget}</th>
          <th><span class="sr-only">{t.common.actions}</span></th>
        </tr>
      </thead>
      <tbody>
        {#each list.entries as e (e.id)}
          {@const who = auditActor(e)}
          {@const lines = detailLines(e.details)}
          <tr
            class:is-open={open[e.id]}
            class:is-failed={e.action.endsWith('.failure') ||
              e.action.endsWith('.failed') ||
              e.action.endsWith('.denied')}
          >
            <td class="c-when" title={formatDateTime(e.created)}>
              <span class="when">{relative(e.created)}</span>
              <span class="muted small">{formatDateTime(e.created)}</span>
            </td>
            <td class="c-who">
              {#if who}
                {who.name}
                {#if who.name !== who.username}<span class="key muted">{who.username}</span>{/if}
              {:else}
                <span class="muted">{e.ip ? t.auditAdmin.unknownWho : t.auditAdmin.system}</span>
              {/if}
            </td>
            <td>
              <button type="button" class="link-btn action" onclick={() => filterAction(e.action)}
                >{actionLabel(e.action)}</button
              >
            </td>
            <td class="c-target">
              {#if e.target}
                <span class="muted small">{targetLabel(e.target.type)}</span>
                {e.target.label ?? e.target.id ?? ''}
              {:else}
                <span class="muted">—</span>
              {/if}
            </td>
            <td class="row-actions">
              <button
                type="button"
                class="link-btn"
                aria-expanded={!!open[e.id]}
                aria-controls="audit-{e.id}"
                onclick={() => (open[e.id] = !open[e.id])}
                >{open[e.id] ? t.auditAdmin.hideDetails : t.auditAdmin.showDetails}</button
              >
            </td>
          </tr>
          {#if open[e.id]}
            <tr class="detail-row" id="audit-{e.id}">
              <td colspan="5">
                <dl class="detail">
                  <div>
                    <dt>{t.auditAdmin.code}</dt>
                    <dd class="mono">{e.action}</dd>
                  </div>
                  {#if e.ip}
                    <div>
                      <dt>{t.auditAdmin.address}</dt>
                      <dd class="mono">{e.ip}</dd>
                    </div>
                  {/if}
                  {#if e.username && who && e.username !== who.username}
                    <div>
                      <dt>{t.auditAdmin.recordedAs}</dt>
                      <dd class="mono">{e.username}</dd>
                    </div>
                  {/if}
                  {#each lines as l, i (i)}
                    <div>
                      <dt>{detailKey(l.key)}</dt>
                      <dd>
                        {#if l.kind === 'change'}
                          {t.auditAdmin.changed(value(l.from), value(l.to))}
                        {:else}
                          {value(l.value)}
                        {/if}
                      </dd>
                    </div>
                  {/each}
                </dl>
                {#if lines.length === 0}<p class="muted small">{t.auditAdmin.noDetails}</p>{/if}
              </td>
            </tr>
          {/if}
        {/each}
      </tbody>
    </table>
  </div>
  <Pager page={list.page} size={list.size} total={list.total} onpage={goto} />
{/if}

<section class="retention card" aria-labelledby="audit-retention">
  <h2 class="card-head" id="audit-retention">{t.auditAdmin.retention.title}</h2>
  <div class="card-body">
    <p class="muted small">{t.auditAdmin.retention.intro}</p>
    {#if retentionError}<p class="error-text" role="alert">{retentionError}</p>{/if}
    {#if stored !== null}
      <p>
        {stored === 0 ? t.auditAdmin.retention.forever : t.auditAdmin.retention.days(stored)}
      </p>
      <form class="retention-row" onsubmit={submit} novalidate>
        <label class="field">
          <span>{t.auditAdmin.retention.label}</span>
          <input
            class="input days"
            inputmode="numeric"
            bind:value={days}
            aria-invalid={!!shownProblem}
            aria-describedby="retention-hint"
          />
        </label>
        <button class="btn btn-primary" type="submit" disabled={saving || !dirty || !!problem}
          >{saving ? t.common.saving : t.auditAdmin.retention.save}</button
        >
      </form>
      <p class="form-hint" id="retention-hint">
        {t.auditAdmin.retention.hint(AUDIT_MIN_RETENTION)}
      </p>
      {#if shownProblem}<p class="field-error">{shownProblem}</p>{/if}
    {:else if !retentionError}
      <p class="muted loading">{t.common.loading}</p>
    {/if}
  </div>
</section>

<ConfirmDialog
  open={shortening}
  title={t.auditAdmin.retention.confirmTitle}
  confirm={t.auditAdmin.retention.confirm}
  onclose={() => (shortening = false)}
  onconfirm={save}
>
  <p>{t.auditAdmin.retention.confirmBody(Number(days.trim()))}</p>
</ConfirmDialog>

<style>
  .count {
    margin-left: 6px;
    font: 500 14px/1 var(--font-mono);
  }

  .filters {
    align-items: flex-end;
  }

  .filters .field {
    min-width: 0;
  }

  .filters .grow {
    flex: 1 1 200px;
  }

  .filters .grow .input {
    width: 100%;
  }

  .chosen {
    display: flex;
    gap: 10px;
    align-items: center;
    margin: -4px 0 12px;
  }

  .c-when {
    white-space: nowrap;
  }

  .when {
    display: block;
  }

  .action {
    text-align: left;
    color: var(--ink);
    text-decoration: none;
  }

  .action:hover {
    text-decoration: underline;
  }

  tr.is-failed .action {
    color: var(--accent-text);
  }

  .detail-row td {
    padding-top: 0;
    background: var(--paper-sunk);
  }

  .detail {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
    gap: 8px 24px;
    margin: 0;
    padding: 10px 0 4px;
  }

  .detail dt {
    font-size: 12px;
    color: var(--muted);
  }

  .detail dd {
    margin: 2px 0 0;
    overflow-wrap: anywhere;
  }

  .retention {
    margin-top: 32px;
    max-width: 720px;
  }

  .retention-row {
    display: flex;
    gap: 10px;
    align-items: flex-end;
    flex-wrap: wrap;
  }

  .days {
    width: 8em;
  }

  @media (max-width: 760px) {
    .c-target {
      display: none;
    }
  }

  @media (max-width: 600px) {
    .c-who {
      display: none;
    }
  }
</style>
