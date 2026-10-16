<script lang="ts">
  import type { VersionRow } from '../../core/admin-types.js';
  import { parseDay, todayISO } from '../../core/dates.js';
  import {
    dayProblem,
    moveItem,
    orderAfterStep,
    otherVersions,
    releaseTargets,
    versionLate,
    versionState,
  } from '../../core/project-admin.js';
  import { longDay } from '../../i18n/format.js';
  import { t } from '../../i18n/t.js';
  import { projectAdminApi } from '../../lib/admin-api.js';
  import { errorMessage } from '../../lib/api.js';
  import { notify } from '../../lib/ui.svelte.js';
  import ConfirmDialog from '../ConfirmDialog.svelte';
  import EmptyState from '../EmptyState.svelte';
  import VersionDialog from './VersionDialog.svelte';

  /**
   * The project's versions in the order they are shipped: create, change,
   * release (moving the unresolved issues on), take back a release, archive,
   * reorder with the arrows or by dragging, merge and delete.
   */
  interface Props {
    projectKey: string;
  }
  const { projectKey }: Props = $props();

  let versions = $state<VersionRow[] | null>(null);
  let error = $state('');
  let dialog = $state<{ version: VersionRow | null } | null>(null);
  let releasing = $state<VersionRow | null>(null);
  let merging = $state<VersionRow | null>(null);
  let removing = $state<VersionRow | null>(null);
  let releaseDay = $state('');
  let moveTo = $state('');
  let ordering = $state(false);
  let orderNote = $state('');
  let busyId = $state<number | null>(null);
  let dragged = $state<number | null>(null);
  let over = $state<number | null>(null);

  const today = todayISO();
  const releaseOptions = $derived(
    releasing && versions ? releaseTargets(versions, releasing.id) : [],
  );
  const otherOptions = $derived(
    merging && versions
      ? otherVersions(versions, merging.id)
      : removing && versions
        ? otherVersions(versions, removing.id)
        : [],
  );

  const day = (s: string | null) => {
    const d = s ? parseDay(s) : null;
    return d ? longDay(d) : '—';
  };

  async function load() {
    try {
      versions = await projectAdminApi.versions(projectKey);
      error = '';
    } catch (e) {
      error = errorMessage(e);
    }
  }

  $effect(() => {
    void projectKey;
    void load();
  });

  function saved(v: VersionRow, created: boolean) {
    notify(created ? t.projectVersions.created(v.name) : t.projectVersions.saved(v.name));
    void load();
  }

  /** Puts the versions in a new order, at once, and goes back if the server refuses. */
  async function reorder(ids: number[], focus?: string) {
    if (!versions || ordering) return;
    const before = versions;
    versions = ids.map((id) => before.find((v) => v.id === id)).filter((v) => v !== undefined);
    ordering = true;
    try {
      versions = await projectAdminApi.orderVersions(projectKey, ids);
      orderNote = t.projectVersions.ordered;
    } catch (e) {
      versions = before;
      notify(errorMessage(e), 'error');
    } finally {
      ordering = false;
      if (focus) queueMicrotask(() => document.getElementById(focus)?.focus());
    }
  }

  function step(v: VersionRow, by: -1 | 1) {
    if (!versions) return;
    const ids = versions.map((x) => x.id);
    const next = orderAfterStep(ids, v.id, by);
    if (!next) return;
    const to = ids.indexOf(v.id) + by;
    const atEdge = to === 0 || to === ids.length - 1;
    // Keep the keyboard on the version: on the same arrow, or the other one at an end.
    const dir = atEdge ? (by < 0 ? 'down' : 'up') : by < 0 ? 'up' : 'down';
    void reorder(next, `version-${dir}-${v.id}`);
  }

  function drop(target: number) {
    if (dragged !== null && versions && dragged !== target) {
      void reorder(moveItem(versions, dragged, target).map((v) => v.id));
    }
    dragged = null;
    over = null;
  }

  async function patch(v: VersionRow, body: { archived: boolean } | { released: boolean }) {
    busyId = v.id;
    try {
      await projectAdminApi.updateVersion(projectKey, v.id, body);
      if ('archived' in body) {
        notify(
          body.archived ? t.projectVersions.archived(v.name) : t.projectVersions.unarchived(v.name),
        );
      } else {
        notify(t.projectVersions.unreleased(v.name));
      }
      await load();
    } catch (e) {
      notify(errorMessage(e), 'error');
    } finally {
      busyId = null;
    }
  }

  function askRelease(v: VersionRow) {
    releasing = v;
    releaseDay = today;
    moveTo = '';
  }

  async function release() {
    const v = releasing;
    if (!v) return;
    if (!releaseDay.trim() || dayProblem(releaseDay)) {
      throw new Error(t.projectVersions.problems.dayInvalid);
    }
    const target = releaseOptions.find((x) => String(x.id) === moveTo)?.id;
    await projectAdminApi.releaseVersion(projectKey, v.id, {
      releaseDate: releaseDay.trim(),
      ...(target ? { moveUnresolvedTo: target } : {}),
    });
    notify(t.projectVersions.released(v.name));
    await load();
  }

  function askMerge(v: VersionRow) {
    merging = v;
    moveTo = '';
  }

  async function merge() {
    const v = merging;
    if (!v) return;
    const target = otherOptions.find((x) => String(x.id) === moveTo);
    if (!target) throw new Error(t.projectVersions.mergeChoose);
    await projectAdminApi.mergeVersion(projectKey, v.id, target.id);
    notify(t.projectVersions.merged(v.name, target.name));
    await load();
  }

  function askRemove(v: VersionRow) {
    removing = v;
    moveTo = '';
  }

  async function remove() {
    const v = removing;
    if (!v) return;
    const target =
      v.issues.total > 0 ? otherOptions.find((x) => String(x.id) === moveTo)?.id : undefined;
    await projectAdminApi.deleteVersion(
      projectKey,
      v.id,
      target ? { fixTo: target, affectsTo: target } : {},
    );
    notify(t.projectVersions.deleted(v.name));
    await load();
  }
</script>

<div class="head">
  <p class="lead-text">{t.projectVersions.intro}</p>
  {#if versions && versions.length > 0}
    <button class="btn btn-primary" onclick={() => (dialog = { version: null })}
      >{t.projectVersions.newVersion}</button
    >
  {/if}
</div>

{#if error}
  <p class="error-text" role="alert">{error}</p>
{:else if !versions}
  <p class="muted loading">{t.common.loading}</p>
{:else if versions.length === 0}
  <EmptyState message={t.projectVersions.empty} hint={t.projectVersions.emptyHint}>
    <button class="btn btn-primary" onclick={() => (dialog = { version: null })}
      >{t.projectVersions.newVersion}</button
    >
  </EmptyState>
{:else}
  <p class="muted small order-hint">{t.catalogAdmin.orderHint}</p>
  <p class="sr-only" role="status" aria-live="polite">{orderNote}</p>
  <div class="table-scroll">
    <table class="ledger versions">
      <thead>
        <tr>
          <th class="c-order"><span class="sr-only">{t.catalogAdmin.order}</span></th>
          <th>{t.projects.name}</th>
          <th>{t.projectsAdmin.state}</th>
          <th>{t.projectVersions.startDate}</th>
          <th>{t.projectVersions.releaseDate}</th>
          <th class="num">{t.projectsAdmin.issues}</th>
          <th><span class="sr-only">{t.common.actions}</span></th>
        </tr>
      </thead>
      <tbody>
        {#each versions as v, i (v.id)}
          {@const state = versionState(v)}
          <tr
            class:is-off={v.archived}
            class:is-dragged={dragged === i}
            class:is-over={over === i && dragged !== null && dragged !== i}
            draggable={!ordering}
            ondragstart={(e) => {
              dragged = i;
              e.dataTransfer?.setData('text/plain', String(v.id));
            }}
            ondragover={(e) => {
              if (dragged === null) return;
              e.preventDefault();
              over = i;
            }}
            ondragleave={() => over === i && (over = null)}
            ondrop={(e) => {
              e.preventDefault();
              drop(i);
            }}
            ondragend={() => ((dragged = null), (over = null))}
          >
            <td class="c-order">
              <span class="grip" aria-hidden="true">⋮⋮</span>
              <button
                type="button"
                class="btn btn-quiet btn-small arrow"
                id="version-up-{v.id}"
                aria-label={t.catalogAdmin.moveUp(v.name)}
                disabled={ordering || i === 0}
                onclick={() => step(v, -1)}>↑</button
              >
              <button
                type="button"
                class="btn btn-quiet btn-small arrow"
                id="version-down-{v.id}"
                aria-label={t.catalogAdmin.moveDown(v.name)}
                disabled={ordering || i === versions.length - 1}
                onclick={() => step(v, 1)}>↓</button
              >
            </td>
            <td>
              <strong>{v.name}</strong>
              {#if v.description}<span class="muted small desc">{v.description}</span>{/if}
            </td>
            <td>
              <span class="stamp" class:done={state === 'released'}
                >{t.projectVersions.states[state]}</span
              >
              {#if versionLate(v, today)}<span class="stamp is-failed"
                  >{t.projectVersions.late}</span
                >{/if}
            </td>
            <td class="nowrap">{day(v.startDate)}</td>
            <td class="nowrap">{day(v.releaseDate)}</td>
            <td class="num">{t.projectVersions.issuesCell(v.issues.total, v.issues.unresolved)}</td>
            <td class="row-actions">
              <button class="link-btn" onclick={() => (dialog = { version: v })}
                >{t.common.edit}</button
              >
              {#if v.released}
                <button
                  class="link-btn"
                  disabled={busyId === v.id}
                  onclick={() => void patch(v, { released: false })}
                  >{t.projectVersions.unrelease}</button
                >
              {:else}
                <button class="link-btn" onclick={() => askRelease(v)}
                  >{t.projectVersions.release}</button
                >
              {/if}
              <button
                class="link-btn"
                disabled={busyId === v.id}
                onclick={() => void patch(v, { archived: !v.archived })}
                >{v.archived ? t.projectsAdmin.unarchive : t.projectsAdmin.archive}</button
              >
              <button class="link-btn" onclick={() => askMerge(v)}>{t.projectVersions.merge}</button
              >
              <button class="link-btn" onclick={() => askRemove(v)}>{t.common.delete}</button>
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
  <p class="form-hint note">{t.projectVersions.archivedHint}</p>
{/if}

<VersionDialog
  open={dialog !== null}
  {projectKey}
  version={dialog?.version ?? null}
  versions={versions ?? []}
  onclose={() => (dialog = null)}
  onsaved={saved}
/>

<ConfirmDialog
  open={releasing !== null}
  title={t.projectVersions.releaseTitle(releasing?.name ?? '')}
  confirm={t.projectVersions.release}
  danger={false}
  onclose={() => (releasing = null)}
  onconfirm={release}
>
  <p>{t.projectVersions.releaseBody}</p>
  <label class="field">
    <span>{t.projectVersions.releaseDate}</span>
    <input class="input" type="date" bind:value={releaseDay} />
  </label>
  {#if releasing && releasing.issues.unresolved > 0}
    <p>{t.projectVersions.releaseUnresolved(releasing.issues.unresolved)}</p>
    <label class="field">
      <span>{t.projectVersions.releaseMoveLabel}</span>
      <select class="select" bind:value={moveTo}>
        <option value="">{t.projectVersions.releaseMoveNone}</option>
        {#each releaseOptions as x (x.id)}<option value={String(x.id)}>{x.name}</option>{/each}
      </select>
    </label>
  {/if}
</ConfirmDialog>

<ConfirmDialog
  open={merging !== null}
  title={t.projectVersions.mergeTitle(merging?.name ?? '')}
  confirm={t.projectVersions.merge}
  onclose={() => (merging = null)}
  onconfirm={merge}
>
  <p>{t.projectVersions.mergeBody}</p>
  {#if otherOptions.length === 0}
    <p class="error-text">{t.projectVersions.mergeNone}</p>
  {:else}
    <label class="field">
      <span>{t.projectVersions.mergeLabel}</span>
      <select class="select" bind:value={moveTo}>
        <option value="" disabled>{t.projectVersions.mergeChoose}</option>
        {#each otherOptions as x (x.id)}<option value={String(x.id)}>{x.name}</option>{/each}
      </select>
    </label>
  {/if}
</ConfirmDialog>

<ConfirmDialog
  open={removing !== null}
  title={t.projectVersions.deleteTitle(removing?.name ?? '')}
  confirm={t.common.delete}
  onclose={() => (removing = null)}
  onconfirm={remove}
>
  {#if removing && removing.issues.total > 0}
    <p>{t.projectVersions.deleteUsed(removing.issues.total)}</p>
    <label class="field">
      <span>{t.projectVersions.deleteMoveLabel}</span>
      <select class="select" bind:value={moveTo}>
        <option value="">{t.projectVersions.deleteMoveNone}</option>
        {#each otherOptions as x (x.id)}<option value={String(x.id)}>{x.name}</option>{/each}
      </select>
    </label>
  {:else}
    <p>{t.projectVersions.deleteUnused}</p>
  {/if}
</ConfirmDialog>

<style>
  .head {
    display: flex;
    flex-wrap: wrap;
    align-items: flex-start;
    justify-content: space-between;
    gap: 10px 16px;
  }

  .head .lead-text {
    flex: 1 1 320px;
    margin-bottom: 14px;
  }

  .order-hint {
    margin: 0 0 8px;
  }

  .c-order {
    width: 92px;
    white-space: nowrap;
  }

  .grip {
    display: inline-block;
    width: 14px;
    color: var(--muted);
    font-size: 11px;
    letter-spacing: -2px;
    cursor: grab;
  }

  .arrow {
    width: 24px;
    padding: 0;
    justify-content: center;
  }

  .desc {
    display: block;
    margin-top: 2px;
    max-width: 50ch;
  }

  .nowrap {
    white-space: nowrap;
  }

  .stamp + .stamp {
    margin-left: 4px;
  }

  .note {
    margin: 10px 0 0;
  }

  tr.is-dragged td {
    opacity: 0.45;
  }

  tr.is-over td {
    box-shadow: inset 0 2px 0 var(--accent);
  }

  /* Many actions per row: they wrap instead of widening the table. */
  .versions .row-actions {
    white-space: normal;
    min-width: 230px;
  }

  @media (max-width: 600px) {
    .c-order {
      width: auto;
    }

    .grip {
      display: none;
    }
  }
</style>
