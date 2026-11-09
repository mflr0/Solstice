<script lang="ts">
  import { tick } from 'svelte';
  import { focusAfterRemoval } from '../../core/focus.js';
  import { findIssues } from '../../core/issue-search.js';
  import { paths } from '../../core/route.js';
  import type { HiddenIssue, IssueDetail, IssueLink, IssueSummary } from '../../core/types.js';
  import { t } from '../../i18n/t.js';
  import { outside, refocus } from '../../lib/actions.js';
  import { api, errorMessage } from '../../lib/api.js';
  import { session } from '../../lib/session.svelte.js';
  import { notify } from '../../lib/ui.svelte.js';
  import Dropdown from '../Dropdown.svelte';
  import Stamp from '../Stamp.svelte';
  import TypeMark from '../TypeMark.svelte';
  import { isSafeUrl } from '../../core/wiki.js';

  /** Issues linked to this one, grouped by relation, and external links. */
  interface Props {
    issue: IssueDetail;
    onupdated: (d: IssueDetail | HiddenIssue) => void;
    onreload: () => Promise<void>;
  }
  const { issue, onupdated, onreload }: Props = $props();
  const uid = $props.id();

  let adding = $state(false);
  let relation = $state('');
  let target = $state('');
  let busy = $state(false);
  let addButton = $state<HTMLElement>();
  let suggestions = $state<IssueSummary[]>([]);
  let open = $state(false);
  let active = $state(-1);
  let timer: ReturnType<typeof setTimeout> | undefined;
  let seq = 0;
  const listId = 'link-target-list';
  const optionId = (i: number) => `${listId}-${i}`;

  const groups = $derived.by(() => {
    const out: [string, IssueLink[]][] = [];
    for (const l of issue.links) {
      const label = session.linkVerb(l.type, l.direction);
      const group = out.find(([g]) => g === label);
      if (group) group[1].push(l);
      else out.push([label, [l]]);
    }
    return out;
  });

  /** Each link type reads two ways ("blocks" / "is blocked by"). */
  const relations = $derived.by(() => {
    const out: { value: string; label: string }[] = [];
    for (const lt of session.meta?.linkTypes ?? []) {
      out.push({ value: `${lt.id}:outward`, label: session.linkVerb(lt, 'outward') });
      if (lt.inward !== lt.outward)
        out.push({ value: `${lt.id}:inward`, label: session.linkVerb(lt, 'inward') });
    }
    return out;
  });

  async function startAdding() {
    adding = true;
    relation = relations[0]?.value ?? '';
    target = '';
    closeList();
    await tick();
    document.getElementById(`link-kind-${uid}`)?.focus();
  }

  async function stopAdding() {
    adding = false;
    closeList();
    await tick();
    refocus(addButton);
  }

  function closeList() {
    clearTimeout(timer);
    seq++;
    open = false;
    active = -1;
    suggestions = [];
  }

  async function fetchSuggestions() {
    const mine = ++seq;
    const exclude = [issue.key, ...issue.links.map((l) => l.issue.key)];
    const found = await findIssues(target, api.issues, { exclude });
    if (mine === seq) suggestions = found;
  }

  function typed() {
    open = true;
    active = -1;
    clearTimeout(timer);
    if (!target.trim()) return closeList();
    timer = setTimeout(() => void fetchSuggestions(), 160);
  }

  function pick(i: IssueSummary) {
    target = i.key;
    closeList();
    document.getElementById('link-target')?.focus();
  }

  function keydown(e: KeyboardEvent) {
    const n = suggestions.length;
    const current = suggestions[active];
    if (e.key === 'ArrowDown' && n) {
      e.preventDefault();
      open = true;
      active = (active + 1) % n;
    } else if (e.key === 'ArrowUp' && n) {
      e.preventDefault();
      open = true;
      active = active <= 0 ? n - 1 : active - 1;
    } else if (e.key === 'Enter' && open && current) {
      e.preventDefault();
      pick(current);
    } else if (e.key === 'Escape' && open && n) {
      e.preventDefault();
      e.stopPropagation();
      open = false;
      active = -1;
    }
  }

  async function add(e: SubmitEvent) {
    e.preventDefault();
    const [type, direction] = relation.split(':');
    if (!type || (direction !== 'outward' && direction !== 'inward') || !target.trim()) return;
    busy = true;
    try {
      onupdated(await api.addLink(issue.key, Number(type), direction, target.trim().toUpperCase()));
      await stopAdding();
    } catch (err) {
      notify(errorMessage(err), 'error');
    } finally {
      busy = false;
    }
  }

  async function remove(id: number) {
    const next = focusAfterRemoval(
      groups.flatMap(([, links]) => links.map((l) => l.id)),
      id,
    );
    try {
      await api.deleteLink(id);
      await onreload();
      await tick();
      refocus(
        document.getElementById(`link-remove-${next}-${uid}`) ??
          addButton ??
          document.getElementById(`link-kind-${uid}`),
      );
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
        <button bind:this={addButton} class="btn btn-small" onclick={() => void startAdding()}
          >{t.issue.addLink}</button
        >
      </span>
    {/if}
  </h2>

  {#if adding}
    <form class="add row" onsubmit={add}>
      <span class="kind">
        <Dropdown
          id="link-kind-{uid}"
          label={t.issue.linkKind}
          value={relation}
          options={relations}
          onchange={(v) => (relation = v)}
        />
      </span>
      <label class="sr-only" for="link-target">{t.issue.linkTarget}</label>
      <span class="pick" use:outside={() => (open = false)}>
        <input
          id="link-target"
          class="input target"
          bind:value={target}
          oninput={typed}
          onkeydown={keydown}
          onfocus={() => target.trim() && (open = true)}
          placeholder={t.issue.linkPlaceholder}
          autocomplete="off"
          spellcheck="false"
          role="combobox"
          aria-expanded={open && suggestions.length > 0}
          aria-controls={listId}
          aria-autocomplete="list"
          aria-activedescendant={open && active >= 0 ? optionId(active) : undefined}
        />
        {#if open && suggestions.length}
          <ul
            class="popover suggestions"
            id={listId}
            role="listbox"
            aria-label={t.issue.linkTarget}
          >
            {#each suggestions as s, i (s.key)}
              <li id={optionId(i)} role="option" aria-selected={i === active}>
                <button
                  type="button"
                  class="item"
                  class:active={i === active}
                  tabindex="-1"
                  onclick={() => pick(s)}
                  onmouseenter={() => (active = i)}
                >
                  <TypeMark type={s.type} />
                  <span class="key">{s.key}</span>
                  <span class="summary">{s.summary}</span>
                  <Stamp status={s.status} />
                </button>
              </li>
            {/each}
          </ul>
        {/if}
      </span>
      <button class="btn btn-primary btn-small" disabled={busy || !target.trim()}
        >{t.common.add}</button
      >
      <button type="button" class="btn btn-quiet btn-small" onclick={() => void stopAdding()}
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
                id={`link-remove-${l.id}-${uid}`}
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
            {#if isSafeUrl(r.url)}
              <a href={r.url} target="_blank" rel="nofollow noopener noreferrer" class="summary"
                >{r.title}</a
              >
            {:else}
              <span class="summary">{r.title}</span>
            {/if}
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
    min-width: 11.25rem;
  }

  .pick {
    position: relative;
    flex: 1 1 16rem;
    max-width: 22rem;
  }

  .target {
    width: 100%;
  }

  .suggestions {
    left: 0;
    top: calc(100% + 4px);
    width: min(28.75rem, calc(100vw - 32px));
    list-style: none;
    margin: 0;
  }

  .suggestions .key {
    flex: none;
    font-family: var(--font-mono);
    color: var(--ink-soft);
  }

  .group + .group {
    margin-top: 10px;
  }

  .relation {
    font: 500 0.75rem/1.4 var(--font-body);
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
    width: 1.125rem;
    text-align: center;
    color: var(--muted);
  }

  .del {
    border: none;
    background: none;
    color: var(--muted);
    cursor: pointer;
    font-size: 0.9375rem;
    padding: 0 4px;
  }

  .del:hover {
    color: var(--accent-text);
  }
</style>
