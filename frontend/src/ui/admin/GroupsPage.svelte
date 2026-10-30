<script lang="ts">
  import { untrack } from 'svelte';
  import { restrictionUsage } from '../../core/admin-forms.js';
  import type { Group } from '../../core/admin-types.js';
  import { formatList, formatNumber } from '../../i18n/format.js';
  import { t } from '../../i18n/t.js';
  import { adminApi } from '../../lib/admin-api.js';
  import { ApiError, errorMessage } from '../../lib/api.js';
  import { router } from '../../lib/router.svelte.js';
  import { notify } from '../../lib/ui.svelte.js';
  import ConfirmDialog from '../ConfirmDialog.svelte';
  import EmptyState from '../EmptyState.svelte';
  import Highlight from '../Highlight.svelte';
  import GroupDialog from './GroupDialog.svelte';

  let groups = $state<Group[] | null>(null);
  let error = $state('');
  let q = $state(router.param('q') ?? '');
  /** The search the shown rows were loaded with. */
  let searched = $state('');
  let dialog = $state<{ group: Group | null } | null>(null);
  let removing = $state<Group | null>(null);
  let blocked = $state<{ comments: number; worklogs: number } | null>(null);
  let timer: ReturnType<typeof setTimeout> | undefined;
  let seq = 0;

  const restrictedTo = $derived(
    blocked
      ? formatList(
          [
            blocked.comments > 0 ? t.adminArea.restricted.comments(blocked.comments) : '',
            blocked.worklogs > 0 ? t.adminArea.restricted.worklogs(blocked.worklogs) : '',
          ].filter(Boolean),
        )
      : '',
  );

  async function load() {
    const mine = ++seq;
    try {
      const search = q.trim();
      const found = await adminApi.groups(search);
      if (mine === seq) {
        groups = found;
        searched = search;
        error = '';
      }
    } catch (e) {
      if (mine === seq) error = errorMessage(e);
    }
  }

  $effect(() => {
    untrack(() => void load());
  });

  function typed() {
    clearTimeout(timer);
    timer = setTimeout(() => {
      router.setParams({ q: q.trim() || null });
      void load();
    }, 250);
  }

  function clear() {
    q = '';
    router.setParams({ q: null });
    void load();
  }

  function remove(g: Group) {
    blocked = null;
    removing = g;
  }

  async function confirmRemove() {
    if (!removing) return;
    try {
      await adminApi.deleteGroup(removing.id);
    } catch (e) {
      if (e instanceof ApiError && e.status === 409) blocked = restrictionUsage(e.data);
      throw e;
    }
    notify(t.adminArea.groupsPage.deleted(removing.name));
    void load();
  }

  function saved(message: string) {
    notify(message);
    void load();
  }
</script>

<header class="page-head">
  <div>
    <p class="eyebrow">{t.adminArea.groups.people}</p>
    <h1 class="page-title">
      {t.adminArea.sections.groups}
      {#if groups}<span class="count muted">{formatNumber(groups.length)}</span>{/if}
    </h1>
  </div>
  <button class="btn btn-primary" onclick={() => (dialog = { group: null })}
    >{t.adminArea.groupsPage.newGroup}</button
  >
</header>
<p class="lead-text">{t.adminArea.groupsPage.intro}</p>

<div class="list-tools">
  <input
    class="input search"
    type="search"
    bind:value={q}
    oninput={typed}
    placeholder={t.adminArea.groupsPage.search}
    aria-label={t.adminArea.groupsPage.search}
  />
</div>

{#if error}
  <p class="error-text" role="alert">{error}</p>
{:else if !groups}
  <p class="muted loading">{t.common.loading}</p>
{:else if groups.length === 0 && q.trim() === ''}
  <EmptyState message={t.adminArea.groupsPage.empty} hint={t.adminArea.groupsPage.emptyHint}>
    <button class="btn btn-primary" onclick={() => (dialog = { group: null })}
      >{t.adminArea.groupsPage.newGroup}</button
    >
  </EmptyState>
{:else if groups.length === 0}
  <EmptyState kind="search" message={t.adminArea.noMatch}>
    <button class="btn" onclick={clear}>{t.adminArea.clearSearch}</button>
    <button class="btn btn-primary" onclick={() => (dialog = { group: null })}
      >{t.adminArea.groupsPage.newGroup}</button
    >
  </EmptyState>
{:else}
  <div class="table-scroll">
    <table class="ledger groups">
      <thead>
        <tr>
          <th>{t.adminArea.groupsPage.name}</th>
          <th class="c-description">{t.adminArea.groupsPage.description}</th>
          <th class="num">{t.adminArea.groupsPage.members}</th>
          <th><span class="sr-only">{t.common.actions}</span></th>
        </tr>
      </thead>
      <tbody>
        {#each groups as g (g.id)}
          <tr>
            <td>
              <button class="link-btn name" onclick={() => (dialog = { group: g })}
                ><Highlight text={g.name} search={searched} /></button
              >
            </td>
            <td class="c-description">{g.description || '—'}</td>
            <td class="num">{t.adminArea.groupsPage.count(g.members)}</td>
            <td class="row-actions">
              <button class="link-btn" onclick={() => (dialog = { group: g })}
                >{t.common.edit}</button
              >
              <button class="link-btn" onclick={() => remove(g)}>{t.common.delete}</button>
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
{/if}

<GroupDialog
  open={dialog !== null}
  group={dialog?.group ?? null}
  onclose={() => (dialog = null)}
  onsaved={saved}
  onchanged={() => void load()}
/>

<ConfirmDialog
  open={removing !== null}
  title={t.adminArea.groupsPage.deleteTitle(removing?.name ?? '')}
  confirm={t.common.delete}
  onclose={() => (removing = null)}
  onconfirm={confirmRemove}
>
  <p>{t.adminArea.groupsPage.deleteBody}</p>
  {#if blocked}<p>{t.adminArea.groupsPage.inUse(restrictedTo)}</p>{/if}
</ConfirmDialog>

<style>
  .count {
    margin-left: 6px;
    font: 500 0.875rem/1 var(--font-mono);
  }

  .name {
    display: block;
    font-weight: 600;
    color: var(--ink);
    text-decoration: none;
    text-align: left;
  }

  .name:hover {
    text-decoration: underline;
  }

  @media (max-width: 760px) {
    .c-description {
      display: none;
    }
  }
</style>
