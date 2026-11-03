<script lang="ts">
  import { untrack } from 'svelte';
  import type { ProjectGroup } from '../../core/admin-types.js';
  import {
    cellAccess,
    cellState,
    grantsPayload,
    grantsSignature,
    matrixRows,
    nextCellState,
    nextTab,
    parseTab,
    PROJECT_TABS,
    removeRow,
    setCell,
    type CellState,
    type MatrixRow,
    type ProjectTab,
  } from '../../core/project-admin.js';
  import { paths } from '../../core/route.js';
  import type {
    Grant,
    Permission,
    PermUser,
    ProjectPermissions,
    UserRef,
  } from '../../core/types.js';
  import { holderLabels, permissionLabels, t } from '../../i18n/t.js';
  import { outside } from '../../lib/actions.js';
  import { projectAdminApi } from '../../lib/admin-api.js';
  import { api, ApiError, errorMessage } from '../../lib/api.js';
  import { router } from '../../lib/router.svelte.js';
  import { notify } from '../../lib/ui.svelte.js';
  import Dropdown from '../Dropdown.svelte';
  import Modal from '../Modal.svelte';
  import ComponentsPanel from '../project-settings/ComponentsPanel.svelte';
  import GeneralPanel from '../project-settings/GeneralPanel.svelte';
  import ImportPanel from '../project-settings/ImportPanel.svelte';
  import NotificationsPanel from '../project-settings/NotificationsPanel.svelte';
  import VersionsPanel from '../project-settings/VersionsPanel.svelte';
  import '../settings.css';
  import UserPicker from '../UserPicker.svelte';
  import WorkflowsPanel from '../workflow/WorkflowsPanel.svelte';
  import NotFoundPage from './NotFoundPage.svelte';

  /**
   * A project's rules, on one page: each permission with who holds it, and
   * the project's workflows. Nothing here is shared with another project —
   * the "no scheme" model. People who do not administer the project only
   * see the workflows, read-only.
   */
  interface Props {
    projectKey: string;
  }
  const { projectKey }: Props = $props();

  let data = $state<ProjectPermissions | null>(null);
  /** The groups with project members here, and those members. */
  let members = $state<ProjectGroup[]>([]);
  let grants = $state<Grant[]>([]);
  let saved = $state('');
  let error = $state('');
  let forbidden = $state(false);
  let saving = $state(false);
  /** What the users with a row inherit, loaded with the rules or when added. */
  let users = $state<PermUser[]>([]);
  /** Rows added and still without a rule: they go away on saving. */
  let extra = $state<string[]>([]);
  let adding = $state(false);
  let kind = $state<'group' | 'user'>('group');
  let pick = $state('');
  /** Read out after a cell changes: its new state. */
  let announced = $state('');
  /** The group whose "add a member" box is open, or 0 for the "add a group" box. */
  let addingTo = $state<number | null>(null);
  let memberKind = $state<'user' | 'group'>('user');
  let memberPick = $state('');
  let changing = $state(false);

  const dirty = $derived(grantsSignature(grants) !== saved);
  const rows = $derived(
    data
      ? matrixRows(
          {
            everyone: holderLabels.everyone,
            lead: holderLabels.lead,
            reporter: holderLabels.reporter,
            assignee: holderLabels.assignee,
          },
          grants,
          data.groups,
          users,
          extra,
        )
      : [],
  );

  const tab = $derived(parseTab(router.param('tab'), !forbidden));
  const tabLabels = $derived<Record<ProjectTab, string>>({
    ...t.projectConfig.tabs,
    rights: t.workflows.rightsTab,
    workflows: t.workflows.tab,
  });
  /** A tab has changes that are not saved yet. */
  let panelDirty = $state(false);
  let leaving = $state<(() => void) | null>(null);

  /** Runs `go`, after asking when a workflow has unsaved changes. */
  function guard(go: () => void) {
    if (panelDirty) leaving = go;
    else go();
  }

  // Another project's settings, reached while a tab has unsaved changes (the
  // back button, a link): the address goes back to where the page was and the
  // switch waits for the answer.
  let shown = { key: untrack(() => projectKey), at: router.here };
  $effect.pre(() => {
    const key = projectKey;
    const here = router.here;
    if (key === shown.key || !untrack(() => panelDirty)) {
      shown = { key, at: here };
      return;
    }
    leaving = () => router.navigate(here, { replace: true });
    router.navigate(shown.at, { replace: true });
  });

  function showTab(next: ProjectTab) {
    if (next === tab) return;
    guard(() => router.setParams({ tab: next === 'general' ? null : next, workflow: null }));
  }

  function tabKey(e: KeyboardEvent) {
    if (e.key !== 'ArrowLeft' && e.key !== 'ArrowRight') return;
    e.preventDefault();
    const next = nextTab(PROJECT_TABS, tab, e.key === 'ArrowRight' ? 1 : -1);
    showTab(next);
    queueMicrotask(() => document.getElementById(`tab-${next}`)?.focus());
  }

  // The guard above bounces back to the same project: that must not reload
  // the page and drop the unsaved tab.
  let loadedKey = '';
  $effect(() => {
    const key = projectKey;
    if (key === loadedKey) return;
    loadedKey = key;
    document.title = `${t.settings.title} ${key} — ${t.appName}`;
    data = null;
    members = [];
    grants = [];
    users = [];
    extra = [];
    saved = '';
    error = '';
    forbidden = false;
    adding = false;
    addingTo = null;
    api
      .permissions(key)
      .then((d) => {
        if (key !== projectKey) return;
        data = d;
        grants = d.grants;
        users = d.users;
        saved = grantsSignature(d.grants);
      })
      .catch((e: unknown) => {
        if (key !== projectKey) return;
        if (e instanceof ApiError && e.status === 403) forbidden = true;
        else error = errorMessage(e);
      });
    // Only shown beside the grants: the page works without it.
    projectAdminApi
      .groups(key)
      .then((list) => {
        if (key === projectKey) members = list.filter((g) => g.users.length || g.groups.length);
      })
      .catch(() => {});
  });

  function cycle(row: MatrixRow, p: Permission) {
    const next = nextCellState(cellState(grants, row.key, p));
    grants = setCell(grants, row, p, next);
    announced = cellName(row, p, next);
  }

  function dropRow(row: MatrixRow) {
    grants = removeRow(grants, row.key);
    extra = extra.filter((k) => k !== row.key);
  }

  function addRow(key: string) {
    if (!rows.some((r) => r.key === key)) extra = [...extra, key];
    adding = false;
    pick = '';
  }

  async function addUser(u: UserRef) {
    const key = projectKey;
    addRow(`user:${u.id}`);
    if (users.some((x) => x.id === u.id)) return;
    try {
      const pu = await api.permissionUser(key, u.id);
      if (key === projectKey && !users.some((x) => x.id === pu.id)) users = [...users, pu];
    } catch (e) {
      notify(errorMessage(e), 'error');
    }
  }

  /** The name of the holder a row key stands for. */
  function holderName(key: string): string {
    const [kind, rest] = key.split(':', 2);
    const id = Number(rest);
    if (kind === 'builtin') return holderLabels[rest as keyof typeof holderLabels] ?? key;
    if (kind === 'group') return data?.groups.find((g) => g.id === id)?.name ?? key;
    return rows.find((r) => r.key === key)?.label ?? key;
  }

  /** What an inherited cell gets: whether, and from which rule, short and in full. */
  function inherited(row: MatrixRow, p: Permission) {
    const s = t.settings.source;
    const a = cellAccess(row.key, p, { grants, globals: data?.globals ?? [], users }, false);
    if (!a) return { granted: null, short: s.loading, long: s.loading };
    if (a.level === 'none' || !a.source)
      return { granted: false, short: s.noneShort, long: s.none };
    const name = holderName(a.source);
    const say =
      a.level === 'globalUser'
        ? s.globalUser
        : a.level === 'projectGroup'
          ? s.projectGroup
          : s.globalGroup;
    return { granted: a.granted, short: name, long: say(a.granted, name) };
  }

  /** A cell's accessible name: the holder, the permission and the state. */
  function cellName(row: MatrixRow, p: Permission, state: CellState): string {
    const word =
      state === 'inherit' ? t.settings.inherited(inherited(row, p).long) : t.settings.states[state];
    return t.settings.cell(row.label, permissionLabels[p].label, word);
  }

  function openAdder(id: number) {
    addingTo = addingTo === id ? null : id;
    memberKind = 'user';
    memberPick = '';
  }

  function addGroup() {
    const found = data?.groups.find((x) => String(x.id) === memberPick);
    addingTo = null;
    if (!found || members.some((m) => m.id === found.id)) return;
    members = [
      ...members,
      {
        id: found.id,
        name: found.name,
        description: null,
        fromDirectory: false,
        users: [],
        groups: [],
      },
    ];
  }

  /** Saves a group's project members at once; on failure nothing changes. */
  async function setMembers(g: ProjectGroup, users: number[], groups: number[]) {
    const key = projectKey;
    changing = true;
    try {
      const next = await projectAdminApi.setGroupMembers(key, g.id, { users, groups });
      if (key === projectKey) members = members.map((m) => (m.id === g.id ? next : m));
      addingTo = null;
    } catch (e) {
      notify(errorMessage(e), 'error');
    } finally {
      changing = false;
    }
  }

  const userIds = (g: ProjectGroup) => g.users.map((u) => u.id);
  const groupIds = (g: ProjectGroup) => g.groups.map((x) => x.id);

  async function memberCandidates(g: ProjectGroup, q: string): Promise<UserRef[]> {
    const ids = new Set(userIds(g));
    return (await api.users(q)).filter((u) => !ids.has(u.id));
  }

  async function save() {
    const key = projectKey;
    saving = true;
    try {
      const d = await api.savePermissions(key, grantsPayload(grants));
      if (key === projectKey) {
        data = d;
        grants = d.grants;
        users = d.users;
        extra = [];
        saved = grantsSignature(d.grants);
      }
      notify(t.settings.saved);
    } catch (e) {
      notify(errorMessage(e), 'error');
    } finally {
      saving = false;
    }
  }
</script>

{#if error}
  <NotFoundPage message={error} />
{:else if !data && !forbidden}
  <div class="page"><p class="muted loading">{t.common.loading}</p></div>
{:else}
  <div class="page settings-area" data-title={`${t.settings.title} ${projectKey}`}>
    <header class="page-head">
      <div>
        <p class="eyebrow">
          <a href={paths.project(projectKey)}>{projectKey}</a> · {t.projects.settings}
        </p>
        <h1 class="page-title">{t.settings.title}</h1>
      </div>
      <a class="btn" href={paths.project(projectKey)}>{t.settings.backToProject}</a>
    </header>
    <p class="intro">{t.settings.intro}</p>

    {#if !forbidden}
      <div class="tabs-scroll">
        <div class="tabs" role="tablist" aria-label={t.workflows.tabsLabel}>
          {#each PROJECT_TABS as k (k)}
            <button
              type="button"
              class="tab"
              role="tab"
              id="tab-{k}"
              aria-controls="panel-{k}"
              aria-selected={tab === k}
              tabindex={tab === k ? 0 : -1}
              onclick={() => showTab(k)}
              onkeydown={tabKey}>{tabLabels[k]}</button
            >
          {/each}
        </div>
      </div>
    {/if}

    {#key projectKey}
      {#if tab === 'workflows' || !data}
        <div
          class="panel"
          role="tabpanel"
          id="panel-workflows"
          aria-labelledby={forbidden ? undefined : 'tab-workflows'}
        >
          <WorkflowsPanel {projectKey} {guard} ondirty={(d) => (panelDirty = d)} />
        </div>
      {:else if tab === 'general'}
        <div class="panel" role="tabpanel" id="panel-general" aria-labelledby="tab-general">
          <GeneralPanel {projectKey} ondirty={(d) => (panelDirty = d)} />
        </div>
      {:else if tab === 'components'}
        <div class="panel" role="tabpanel" id="panel-components" aria-labelledby="tab-components">
          <ComponentsPanel {projectKey} />
        </div>
      {:else if tab === 'versions'}
        <div class="panel" role="tabpanel" id="panel-versions" aria-labelledby="tab-versions">
          <VersionsPanel {projectKey} />
        </div>
      {:else if tab === 'notifications'}
        <div
          class="panel"
          role="tabpanel"
          id="panel-notifications"
          aria-labelledby="tab-notifications"
        >
          <NotificationsPanel {projectKey} groups={data.groups} ondirty={(d) => (panelDirty = d)} />
        </div>
      {:else if tab === 'import'}
        <div class="panel" role="tabpanel" id="panel-import" aria-labelledby="tab-import">
          <ImportPanel {projectKey} />
        </div>
      {:else}
        <div class="panel" role="tabpanel" id="panel-rights" aria-labelledby="tab-rights">
          <h2 class="section-title">{t.settings.permissions}</h2>
          <p class="matrix-hint">{t.settings.matrixHint}</p>
          <div class="table-scroll matrix-scroll">
            <table class="ledger matrix">
              <caption class="sr-only">{t.settings.matrix}</caption>
              <thead>
                <tr>
                  <th scope="col" class="who">{t.settings.holder}</th>
                  {#each data.permissions as p (p)}
                    <th scope="col" title={permissionLabels[p].help}>{permissionLabels[p].label}</th
                    >
                  {/each}
                </tr>
              </thead>
              <tbody>
                {#each rows as row (row.key)}
                  <tr>
                    <th scope="row" class="who">
                      <span class="who-line">
                        <span class="who-name">{row.label}</span>
                        {#if !row.builtin}
                          <button
                            type="button"
                            class="drop"
                            aria-label={t.settings.removeRow(row.label)}
                            title={t.settings.removeRow(row.label)}
                            onclick={() => dropRow(row)}>×</button
                          >
                        {/if}
                      </span>
                      <span class="kind"
                        >{row.builtin ? t.builtInGroups.mark : holderLabels[row.holder]}</span
                      >
                    </th>
                    {#each data.permissions as p (p)}
                      {@const state = cellState(grants, row.key, p)}
                      {@const from = state === 'inherit' ? inherited(row, p) : null}
                      <td>
                        <button
                          type="button"
                          class="cell {state}"
                          aria-label={cellName(row, p, state)}
                          title={from ? from.long : undefined}
                          onclick={() => cycle(row, p)}
                        >
                          <span class="word"
                            ><span class="mark" aria-hidden="true"
                              >{state === 'allow' ? '✓' : state === 'deny' ? '⊘' : '–'}</span
                            >{t.settings.states[state]}</span
                          >
                          {#if from}
                            <span class="from" class:no={from.granted === false}
                              ><span aria-hidden="true"
                                >{from.granted === null ? '' : from.granted ? '✓' : '✗'}</span
                              >
                              {from.short}</span
                            >
                          {/if}
                        </button>
                      </td>
                    {/each}
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>
          <p class="sr-only" aria-live="polite">{announced}</p>
          <div class="adder row-adder" use:outside={() => adding && (adding = false)}>
            <button
              type="button"
              class="link-btn"
              aria-expanded={adding}
              onclick={() => {
                adding = !adding;
                kind = 'group';
                pick = '';
              }}>+ {t.settings.addRow}</button
            >
            {#if adding}
              <div class="popover add">
                <div class="field">
                  <span>{t.settings.rowKind}</span>
                  <Dropdown
                    label={t.settings.rowKind}
                    value={kind}
                    options={(['group', 'user'] as const).map((k) => ({
                      value: k,
                      label: holderLabels[k],
                    }))}
                    onchange={(v) => {
                      kind = v as 'group' | 'user';
                      pick = '';
                    }}
                  />
                </div>
                {#if kind === 'group'}
                  <div class="field">
                    <span>{t.settings.chooseGroup}</span>
                    <Dropdown
                      label={t.settings.chooseGroup}
                      value={pick}
                      search
                      options={data.groups
                        .filter((x) => !rows.some((r) => r.key === `group:${x.id}`))
                        .map((x) => ({ value: String(x.id), label: x.name }))}
                      placeholder="…"
                      onchange={(v) => (pick = v)}
                    />
                  </div>
                  <button
                    class="btn btn-small btn-primary"
                    disabled={!pick}
                    onclick={() => addRow(`group:${pick}`)}>{t.common.add}</button
                  >
                {:else}
                  <div class="field">
                    <span class="field-label">{t.settings.chooseUser}</span>
                    <UserPicker
                      field
                      value={null}
                      label={t.settings.chooseUser}
                      allowNone={false}
                      noneLabel="…"
                      search={async (q) =>
                        (await api.users(q)).filter(
                          (u) => !rows.some((r) => r.key === `user:${u.id}`),
                        )}
                      onpick={(u: UserRef | null) => u && void addUser(u)}
                    />
                  </div>
                {/if}
              </div>
            {/if}
          </div>

          <h2 class="section-title">{t.settings.projectMembers}</h2>
          {#if members.length}
            <dl class="roles">
              {#each members as r (r.id)}
                <div>
                  <dt>{r.name}</dt>
                  <dd>
                    {#each r.groups as g (g.id)}<span class="chip"
                        ><span class="kind">{holderLabels.group}</span>
                        {g.name}<button
                          type="button"
                          disabled={changing}
                          aria-label={`${t.common.remove} ${g.name}`}
                          onclick={() =>
                            void setMembers(
                              r,
                              userIds(r),
                              groupIds(r).filter((x) => x !== g.id),
                            )}>×</button
                        ></span
                      >{/each}
                    {#each r.users as u (u.id)}<span class="chip"
                        >{u.name}<button
                          type="button"
                          disabled={changing}
                          aria-label={`${t.common.remove} ${u.name}`}
                          onclick={() =>
                            void setMembers(
                              r,
                              userIds(r).filter((x) => x !== u.id),
                              groupIds(r),
                            )}>×</button
                        ></span
                      >{/each}
                    {#if !r.users.length && !r.groups.length}
                      <span class="nobody">{t.settings.noMembers}</span>
                    {/if}
                    <span class="adder" use:outside={() => addingTo === r.id && (addingTo = null)}>
                      <button
                        type="button"
                        class="link-btn"
                        aria-expanded={addingTo === r.id}
                        aria-label={t.settings.addMemberTo(r.name)}
                        onclick={() => openAdder(r.id)}>+ {t.settings.addMember}</button
                      >
                      {#if addingTo === r.id}
                        <div class="popover add">
                          <div class="field">
                            <span>{t.settings.memberKind}</span>
                            <Dropdown
                              label={t.settings.memberKind}
                              value={memberKind}
                              options={(['user', 'group'] as const).map((k) => ({
                                value: k,
                                label: holderLabels[k],
                              }))}
                              onchange={(v) => {
                                memberKind = v as 'user' | 'group';
                                memberPick = '';
                              }}
                            />
                          </div>
                          {#if memberKind === 'user'}
                            <div class="field">
                              <span class="field-label">{t.settings.chooseUser}</span>
                              <UserPicker
                                field
                                value={null}
                                label={t.settings.chooseUser}
                                allowNone={false}
                                noneLabel="…"
                                disabled={changing}
                                search={(q) => memberCandidates(r, q)}
                                onpick={(u: UserRef | null) =>
                                  u && void setMembers(r, [...userIds(r), u.id], groupIds(r))}
                              />
                            </div>
                          {:else}
                            <div class="field">
                              <span>{t.settings.chooseGroup}</span>
                              <Dropdown
                                label={t.settings.chooseGroup}
                                value={memberPick}
                                search
                                options={data.groups
                                  .filter((x) => x.id !== r.id && !groupIds(r).includes(x.id))
                                  .map((x) => ({ value: String(x.id), label: x.name }))}
                                placeholder="…"
                                onchange={(v) => (memberPick = v)}
                              />
                            </div>
                            <button
                              class="btn btn-small btn-primary"
                              disabled={!memberPick || changing}
                              onclick={() =>
                                void setMembers(r, userIds(r), [
                                  ...groupIds(r),
                                  Number(memberPick),
                                ])}>{t.common.add}</button
                            >
                          {/if}
                        </div>
                      {/if}
                    </span>
                  </dd>
                </div>
              {/each}
            </dl>
          {:else}
            <p class="nobody">{t.settings.noProjectMembers}</p>
          {/if}
          <div class="adder group-adder" use:outside={() => addingTo === 0 && (addingTo = null)}>
            <button
              type="button"
              class="link-btn"
              aria-expanded={addingTo === 0}
              onclick={() => openAdder(0)}>+ {t.settings.addGroup}</button
            >
            {#if addingTo === 0}
              <div class="popover add">
                <div class="field">
                  <span>{t.settings.chooseGroup}</span>
                  <Dropdown
                    label={t.settings.chooseGroup}
                    value={memberPick}
                    search
                    options={data.groups
                      .filter((x) => !members.some((m) => m.id === x.id))
                      .map((x) => ({ value: String(x.id), label: x.name }))}
                    placeholder="…"
                    onchange={(v) => (memberPick = v)}
                  />
                </div>
                <button class="btn btn-small btn-primary" disabled={!memberPick} onclick={addGroup}
                  >{t.common.add}</button
                >
              </div>
            {/if}
          </div>

          {#if dirty}
            <div class="savebar" role="region" aria-label={t.settings.unsaved}>
              <span>{t.settings.unsaved}</span>
              <button
                class="btn btn-quiet"
                onclick={() => {
                  if (!data) return;
                  grants = data.grants;
                  extra = [];
                }}>{t.settings.discard}</button
              >
              <button class="btn btn-primary" disabled={saving} onclick={() => void save()}
                >{saving ? t.common.saving : t.common.save}</button
              >
            </div>
          {/if}
        </div>
      {/if}
    {/key}
  </div>

  <Modal open={leaving !== null} title={t.workflows.leaveTitle} onclose={() => (leaving = null)}>
    <p class="leave">{tab === 'workflows' ? t.workflows.leaveBody : t.projectConfig.leaveBody}</p>
    {#snippet footer()}
      <button type="button" class="btn btn-quiet" onclick={() => (leaving = null)}
        >{t.workflows.stay}</button
      >
      <button
        type="button"
        class="btn btn-danger"
        onclick={() => {
          const go = leaving;
          leaving = null;
          panelDirty = false;
          go?.();
        }}>{t.workflows.leave}</button
      >
    {/snippet}
  </Modal>
{/if}

<style>
  .intro {
    max-width: 70ch;
    margin: 0 0 6px;
    font: var(--title-italic) 0.9375rem/1.5 var(--font-title);
    color: var(--ink-soft);
  }

  .matrix-hint {
    max-width: 70ch;
    margin: 0 0 10px;
    font-size: 0.8125rem;
    color: var(--muted);
  }

  /* The matrix scrolls in its own box, both ways: the holders stay on the
     left and the permissions on top. */
  .matrix-scroll {
    max-height: 70vh;
    overflow: auto;
    border: 1px solid var(--rule);
    background: var(--paper);
  }

  .matrix {
    width: max-content;
    min-width: 100%;
    border-collapse: separate;
    border-spacing: 0;
  }

  .matrix thead th {
    position: sticky;
    top: 0;
    z-index: 2;
    min-width: 6.5rem;
    max-width: 8rem;
    padding: 8px 6px;
    white-space: normal;
    line-height: 1.2;
    vertical-align: bottom;
    background: var(--paper);
  }

  .matrix .who {
    position: sticky;
    left: 0;
    z-index: 1;
    min-width: 11rem;
    max-width: 14rem;
    padding: 6px 10px;
    text-align: left;
    text-transform: none;
    letter-spacing: 0;
    background: var(--paper);
    border-right: 1px solid var(--rule);
    border-bottom: 1px solid var(--rule);
  }

  .matrix thead .who {
    z-index: 3;
  }

  .who-line {
    display: flex;
    align-items: center;
    gap: 4px;
  }

  .who-name {
    font: 600 0.84375rem/1.3 var(--font-body);
    color: var(--ink);
    overflow-wrap: anywhere;
  }

  .who .kind {
    display: block;
    font: 500 0.6875rem/1.2 var(--font-body);
    color: var(--muted);
  }

  .drop {
    margin-left: auto;
    padding: 0 4px;
    border: 0;
    background: none;
    color: var(--muted);
    font-size: 1rem;
    line-height: 1;
    cursor: pointer;
  }

  .drop:hover {
    color: var(--accent-text);
  }

  .matrix td {
    padding: 4px;
    vertical-align: middle;
  }

  .matrix tbody tr:hover td {
    background: none;
  }

  /* Allow is filled, deny outlined with a struck pattern, inherit plain:
     each also has its own mark and word, so color is never the only cue. */
  .cell {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: 2px;
    width: 100%;
    min-height: 2.75rem;
    padding: 4px 7px;
    border: 1px solid var(--rule);
    border-radius: var(--radius-s, 3px);
    background: transparent;
    color: var(--muted);
    font: 400 0.75rem/1.2 var(--font-body);
    text-align: left;
    cursor: pointer;
  }

  .cell .word {
    display: inline-flex;
    gap: 4px;
    font-weight: 600;
  }

  .cell .mark {
    width: 1em;
    text-align: center;
  }

  .cell.allow {
    border-color: var(--stamp-done);
    background: var(--stamp-done);
    color: var(--paper);
  }

  .cell.deny {
    border: 2px solid var(--accent-text);
    background: repeating-linear-gradient(135deg, transparent 0 6px, var(--accent-wash) 6px 9px);
    color: var(--accent-text);
  }

  .cell .from {
    font-size: 0.6875rem;
    color: var(--ink-soft);
    overflow-wrap: anywhere;
  }

  .cell .from.no {
    color: var(--muted);
  }

  .cell:hover {
    border-color: var(--rule-strong);
  }

  .row-adder {
    display: inline-block;
    margin: 8px 0 18px;
  }

  .nobody {
    color: var(--muted);
    font-style: italic;
    font-size: 0.8125rem;
  }

  .adder {
    position: relative;
    font-size: 0.78125rem;
  }

  .add {
    left: 0;
    top: calc(100% + 6px);
    width: 17.5rem;
    padding: 12px;
    display: grid;
    gap: 10px;
    justify-items: start;
  }

  .add .field {
    width: 100%;
  }

  .roles {
    margin: 0;
    display: grid;
    gap: 6px;
  }

  .roles > div {
    display: grid;
    grid-template-columns: 12.5rem 1fr;
    gap: 10px;
    padding: 6px 0;
    border-bottom: 1px solid var(--rule);
  }

  .roles dt {
    font-weight: 600;
  }

  .roles dd {
    margin: 0;
    display: flex;
    flex-wrap: wrap;
    gap: 4px;
    align-items: center;
    min-width: 0;
  }

  .roles .kind {
    font: 500 0.6875rem/1 var(--font-body);
    color: var(--muted);
  }

  .group-adder {
    display: inline-block;
    margin-top: 8px;
  }

  /* On a phone the group's name goes above its members, and "add a member"
     starts its own line, so its box opens at the left edge and fits. */
  @media (max-width: 600px) {
    .roles > div {
      grid-template-columns: 1fr;
      gap: 4px;
    }

    .roles .adder {
      flex-basis: 100%;
    }
  }

  /* On a phone the tabs scroll sideways rather than widen the page. The box
     that scrolls is around the tab list, so the skins' tabs can still reach
     over their rule; its padding keeps the tabs that hang lower (Retro), and
     the panel's margin gives that padding back. */
  .tabs-scroll {
    margin: 18px 0 4px;
    padding-bottom: 2px;
    overflow-x: auto;
  }

  .tabs-scroll + .panel {
    margin-top: -2px;
  }

  .tabs {
    width: max-content;
    min-width: 100%;
  }

  .tab:focus-visible {
    outline-offset: -2px;
  }

  .leave {
    margin: 0;
  }
</style>
