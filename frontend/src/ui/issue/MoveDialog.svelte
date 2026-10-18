<script lang="ts">
  import { untrack } from 'svelte';
  import { paths } from '../../core/route.js';
  import type { IdName, IssueDetail, Project, StatusRef, TypeRef } from '../../core/types.js';
  import { formatList } from '../../i18n/format.js';
  import { t } from '../../i18n/t.js';
  import {
    api,
    errorMessage,
    type MoveCheck,
    type MoveChoices,
    type MoveParts,
    type MoveRestricted,
  } from '../../lib/api.js';
  import { router } from '../../lib/router.svelte.js';
  import { notify } from '../../lib/ui.svelte.js';
  import Modal from '../Modal.svelte';
  import MultiPick from '../MultiPick.svelte';

  /**
   * "Move": to another project, in steps. First the project, then what the
   * new project asks the user to choose (the server's dry run says what), then
   * a last look at what the move changes before it is done.
   */
  interface Props {
    open: boolean;
    issue: IssueDetail;
    onclose: () => void;
  }
  const { open, issue, onclose }: Props = $props();

  type Step = 'project' | 'choices' | 'confirm';
  let step = $state<Step>('project');
  let projects = $state<Project[]>([]);
  let target = $state('');
  let check = $state<MoveCheck | null>(null);
  let busy = $state(false);
  let error = $state('');

  // What the dry runs have asked for so far, each with the choice made. A
  // question stays once asked: the server stops asking when it is answered.
  let types = $state<TypeRef[]>([]);
  let subtaskTypes = $state<TypeRef[]>([]);
  let statuses = $state<StatusRef[]>([]);
  let components = $state<MoveParts | null>(null);
  let affectsVersions = $state<MoveParts | null>(null);
  let fixVersions = $state<MoveParts | null>(null);
  let securityLevel = $state<MoveParts | null>(null);
  let restricted = $state<MoveRestricted | null>(null);
  let typeId = $state('');
  let subtaskTypeId = $state('');
  let statusId = $state('');
  let componentIds = $state<number[]>([]);
  let affectsIds = $state<number[]>([]);
  let fixIds = $state<number[]>([]);
  let levelId = $state('');
  let restrictTo = $state<'role' | 'group'>('role');
  let roleId = $state('');
  let groupId = $state('');

  const targetProject = $derived(projects.find((p) => p.key === target));
  const targetLabel = $derived(
    targetProject ? `${targetProject.name} — ${targetProject.key}` : target,
  );
  const asked = $derived(
    types.length > 0 ||
      subtaskTypes.length > 0 ||
      statuses.length > 0 ||
      components !== null ||
      affectsVersions !== null ||
      fixVersions !== null ||
      securityLevel !== null ||
      restricted !== null,
  );
  // Nothing to restrict the items to: the move cannot be made.
  const stuck = $derived(
    restricted !== null && !restricted.roles.length && !restricted.groups.length,
  );

  function forget() {
    types = [];
    subtaskTypes = [];
    statuses = [];
    components = null;
    affectsVersions = null;
    fixVersions = null;
    securityLevel = null;
    restricted = null;
    typeId = '';
    subtaskTypeId = '';
    statusId = '';
    componentIds = [];
    affectsIds = [];
    fixIds = [];
    levelId = '';
    roleId = '';
    groupId = '';
  }

  // Each opening starts from the project list.
  $effect(() => {
    if (!open) return;
    untrack(() => {
      step = 'project';
      check = null;
      error = '';
      busy = false;
      forget();
      void loadProjects();
    });
  });

  async function loadProjects() {
    try {
      projects = (await api.projects()).filter((p) => p.key !== issue.project.key);
      target = projects[0]?.key ?? '';
    } catch (e) {
      error = errorMessage(e);
    }
  }

  /** The options with the name of one of the issue's own, as the server matches them. */
  function sameNames(own: IdName[], options: IdName[]): number[] {
    const names = new Set(own.map((o) => o.name.toLowerCase()));
    return options.filter((o) => names.has(o.name.toLowerCase())).map((o) => o.id);
  }

  /** Takes in the questions of a dry run that were not asked before; tells if there were any. */
  function absorb(c: MoveCheck): boolean {
    let news = false;
    if (c.types?.length && !types.length) {
      types = c.types;
      typeId = c.types[0]?.id ?? '';
      news = true;
    }
    if (c.subtaskTypes?.length && !subtaskTypes.length) {
      subtaskTypes = c.subtaskTypes;
      subtaskTypeId = c.subtaskTypes[0]?.id ?? '';
      news = true;
    }
    if (c.statuses?.length && !statuses.length) {
      statuses = c.statuses;
      statusId = c.statuses[0]?.id ?? '';
      news = true;
    }
    if (c.components && !components) {
      components = c.components;
      componentIds = sameNames(issue.components, c.components.options);
      news = true;
    }
    if (c.affectsVersions && !affectsVersions) {
      affectsVersions = c.affectsVersions;
      affectsIds = sameNames(issue.affectsVersions, c.affectsVersions.options);
      news = true;
    }
    if (c.fixVersions && !fixVersions) {
      fixVersions = c.fixVersions;
      fixIds = sameNames(issue.fixVersions, c.fixVersions.options);
      news = true;
    }
    if (c.securityLevel && !securityLevel) {
      securityLevel = c.securityLevel;
      levelId = String(c.securityLevel.options[0]?.id ?? '');
      news = true;
    }
    if (c.restricted && !restricted) {
      restricted = c.restricted;
      restrictTo = c.restricted.roles.length ? 'role' : 'group';
      roleId = String(c.restricted.roles[0]?.id ?? '');
      groupId = String(c.restricted.groups[0]?.id ?? '');
      news = true;
    }
    return news;
  }

  function choices(): MoveChoices {
    const c: MoveChoices = {};
    if (types.length) c.type = typeId;
    if (subtaskTypes.length) c.subtaskType = subtaskTypeId;
    if (statuses.length) c.status = statusId;
    if (components) c.components = componentIds;
    if (affectsVersions) c.affectsVersions = affectsIds;
    if (fixVersions) c.fixVersions = fixIds;
    if (securityLevel) c.securityLevel = levelId === '' ? null : Number(levelId);
    if (restricted) {
      c.restricted =
        restrictTo === 'role'
          ? { to: 'role', role: Number(roleId) }
          : { to: 'group', group: Number(groupId) };
    }
    return c;
  }

  async function run(action: () => Promise<void>) {
    busy = true;
    error = '';
    try {
      await action();
    } catch (e) {
      error = errorMessage(e);
    } finally {
      busy = false;
    }
  }

  function toChoices() {
    void run(async () => {
      forget();
      const c = await api.checkMove(issue.key, target, {});
      absorb(c);
      check = c;
      step = c.ready ? 'confirm' : 'choices';
    });
  }

  function toConfirm() {
    void run(async () => {
      const c = await api.checkMove(issue.key, target, choices());
      const news = absorb(c);
      check = c;
      if (c.ready) step = 'confirm';
      else if (!news) error = t.errors.generic;
    });
  }

  function move() {
    void run(async () => {
      const { key } = await api.moveIssue(issue.key, target, choices());
      notify(t.issue.moved(issue.key, key));
      onclose();
      router.navigate(paths.issue(key));
    });
  }

  function back() {
    error = '';
    step = step === 'confirm' && asked ? 'choices' : 'project';
  }
</script>

{#snippet pick(label: string, parts: MoveParts, value: number[], set: (ids: number[]) => void)}
  <div class="field">
    <span class="field-label">{label}</span>
    <div class="boxed">
      <MultiPick options={parts.options} selected={value} {label} onchange={set} />
    </div>
    <span class="hint">{t.issue.moveUnmatched(formatList(parts.unmatched.map((u) => u.name)))}</span
    >
  </div>
{/snippet}

<Modal {open} title={t.issue.moveTitle(issue.key)} {onclose}>
  <div class="form">
    {#if step === 'project'}
      {#if projects.length === 0 && !error}
        <p class="muted">{t.issue.moveNoProject}</p>
      {:else}
        <label class="field">
          <span>{t.create.project}</span>
          <select class="select" bind:value={target}>
            {#each projects as p (p.key)}
              <option value={p.key}>{p.name} — {p.key}</option>
            {/each}
          </select>
          <span class="hint">{t.issue.moveProjectHint}</span>
        </label>
      {/if}
    {:else if step === 'choices'}
      <p>{t.issue.moveChoicesHint(targetLabel)}</p>
      {#if types.length}
        <label class="field">
          <span>{t.create.type}</span>
          <select class="select" bind:value={typeId} onchange={() => (statuses = [])}>
            {#each types as ty (ty.id)}
              <option value={ty.id}>{ty.name}</option>
            {/each}
          </select>
        </label>
      {/if}
      {#if subtaskTypes.length}
        <label class="field">
          <span>{t.issue.moveSubtaskType}</span>
          <select class="select" bind:value={subtaskTypeId}>
            {#each subtaskTypes as ty (ty.id)}
              <option value={ty.id}>{ty.name}</option>
            {/each}
          </select>
        </label>
      {/if}
      {#if statuses.length}
        <label class="field">
          <span>{t.issue.moveStatus}</span>
          <select class="select" bind:value={statusId}>
            {#each statuses as st (st.id)}
              <option value={st.id}>{st.name}</option>
            {/each}
          </select>
        </label>
      {/if}
      {#if components}
        {@render pick(t.create.components, components, componentIds, (ids) => (componentIds = ids))}
      {/if}
      {#if affectsVersions}
        {@render pick(
          t.issue.moveAffectsVersions,
          affectsVersions,
          affectsIds,
          (ids) => (affectsIds = ids),
        )}
      {/if}
      {#if fixVersions}
        {@render pick(t.create.fixVersions, fixVersions, fixIds, (ids) => (fixIds = ids))}
      {/if}
      {#if securityLevel}
        <label class="field">
          <span>{t.create.security}</span>
          <select class="select" bind:value={levelId}>
            <option value="">{t.issue.public}</option>
            {#each securityLevel.options as l (l.id)}
              <option value={String(l.id)}>{l.name}</option>
            {/each}
          </select>
          <span class="hint"
            >{t.issue.moveUnmatched(formatList(securityLevel.unmatched.map((u) => u.name)))}</span
          >
        </label>
      {/if}
      {#if restricted}
        <fieldset class="restricted">
          <legend>{t.issue.moveRestricted(restricted.comments + restricted.worklogs)}</legend>
          {#if stuck}
            <p class="hint">{t.issue.moveRestrictedNone}</p>
          {:else}
            {#if restricted.roles.length}
              <label class="check">
                <input type="radio" name="move-restrict" value="role" bind:group={restrictTo} />
                <span>{t.issue.moveRestrictedRole}</span>
              </label>
              <select
                class="select"
                bind:value={roleId}
                disabled={restrictTo !== 'role'}
                aria-label={t.issue.moveRestrictedRole}
              >
                {#each restricted.roles as r (r.id)}
                  <option value={String(r.id)}>{r.name}</option>
                {/each}
              </select>
            {/if}
            {#if restricted.groups.length}
              <label class="check">
                <input type="radio" name="move-restrict" value="group" bind:group={restrictTo} />
                <span>{t.issue.moveRestrictedGroup}</span>
              </label>
              <select
                class="select"
                bind:value={groupId}
                disabled={restrictTo !== 'group'}
                aria-label={t.issue.moveRestrictedGroup}
              >
                {#each restricted.groups as g (g.id)}
                  <option value={String(g.id)}>{g.name}</option>
                {/each}
              </select>
            {/if}
          {/if}
        </fieldset>
      {/if}
    {:else if check}
      <p>{t.issue.moveSummary(issue.key, targetLabel)}</p>
      <ul class="notes">
        {#if check.droppedFields.length}
          <li>{t.issue.moveDropped(formatList(check.droppedFields))}</li>
        {/if}
        {#if check.sprints.length}
          <li>{t.issue.moveSprints(formatList(check.sprints))}</li>
        {/if}
        {#if check.assignee}
          <li>
            {t.issue.moveAssignee(
              check.assignee.from?.name ?? t.common.unassigned,
              check.assignee.to?.name ?? t.common.unassigned,
            )}
          </li>
        {/if}
        {#if check.subtasks.count}<li>{t.issue.moveSubtasks(check.subtasks.count)}</li>{/if}
        {#if check.subtasks.reset}<li>{t.issue.moveSubtasksReset(check.subtasks.reset)}</li>{/if}
        {#if check.subtasks.lost}<li>{t.issue.moveSubtasksLost(check.subtasks.lost)}</li>{/if}
        {#if check.subtasks.reassigned}
          <li>{t.issue.moveSubtasksReassigned(check.subtasks.reassigned)}</li>
        {/if}
      </ul>
    {/if}
    {#if error}<p class="error-text" role="alert">{error}</p>{/if}
  </div>
  {#snippet footer()}
    <button type="button" class="btn btn-quiet" disabled={busy} onclick={onclose}
      >{t.common.cancel}</button
    >
    {#if step !== 'project'}
      <button type="button" class="btn" disabled={busy} onclick={back}>{t.issue.moveBack}</button>
    {/if}
    {#if step === 'confirm'}
      <button type="button" class="btn btn-primary" disabled={busy} onclick={move}
        >{busy ? t.adminArea.working : t.issue.moveSubmit}</button
      >
    {:else}
      <button
        type="button"
        class="btn btn-primary"
        disabled={busy || (step === 'project' && !target) || (step === 'choices' && stuck)}
        onclick={step === 'project' ? toChoices : toConfirm}
        >{busy ? t.adminArea.working : t.issue.moveNext}</button
      >
    {/if}
  {/snippet}
</Modal>

<style>
  .form {
    display: grid;
    gap: 14px;
  }

  .form :global(p) {
    margin: 0;
  }

  .boxed {
    border: 1px solid var(--rule-strong);
    background: var(--paper-raised);
    padding: 6px 8px;
    min-height: 34px;
    display: flex;
    align-items: center;
  }

  .restricted {
    margin: 0;
    padding: 0;
    border: 0;
    display: grid;
    gap: 8px;
  }

  .restricted legend {
    padding: 0;
    margin-bottom: 8px;
  }

  .check {
    display: flex;
    gap: 8px;
    align-items: center;
  }

  .notes {
    margin: 0;
    padding-left: 18px;
    display: grid;
    gap: 6px;
  }
</style>
