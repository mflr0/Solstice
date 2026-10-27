<script lang="ts">
  import { untrack } from 'svelte';
  import type { ProjectDetail, SprintSuggestion, UserRef } from '../../core/types.js';
  import { t } from '../../i18n/t.js';
  import {
    api,
    errorMessage,
    type BulkEdit,
    type BulkResult,
    type BulkSet,
  } from '../../lib/api.js';
  import { session } from '../../lib/session.svelte.js';
  import { notify } from '../../lib/ui.svelte.js';
  import Dropdown from '../Dropdown.svelte';
  import LabelEditor from '../LabelEditor.svelte';
  import Modal from '../Modal.svelte';
  import MultiPick from '../MultiPick.svelte';
  import PriorityMark from '../PriorityMark.svelte';
  import TypedConfirmDialog from '../TypedConfirmDialog.svelte';
  import UserPicker from '../UserPicker.svelte';
  import BulkReport from './BulkReport.svelte';

  /**
   * One change to the selected issues: edit some fields, change the status,
   * or delete them. Each issue is changed as if on its own page; those the
   * server refuses are listed afterwards.
   */
  interface Props {
    op: 'edit' | 'transition' | 'delete' | null;
    keys: string[];
    onclose: () => void;
    /** After a change: the keys that were refused, to keep selected. */
    ondone: (refused: string[]) => void;
  }
  const { op, keys, onclose, ondone }: Props = $props();

  type Mode = BulkSet<unknown>['mode'];
  const MODES: Mode[] = ['add', 'remove', 'replace'];

  const projectKeys = $derived([...new Set(keys.map((k) => k.slice(0, k.lastIndexOf('-'))))]);
  const oneProject = $derived(projectKeys.length === 1 ? projectKeys[0] : undefined);

  let project = $state<ProjectDetail | null>(null);
  let sprints = $state<SprintSuggestion[]>([]);
  let busy = $state(false);
  let error = $state('');
  let report = $state<BulkResult[] | null>(null);

  let useAssignee = $state(false);
  let assignee = $state<UserRef | null>(null);
  let usePriority = $state(false);
  let priority = $state('');
  let useLabels = $state(false);
  let labelsMode = $state<Mode>('add');
  let labels = $state<string[]>([]);
  let useComponents = $state(false);
  let componentsMode = $state<Mode>('add');
  let componentIds = $state<number[]>([]);
  let useFix = $state(false);
  let fixMode = $state<Mode>('add');
  let fixIds = $state<number[]>([]);
  let useSprint = $state(false);
  let sprint = $state('0');
  let status = $state('');
  let resolution = $state('');

  const nothing = $derived(
    !useAssignee && !usePriority && !useLabels && !useComponents && !useFix && !useSprint,
  );

  // Each opening starts afresh.
  $effect(() => {
    if (!op) return;
    untrack(() => {
      error = '';
      busy = false;
      useAssignee = usePriority = useLabels = useComponents = useFix = useSprint = false;
      assignee = null;
      priority = session.meta?.priorities[0]?.id ?? '';
      labelsMode = componentsMode = fixMode = 'add';
      labels = [];
      componentIds = [];
      fixIds = [];
      sprint = '0';
      status = session.meta?.statuses[0]?.id ?? '';
      resolution = '';
      if (op === 'edit') void load();
    });
  });

  async function load() {
    project = null;
    const key = oneProject;
    try {
      const [p, s] = await Promise.all([
        key ? api.project(key) : Promise.resolve(null),
        api.sprintSuggestions(''),
      ]);
      project = p;
      sprints = s.filter((sp) => sp.state !== 'closed');
    } catch (e) {
      error = errorMessage(e);
    }
  }

  function change(): BulkEdit {
    const c: BulkEdit = {};
    if (useAssignee) c.assignee = assignee?.id ?? null;
    if (usePriority) c.priority = priority || null;
    if (useLabels) c.labels = { mode: labelsMode, values: labels };
    if (useComponents) c.components = { mode: componentsMode, values: componentIds };
    if (useFix) c.fixVersions = { mode: fixMode, values: fixIds };
    if (useSprint) c.sprint = Number(sprint);
    return c;
  }

  /** Sends the change; the dialog closes, and what was refused is listed. */
  async function send(body: Parameters<typeof api.bulk>[1]) {
    const { results } = await api.bulk(keys, body);
    const refused = results.filter((r) => !r.ok);
    const changed = results.length - refused.length;
    if (changed) notify(t.search.bulk.done(changed));
    report = refused.length ? results : null;
    ondone(refused.map((r) => r.key));
  }

  async function apply() {
    if (busy) return;
    busy = true;
    error = '';
    try {
      await send(
        op === 'edit'
          ? { op: 'edit', ...change() }
          : { op: 'transition', status, ...(resolution ? { resolution } : {}) },
      );
      onclose();
    } catch (e) {
      error = errorMessage(e);
    } finally {
      busy = false;
    }
  }
</script>

{#snippet toggle(label: string, value: boolean, set: (v: boolean) => void, disabled = false)}
  <label class="check">
    <input
      type="checkbox"
      class="checkbox"
      checked={value}
      {disabled}
      onchange={(e) => set(e.currentTarget.checked)}
    />
    <span>{t.search.bulk.change(label)}</span>
  </label>
{/snippet}

{#snippet modes(label: string, value: Mode, set: (m: Mode) => void)}
  <span class="mode">
    <Dropdown
      {label}
      {value}
      options={MODES.map((m) => ({ value: m, label: t.search.bulk.modes[m] }))}
      onchange={(v) => set(v as Mode)}
    />
  </span>
{/snippet}

<Modal
  open={op === 'edit' || op === 'transition'}
  title={op === 'edit'
    ? t.search.bulk.editTitle(keys.length)
    : t.search.bulk.transitionTitle(keys.length)}
  onclose={() => !busy && onclose()}
>
  <form
    id="bulk-form"
    class="form"
    onsubmit={(e) => {
      e.preventDefault();
      void apply();
    }}
  >
    {#if op === 'edit'}
      <p class="muted">{t.search.bulk.editHint}</p>
      <div class="item">
        {@render toggle(t.issue.assignee, useAssignee, (v) => (useAssignee = v))}
        {#if useAssignee}
          <UserPicker
            field
            value={assignee}
            label={t.issue.assignee}
            search={(q) => (oneProject ? api.assignable(oneProject, q) : api.users(q))}
            onpick={(u) => (assignee = u)}
          />
        {/if}
      </div>
      <div class="item">
        {@render toggle(t.issue.priority, usePriority, (v) => (usePriority = v))}
        {#if usePriority}
          <Dropdown
            label={t.issue.priority}
            value={priority}
            options={[
              ...(session.meta?.priorities ?? []).map((p) => ({ value: p.id, label: p.name })),
              { value: '', label: t.search.bulk.noPriority },
            ]}
            onchange={(v) => (priority = v)}
          >
            {#snippet icon(id)}
              {#if id}
                <PriorityMark
                  priority={session.meta?.priorities.find((p) => p.id === id) ?? null}
                />
              {/if}
            {/snippet}
          </Dropdown>
        {/if}
      </div>
      <div class="item">
        {@render toggle(t.issue.labels, useLabels, (v) => (useLabels = v))}
        {#if useLabels}
          <div class="row">
            {@render modes(t.issue.labels, labelsMode, (m) => (labelsMode = m))}
            <div class="boxed grow">
              <LabelEditor {labels} onchange={(l) => (labels = l)} />
            </div>
          </div>
        {/if}
      </div>
      <div class="item">
        {@render toggle(t.issue.components, useComponents, (v) => (useComponents = v), !oneProject)}
        {#if useComponents && project}
          <div class="row">
            {@render modes(t.issue.components, componentsMode, (m) => (componentsMode = m))}
            <div class="boxed grow">
              <MultiPick
                options={project.components}
                selected={componentIds}
                label={t.issue.components}
                onchange={(ids) => (componentIds = ids)}
              />
            </div>
          </div>
        {/if}
      </div>
      <div class="item">
        {@render toggle(t.issue.fixVersions, useFix, (v) => (useFix = v), !oneProject)}
        {#if useFix && project}
          <div class="row">
            {@render modes(t.issue.fixVersions, fixMode, (m) => (fixMode = m))}
            <div class="boxed grow">
              <MultiPick
                options={project.versions.map((v) => ({ id: v.id, name: v.name }))}
                selected={fixIds}
                label={t.issue.fixVersions}
                onchange={(ids) => (fixIds = ids)}
              />
            </div>
          </div>
        {/if}
        {#if !oneProject}<p class="hint">{t.search.bulk.oneProject}</p>{/if}
      </div>
      <div class="item">
        {@render toggle(t.boards.sprint, useSprint, (v) => (useSprint = v))}
        {#if useSprint}
          <Dropdown
            label={t.boards.sprint}
            value={sprint}
            options={[
              { value: '0', label: t.search.bulk.noSprint },
              ...sprints.map((sp) => ({
                value: String(sp.id),
                label: `${sp.name} — ${sp.boardName}`,
              })),
            ]}
            onchange={(v) => (sprint = v)}
          />
        {/if}
      </div>
    {:else if op === 'transition'}
      <div class="field">
        <span>{t.search.bulk.status}</span>
        <Dropdown
          label={t.search.bulk.status}
          value={status}
          options={(session.meta?.statuses ?? []).map((st) => ({ value: st.id, label: st.name }))}
          onchange={(v) => (status = v)}
        />
        <span class="hint">{t.search.bulk.statusHint}</span>
      </div>
      <div class="field">
        <span>{t.issue.chooseResolution}</span>
        <Dropdown
          label={t.issue.chooseResolution}
          value={resolution}
          options={[
            { value: '', label: t.search.bulk.defaultResolution },
            ...(session.meta?.resolutions ?? []).map((x) => ({ value: x.id, label: x.name })),
          ]}
          onchange={(v) => (resolution = v)}
        />
      </div>
    {/if}
    {#if error}<p class="error-text" role="alert">{error}</p>{/if}
  </form>
  {#snippet footer()}
    <button type="button" class="btn btn-quiet" disabled={busy} onclick={onclose}
      >{t.common.cancel}</button
    >
    <button
      type="submit"
      form="bulk-form"
      class="btn btn-primary"
      disabled={busy || (op === 'edit' && nothing) || (op === 'transition' && !status)}
      >{busy ? t.adminArea.working : t.search.bulk.apply}</button
    >
  {/snippet}
</Modal>

<TypedConfirmDialog
  open={op === 'delete'}
  title={t.search.bulk.deleteTitle(keys.length)}
  phrase={String(keys.length)}
  label={t.search.bulk.typeToConfirm(keys.length)}
  confirm={t.search.bulk.deleteSubmit}
  onconfirm={() => send({ op: 'delete' })}
  {onclose}
>
  <p>{t.search.bulk.deleteBody(keys.length)}</p>
</TypedConfirmDialog>

<BulkReport results={report} onclose={() => (report = null)} />

<style>
  .form {
    display: grid;
    gap: 14px;
  }

  .form :global(p) {
    margin: 0;
  }

  .item {
    display: grid;
    gap: 8px;
  }

  .item .hint {
    font-size: 12px;
    color: var(--muted);
  }

  .check {
    display: flex;
    gap: 8px;
    align-items: center;
  }

  .mode {
    flex: none;
  }

  .boxed {
    border: 1px solid var(--rule-strong);
    background: var(--paper-raised);
    padding: 6px 8px;
    min-height: 34px;
    display: flex;
    align-items: center;
    flex-wrap: wrap;
  }
</style>
