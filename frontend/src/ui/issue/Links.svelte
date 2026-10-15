<script lang="ts">
  import { paths } from '../../core/route.js';
  import type { HiddenIssue, IssueDetail, IssueLink } from '../../core/types.js';
  import { t } from '../../i18n/t.js';
  import { api, errorMessage } from '../../lib/api.js';
  import { session } from '../../lib/session.svelte.js';
  import { notify } from '../../lib/ui.svelte.js';
  import Stamp from '../Stamp.svelte';
  import TypeMark from '../TypeMark.svelte';

  /** Issues linked to this one, grouped by relation, and external links. */
  interface Props {
    issue: IssueDetail;
    onupdated: (d: IssueDetail | HiddenIssue) => void;
    onreload: () => void;
  }
  const { issue, onupdated, onreload }: Props = $props();

  let adding = $state(false);
  let relation = $state('');
  let target = $state('');
  let busy = $state(false);

  const groups = $derived.by(() => {
    const out: [string, IssueLink[]][] = [];
    for (const l of issue.links) {
      const group = out.find(([label]) => label === l.label);
      if (group) group[1].push(l);
      else out.push([l.label, [l]]);
    }
    return out;
  });

  /** Each link type reads two ways ("blocks" / "is blocked by"). */
  const relations = $derived.by(() => {
    const out: { value: string; label: string }[] = [];
    for (const lt of session.meta?.linkTypes ?? []) {
      out.push({ value: `${lt.id}:outward`, label: lt.outward });
      if (lt.inward !== lt.outward) out.push({ value: `${lt.id}:inward`, label: lt.inward });
    }
    return out;
  });

  function startAdding() {
    adding = true;
    relation = relations[0]?.value ?? '';
    target = '';
  }

  async function add(e: SubmitEvent) {
    e.preventDefault();
    const [type, direction] = relation.split(':');
    if (!type || (direction !== 'outward' && direction !== 'inward') || !target.trim()) return;
    busy = true;
    try {
      onupdated(await api.addLink(issue.key, Number(type), direction, target.trim().toUpperCase()));
      adding = false;
    } catch (err) {
      notify(errorMessage(err), 'error');
    } finally {
      busy = false;
    }
  }

  async function remove(id: number) {
    try {
      await api.deleteLink(id);
      onreload();
    } catch (err) {
      notify(errorMessage(err), 'error');
    }
  }
</script>

<section aria-label={t.issue.links}>
  <h2 class="section-title">
    {t.issue.links}
    <span class="count">{issue.links.length + issue.remoteLinks.length}</span>
    {#if issue.can.link && !adding}
      <span class="actions">
        <button class="btn btn-small" onclick={startAdding}>{t.issue.addLink}</button>
      </span>
    {/if}
  </h2>

  {#if adding}
    <form class="add row" onsubmit={add}>
      <label class="sr-only" for="link-kind">{t.issue.linkKind}</label>
      <select id="link-kind" class="select kind" bind:value={relation}>
        {#each relations as r (r.value)}
          <option value={r.value}>{r.label}</option>
        {/each}
      </select>
      <label class="sr-only" for="link-target">{t.issue.linkTarget}</label>
      <input
        id="link-target"
        class="input target"
        bind:value={target}
        placeholder={t.issue.linkPlaceholder}
        autocomplete="off"
        spellcheck="false"
      />
      <button class="btn btn-primary btn-small" disabled={busy || !target.trim()}
        >{t.common.add}</button
      >
      <button type="button" class="btn btn-quiet btn-small" onclick={() => (adding = false)}
        >{t.common.cancel}</button
      >
    </form>
  {/if}

  {#if groups.length === 0 && issue.remoteLinks.length === 0 && !adding}
    <p class="empty">{t.issue.noLinks}</p>
  {/if}

  {#each groups as [label, links] (label)}
    <div class="group">
      <h3 class="relation">{label}</h3>
      <ul class="list">
        {#each links as l (l.id)}
          <li>
            <TypeMark type={l.issue.type} />
            <a class="key" href={paths.issue(l.issue.key)}>{l.issue.key}</a>
            <span class="summary">{l.issue.summary}</span>
            <Stamp status={l.issue.status} />
            {#if issue.can.link}
              <button
                class="del"
                aria-label={`${t.common.remove} ${l.issue.key}`}
                onclick={() => void remove(l.id)}>×</button
              >
            {/if}
          </li>
        {/each}
      </ul>
    </div>
  {/each}

  {#if issue.remoteLinks.length}
    <div class="group">
      <h3 class="relation">{t.issue.remoteLinks}</h3>
      <ul class="list">
        {#each issue.remoteLinks as r (r.id)}
          <li>
            <span class="arrow" aria-hidden="true">↗</span>
            <a href={r.url} target="_blank" rel="nofollow noopener noreferrer" class="summary"
              >{r.title}</a
            >
            {#if r.relationship}<span class="muted">{r.relationship}</span>{/if}
          </li>
        {/each}
      </ul>
    </div>
  {/if}
</section>

<style>
  .add {
    margin-bottom: 12px;
  }

  .kind {
    width: auto;
    min-width: 180px;
  }

  .target {
    width: 140px;
    font-family: var(--font-mono);
    text-transform: uppercase;
  }

  .group + .group {
    margin-top: 10px;
  }

  .relation {
    font: 500 12px/1.4 var(--font-body);
    color: var(--muted);
    margin: 0 0 4px;
  }

  .list {
    list-style: none;
    margin: 0;
    padding: 0;
  }

  .list li {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 5px 0;
    border-bottom: 1px solid var(--rule);
  }

  .summary {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .arrow {
    width: 18px;
    text-align: center;
    color: var(--muted);
  }

  .del {
    border: none;
    background: none;
    color: var(--muted);
    cursor: pointer;
    font-size: 15px;
    padding: 0 4px;
  }

  .del:hover {
    color: var(--accent-text);
  }
</style>
