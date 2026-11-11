<script lang="ts">
  import { untrack } from 'svelte';
  import { restrictionUsage } from '../../core/admin-forms.js';
  import type { BuiltinHolder, Group } from '../../core/admin-types.js';
  import { focusOrderAfterRemoval } from '../../core/focus.js';
  import { BUILTIN_HOLDERS, holderTag } from '../../core/project-admin.js';
  import { matches } from '../../core/text.js';
  import { formatList, formatNumber } from '../../i18n/format.js';
  import { holderLabels, t } from '../../i18n/t.js';
  import { refocusLater } from '../../lib/actions.js';
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
  let dialog = $state<{ group: Group | null; builtin?: BuiltinHolder } | null>(null);
  let removing = $state<Group | null>(null);
  let blocked = $state<{ comments: number; worklogs: number } | null>(null);
  let timer: ReturnType<typeof setTimeout> | undefined;
  let seq = 0;

  /** Everyone and the roles the search matches; they always come first. */
  const builtins = $derived(BUILTIN_HOLDERS.filter((b) => matches(searched, holderLabels[b])));

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

  const deleteId = (g: Group) => `group-delete-${g.id}`;

  function remove(g: Group) {
    blocked = null;
    removing = g;
  }

  async function confirmRemove() {
    if (!removing) return;
    const focus = focusOrderAfterRemoval(groups ?? [], removing, deleteId, 'group-new');
    try {
      await adminApi.deleteGroup(removing.id);
    } catch (e) {
      if (e instanceof ApiError && e.status === 409) blocked = restrictionUsage(e.data);
      throw e;
    }
    notify(t.adminArea.groupsPage.deleted(removing.name));
    await load();
    refocusLater(focus);
  }

  function saved(message: string) {
    notify(message);
    void load();
  }
</script>

{#snippet head()}
  <thead>
    <tr>
      <th>{t.adminArea.groupsPage.name}</th>
      <th class="c-description">{t.adminArea.groupsPage.description}</th>
      <th class="num">{t.adminArea.groupsPage.members}</th>
      <th><span class="sr-only">{t.common.actions}</span></th>
    </tr>
  </thead>
{/snippet}

<header class="page-head">
  <div>
    <p class="eyebrow">{t.adminArea.groups.people}</p>
    <h1 class="page-title">
      {t.adminArea.sections.groups}
      {#if groups}<span class="count muted">{formatNumber(groups.length)}</span>{/if}
    </h1>
  </div>
  <button id="group-new" class="btn btn-primary" onclick={() => (dialog = { group: null })}
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
{:else if groups.length === 0 && builtins.length === 0}
  <EmptyState kind="search" message={t.adminArea.noMatch}>
    <button class="btn" onclick={clear}>{t.adminArea.clearSearch}</button>
    <button class="btn btn-primary" onclick={() => (dialog = { group: null })}
      >{t.adminArea.groupsPage.newGroup}</button
    >
  </EmptyState>
{:else}
  {#if builtins.length > 0}
    <h2 class="section-title">{t.adminArea.groupsPage.builtinHeading}</h2>
    <div class="table-scroll">
      <table class="ledger groups">
        {@render head()}
        <tbody>
          {#each builtins as b (b)}
            <tr>
              <td>
                <button class="link-btn name" onclick={() => (dialog = { group: null, builtin: b })}
                  ><Highlight text={holderLabels[b]} search={searched} /></button
                >
                <span class="stamp">{holderTag(b, t.holderTags, holderLabels)}</span>
              </td>
              <td class="c-description">{t.adminArea.groupsPage.builtinDescriptions[b]}</td>
              <td class="num muted">—</td>
              <td class="row-actions">
                <button class="link-btn" onclick={() => (dialog = { group: null, builtin: b })}
                  >{t.common.edit}</button
                >
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
  {#if groups.length > 0}
    <h2 class="section-title">{t.holderTags.groups}</h2>
    <div class="table-scroll">
      <table class="ledger groups">
        {@render head()}
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
                <button id={deleteId(g)} class="link-btn" onclick={() => remove(g)}
                  >{t.common.delete}</button
                >
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {:else if searched === ''}
    <h2 class="section-title">{t.holderTags.groups}</h2>
    <EmptyState message={t.adminArea.groupsPage.empty} hint={t.adminArea.groupsPage.emptyHint}>
      <button class="btn btn-primary" onclick={() => (dialog = { group: null })}
        >{t.adminArea.groupsPage.newGroup}</button
      >
    </EmptyState>
  {/if}
{/if}

<GroupDialog
  open={dialog !== null}
  group={dialog?.group ?? null}
  builtin={dialog?.builtin ?? null}
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
