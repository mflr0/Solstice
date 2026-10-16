<script lang="ts">
  import type { Group, GroupDetail } from '../../core/admin-types.js';
  import { filterGroups, groupNameProblem, namedPayload } from '../../core/people-admin.js';
  import type { UserRef } from '../../core/types.js';
  import { formatNumber } from '../../i18n/format.js';
  import { t } from '../../i18n/t.js';
  import { adminApi } from '../../lib/admin-api.js';
  import { errorMessage } from '../../lib/api.js';
  import { router } from '../../lib/router.svelte.js';
  import { notify } from '../../lib/ui.svelte.js';
  import EmptyState from '../EmptyState.svelte';
  import UserPicker from '../UserPicker.svelte';
  import Who from '../Who.svelte';
  import GroupDeleteDialog from './GroupDeleteDialog.svelte';
  import NamedDialog from './NamedDialog.svelte';

  /**
   * Groups: find, create, rename and delete them (after showing where one is
   * used), and, beside the list, the members of the chosen group, added and
   * removed one by one. The chosen group is kept in the address (?group=).
   */
  let groups = $state<Group[] | null>(null);
  let error = $state('');
  let q = $state(router.param('q') ?? '');
  let selected = $state<number | null>(parseId(router.param('group')));
  let detail = $state<GroupDetail | null>(null);
  let detailError = $state('');
  let dialog = $state<{ group: Group | null } | null>(null);
  let checking = $state<number | null>(null);
  let deleting = $state<{ group: Group; detail: GroupDetail } | null>(null);
  let busyMember = $state<number | null>(null);
  let timer: ReturnType<typeof setTimeout> | undefined;
  let seq = 0;

  const list = $derived(filterGroups(groups ?? [], q));
  const current = $derived(groups?.find((g) => g.id === selected) ?? null);

  function parseId(param: string | null): number | null {
    const n = Number(param);
    return param && Number.isInteger(n) && n > 0 ? n : null;
  }

  async function load() {
    try {
      groups = await adminApi.groups();
      error = '';
    } catch (e) {
      error = errorMessage(e);
    }
  }

  async function loadDetail(id: number | null) {
    const mine = ++seq;
    detailError = '';
    if (id === null) {
      detail = null;
      return;
    }
    try {
      const found = await adminApi.group(id);
      if (mine === seq) detail = found;
    } catch (e) {
      if (mine === seq) {
        detail = null;
        detailError = errorMessage(e);
      }
    }
  }

  $effect(() => {
    void load();
  });

  $effect(() => {
    void loadDetail(selected);
  });

  /** A group that no longer exists (deleted elsewhere) is dropped from the address. */
  $effect(() => {
    if (groups && selected !== null && !current) choose(null);
  });

  function choose(id: number | null) {
    selected = id;
    router.setParams({ group: id === null ? null : String(id) });
  }

  function typed() {
    clearTimeout(timer);
    timer = setTimeout(() => router.setParams({ q: q.trim() || null }), 250);
  }

  function clear() {
    q = '';
    router.setParams({ q: null });
  }

  const problem = (name: string) => {
    const p = groupNameProblem(name, groups ?? [], dialog?.group?.id ?? null);
    return p ? t.groupsAdmin.problems[p] : '';
  };

  async function save(input: { name: string; description: string }) {
    const group = dialog?.group ?? null;
    const body = namedPayload(input.name, input.description);
    const saved = group
      ? await adminApi.updateGroup(group.id, body)
      : await adminApi.createGroup(body);
    notify(group ? t.groupsAdmin.saved(saved.name) : t.groupsAdmin.created(saved.name));
    await load();
    if (!group) choose(saved.id);
    else if (saved.id === selected) void loadDetail(saved.id);
  }

  /** Reads where the group is used, then opens the matching confirmation. */
  async function askDelete(g: Group) {
    checking = g.id;
    try {
      deleting = { group: g, detail: await adminApi.group(g.id) };
    } catch (e) {
      notify(errorMessage(e), 'error');
    } finally {
      checking = null;
    }
  }

  async function deleted(name: string) {
    notify(t.groupsAdmin.deleted(name));
    await load();
  }

  async function searchPeople(term: string): Promise<UserRef[]> {
    const have = new Set((detail?.members ?? []).map((m) => m.id));
    const found = await adminApi.users(term);
    return found
      .filter((u) => !have.has(u.id))
      .map((u) => ({ id: u.id, username: u.username, name: u.name, active: u.active }));
  }

  async function addMember(u: UserRef | null) {
    if (!u || !current) return;
    busyMember = u.id;
    try {
      detail = await adminApi.addMembers(current.id, [u.id]);
      notify(t.groupsAdmin.memberAdded(u.name, current.name));
      await load();
    } catch (e) {
      notify(errorMessage(e), 'error');
    } finally {
      busyMember = null;
    }
  }

  async function removeMember(u: UserRef) {
    if (!current) return;
    busyMember = u.id;
    try {
      await adminApi.removeMember(current.id, u.id);
      notify(t.groupsAdmin.memberRemoved(u.name, current.name));
      await Promise.all([load(), loadDetail(current.id)]);
    } catch (e) {
      notify(errorMessage(e), 'error');
    } finally {
      busyMember = null;
    }
  }
</script>

<header class="page-head">
  <div>
    <p class="eyebrow">{t.adminArea.groups.people}</p>
    <h1 class="page-title">
      {t.groupsAdmin.title}
      {#if groups}<span class="count muted">{formatNumber(groups.length)}</span>{/if}
    </h1>
  </div>
  <button class="btn btn-primary" onclick={() => (dialog = { group: null })}
    >{t.groupsAdmin.newGroup}</button
  >
</header>
<p class="lead-text">{t.groupsAdmin.intro}</p>

{#if error}
  <p class="error-text" role="alert">{error}</p>
{:else if !groups}
  <p class="muted loading">{t.common.loading}</p>
{:else if groups.length === 0}
  <EmptyState message={t.groupsAdmin.empty} hint={t.groupsAdmin.emptyHint}>
    <button class="btn btn-primary" onclick={() => (dialog = { group: null })}
      >{t.groupsAdmin.newGroup}</button
    >
  </EmptyState>
{:else}
  <div class="list-tools">
    <input
      class="input search"
      type="search"
      bind:value={q}
      oninput={typed}
      placeholder={t.groupsAdmin.search}
      aria-label={t.groupsAdmin.search}
    />
  </div>

  <div class="split" class:has-panel={current !== null}>
    <div class="list">
      {#if list.length === 0}
        <EmptyState kind="search" message={t.adminArea.noMatch}>
          <button class="btn" onclick={clear}>{t.adminArea.clearSearch}</button>
        </EmptyState>
      {:else}
        <div class="table-scroll">
          <table class="ledger groups">
            <thead>
              <tr>
                <th>{t.groupsAdmin.name}</th>
                <th class="c-desc">{t.groupsAdmin.description}</th>
                <th class="num">{t.groupsAdmin.members}</th>
                <th><span class="sr-only">{t.common.actions}</span></th>
              </tr>
            </thead>
            <tbody>
              {#each list as g (g.id)}
                <tr class:is-selected={g.id === selected}>
                  <td>
                    <button
                      class="link-btn name"
                      aria-pressed={g.id === selected}
                      onclick={() => choose(g.id)}>{g.name}</button
                    >
                  </td>
                  <td class="c-desc muted">{g.description ?? '—'}</td>
                  <td class="num" class:muted={g.members === 0}>{formatNumber(g.members)}</td>
                  <td class="row-actions">
                    <button class="link-btn" onclick={() => choose(g.id)}
                      >{t.groupsAdmin.manage}</button
                    >
                    <button class="link-btn" onclick={() => (dialog = { group: g })}
                      >{t.common.edit}</button
                    >
                    <button
                      class="link-btn"
                      disabled={checking === g.id}
                      onclick={() => void askDelete(g)}>{t.common.delete}</button
                    >
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {/if}
    </div>

    {#if current}
      <section class="panel" aria-labelledby="members-title">
        <div class="panel-head">
          <h2 class="section-title" id="members-title">
            {t.groupsAdmin.membersTitle(current.name)}
          </h2>
          <button class="link-btn" onclick={() => choose(null)}>{t.groupsAdmin.closePanel}</button>
        </div>
        <p class="muted small">{t.groupsAdmin.membersIntro}</p>
        <UserPicker
          field
          value={null}
          label={t.groupsAdmin.addMemberLabel}
          noneLabel={t.groupsAdmin.addMember}
          allowNone={false}
          search={searchPeople}
          onpick={(u) => void addMember(u)}
        />
        {#if detailError}
          <p class="error-text" role="alert">{detailError}</p>
        {:else if !detail}
          <p class="muted loading">{t.common.loading}</p>
        {:else if detail.members.length === 0}
          <p class="muted">{t.groupsAdmin.noMembers}</p>
        {:else}
          <ul class="members">
            {#each detail.members as m (m.id)}
              <li>
                <Who user={m} />
                <span class="muted user">{m.username}</span>
                <button
                  class="link-btn"
                  disabled={busyMember === m.id}
                  aria-label={t.groupsAdmin.removeMember(m.name)}
                  onclick={() => void removeMember(m)}>{t.common.remove}</button
                >
              </li>
            {/each}
          </ul>
        {/if}
      </section>
    {/if}
  </div>
{/if}

<NamedDialog
  open={dialog !== null}
  title={dialog?.group ? t.groupsAdmin.editTitle(dialog.group.name) : t.groupsAdmin.createTitle}
  name={dialog?.group?.name ?? ''}
  description={dialog?.group?.description ?? ''}
  {problem}
  hint={t.groupsAdmin.nameHint}
  nameLabel={t.groupsAdmin.name}
  descriptionLabel={t.groupsAdmin.description}
  submit={dialog?.group ? t.common.save : t.groupsAdmin.create}
  onsave={save}
  onclose={() => (dialog = null)}
/>

<GroupDeleteDialog
  group={deleting?.group ?? null}
  detail={deleting?.detail ?? null}
  ondeleted={(name) => void deleted(name)}
  onclose={() => (deleting = null)}
/>

<style>
  .count {
    margin-left: 6px;
    font: 500 14px/1 var(--font-mono);
  }

  .split {
    display: grid;
    gap: 24px;
    align-items: start;
  }

  .split.has-panel {
    grid-template-columns: minmax(0, 1fr) minmax(280px, 380px);
  }

  .name {
    font-weight: 600;
    color: var(--ink);
    text-decoration: none;
    text-align: left;
  }

  .name:hover {
    text-decoration: underline;
  }

  tr.is-selected {
    background: var(--paper-sunk);
  }

  .c-desc {
    max-width: 40ch;
  }

  .panel {
    display: grid;
    gap: 10px;
    border: var(--frame) solid var(--rule-strong);
    border-radius: var(--radius-xs);
    background: var(--paper-raised);
    padding: 14px 16px;
  }

  .panel-head {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: 12px;
  }

  .panel-head .section-title,
  .panel p {
    margin: 0;
  }

  .members {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    max-height: 420px;
    overflow: auto;
  }

  .members li {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 6px 0;
    border-top: 1px solid var(--rule);
    min-width: 0;
  }

  .members li:first-child {
    border-top: 0;
  }

  .members .user {
    font: 11.5px var(--font-mono);
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .members .link-btn {
    margin-left: auto;
  }

  @media (max-width: 900px) {
    .split.has-panel {
      grid-template-columns: minmax(0, 1fr);
    }

    .c-desc {
      display: none;
    }
  }
</style>
