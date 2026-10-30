<script lang="ts">
  import { emptyFilters, filtersToJql } from '../../core/filters.js';
  import { searchHref } from '../../core/filters-url.js';
  import { quote, quoteString } from '../../core/jql.js';
  import { isIssueKey } from '../../core/marks.js';
  import { paths } from '../../core/route.js';
  import type { IssueSummary, Project, UserRef } from '../../core/types.js';
  import { t } from '../../i18n/t.js';
  import { api } from '../../lib/api.js';
  import { outside } from '../../lib/actions.js';
  import { router } from '../../lib/router.svelte.js';
  import Highlight from '../Highlight.svelte';
  import Stamp from '../Stamp.svelte';
  import TypeMark from '../TypeMark.svelte';
  import { catalog, matches } from './sources.svelte.js';

  /**
   * The top bar's search: issues (by key or words), projects and people as
   * one types; Enter on a key opens the issue, otherwise searches all
   * issues for the words. "/" focuses it (see App).
   */
  interface Row {
    group: 'issue' | 'project' | 'person' | 'all';
    /** For a person: their tickets, then the narrower lists and the profile. */
    kind: 'issue' | 'project' | 'person' | 'assigned' | 'reported' | 'profile' | 'all';
    href: string;
    issue?: IssueSummary;
    project?: Project;
    person?: UserRef;
  }

  /** The search page for a person in a role. */
  const peopleHref = (field: string, u: UserRef) => searchHref(`${field} = ${quote(u.username)}`);

  let q = $state('');
  let open = $state(false);
  let active = $state(-1);
  let issues = $state<IssueSummary[]>([]);
  let people = $state<UserRef[]>([]);
  let input: HTMLInputElement | undefined = $state();
  let timer: ReturnType<typeof setTimeout> | undefined;
  let seq = 0;
  const listId = 'quick-search-list';

  const typedKey = $derived(isIssueKey(q) ? q.trim().toUpperCase() : null);
  const projects = $derived(
    q.trim() ? (catalog.projects ?? []).filter((p) => matches(q, p.key, p.name)).slice(0, 3) : [],
  );
  const allHref = $derived(searchHref(filtersToJql({ ...emptyFilters(), text: q.trim() })));

  const rows = $derived.by<Row[]>(() => {
    if (!q.trim()) return [];
    return [
      ...issues.map((i): Row => ({
        group: 'issue',
        kind: 'issue',
        href: paths.issue(i.key),
        issue: i,
      })),
      ...projects.map((p): Row => ({
        group: 'project',
        kind: 'project',
        href: paths.project(p.key),
        project: p,
      })),
      ...people.flatMap((u): Row[] => [
        { group: 'person', kind: 'person', href: peopleHref('participant', u), person: u },
        { group: 'person', kind: 'assigned', href: peopleHref('assignee', u), person: u },
        { group: 'person', kind: 'reported', href: peopleHref('reporter', u), person: u },
        { group: 'person', kind: 'profile', href: paths.person(u.username), person: u },
      ]),
      { group: 'all', kind: 'all', href: allHref },
    ];
  });

  async function fetchRows() {
    const words = q.trim();
    const mine = ++seq;
    if (!words) {
      issues = [];
      people = [];
      return;
    }
    const key = isIssueKey(words) ? words.toUpperCase() : null;
    const [byKey, byText, users] = await Promise.all([
      key ? api.issues({ jql: `key = ${quote(key)}`, size: 1 }).catch(() => null) : null,
      api
        .issues({ jql: `text ~ ${quoteString(words)} ORDER BY updated DESC`, size: 6 })
        .catch(() => null),
      words.length >= 2 ? api.users(words).catch(() => []) : Promise.resolve([]),
    ]);
    if (mine !== seq) return;
    const found = [...(byKey?.issues ?? []), ...(byText?.issues ?? [])];
    issues = found.filter((i, n) => found.findIndex((j) => j.key === i.key) === n).slice(0, 6);
    people = users.slice(0, 3);
  }

  function typed() {
    open = true;
    active = -1;
    void catalog.loadProjects();
    clearTimeout(timer);
    timer = setTimeout(() => void fetchRows(), 160);
  }

  function go(href: string) {
    open = false;
    q = '';
    issues = [];
    people = [];
    input?.blur();
    router.navigate(href);
  }

  function submit(e: SubmitEvent) {
    e.preventDefault();
    const row = rows[active];
    if (row) return go(row.href);
    if (!q.trim()) return;
    // An exact key goes straight to the issue, without waiting for the list.
    go(typedKey ? paths.issue(typedKey) : allHref);
  }

  function keydown(e: KeyboardEvent) {
    if (e.key === 'ArrowDown' && rows.length) {
      e.preventDefault();
      open = true;
      active = (active + 1) % rows.length;
    } else if (e.key === 'ArrowUp' && rows.length) {
      e.preventDefault();
      open = true;
      active = active <= 0 ? rows.length - 1 : active - 1;
    } else if (e.key === 'Escape') {
      if (open && q) {
        e.preventDefault();
        open = false;
        active = -1;
      } else {
        input?.blur();
      }
    }
  }

  const optionId = (i: number) => `${listId}-${i}`;

  function actionLabel(row: Row): string {
    const name = row.person?.name ?? '';
    if (row.kind === 'assigned') return t.quick.assignedTo(name);
    if (row.kind === 'reported') return t.quick.reportedBy(name);
    return t.quick.profileOf(name);
  }
</script>

<form
  class="topbar-search quick-search"
  role="search"
  onsubmit={submit}
  use:outside={() => (open = false)}
>
  <label class="sr-only" for="quick-search">{t.quick.label}</label>
  <input
    bind:this={input}
    id="quick-search"
    class="input"
    bind:value={q}
    oninput={typed}
    onkeydown={keydown}
    onfocus={() => q.trim() && (open = true)}
    placeholder={t.quick.placeholder}
    autocomplete="off"
    spellcheck="false"
    role="combobox"
    aria-expanded={open && rows.length > 0}
    aria-controls={listId}
    aria-autocomplete="list"
    aria-activedescendant={open && active >= 0 ? optionId(active) : undefined}
  />
  <kbd class="kbd" aria-hidden="true">/</kbd>
  {#if open && rows.length}
    <ul class="popover quick-search-results" id={listId} role="listbox" aria-label={t.quick.label}>
      {#each rows as row, i (row.href + row.kind)}
        {#if row.group !== rows[i - 1]?.group && row.group !== 'all'}
          <li class="group-label" role="presentation">
            {row.group === 'issue'
              ? t.quick.issues
              : row.group === 'project'
                ? t.quick.projects
                : t.quick.people}
          </li>
        {/if}
        {#if row.kind === 'all'}<li class="sep" role="presentation"></li>{/if}
        {@const sub = row.kind === 'assigned' || row.kind === 'reported' || row.kind === 'profile'}
        <li
          id={optionId(i)}
          role="option"
          aria-selected={i === active}
          class:sub
          class:first={row.kind === 'assigned'}
        >
          <a
            class="item quick-{row.kind}"
            class:active={i === active}
            href={row.href}
            tabindex="-1"
            aria-label={row.person && row.kind !== 'person' ? actionLabel(row) : undefined}
            onclick={(e) => {
              e.preventDefault();
              go(row.href);
            }}
            onmouseenter={() => (active = i)}
          >
            {#if row.issue}
              <TypeMark type={row.issue.type} />
              <span class="key">{row.issue.key}</span>
              <span class="summary">{row.issue.summary}</span>
              <Stamp status={row.issue.status} />
            {:else if row.project}
              <span class="key"><Highlight text={row.project.key} search={q} /></span>
              <span class="summary"><Highlight text={row.project.name} search={q} /></span>
            {:else if row.person && row.kind === 'person'}
              <span class="summary"
                ><Highlight text={row.person.name} search={q} />
                <span class="who"><Highlight text={row.person.username} search={q} /></span></span
              >
              <span class="hint">{t.quick.ticketsOf(row.person.name)}</span>
            {:else if row.person}
              {row.kind === 'assigned'
                ? t.quick.assigned
                : row.kind === 'reported'
                  ? t.quick.reported
                  : t.quick.profile}
            {:else}
              <span class="glass" aria-hidden="true">⌕</span>
              <span class="summary">{t.quick.searchAll(q.trim())}</span>
              {#if active < 0 && !typedKey}<kbd class="kbd" aria-hidden="true">↵</kbd>{/if}
            {/if}
          </a>
        </li>
      {/each}
    </ul>
  {/if}
</form>

<style>
  /* The hint is prose: the narrower body face keeps it whole in the bar. */
  .quick-search .input::placeholder {
    font-family: var(--font-body);
  }

  .quick-search-results {
    left: auto;
    right: 0;
    top: calc(100% + 6px);
    width: min(28.75rem, calc(100vw - 24px));
    max-height: min(70vh, 32.5rem);
    list-style: none;
    margin: 0;
    color: var(--ink);
    font: 0.84375rem/1.35 var(--font-body);
  }

  .quick-search-results li[role='option'] {
    display: block;
  }

  .quick-search-results .item {
    gap: 8px;
    color: var(--ink);
  }

  .quick-search-results .key {
    flex: none;
    color: var(--ink-soft);
  }

  .summary {
    min-width: 0;
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .who {
    font: 0.71875rem var(--font-mono);
    color: var(--muted);
  }

  .hint {
    flex: none;
    font-size: 0.75rem;
    color: var(--muted);
  }

  /* A person's other lists and profile: small options on one line below them. */
  .quick-search-results li.sub {
    display: inline-block;
  }

  .quick-search-results li.sub.first {
    margin-left: 22px;
  }

  .quick-search-results li.sub .item {
    display: inline-flex;
    width: auto;
    padding: 3px 8px 5px;
    font-size: 0.78125rem;
    color: var(--ink-soft);
    text-decoration: underline;
    text-decoration-style: dotted;
    text-underline-offset: 3px;
  }

  .glass {
    color: var(--muted);
    width: 1.125rem;
    text-align: center;
  }

  .quick-search-results .kbd {
    flex: none;
  }

  .group-label {
    list-style: none;
  }
</style>
