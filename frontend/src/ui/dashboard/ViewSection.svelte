<script lang="ts">
  import type { SortTerm } from '../../core/jql.js';
  import type { UserRef } from '../../core/types.js';
  import { t } from '../../i18n/t.js';
  import Avatar from '../Avatar.svelte';
  import CardMenu, { type MenuGroup } from '../board/CardMenu.svelte';
  import IssueQuery from '../IssueQuery.svelte';
  import QuerySummary from './QuerySummary.svelte';

  /** The issues the server gives at most in one page. */
  const MAX_SHOWN = 200;

  /**
   * One view in the user's space: a header that folds it and says whose it
   * is, then its first issues, a few more on request.
   */
  interface Props {
    /** Unique on the page: the ids of the toggle and the body. */
    key: string;
    title: string;
    /** The scoped query shown; null when the view is unavailable. */
    query: string | null;
    /** The view with its filters (its own page, or a new view on a built-in). */
    href: string | null;
    /** The query as the summary reads it, in JQL. */
    jql: string;
    size: number;
    collapsed: boolean;
    /** "Shared with …" on one's own view. */
    sharedWith: string | null;
    /** The owner of someone else's view. */
    sharedBy: UserRef | null;
    columns: string[] | undefined;
    names: Record<string, string>;
    empty: string;
    showAssignee: boolean;
    sort: SortTerm[];
    onsort: (column: string) => void;
    ontoggle: () => void;
    /** Opens the share dialog: on one's own views only. */
    onshare?: (() => void) | undefined;
    menu: () => MenuGroup[];
    /** The count the header shows, null until known. */
    ontotal?: ((total: number | null) => void) | undefined;
  }
  const {
    key,
    title,
    query,
    href,
    jql,
    size,
    collapsed,
    sharedWith,
    sharedBy,
    columns,
    names,
    empty,
    showAssignee,
    sort,
    onsort,
    ontoggle,
    onshare,
    menu,
    ontotal,
  }: Props = $props();

  let total = $state<number | null>(null);
  let extra = $state(0);
  const shown = $derived(Math.min(size + extra, MAX_SHOWN));
  const more = $derived(total === null ? 0 : Math.min(size, total - shown, MAX_SHOWN - shown));

  $effect(() => {
    ontotal?.(query === null ? null : total);
  });
</script>

<section id="view-section-{key}" class="view" aria-labelledby="view-title-{key}">
  <div class="head">
    <button
      id="view-toggle-{key}"
      type="button"
      class="toggle"
      aria-expanded={!collapsed}
      aria-controls="view-body-{key}"
      aria-label={collapsed ? t.home.expand(title) : t.home.collapse(title)}
      onclick={ontoggle}
    >
      <svg class:folded={collapsed} viewBox="0 0 14 14" width="14" height="14" aria-hidden="true">
        <path d="M4 5.5l3 3 3-3" />
      </svg>
    </button>
    <h2 id="view-title-{key}" class="name">{title}</h2>
    {#if query !== null && total !== null}<span class="count">{total}</span>{/if}
    {#if sharedWith}
      <span class="shared out">{sharedWith}</span>
    {:else if sharedBy}
      <span class="shared by"><Avatar user={sharedBy} />{t.home.sharedBy(sharedBy.name)}</span>
    {/if}
    <span class="gap"></span>
    {#if href}
      <a class="filters" {href} title={t.home.editFilters}><QuerySummary {jql} /></a>
    {/if}
    <div class="tools">
      {#if onshare}
        <button
          type="button"
          class="icon-btn"
          aria-label={t.viewShare.open(title)}
          title={t.viewShare.open(title)}
          onclick={onshare}
        >
          <svg width="15" height="15" viewBox="0 0 16 16" aria-hidden="true"
            ><path d="M8 10V2.5M5 5.5l3-3 3 3M3 9v4.5h10V9" /></svg
          >
        </button>
      {/if}
      <CardMenu label={t.home.actions(title)} groups={menu} />
    </div>
  </div>

  <div id="view-body-{key}" class="body" hidden={collapsed}>
    {#if query === null}
      <p class="unavailable">{t.home.unavailable}</p>
    {:else}
      <IssueQuery
        q={query}
        size={shown}
        {empty}
        {showAssignee}
        {columns}
        {names}
        {sort}
        {onsort}
        fixed
        ontotal={(n) => (total = n)}
      />
      {#if more > 0}
        <div class="foot">
          <button type="button" class="link-btn" onclick={() => (extra += size)}
            >{t.home.showMore(more)}</button
          >
          {#if href}<a {href}>{t.home.openFull}</a>{/if}
        </div>
      {/if}
    {/if}
  </div>
</section>

<style>
  .view {
    min-width: 0;
    scroll-margin-top: calc(var(--topbar-height, 0px) + var(--view-index-height, 0px) + 16px);
    border: 1px solid var(--rule);
    border-radius: var(--radius-m);
    background: var(--paper-raised);
  }

  .head {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px 12px;
    min-height: 52px;
    padding: 10px 12px 10px 8px;
    box-sizing: border-box;
  }

  .toggle {
    display: grid;
    place-items: center;
    width: 32px;
    height: 32px;
    padding: 0;
    border: 0;
    border-radius: var(--radius-s);
    background: transparent;
    color: var(--ink-soft);
    cursor: pointer;
  }

  .toggle:hover {
    background: var(--paper-sunk);
  }

  .toggle svg {
    fill: none;
    stroke: currentColor;
    stroke-width: 1.6;
    stroke-linecap: round;
    stroke-linejoin: round;
    transition: transform 0.15s;
  }

  .toggle svg.folded {
    transform: rotate(-90deg);
  }

  .name {
    min-width: 0;
    margin: 0;
    font-size: 1.0625rem;
    line-height: 1.2;
    overflow-wrap: anywhere;
  }

  .count {
    padding: 1px 6px;
    border-radius: var(--radius-xs);
    background: var(--paper-sunk);
    color: var(--ink-soft);
    font: 0.75rem var(--font-mono);
  }

  .shared {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    min-width: 0;
    font-size: 0.75rem;
    color: var(--ink-soft);
  }

  .gap {
    flex: 1 1 40px;
  }

  .tools {
    display: flex;
    align-items: center;
    gap: 2px;
  }

  .icon-btn {
    display: grid;
    place-items: center;
    width: 32px;
    height: 32px;
    padding: 0;
    border: 0;
    border-radius: var(--radius-s);
    background: transparent;
    color: var(--ink-soft);
    cursor: pointer;
  }

  .icon-btn:hover {
    background: var(--paper-sunk);
    color: var(--ink);
  }

  .icon-btn svg {
    fill: none;
    stroke: currentColor;
    stroke-width: 1.5;
    stroke-linecap: round;
    stroke-linejoin: round;
  }

  .filters {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 4px;
    min-width: 0;
    max-width: 100%;
    padding: 4px 8px;
    border: 1px dashed var(--rule-strong);
    border-radius: var(--radius-s);
    color: var(--ink-soft);
    font-size: 0.75rem;
    text-decoration: none;
  }

  .filters:hover {
    border-color: var(--accent);
    color: var(--ink);
  }

  .body {
    border-top: 1px solid var(--rule);
    padding: 4px 6px 0;
  }

  .body[hidden] {
    display: none;
  }

  .body :global(.ledger tbody tr:last-child td) {
    border-bottom: 0;
  }

  .body :global(.empty),
  .body :global(.loading),
  .body :global(.error-text) {
    padding: 0 10px;
  }

  .unavailable {
    margin: 10px;
    padding: 8px 12px;
    border-left: 3px solid var(--rule);
    color: var(--muted);
  }

  .foot {
    display: flex;
    flex-wrap: wrap;
    justify-content: space-between;
    align-items: center;
    gap: 8px;
    padding: 8px 10px;
    border-top: 1px solid var(--rule);
    font-size: 0.8125rem;
  }
</style>
