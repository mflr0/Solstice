<script lang="ts">
  import { nextTab, parseTab, type ProjectTab } from '../../core/project-admin.js';
  import { paths } from '../../core/route.js';
  import type { Grant, Holder, Permission, ProjectPermissions, UserRef } from '../../core/types.js';
  import { grantLabel, holderLabels, permissionLabels, t } from '../../i18n/t.js';
  import { outside } from '../../lib/actions.js';
  import { api, ApiError, errorMessage } from '../../lib/api.js';
  import { router } from '../../lib/router.svelte.js';
  import { notify } from '../../lib/ui.svelte.js';
  import Modal from '../Modal.svelte';
  import ComponentsPanel from '../project-settings/ComponentsPanel.svelte';
  import GeneralPanel from '../project-settings/GeneralPanel.svelte';
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
  let grants = $state<Grant[]>([]);
  let saved = $state('');
  let error = $state('');
  let forbidden = $state(false);
  let saving = $state(false);
  let adding = $state<Permission | null>(null);
  let kind = $state<Holder>('group');
  let pick = $state('');

  const signature = (list: Grant[]) =>
    list
      .map((g) => `${g.permission}:${g.holder}:${g.id ?? ''}`)
      .sort()
      .join('|');
  const dirty = $derived(signature(grants) !== saved);

  const tabs: ProjectTab[] = ['general', 'components', 'versions', 'rights', 'workflows'];
  const asked = $derived(parseTab(router.param('tab'), !forbidden));
  const tab = $derived(tabs.includes(asked) ? asked : 'general');
  const tabLabels = $derived<Partial<Record<ProjectTab, string>>>({
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

  function showTab(next: ProjectTab) {
    if (next === tab) return;
    guard(() => router.setParams({ tab: next === 'general' ? null : next, workflow: null }));
  }

  function tabKey(e: KeyboardEvent) {
    if (e.key !== 'ArrowLeft' && e.key !== 'ArrowRight') return;
    e.preventDefault();
    const next = nextTab(tabs, tab, e.key === 'ArrowRight' ? 1 : -1);
    showTab(next);
    queueMicrotask(() => document.getElementById(`tab-${next}`)?.focus());
  }

  $effect(() => {
    const key = projectKey;
    document.title = `${t.settings.title} ${key} — ${t.appName}`;
    api
      .permissions(key)
      .then((d) => {
        data = d;
        grants = d.grants;
        saved = signature(d.grants);
      })
      .catch((e: unknown) => {
        if (e instanceof ApiError && e.status === 403) forbidden = true;
        else error = errorMessage(e);
      });
  });

  const kinds: Holder[] = ['everyone', 'group', 'role', 'user', 'lead', 'reporter', 'assignee'];

  function holdersOf(p: Permission): Grant[] {
    return grants.filter((g) => g.permission === p);
  }

  function remove(g: Grant) {
    grants = grants.filter((x) => x !== g);
  }

  function add(p: Permission, g: Omit<Grant, 'permission'>) {
    const grant: Grant = { permission: p, ...g };
    if (!grants.some((x) => signature([x]) === signature([grant]))) grants = [...grants, grant];
    adding = null;
    pick = '';
  }

  function addPicked(p: Permission) {
    if (!data) return;
    if (kind === 'group' || kind === 'role') {
      const list = kind === 'group' ? data.groups : data.roles;
      const found = list.find((x) => String(x.id) === pick);
      if (found) add(p, { holder: kind, id: found.id, name: found.name });
    } else if (kind !== 'user') {
      add(p, { holder: kind });
    }
  }

  async function save() {
    saving = true;
    try {
      const d = await api.savePermissions(
        projectKey,
        grants.map(({ permission, holder, id }) =>
          id ? { permission, holder, id } : { permission, holder },
        ),
      );
      data = d;
      grants = d.grants;
      saved = signature(d.grants);
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
      <div class="tabs" role="tablist" aria-label={t.workflows.tabsLabel}>
        {#each tabs as k (k)}
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
    {/if}

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
    {:else}
      <div class="panel" role="tabpanel" id="panel-rights" aria-labelledby="tab-rights">
        <h2 class="section-title">{t.settings.permissions}</h2>
        <div class="table-scroll">
          <table class="ledger matrix">
            <tbody>
              {#each data.permissions as p (p)}
                <tr>
                  <th scope="row">
                    <span class="label">{permissionLabels[p].label}</span>
                    <span class="help">{permissionLabels[p].help}</span>
                  </th>
                  <td>
                    <div class="holders">
                      {#each holdersOf(p) as g (signature([g]))}
                        <span class="chip holder {g.holder}">
                          {#if g.holder === 'group' || g.holder === 'role' || g.holder === 'user'}<span
                              class="kind">{holderLabels[g.holder]}</span
                            >{/if}
                          {grantLabel(g)}
                          <button
                            type="button"
                            aria-label={`${t.common.remove} ${grantLabel(g)}`}
                            onclick={() => remove(g)}>×</button
                          >
                        </span>
                      {:else}
                        <span class="nobody">{t.settings.nobody}</span>
                      {/each}
                      <span class="adder" use:outside={() => adding === p && (adding = null)}>
                        <button
                          type="button"
                          class="link-btn"
                          aria-expanded={adding === p}
                          onclick={() => {
                            adding = adding === p ? null : p;
                            kind = 'group';
                            pick = '';
                          }}>+ {t.settings.addHolder}</button
                        >
                        {#if adding === p}
                          <div class="popover add">
                            <label class="field">
                              <span>{t.settings.chooseKind}</span>
                              <select class="select" bind:value={kind}>
                                {#each kinds as k (k)}<option value={k}>{holderLabels[k]}</option
                                  >{/each}
                              </select>
                            </label>
                            {#if kind === 'group' || kind === 'role'}
                              <label class="field">
                                <span
                                  >{kind === 'group'
                                    ? t.settings.chooseGroup
                                    : t.settings.chooseRole}</span
                                >
                                <select class="select" bind:value={pick}>
                                  <option value="" disabled></option>
                                  {#each kind === 'group' ? data.groups : data.roles as x (x.id)}
                                    <option value={String(x.id)}>{x.name}</option>
                                  {/each}
                                </select>
                              </label>
                            {:else if kind === 'user'}
                              <div class="field">
                                <span class="field-label">{t.settings.chooseUser}</span>
                                <div class="boxed">
                                  <UserPicker
                                    value={null}
                                    label={t.settings.chooseUser}
                                    allowNone={false}
                                    noneLabel="…"
                                    search={(q) => api.users(q)}
                                    onpick={(u: UserRef | null) =>
                                      u && add(p, { holder: 'user', id: u.id, name: u.name })}
                                  />
                                </div>
                              </div>
                            {/if}
                            {#if kind !== 'user'}
                              <button
                                class="btn btn-small btn-primary"
                                disabled={(kind === 'group' || kind === 'role') && !pick}
                                onclick={() => addPicked(p)}>{t.common.add}</button
                              >
                            {/if}
                          </div>
                        {/if}
                      </span>
                    </div>
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>

        <h2 class="section-title">{t.settings.roles}</h2>
        <dl class="roles">
          {#each data.roles as r (r.id)}
            <div>
              <dt>{r.name}</dt>
              <dd>
                {#each r.groups as g (g.id)}<span class="chip">{holderLabels.group} {g.name}</span
                  >{/each}
                {#each r.users as u (u.id)}<span class="chip">{u.name}</span>{/each}
                {#if !r.groups.length && !r.users.length}<span class="nobody"
                    >{t.settings.noMembers}</span
                  >{/if}
              </dd>
            </div>
          {/each}
        </dl>

        {#if dirty}
          <div class="savebar" role="region" aria-label={t.settings.unsaved}>
            <span>{t.settings.unsaved}</span>
            <button
              class="btn btn-quiet"
              onclick={() => data && ((grants = data.grants), undefined)}
              >{t.settings.discard}</button
            >
            <button class="btn btn-primary" disabled={saving} onclick={() => void save()}
              >{saving ? t.common.saving : t.common.save}</button
            >
          </div>
        {/if}
      </div>
    {/if}
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
    font: var(--title-italic) 15px/1.5 var(--font-title);
    color: var(--ink-soft);
  }

  .matrix th {
    width: 280px;
    text-align: left;
    text-transform: none;
    letter-spacing: 0;
    vertical-align: top;
    padding: 9px 16px 9px 0;
    border-bottom: 1px solid var(--rule);
  }

  .label {
    display: block;
    font: 600 14px/1.3 var(--font-body);
    color: var(--ink);
  }

  .help {
    display: block;
    font: 400 12.5px/1.35 var(--font-body);
    color: var(--muted);
  }

  .matrix td {
    vertical-align: top;
    padding-top: 10px;
  }

  .matrix tbody tr:hover td {
    background: none;
  }

  .holders {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    align-items: center;
  }

  .holder {
    font-family: var(--font-body);
    font-size: 12.5px;
  }

  .holder .kind {
    font: 500 11px/1 var(--font-body);
    color: var(--muted);
  }

  .holder.everyone {
    border-color: var(--ink-soft);
    font-weight: 600;
  }

  .nobody {
    color: var(--muted);
    font-style: italic;
    font-size: 13px;
  }

  .adder {
    position: relative;
    font-size: 12.5px;
  }

  .add {
    left: 0;
    top: calc(100% + 6px);
    width: 280px;
    padding: 12px;
    display: grid;
    gap: 10px;
    justify-items: start;
  }

  .add .field {
    width: 100%;
  }

  .boxed {
    border: 1px solid var(--rule-strong);
    padding: 6px 8px;
  }

  .roles {
    margin: 0;
    display: grid;
    gap: 6px;
  }

  .roles div {
    display: grid;
    grid-template-columns: 200px 1fr;
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
  }

  .tabs {
    margin: 18px 0 4px;
  }

  .leave {
    margin: 0;
  }
</style>
