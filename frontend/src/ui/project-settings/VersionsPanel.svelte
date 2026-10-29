<script lang="ts">
  import type { VersionRow } from '../../core/admin-types.js';
  import { todayISO } from '../../core/dates.js';
  import {
    datesProblem,
    moveItem,
    otherVersions,
    releaseTargets,
  } from '../../core/project-admin.js';
  import { formatDay, t } from '../../i18n/t.js';
  import { projectAdminApi } from '../../lib/admin-api.js';
  import { errorMessage } from '../../lib/api.js';
  import { notify } from '../../lib/ui.svelte.js';
  import OrderArrows from '../admin/OrderArrows.svelte';
  import ConfirmDialog from '../ConfirmDialog.svelte';
  import Dropdown from '../Dropdown.svelte';
  import EmptyState from '../EmptyState.svelte';
  import VersionDialog from './VersionDialog.svelte';

  /**
   * The Versions tab: the project's versions in the order the version lists
   * show them, and their creation, change, release, archiving, merging and
   * removal. Issues move to another version when one is released, merged or
   * deleted; the server records it in their history.
   */
  interface Props {
    projectKey: string;
  }
  const { projectKey }: Props = $props();

  let items = $state<VersionRow[] | null>(null);
  let error = $state('');
  let ordering = $state(false);
  let dialog = $state<{ item: VersionRow | null } | null>(null);
  let releasing = $state<VersionRow | null>(null);
  let merging = $state<VersionRow | null>(null);
  let removing = $state<VersionRow | null>(null);
  let releaseDate = $state('');
  let moveTo = $state('');
  let into = $state('');
  let fixTo = $state('');
  let affectsTo = $state('');

  const releaseChoices = $derived(items && releasing ? releaseTargets(items, releasing.id) : []);
  const releaseProblem = $derived(
    releasing ? datesProblem(releasing.startDate ?? '', releaseDate || todayISO()) : null,
  );
  const mergeChoices = $derived(items && merging ? otherVersions(items, merging.id) : []);
  const removeChoices = $derived(items && removing ? otherVersions(items, removing.id) : []);

  async function load() {
    try {
      items = await projectAdminApi.versions(projectKey);
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
    notify(
      created ? t.projectConfig.versions.created(v.name) : t.projectConfig.versions.saved(v.name),
    );
    void load();
  }

  async function step(v: VersionRow, by: -1 | 1) {
    if (!items || ordering) return;
    const from = items.findIndex((x) => x.id === v.id);
    const to = from + by;
    if (to < 0 || to >= items.length) return;
    const before = items;
    const next = moveItem(items, from, to);
    // Keep the keyboard on the item: on the same arrow, or the other one at an end.
    const atEdge = to === 0 || to === items.length - 1;
    const dir = atEdge ? (by < 0 ? 'down' : 'up') : by < 0 ? 'up' : 'down';
    items = next;
    ordering = true;
    try {
      items = await projectAdminApi.orderVersions(
        projectKey,
        next.map((x) => x.id),
      );
    } catch (e) {
      items = before;
      notify(errorMessage(e), 'error');
    } finally {
      ordering = false;
      queueMicrotask(() => document.getElementById(`order-${dir}-${v.id}`)?.focus());
    }
  }

  /** The switches that need no question: un-releasing, archiving and restoring. */
  async function change(
    v: VersionRow,
    patch: { released?: boolean; archived?: boolean },
    done: string,
  ) {
    try {
      await projectAdminApi.updateVersion(projectKey, v.id, patch);
      notify(done);
      await load();
    } catch (e) {
      notify(errorMessage(e), 'error');
    }
  }

  function askRelease(v: VersionRow) {
    releasing = v;
    releaseDate = todayISO();
    moveTo = '';
  }

  async function release() {
    const v = releasing;
    if (!v) return;
    const target = releaseChoices.find((x) => String(x.id) === moveTo)?.id;
    await projectAdminApi.releaseVersion(projectKey, v.id, {
      releaseDate: releaseDate || todayISO(),
      ...(target ? { moveUnresolvedTo: target } : {}),
    });
    notify(t.projectConfig.versions.released(v.name));
    await load();
  }

  function askMerge(v: VersionRow) {
    merging = v;
    into = '';
  }

  async function merge() {
    const v = merging;
    if (!v) return;
    const target = mergeChoices.find((x) => String(x.id) === into);
    if (!target) throw new Error(t.catalogAdmin.chooseTarget);
    await projectAdminApi.mergeVersion(projectKey, v.id, target.id);
    notify(t.projectConfig.versions.merged(v.name, target.name));
    await load();
  }

  function askRemove(v: VersionRow) {
    removing = v;
    fixTo = '';
    affectsTo = '';
  }

  async function remove() {
    const v = removing;
    if (!v) return;
    const fix = removeChoices.find((x) => String(x.id) === fixTo)?.id;
    const affects = removeChoices.find((x) => String(x.id) === affectsTo)?.id;
    await projectAdminApi.deleteVersion(projectKey, v.id, {
      ...(fix ? { fixTo: fix } : {}),
      ...(affects ? { affectsTo: affects } : {}),
    });
    notify(t.projectConfig.versions.deleted(v.name));
    await load();
  }
</script>

<h2 class="section-title">
  {t.projectConfig.versions.title}
  {#if items}<span class="count">{items.length}</span>{/if}
  {#if items}
    <span class="actions">
      <button type="button" class="btn btn-small" onclick={() => (dialog = { item: null })}
        >{t.projectConfig.versions.new}</button
      >
    </span>
  {/if}
</h2>
<p class="lead-text">{t.projectConfig.versions.intro}</p>

{#if error}
  <p class="error-text" role="alert">{error}</p>
{:else if !items}
  <p class="muted loading">{t.common.loading}</p>
{:else if items.length === 0}
  <EmptyState message={t.projectConfig.versions.empty} hint={t.projectConfig.versions.emptyHint}>
    <button type="button" class="btn btn-primary" onclick={() => (dialog = { item: null })}
      >{t.projectConfig.versions.new}</button
    >
  </EmptyState>
{:else}
  <p class="muted small order-hint">{t.projectConfig.versions.orderHint}</p>
  <div class="table-scroll">
    <table class="ledger">
      <thead>
        <tr>
          <th scope="col" class="c-order"><span class="sr-only">{t.catalogAdmin.order}</span></th>
          <th scope="col">{t.projects.name}</th>
          <th scope="col">{t.projectConfig.versions.start}</th>
          <th scope="col">{t.projectConfig.versions.release}</th>
          <th scope="col">{t.projectConfig.versions.state}</th>
          <th scope="col" class="num">{t.projectsAdmin.issues}</th>
          <th scope="col"><span class="sr-only">{t.common.actions}</span></th>
        </tr>
      </thead>
      <tbody>
        {#each items as v, i (v.id)}
          <tr class:is-off={v.archived}>
            <td class="c-order">
              <OrderArrows
                id={v.id}
                name={v.name}
                first={i === 0}
                last={i === items.length - 1}
                disabled={ordering}
                onmove={(by) => void step(v, by)}
              />
            </td>
            <td>
              <strong>{v.name}</strong>
              {#if v.description}<span class="muted small desc">{v.description}</span>{/if}
            </td>
            <td class="when">{v.startDate ? formatDay(v.startDate) : '—'}</td>
            <td class="when">{v.releaseDate ? formatDay(v.releaseDate) : '—'}</td>
            <td>
              {#if v.released}<span class="stamp done">{t.projects.released}</span>{/if}
              {#if v.archived}<span class="stamp">{t.projects.archived}</span>{/if}
            </td>
            <td class="num">
              {v.issues.total}
              {#if v.issues.unresolved > 0}
                <span class="muted small"
                  >{t.projectConfig.versions.unresolved(v.issues.unresolved)}</span
                >
              {/if}
            </td>
            <td class="row-actions">
              <button type="button" class="link-btn" onclick={() => (dialog = { item: v })}
                >{t.common.edit}</button
              >
              {#if v.released}
                <button
                  type="button"
                  class="link-btn"
                  onclick={() =>
                    void change(
                      v,
                      { released: false },
                      t.projectConfig.versions.unreleased(v.name),
                    )}>{t.projectConfig.versions.unreleaseAction}</button
                >
              {:else}
                <button type="button" class="link-btn" onclick={() => askRelease(v)}
                  >{t.projectConfig.versions.releaseAction}</button
                >
              {/if}
              <button
                type="button"
                class="link-btn"
                onclick={() =>
                  void change(
                    v,
                    { archived: !v.archived },
                    v.archived
                      ? t.projectConfig.versions.unarchived(v.name)
                      : t.projectConfig.versions.archived(v.name),
                  )}>{v.archived ? t.projectsAdmin.unarchive : t.projectsAdmin.archive}</button
              >
              <button type="button" class="link-btn" onclick={() => askMerge(v)}
                >{t.projectConfig.versions.mergeAction}</button
              >
              <button type="button" class="link-btn" onclick={() => askRemove(v)}
                >{t.common.delete}</button
              >
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
{/if}

{#if items}
  <VersionDialog
    {projectKey}
    open={dialog !== null}
    version={dialog?.item ?? null}
    versions={items}
    onclose={() => (dialog = null)}
    onsaved={saved}
  />
{/if}

<ConfirmDialog
  open={releasing !== null}
  title={t.projectConfig.versions.releaseTitle(releasing?.name ?? '')}
  confirm={t.projectConfig.versions.releaseAction}
  danger={false}
  disabled={!!releaseProblem}
  onclose={() => (releasing = null)}
  onconfirm={release}
>
  <label class="field">
    <span>{t.projectConfig.versions.releaseDate}</span>
    <input class="input" type="date" bind:value={releaseDate} aria-invalid={!!releaseProblem} />
    {#if releaseProblem}
      <span class="field-error">{t.projectConfig.problems[releaseProblem]}</span>
    {/if}
  </label>
  {#if releasing && releasing.issues.unresolved > 0}
    <p>{t.projectConfig.versions.releaseUnresolved(releasing.issues.unresolved)}</p>
    <div class="field">
      <span>{t.projectConfig.versions.releaseMove}</span>
      <Dropdown
        label={t.projectConfig.versions.releaseMove}
        value={moveTo}
        options={[
          { value: '', label: t.projectConfig.versions.releaseKeep },
          ...releaseChoices.map((x) => ({ value: String(x.id), label: x.name })),
        ]}
        onchange={(v) => (moveTo = v)}
      />
    </div>
  {:else}
    <p class="muted">{t.projectConfig.versions.releaseNoUnresolved}</p>
  {/if}
</ConfirmDialog>

<ConfirmDialog
  open={merging !== null}
  title={t.projectConfig.versions.mergeTitle(merging?.name ?? '')}
  confirm={t.projectConfig.versions.mergeAction}
  disabled={!into}
  onclose={() => (merging = null)}
  onconfirm={merge}
>
  {#if merging}
    <p>{t.projectConfig.versions.mergeBody(merging.name)}</p>
    {#if mergeChoices.length === 0}
      <p class="error-text">{t.projectConfig.versions.mergeNoOther}</p>
    {:else}
      <div class="field">
        <span>{t.projectConfig.versions.mergeInto}</span>
        <Dropdown
          label={t.projectConfig.versions.mergeInto}
          value={into}
          options={mergeChoices.map((x) => ({ value: String(x.id), label: x.name }))}
          placeholder={t.catalogAdmin.choose}
          onchange={(v) => (into = v)}
        />
      </div>
    {/if}
  {/if}
</ConfirmDialog>

<ConfirmDialog
  open={removing !== null}
  title={t.projectConfig.versions.deleteTitle(removing?.name ?? '')}
  confirm={t.common.delete}
  onclose={() => (removing = null)}
  onconfirm={remove}
>
  {#if removing && removing.issues.total > 0}
    <p>{t.projectConfig.versions.deleteUsed(removing.issues.total)}</p>
    <div class="field">
      <span>{t.projectConfig.versions.deleteFix}</span>
      <Dropdown
        label={t.projectConfig.versions.deleteFix}
        value={fixTo}
        options={[
          { value: '', label: t.projectConfig.versions.deleteNone },
          ...removeChoices.map((x) => ({ value: String(x.id), label: x.name })),
        ]}
        onchange={(v) => (fixTo = v)}
      />
    </div>
    <div class="field">
      <span>{t.projectConfig.versions.deleteAffects}</span>
      <Dropdown
        label={t.projectConfig.versions.deleteAffects}
        value={affectsTo}
        options={[
          { value: '', label: t.projectConfig.versions.deleteNone },
          ...removeChoices.map((x) => ({ value: String(x.id), label: x.name })),
        ]}
        onchange={(v) => (affectsTo = v)}
      />
    </div>
  {:else}
    <p>{t.projectConfig.versions.deleteUnused}</p>
  {/if}
</ConfirmDialog>

<style>
  .order-hint {
    margin: 0 0 8px;
  }

  .c-order {
    width: 4rem;
    white-space: nowrap;
  }

  .when {
    white-space: nowrap;
  }

  .desc {
    display: block;
    margin-top: 2px;
    max-width: 50ch;
  }

  /* Five actions: let them wrap rather than widen the table. */
  td.row-actions {
    max-width: 17.5rem;
    white-space: normal;
  }
</style>
