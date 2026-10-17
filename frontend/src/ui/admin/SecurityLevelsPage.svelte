<script lang="ts">
  import { levelTargets } from '../../core/admin-forms.js';
  import type { AdminProjectRow } from '../../core/project-admin.js';
  import { formatNumber } from '../../i18n/format.js';
  import { holderLabels, t } from '../../i18n/t.js';
  import {
    adminApi,
    securityApi,
    type SecurityLevel,
    type SecurityMember,
  } from '../../lib/admin-api.js';
  import { errorMessage } from '../../lib/api.js';
  import { notify } from '../../lib/ui.svelte.js';
  import ConfirmDialog from '../ConfirmDialog.svelte';
  import EmptyState from '../EmptyState.svelte';
  import SecurityLevelDialog from './SecurityLevelDialog.svelte';
  import SecurityMemberDialog from './SecurityMemberDialog.svelte';

  /**
   * The issue security levels of a project: the levels, and who sees the
   * issues at each. Each project has its own levels, so one chooses the
   * project first. A level in use goes only once its issues move to another
   * level or lose theirs.
   */
  let projects = $state<AdminProjectRow[] | null>(null);
  let project = $state('');
  let levels = $state<SecurityLevel[] | null>(null);
  let error = $state('');
  let dialog = $state<{ level: SecurityLevel | null } | null>(null);
  let adding = $state<SecurityLevel | null>(null);
  let removing = $state<SecurityLevel | null>(null);
  let how = $state<'move' | 'clear'>('move');
  let moveTo = $state('');
  let seq = 0;

  const targets = $derived(levels && removing ? levelTargets(levels, removing) : []);

  $effect(() => {
    adminApi
      .projects()
      .then((p) => (projects = p))
      .catch((e: unknown) => (error = errorMessage(e)));
  });

  async function load() {
    const mine = ++seq;
    levels = null;
    error = '';
    if (!project) return;
    try {
      const found = await securityApi.levels(project);
      if (mine === seq) levels = found;
    } catch (e) {
      if (mine === seq) error = errorMessage(e);
    }
  }

  function choose() {
    void load();
  }

  function saved(level: SecurityLevel, created: boolean) {
    notify(created ? t.securityAdmin.created(level.name) : t.securityAdmin.saved(level.name));
    void load();
  }

  function memberAdded() {
    notify(t.securityAdmin.memberAdded);
    void load();
  }

  async function removeMember(level: SecurityLevel, m: SecurityMember) {
    try {
      await securityApi.removeMember(project, level.id, m.id);
      void load();
    } catch (e) {
      notify(errorMessage(e), 'error');
    }
  }

  function askRemove(level: SecurityLevel) {
    removing = level;
    how = levels && levelTargets(levels, level).length > 0 ? 'move' : 'clear';
    moveTo = '';
  }

  async function remove() {
    const level = removing;
    if (!level) return;
    const used = level.issues > 0;
    if (used && how === 'move' && !moveTo) throw new Error(t.securityAdmin.chooseTarget);
    const choice = !used ? {} : how === 'move' ? { moveTo: Number(moveTo) } : { clear: true };
    await securityApi.remove(project, level.id, choice);
    notify(t.securityAdmin.deleted(level.name));
    await load();
  }

  /** Who a member is: the name of the user, group or role, else the kind of holder. */
  function memberName(m: SecurityMember): string {
    return m.user?.name ?? m.group?.name ?? m.role?.name ?? holderLabels[m.holder];
  }
</script>

<header class="page-head">
  <div>
    <p class="eyebrow">{t.adminArea.groups.projects}</p>
    <h1 class="page-title">{t.securityAdmin.title}</h1>
  </div>
  {#if levels}
    <button class="btn btn-primary" onclick={() => (dialog = { level: null })}
      >{t.securityAdmin.newLevel}</button
    >
  {/if}
</header>
<p class="lead-text">{t.securityAdmin.intro}</p>

{#if projects && projects.length === 0}
  <p class="empty">{t.securityAdmin.noProjects}</p>
{:else if projects}
  <div class="list-tools">
    <label class="field project-pick">
      <span class="sr-only">{t.securityAdmin.project}</span>
      <select
        class="select"
        aria-label={t.securityAdmin.project}
        bind:value={project}
        onchange={choose}
      >
        <option value="" disabled>{t.securityAdmin.chooseProject}</option>
        {#each projects as p (p.id)}<option value={p.key}>{p.key} — {p.name}</option>{/each}
      </select>
    </label>
  </div>
{/if}

{#if error}
  <p class="error-text" role="alert">{error}</p>
{:else if !projects}
  <p class="muted loading">{t.common.loading}</p>
{:else if !project}
  <p class="muted">{t.securityAdmin.pickProject}</p>
{:else if !levels}
  <p class="muted loading">{t.common.loading}</p>
{:else if levels.length === 0}
  <EmptyState message={t.securityAdmin.empty} hint={t.securityAdmin.emptyHint}>
    <button class="btn btn-primary" onclick={() => (dialog = { level: null })}
      >{t.securityAdmin.newLevel}</button
    >
  </EmptyState>
{:else}
  <ul class="levels">
    {#each levels as level (level.id)}
      <li class="level">
        <div class="head">
          <div>
            <strong>{level.name}</strong>
            <span class="muted small">
              {level.issues === 0 ? t.securityAdmin.noIssues : t.securityAdmin.issues(level.issues)}
            </span>
            {#if level.description}<span class="muted small desc">{level.description}</span>{/if}
          </div>
          <span class="row-actions">
            <button class="link-btn" onclick={() => (dialog = { level })}>{t.common.edit}</button>
            <button class="link-btn" onclick={() => askRemove(level)}>{t.common.delete}</button>
          </span>
        </div>
        <div class="members">
          <span class="muted small label">{t.securityAdmin.members}</span>
          {#each level.members as m (m.id)}
            <span class="chip member">
              {#if m.user || m.group || m.role}<span class="kind">{holderLabels[m.holder]}</span
                >{/if}
              {memberName(m)}
              <button
                type="button"
                aria-label={t.securityAdmin.removeMember(memberName(m))}
                onclick={() => void removeMember(level, m)}>×</button
              >
            </span>
          {:else}
            <span class="muted small">{t.securityAdmin.noMembers}</span>
          {/each}
          <button class="link-btn" onclick={() => (adding = level)}
            >+ {t.securityAdmin.addMember}</button
          >
        </div>
      </li>
    {/each}
  </ul>
  <p class="muted small count">{formatNumber(levels.length)}</p>
{/if}

{#if levels}
  <SecurityLevelDialog
    open={dialog !== null}
    {project}
    level={dialog?.level ?? null}
    {levels}
    onclose={() => (dialog = null)}
    onsaved={saved}
  />
{/if}

<SecurityMemberDialog
  open={adding !== null}
  {project}
  level={adding}
  onclose={() => (adding = null)}
  onsaved={memberAdded}
/>

<ConfirmDialog
  open={removing !== null}
  title={t.securityAdmin.deleteTitle(removing?.name ?? '')}
  confirm={removing && removing.issues > 0
    ? how === 'move'
      ? t.securityAdmin.moveAndDelete
      : t.securityAdmin.clearAndDelete
    : t.common.delete}
  onclose={() => (removing = null)}
  onconfirm={remove}
>
  {#if removing && removing.issues > 0}
    <p>{t.securityAdmin.deleteUsed(removing.issues)}</p>
    <fieldset class="choices">
      <legend class="sr-only">{t.securityAdmin.moveLabel}</legend>
      <label class="check">
        <input
          type="radio"
          name="level-how"
          value="move"
          bind:group={how}
          disabled={targets.length === 0}
        />
        <span>{t.securityAdmin.howMove}</span>
      </label>
      <label class="check">
        <input type="radio" name="level-how" value="clear" bind:group={how} />
        <span>{t.securityAdmin.howClear}</span>
      </label>
    </fieldset>
    {#if targets.length === 0}
      <p class="muted small">{t.securityAdmin.noTarget}</p>
    {:else if how === 'move'}
      <label class="field">
        <span>{t.securityAdmin.moveLabel}</span>
        <select class="select" bind:value={moveTo}>
          <option value="" disabled>{t.securityAdmin.choose}</option>
          {#each targets as x (x.id)}<option value={String(x.id)}>{x.name}</option>{/each}
        </select>
      </label>
    {/if}
  {:else}
    <p>{t.securityAdmin.deleteUnused}</p>
  {/if}
</ConfirmDialog>

<style>
  .project-pick {
    min-width: 260px;
  }

  .levels {
    display: grid;
    gap: 12px;
    margin: 0;
    padding: 0;
    list-style: none;
  }

  .level {
    display: grid;
    gap: 10px;
    padding: 12px 14px;
    border: var(--frame) solid var(--rule);
    border-radius: var(--radius-m);
  }

  .head {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    justify-content: space-between;
    gap: 6px 16px;
  }

  .desc {
    display: block;
    margin-top: 2px;
    max-width: 60ch;
  }

  .members {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 6px 8px;
  }

  .label {
    min-width: 64px;
  }

  .member {
    font-family: var(--font-body);
    font-size: 12.5px;
  }

  .kind {
    font: 500 11px/1 var(--font-body);
    color: var(--muted);
  }

  .count {
    margin-top: 10px;
  }
</style>
