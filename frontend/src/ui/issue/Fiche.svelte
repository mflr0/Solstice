<script lang="ts">
  import { paths } from '../../core/route.js';
  import type { IssueDetail, ProjectDetail } from '../../core/types.js';
  import { formatDateTime, formatDay, formatDuration, t } from '../../i18n/fr.js';
  import { api, type IssueFields } from '../../lib/api.js';
  import { session } from '../../lib/session.svelte.js';
  import LabelEditor from '../LabelEditor.svelte';
  import Lock from '../Lock.svelte';
  import MultiPick from '../MultiPick.svelte';
  import PriorityMark from '../PriorityMark.svelte';
  import RelTime from '../RelTime.svelte';
  import TypeMark from '../TypeMark.svelte';
  import UserPicker from '../UserPicker.svelte';
  import Who from '../Who.svelte';
  import WikiText from '../WikiText.svelte';

  /**
   * The issue's record card: every field on its own ruled line, editable in
   * place for those allowed to.
   */
  interface Props {
    issue: IssueDetail;
    onchange: (fields: IssueFields) => Promise<void>;
  }
  const { issue, onchange }: Props = $props();

  let project = $state<ProjectDetail | null>(null);
  const edit = $derived(issue.can.edit === true);

  // Components, versions and types come from the project, only needed to edit.
  $effect(() => {
    const key = issue.project.key;
    if (!edit) return;
    void api.project(key).then((p) => {
      if (key === issue.project.key) project = p;
    });
  });

  const types = $derived(
    (project?.issueTypes ?? []).filter((ty) => ty.subtask === issue.type.subtask),
  );
  const ids = (list: { id: number }[]) => list.map((x) => x.id);
  const meIsAssignee = $derived(issue.assignee?.id === session.me?.id);
  const spentRatio = $derived.by(() => {
    const { original, spent } = issue.time;
    if (!original || !spent) return 0;
    return Math.min(1, spent / original);
  });
</script>

<aside class="fiche" aria-label={t.issue.fiche}>
  <div class="head">{t.issue.fiche} <span class="key">{issue.key}</span></div>
  <dl>
    <div class="line">
      <dt>{t.issue.assignee}</dt>
      <dd class="stack">
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
            >{t.issue.assignToMe}</button
          >
        {/if}
      </dd>
    </div>
    <div class="line">
      <dt>{t.issue.reporter}</dt>
      <dd><Who user={issue.reporter} empty="—" /></dd>
    </div>
    <div class="line">
      <dt>{t.issue.priority}</dt>
      <dd>
        {#if edit}
          <span class="inline-select">
            <PriorityMark priority={issue.priority} />
            <select
              aria-label={t.issue.priority}
              value={issue.priority?.id ?? ''}
              onchange={(e) => void onchange({ priority: e.currentTarget.value || null })}
            >
              {#each session.meta?.priorities ?? [] as p (p.id)}
                <option value={p.id}>{p.name}</option>
              {/each}
            </select>
          </span>
        {:else}
          <PriorityMark priority={issue.priority} withName />
        {/if}
      </dd>
    </div>
    <div class="line">
      <dt>{t.issue.type}</dt>
      <dd>
        {#if edit && types.length > 1}
          <span class="inline-select">
            <TypeMark type={issue.type} />
            <select
              aria-label={t.issue.type}
              value={issue.type.id}
              onchange={(e) => void onchange({ type: e.currentTarget.value })}
            >
              {#each types as ty (ty.id)}
                <option value={ty.id}>{ty.name}</option>
              {/each}
            </select>
          </span>
        {:else}
          <TypeMark type={issue.type} withName />
        {/if}
      </dd>
    </div>
    <div class="line">
      <dt>{t.issue.components}</dt>
      <dd>
        <MultiPick
          options={project?.components ?? issue.components.map((c) => ({ ...c, archived: false }))}
          selected={ids(issue.components)}
          label={t.issue.components}
          disabled={!edit || !project}
          onchange={(v) => void onchange({ components: v })}
        />
      </dd>
    </div>
    <div class="line">
      <dt>{t.issue.fixVersions}</dt>
      <dd>
        <MultiPick
          options={(project?.versions ?? issue.fixVersions).map((v) => ({
            id: v.id,
            name: v.name,
            note: 'released' in v && v.released ? t.projects.released : undefined,
          }))}
          selected={ids(issue.fixVersions)}
          label={t.issue.fixVersions}
          disabled={!edit || !project}
          onchange={(v) => void onchange({ fixVersions: v })}
        />
      </dd>
    </div>
    <div class="line">
      <dt>{t.issue.affectsVersions}</dt>
      <dd>
        <MultiPick
          options={(project?.versions ?? issue.affectsVersions).map((v) => ({
            id: v.id,
            name: v.name,
          }))}
          selected={ids(issue.affectsVersions)}
          label={t.issue.affectsVersions}
          disabled={!edit || !project}
          onchange={(v) => void onchange({ affectsVersions: v })}
        />
      </dd>
    </div>
    <div class="line">
      <dt>{t.issue.labels}</dt>
      <dd>
        <LabelEditor
          labels={issue.labels}
          editable={edit}
          onchange={(l) => void onchange({ labels: l })}
        />
      </dd>
    </div>
    <div class="line">
      <dt>{t.issue.due}</dt>
      <dd>
        {#if edit}
          <input
            class="date"
            type="date"
            aria-label={t.issue.due}
            value={issue.due ?? ''}
            onchange={(e) => void onchange({ due: e.currentTarget.value || null })}
          />
        {:else}
          {issue.due ? formatDay(issue.due) : '—'}
        {/if}
      </dd>
    </div>
    {#if issue.securityLevel || issue.can.security}
      <div class="line">
        <dt>{t.issue.security}</dt>
        <dd>
          {#if issue.can.security && issue.securityLevels.length}
            <span class="inline-select">
              {#if issue.securityLevel}<Lock />{/if}
              <select
                aria-label={t.issue.security}
                value={issue.securityLevel ? String(issue.securityLevel.id) : ''}
                onchange={(e) =>
                  void onchange({
                    securityLevel: e.currentTarget.value ? Number(e.currentTarget.value) : null,
                  })}
              >
                <option value="">{t.issue.public}</option>
                {#each issue.securityLevels as l (l.id)}
                  <option value={String(l.id)}>{l.name}</option>
                {/each}
              </select>
            </span>
          {:else if issue.securityLevel}
            <span class="conf"><Lock /> {issue.securityLevel.name}</span>
          {:else}
            {t.issue.public}
          {/if}
        </dd>
      </div>
    {/if}
  </dl>

  <dl class="dates">
    <div class="line">
      <dt>{t.issue.created}</dt>
      <dd><RelTime iso={issue.created} /></dd>
    </div>
    <div class="line">
      <dt>{t.issue.updated}</dt>
      <dd><RelTime iso={issue.updated} /></dd>
    </div>
    {#if issue.resolved}
      <div class="line">
        <dt>{t.issue.resolved}</dt>
        <dd>
          <span title={formatDateTime(issue.resolved)}><RelTime iso={issue.resolved} /></span>
          {#if issue.resolution}<span class="muted">· {issue.resolution.name}</span>{/if}
        </dd>
      </div>
    {/if}
    {#if issue.time.original || issue.time.spent || issue.time.remaining}
      <div class="line">
        <dt>{t.issue.time}</dt>
        <dd class="time">
          <span class="bar" aria-hidden="true"
            ><span style:width={`${spentRatio * 100}%`}></span></span
          >
          <span class="nums">
            {#if issue.time.spent}<span>{t.issue.spent} {formatDuration(issue.time.spent)}</span
              >{/if}
            {#if issue.time.original}<span
                >{t.issue.estimate} {formatDuration(issue.time.original)}</span
              >{/if}
            {#if issue.time.remaining !== null}<span
                >{t.issue.remaining} {formatDuration(issue.time.remaining)}</span
              >{/if}
          </span>
        </dd>
      </div>
    {/if}
  </dl>

  {#if issue.customFields.length}
    <div class="head sub">{t.issue.customFields}</div>
    <dl>
      {#each issue.customFields as f (f.id)}
        <div class="line">
          <dt>{f.name}</dt>
          <dd>
            {#if f.kind === 'user'}
              <span class="stack">
                {#each f.users ?? [] as u (u.id)}<Who user={u} />{/each}
              </span>
            {:else if f.kind === 'wiki'}
              {#each f.values as v, i (i)}<WikiText
                  source={v}
                  attachments={issue.attachments}
                />{/each}
            {:else if f.kind === 'url'}
              {#each f.values as v, i (i)}<a
                  href={v}
                  rel="nofollow noopener noreferrer"
                  target="_blank">{v}</a
                >{/each}
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
            {:else}
              {f.values.join(', ')}
            {/if}
          </dd>
        </div>
      {/each}
    </dl>
  {/if}
</aside>

<style>
  .fiche {
    background: var(--paper-raised);
    border: 1px solid var(--rule);
    border-top: 2px solid var(--accent);
    border-radius: 4px;
    font-size: 13.5px;
  }

  .head {
    display: flex;
    justify-content: space-between;
    align-items: baseline;
    padding: 10px 14px 8px;
    border-bottom: 1px solid var(--rule);
    font: 600 13px/1 var(--sans);
    color: var(--ink);
  }

  .head .key {
    letter-spacing: 0;
    font-size: 11.5px;
    color: var(--muted);
  }

  .head.sub {
    border-bottom-width: 1px;
    border-top: 1px solid var(--rule-strong);
    margin-top: 6px;
  }

  dl {
    margin: 0;
  }

  .dates {
    border-top: 1px solid var(--rule-strong);
  }

  /* Ruled like an index card, in pale blue ink. */
  .line {
    display: grid;
    grid-template-columns: 112px minmax(0, 1fr);
    gap: 10px;
    align-items: start;
    padding: 7px 14px 6px;
    border-bottom: 1px solid color-mix(in srgb, var(--rule) 70%, transparent);
    min-height: 34px;
  }

  .line:last-child {
    border-bottom: none;
  }

  dt {
    font-size: 12px;
    color: var(--muted);
    padding-top: 2px;
  }

  dd {
    margin: 0;
    min-width: 0;
  }

  .stack {
    display: grid;
    gap: 4px;
    justify-items: start;
  }

  .small {
    font-size: 12px;
  }

  .inline-select {
    position: relative;
    display: inline-flex;
    align-items: center;
    gap: 6px;
  }

  .inline-select select {
    appearance: none;
    border: 1px solid transparent;
    background: transparent;
    padding: 1px 18px 1px 4px;
    margin-left: -4px;
    cursor: pointer;
    background-image:
      linear-gradient(45deg, transparent 50%, var(--muted) 50%),
      linear-gradient(135deg, var(--muted) 50%, transparent 50%);
    background-position:
      right 7px center,
      right 3px center;
    background-size: 4px 4px;
    background-repeat: no-repeat;
  }

  .inline-select select:hover {
    border-color: var(--rule-strong);
    background-color: var(--paper);
  }

  .date {
    border: 1px solid transparent;
    background: transparent;
    padding: 1px 4px;
    margin-left: -5px;
    font: 13px var(--mono);
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
    font: 12px var(--mono);
    color: var(--ink-soft);
  }

  .chips {
    display: flex;
    flex-wrap: wrap;
    gap: 4px;
  }

  dd :global(.wiki) {
    font-size: 13.5px;
  }
</style>
