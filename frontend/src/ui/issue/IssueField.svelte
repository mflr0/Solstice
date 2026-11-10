<script lang="ts">
  import type { LayoutZone } from '../../core/admin-types.js';
  import { draftOf } from '../../core/custom-fields.js';
  import { estimateChanges, estimateText } from '../../core/estimates.js';
  import { customOf } from '../../core/issue-view.js';
  import { paths } from '../../core/route.js';
  import type { CustomValue, IssueDetail } from '../../core/types.js';
  import { isSafeUrl } from '../../core/wiki.js';
  import { formatDateTime, formatDay, formatDuration, t } from '../../i18n/t.js';
  import { projectAdminApi } from '../../lib/admin-api.js';
  import { api, type IssueFields } from '../../lib/api.js';
  import { session } from '../../lib/session.svelte.js';
  import CustomFieldInput from '../CustomFieldInput.svelte';
  import Dropdown from '../Dropdown.svelte';
  import LabelEditor from '../LabelEditor.svelte';
  import Lock from '../Lock.svelte';
  import MultiPick from '../MultiPick.svelte';
  import PriorityMark from '../PriorityMark.svelte';
  import RelTime from '../RelTime.svelte';
  import UserPicker from '../UserPicker.svelte';
  import Who from '../Who.svelte';
  import WikiText from '../WikiText.svelte';
  import FieldFrame from './FieldFrame.svelte';
  import type { FieldState } from './field-state.svelte.js';
  import WikiField from './WikiField.svelte';

  /**
   * One field of the issue view, by its layout id, in its zone: shown when
   * it has a value or the reader may set one, editable in place for those
   * allowed to. A custom value the layout doesn't place comes as `value`.
   */
  interface Props {
    issue: IssueDetail;
    field: string;
    zone: LayoutZone;
    fields: FieldState;
    value?: CustomValue | undefined;
  }
  const { issue, field, zone, fields, value }: Props = $props();

  const uid = $props.id();
  const edit = $derived(issue.can.edit === true);
  const onchange = (f: IssueFields) => fields.onchange(f);
  const project = $derived(fields.project);
  const configure = $derived(project?.can.configure === true);
  const ids = (list: { id: number }[]) => list.map((x) => x.id);
  const meIsAssignee = $derived(issue.assignee?.id === session.me?.id);

  const customId = $derived(value ? value.id : customOf(field));
  const custom = $derived(
    value ?? (customId ? issue.customFields.find((v) => v.id === customId) : undefined),
  );
  const offered = $derived(edit && !value && customId ? fields.offeredField(customId) : undefined);

  const time = $derived(issue.time);
  const hasTime = $derived(!!time.original || !!time.spent || time.remaining !== null);

  // Whether the field has nothing to show: the "Required" hint's test.
  const blank = $derived.by(() => {
    switch (field) {
      case 'description':
        return !issue.description;
      case 'environment':
        return !issue.environment;
      case 'priority':
        return !issue.priority;
      case 'assignee':
        return !issue.assignee;
      case 'reporter':
        return !issue.reporter;
      case 'components':
        return !issue.components.length;
      case 'fixVersions':
        return !issue.fixVersions.length;
      case 'versions':
        return !issue.affectsVersions.length;
      case 'labels':
        return !issue.labels.length;
      case 'duedate':
        return !issue.due;
      case 'timetracking':
        return !time.original;
      case 'security':
        return !issue.securityLevel;
      default:
        return customId ? !custom?.values.length && !custom?.users?.length : false;
    }
  });
  const required = $derived(edit && blank && fields.required.includes(field));

  // The estimates as typed, kept in step with the issue.
  let originalText = $state('');
  let remainingText = $state('');
  let estimateError = $state(false);

  $effect(() => {
    if (field !== 'timetracking') return;
    originalText = estimateText(issue.time.original, t.units.duration);
    remainingText = estimateText(issue.time.remaining, t.units.duration);
    estimateError = false;
  });

  async function saveEstimates() {
    const { fields: changed, invalid } = estimateChanges(
      { original: originalText, remaining: remainingText },
      issue.time,
      t.units.duration,
    );
    estimateError = invalid !== null;
    if (invalid !== null || Object.keys(changed).length === 0) return;
    if (!(await onchange(changed))) {
      // The note already says why; the fields show the issue's own values again.
      originalText = estimateText(issue.time.original, t.units.duration);
      remainingText = estimateText(issue.time.remaining, t.units.duration);
    }
  }

  // A refusal leaves the issue as it was, so Svelte has nothing to redraw and
  // the control would keep the refused value.
  async function save(el: HTMLInputElement, changed: IssueFields, stored: () => string) {
    if (!(await onchange(changed))) el.value = stored();
  }

  async function createComponent(name: string): Promise<number> {
    const c = await projectAdminApi.createComponent(issue.project.key, { name });
    if (fields.project) fields.project.components = [...fields.project.components, c];
    return c.id;
  }

  async function createVersion(name: string): Promise<number> {
    const v = await projectAdminApi.createVersion(issue.project.key, { name });
    if (fields.project) fields.project.versions = [...fields.project.versions, v];
    return v.id;
  }

  const spentRatio = $derived.by(() => {
    const { original, spent } = issue.time;
    if (!original || !spent) return 0;
    return Math.min(1, spent / original);
  });

  const saveCustomText = (id: number) => (v: string | null) =>
    onchange({ customFields: { [String(id)]: v } });
</script>

{#if field === 'description'}
  <WikiField
    {issue}
    {zone}
    {required}
    label={t.issue.description}
    source={issue.description}
    editable={edit}
    empty={t.issue.noDescription}
    add={t.issue.addDescription}
    onsave={(v) => onchange({ description: v })}
  />
{:else if field === 'environment'}
  {#if issue.environment || edit}
    <WikiField
      {issue}
      {zone}
      {required}
      label={t.issue.environment}
      source={issue.environment}
      editable={edit}
      onsave={(v) => onchange({ environment: v })}
    />
  {/if}
{:else if field === 'assignee'}
  <FieldFrame {zone} {required} label={t.issue.assignee}>
    <span class="stack">
      <UserPicker
        value={issue.assignee}
        label={t.issue.assignee}
        disabled={!issue.can.assign}
        search={(q) => api.assignable(issue.project.key, q)}
        onpick={(u) => void onchange({ assignee: u?.id ?? null })}
      />
      {#if !meIsAssignee && (issue.can.assign || issue.can.assignable) && session.me}
        <button
          class="link-btn small"
          onclick={() => void onchange({ assignee: session.me?.id ?? null })}
          >{t.common.assignToMe}</button
        >
      {/if}
    </span>
  </FieldFrame>
{:else if field === 'reporter'}
  <FieldFrame {zone} {required} label={t.issue.reporter}>
    <Who user={issue.reporter} empty="—" />
  </FieldFrame>
{:else if field === 'priority'}
  <FieldFrame {zone} {required} label={t.issue.priority}>
    {#if edit}
      <Dropdown
        quiet
        label={t.issue.priority}
        value={issue.priority?.id ?? ''}
        options={(session.meta?.priorities ?? []).map((p) => ({
          value: p.id,
          label: session.priorityName(p),
        }))}
        onchange={(v) => void onchange({ priority: v || null })}
      >
        {#snippet icon(id)}
          <PriorityMark priority={session.meta?.priorities.find((p) => p.id === id) ?? null} />
        {/snippet}
      </Dropdown>
    {:else}
      <PriorityMark priority={issue.priority} withName />
    {/if}
  </FieldFrame>
{:else if field === 'components'}
  <FieldFrame {zone} {required} label={t.issue.components}>
    <MultiPick
      options={project?.components ?? issue.components.map((c) => ({ ...c, archived: false }))}
      selected={ids(issue.components)}
      label={t.issue.components}
      disabled={!edit || !project}
      oncreate={configure ? createComponent : undefined}
      onchange={(v) => void onchange({ components: v })}
    />
  </FieldFrame>
{:else if field === 'fixVersions'}
  <FieldFrame {zone} {required} label={t.issue.fixVersions}>
    <MultiPick
      options={(project?.versions ?? issue.fixVersions).map((v) => ({
        id: v.id,
        name: v.name,
        note: 'released' in v && v.released ? t.projects.released : undefined,
      }))}
      selected={ids(issue.fixVersions)}
      label={t.issue.fixVersions}
      disabled={!edit || !project}
      oncreate={configure ? createVersion : undefined}
      onchange={(v) => void onchange({ fixVersions: v })}
    />
  </FieldFrame>
{:else if field === 'versions'}
  <FieldFrame {zone} {required} label={t.issue.affectsVersions}>
    <MultiPick
      options={(project?.versions ?? issue.affectsVersions).map((v) => ({
        id: v.id,
        name: v.name,
      }))}
      selected={ids(issue.affectsVersions)}
      label={t.issue.affectsVersions}
      disabled={!edit || !project}
      oncreate={configure ? createVersion : undefined}
      onchange={(v) => void onchange({ affectsVersions: v })}
    />
  </FieldFrame>
{:else if field === 'epic'}
  {#if issue.epic}
    <FieldFrame {zone} label={t.boards.epic}>
      <a class="key" href={paths.issue(issue.epic.key)}>{issue.epic.key}</a>
      {issue.epic.summary}
    </FieldFrame>
  {/if}
{:else if field === 'sprint'}
  {#if issue.sprints?.length}
    <FieldFrame {zone} label={t.boards.sprint}>
      <span class="sprints">
        {#each issue.sprints as sp (sp.id)}
          {#if sp.board && sp.state !== 'closed'}
            <a href={sp.state === 'active' ? paths.board(sp.board) : paths.backlog(sp.board)}
              >{sp.name}</a
            >
          {:else}
            <span class:muted={sp.state === 'closed'}>{sp.name}</span>
          {/if}
          <span class="muted">({t.boards.sprintStates[sp.state]})</span>
        {/each}
      </span>
    </FieldFrame>
  {/if}
{:else if field === 'labels'}
  <FieldFrame {zone} {required} label={t.issue.labels}>
    <LabelEditor labels={issue.labels} editable={edit} onchange={(l) => onchange({ labels: l })} />
  </FieldFrame>
{:else if field === 'duedate'}
  <FieldFrame {zone} {required} label={t.issue.due}>
    {#if edit}
      <input
        class="date"
        type="date"
        aria-label={t.issue.due}
        value={issue.due ?? ''}
        onchange={(e) =>
          void save(e.currentTarget, { due: e.currentTarget.value || null }, () => issue.due ?? '')}
      />
    {:else}
      {issue.due ? formatDay(issue.due) : '—'}
    {/if}
  </FieldFrame>
{:else if field === 'security'}
  {#if issue.securityLevel || issue.can.security}
    <FieldFrame {zone} {required} label={t.issue.security}>
      {#if issue.can.security && issue.securityLevels.length}
        <Dropdown
          quiet
          label={t.issue.security}
          value={issue.securityLevel ? String(issue.securityLevel.id) : ''}
          options={[
            { value: '', label: t.issue.public },
            ...issue.securityLevels.map((l) => ({ value: String(l.id), label: l.name })),
          ]}
          onchange={(v) => void onchange({ securityLevel: v ? Number(v) : null })}
        >
          {#snippet icon(id)}
            {#if id}<Lock />{/if}
          {/snippet}
        </Dropdown>
      {:else if issue.securityLevel}
        <span class="conf"><Lock /> {issue.securityLevel.name}</span>
      {:else}
        {t.issue.public}
      {/if}
    </FieldFrame>
  {/if}
{:else if field === 'created'}
  <FieldFrame {zone} label={t.issue.created}><RelTime iso={issue.created} /></FieldFrame>
{:else if field === 'updated'}
  <FieldFrame {zone} label={t.issue.updated}><RelTime iso={issue.updated} /></FieldFrame>
{:else if field === 'resolutiondate'}
  {#if issue.resolved}
    <FieldFrame {zone} label={t.issue.resolved}>
      <RelTime iso={issue.resolved} />
      {#if issue.resolution}<span class="muted">· {session.resolutionName(issue.resolution)}</span
        >{/if}
    </FieldFrame>
  {/if}
{:else if field === 'timetracking'}
  {#if edit || hasTime}
    <FieldFrame {zone} {required} label={t.issue.time}>
      <span class="time">
        {#if hasTime}
          <span class="bar" aria-hidden="true"
            ><span style:width={`${spentRatio * 100}%`}></span></span
          >
          <span class="nums">
            {#if time.spent}<span>{t.issue.spent} {formatDuration(time.spent)}</span>{/if}
            {#if time.original}<span>{t.issue.estimate} {formatDuration(time.original)}</span>{/if}
            {#if time.remaining !== null}<span
                >{t.issue.remaining} {formatDuration(time.remaining)}</span
              >{/if}
          </span>
        {/if}
        {#if edit}
          <span class="estimates">
            <label>
              <span class="muted">{t.issue.originalEstimate}</span>
              <input
                class="est"
                bind:value={originalText}
                placeholder={t.issue.estimateHint}
                aria-invalid={estimateError}
                onchange={() => void saveEstimates()}
              />
            </label>
            <label>
              <span class="muted">{t.issue.remainingEstimate}</span>
              <input
                class="est"
                bind:value={remainingText}
                placeholder={t.issue.estimateHint}
                aria-invalid={estimateError}
                onchange={() => void saveEstimates()}
              />
            </label>
          </span>
          {#if estimateError}<span class="error-text small" role="alert"
              >{t.issue.estimateInvalid}</span
            >{/if}
        {/if}
      </span>
    </FieldFrame>
  {/if}
{:else if offered?.type === 'textarea' || (!offered && custom?.kind === 'wiki')}
  <WikiField
    {issue}
    {zone}
    {required}
    label={offered?.name ?? custom?.name ?? ''}
    source={custom?.values[0] ?? null}
    editable={!!offered}
    onsave={saveCustomText(customId)}
  />
{:else if offered}
  {@const f = offered}
  <FieldFrame {zone} {required} label={f.name} labelFor="{uid}-cf-{f.id}">
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div
      class="cf-edit"
      onfocusout={(e) => fields.leaveCustom(e, f)}
      onkeydown={(e) => fields.customKeys(e, f)}
    >
      <CustomFieldInput
        field={f}
        id="{uid}-cf-{f.id}"
        quiet
        attachments={issue.attachments}
        value={fields.drafts[f.id] ?? draftOf(f, custom)}
        onchange={(v) => fields.setCustom(f, v)}
      />
    </div>
  </FieldFrame>
{:else if custom}
  {@const f = custom}
  <FieldFrame {zone} label={f.name}>
    {#if f.kind === 'user'}
      <span class="stack">
        {#each f.users ?? [] as u (u.id)}<Who user={u} />{/each}
      </span>
    {:else if f.kind === 'url'}
      {#each f.values as v, i (i)}
        {#if isSafeUrl(v)}
          <a href={v} rel="nofollow noopener noreferrer" target="_blank">{v}</a>
        {:else}
          {v}
        {/if}
      {/each}
    {:else if f.kind === 'date'}
      {f.values.map(formatDay).join(', ')}
    {:else if f.kind === 'datetime'}
      {f.values.map(formatDateTime).join(', ')}
    {:else if f.kind === 'issue'}
      {#each f.values as v (v)}<a class="key" href={paths.issue(v)}>{v}</a>{/each}
    {:else if f.kind === 'label'}
      <span class="chips"
        >{#each f.values as v (v)}<span class="chip">{v}</span>{/each}</span
      >
    {:else if f.kind === 'wiki'}
      {#each f.values as v, i (i)}<WikiText source={v} attachments={issue.attachments} />{/each}
    {:else}
      {f.values.join(', ')}
    {/if}
  </FieldFrame>
{/if}

<style>
  .sprints {
    display: flex;
    flex-wrap: wrap;
    gap: 2px 6px;
  }

  .stack {
    display: grid;
    gap: 4px;
    justify-items: start;
  }

  .small {
    font-size: 0.75rem;
  }

  .date {
    border: 1px solid transparent;
    background: transparent;
    padding: 1px 4px;
    margin-left: -5px;
    font: 0.8125rem var(--font-mono);
  }

  .date:hover {
    border-color: var(--rule-strong);
  }

  .conf {
    display: inline-flex;
    gap: 6px;
    align-items: center;
  }

  .time {
    display: grid;
    gap: 5px;
  }

  .bar {
    display: block;
    height: 5px;
    background: var(--paper-sunk);
    border: 1px solid var(--rule);
  }

  .bar span {
    display: block;
    height: 100%;
    background: var(--stamp-doing);
  }

  .nums {
    display: flex;
    flex-wrap: wrap;
    gap: 2px 10px;
    font: 0.75rem var(--font-mono);
    color: var(--ink-soft);
  }

  .estimates {
    display: grid;
    /* Side by side when there is room, each wide enough for the longest placeholder. */
    grid-template-columns: repeat(auto-fit, minmax(min(10rem, 100%), 1fr));
    gap: 4px 14px;
  }

  .estimates label {
    display: grid;
    gap: 1px;
    font-size: 0.75rem;
  }

  .est {
    border: 1px solid transparent;
    background: transparent;
    padding: 1px 4px;
    margin-left: -5px;
    font: 0.78125rem var(--font-mono);
    width: 100%;
  }

  .est:hover,
  .est:focus {
    border-color: var(--rule-strong);
  }

  .est[aria-invalid='true'] {
    border-color: var(--accent-text);
  }

  .chips {
    display: flex;
    flex-wrap: wrap;
    gap: 4px;
  }
</style>
