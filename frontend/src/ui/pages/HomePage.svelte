<script lang="ts">
  import { moveItem } from '../../core/catalogs.js';
  import {
    clickGadgetSort,
    copyOnEditPath,
    gadgetQuery,
    MAX_GADGETS,
    onFilter,
    queryOrder,
    setCollapsed,
    sortedQuery,
    toInput,
    viewJql,
    type Dashboard,
    type Gadget,
  } from '../../core/dashboard.js';
  import { writeLocation } from '../../core/filters-url.js';
  import { focusOrderAfterRemoval } from '../../core/focus.js';
  import type { SortTerm } from '../../core/jql.js';
  import { paths } from '../../core/route.js';
  import { scopeGadgetQuery } from '../../core/scope.js';
  import { NATIVE_COLUMNS, type ColumnChoice } from '../../core/search-columns.js';
  import type { FilterShare, SavedFilter } from '../../core/types.js';
  import { formatList } from '../../i18n/format.js';
  import { t } from '../../i18n/t.js';
  import { refocusLater } from '../../lib/actions.js';
  import { api, errorMessage } from '../../lib/api.js';
  import { router } from '../../lib/router.svelte.js';
  import { session } from '../../lib/session.svelte.js';
  import { notify } from '../../lib/ui.svelte.js';
  import type { MenuGroup } from '../board/CardMenu.svelte';
  import ConfirmDialog from '../ConfirmDialog.svelte';
  import NewViewMenu from '../dashboard/NewViewMenu.svelte';
  import ViewSection from '../dashboard/ViewSection.svelte';
  import EmptyState from '../EmptyState.svelte';
  import Modal from '../Modal.svelte';

  /**
   * The user's space: their views, and the ones shared with them, one
   * section each, folded, ordered and managed in place. A built-in view is
   * copied into one of the user's own filters when they change it.
   */

  let dash = $state<Dashboard | null>(null);
  let error = $state('');
  let saving = $state(false);
  let choices = $state<ColumnChoice[] | null>(null);
  let filters = $state<SavedFilter[]>([]);
  let renaming = $state<{ filter: SavedFilter; name: string } | null>(null);
  let deleting = $state<SavedFilter | null>(null);

  const gadgets = $derived(dash?.gadgets ?? []);
  const names = $derived(Object.fromEntries((choices ?? []).map((c) => [c.id, c.name])));
  const allCollapsed = $derived(gadgets.length > 0 && gadgets.every((g) => g.config.collapsed));

  $effect(() => {
    document.title = `${t.home.title} — ${t.appName}`;
  });

  $effect(() => {
    api.dashboard().then(
      (d) => (dash = d),
      (e: unknown) => (error = errorMessage(e)),
    );
    api.issueColumns().then(
      (list) => (choices = list),
      () => (choices = []),
    );
    void loadFilters();
  });

  async function loadFilters() {
    try {
      filters = await api.filters();
    } catch {
      // The sections show without who shares them.
    }
  }

  /** A key per gadget, the defaults having no id yet. */
  const keyOf = (g: Gadget, i: number) => (g.id ? `g${g.id}` : `d${i}`);

  /** Focus to the toggle of the section now at i: saved, a default gadget gets an id. */
  function focusToggle(i: number) {
    const g = gadgets[i];
    if (g) queueMicrotask(() => document.getElementById(`view-toggle-${keyOf(g, i)}`)?.focus());
  }

  const filterOf = (g: Gadget) =>
    onFilter(g.kind) && !g.unavailable && g.filter
      ? filters.find((f) => f.id === g.filter?.id)
      : undefined;

  function title(g: Gadget): string {
    if (onFilter(g.kind)) return g.filter?.name ?? t.home.unavailableTitle;
    return t.home.kinds[g.kind];
  }

  function empty(g: Gadget): string {
    switch (g.kind) {
      case 'assigned':
        return t.home.emptyAssigned;
      case 'reported':
        return t.home.emptyReported;
      case 'recent':
        return t.home.emptyRecent;
      case 'watching':
        return t.home.emptyWatching;
      case 'due':
        return t.home.emptyDue;
      default:
        return t.home.emptyFilter;
    }
  }

  /** The filter's columns the search still knows (a custom field may be gone); the defaults without. */
  function columnsOf(g: Gadget): string[] | undefined {
    const cols = filterOf(g)?.columns;
    if (!cols?.length) return undefined;
    const known = choices ?? [];
    return known.length
      ? cols.filter((c) => known.some((x) => x.id === c))
      : cols.filter((c) => NATIVE_COLUMNS.has(c));
  }

  /** The gadget's sort, unless it is on a custom field gone since. */
  function sortOf(g: Gadget): SortTerm | undefined {
    const sort = g.config.sort;
    const cf = sort && /^cf\[(\d+)\]$/.exec(sort.field);
    const gone = cf && choices?.length && !choices.some((c) => c.id === `customfield_${cf[1]}`);
    return gone ? undefined : sort;
  }

  /** The gadget's query as a view, in JQL, unscoped; null when it has none. */
  function viewOf(g: Gadget): string | null {
    const q = gadgetQuery(g);
    return q === null ? null : viewJql(q, sortOf(g));
  }

  /** The query the section shows: its own, scoped, in the order saved on it. */
  function shownQuery(g: Gadget): string | null {
    const q = scopeGadgetQuery(gadgetQuery(g), g.kind, session.scope);
    return q === null ? null : sortedQuery(q, sortOf(g));
  }

  function hrefOf(g: Gadget): string | null {
    if (onFilter(g.kind)) return g.filter && !g.unavailable ? paths.view(g.filter.id) : null;
    const jql = viewOf(g);
    return jql === null
      ? null
      : `/views/new${writeLocation({ jql, mode: null, filter: null, page: 1 })}`;
  }

  function shareName(s: FilterShare): string {
    return s.kind === 'everyone' ? t.home.everyone : (s.name ?? t.boards.shareKinds[s.kind]);
  }

  function sharedWith(g: Gadget): string | null {
    const f = filterOf(g);
    return f?.mine && f.shares.length
      ? t.home.sharedWith(formatList(f.shares.map(shareName)))
      : null;
  }

  function sharedBy(g: Gadget) {
    const f = filterOf(g);
    return f && !f.mine ? f.owner : null;
  }

  /** Saves the gadgets in this order; the list shows the change at once. */
  async function save(next: Gadget[]) {
    const before = dash;
    dash = { customised: true, gadgets: next };
    saving = true;
    try {
      dash = await api.saveDashboard(next.map(toInput));
    } catch (e) {
      dash = before;
      throw e;
    } finally {
      saving = false;
    }
  }

  function saveQuietly(next: Gadget[], then?: () => void) {
    save(next).then(then, (e: unknown) => notify(errorMessage(e), 'error'));
  }

  function sortGadget(g: Gadget, column: string, shown: SortTerm[]) {
    if (saving) return;
    const sort = clickGadgetSort(column, g.config.sort, shown);
    saveQuietly(gadgets.map((x) => (x === g ? { ...g, config: { ...g.config, sort } } : x)));
  }

  function toggle(i: number) {
    const g = gadgets[i];
    if (!g || saving) return;
    const keep = !dash?.customised;
    saveQuietly(
      setCollapsed(gadgets, (x) => x === g, !g.config.collapsed),
      keep ? () => focusToggle(i) : undefined,
    );
  }

  function toggleAll() {
    if (!saving) saveQuietly(setCollapsed(gadgets, () => true, !allCollapsed));
  }

  function step(i: number, by: -1 | 1) {
    const to = i + by;
    if (saving || to < 0 || to >= gadgets.length) return;
    saveQuietly(moveItem(gadgets, i, to), () => focusToggle(to));
  }

  /** By position in the ids: once saved, a default gadget gets an id. */
  function remove(i: number) {
    if (saving) return;
    const focus = focusOrderAfterRemoval(
      gadgets.map((_, k) => k),
      i,
      (k) => `view-toggle-${keyOf(gadgets[k]!, k)}`,
      'view-new',
    );
    const customised = dash?.customised;
    save(gadgets.filter((_, k) => k !== i)).then(
      () => (customised ? refocusLater(focus) : focusToggle(Math.min(i, gadgets.length - 1))),
      (e: unknown) => notify(errorMessage(e), 'error'),
    );
  }

  /** Adds the view at the end of the space, expanded. */
  function add(g: Gadget) {
    if (saving || gadgets.length >= MAX_GADGETS) return;
    const at = gadgets.length;
    saveQuietly([...gadgets, g], () => focusToggle(at));
  }

  /** A built-in view edited becomes a new view of the user's, in its place. */
  async function editBuiltin(i: number) {
    try {
      // A default dashboard has no ids yet: the new view needs one to replace.
      if (!dash?.customised) await save(gadgets);
      const g = gadgets[i];
      const jql = g ? viewOf(g) : null;
      if (g?.id && jql !== null) router.navigate(copyOnEditPath(jql, g.id));
    } catch (e) {
      notify(errorMessage(e), 'error');
    }
  }

  async function duplicate(i: number) {
    const g = gadgets[i];
    const query = g ? viewOf(g) : null;
    if (!g || query === null || saving) return;
    try {
      const columns = filterOf(g)?.columns;
      const f = await api.createFilter({
        name: t.home.copyName(title(g)),
        query,
        ...(columns ? { columns } : {}),
      });
      const copy: Gadget = {
        id: 0,
        kind: 'results',
        filter: { id: f.id, name: f.name, query: f.query },
        unavailable: false,
        config: g.config.size ? { size: g.config.size } : {},
      };
      await save([...gadgets.slice(0, i + 1), copy, ...gadgets.slice(i + 1)]);
      await loadFilters();
      notify(t.view.created(f.name));
      focusToggle(i + 1);
    } catch (e) {
      notify(errorMessage(e), 'error');
    }
  }

  async function rename() {
    if (!renaming) return;
    const { filter, name } = renaming;
    const next = name.trim();
    if (!next || next === filter.name) {
      renaming = null;
      return;
    }
    try {
      await api.updateFilter(filter.id, { name: next });
      renaming = null;
      dash = await api.dashboard();
      await loadFilters();
    } catch (e) {
      notify(errorMessage(e), 'error');
    }
  }

  async function deleteView(f: SavedFilter) {
    await api.deleteFilter(f.id);
    await save(gadgets.filter((g) => !(onFilter(g.kind) && g.filter?.id === f.id)));
    await loadFilters();
    notify(t.home.deleted(f.name));
  }

  function menu(g: Gadget, i: number): MenuGroup[] {
    const f = filterOf(g);
    const view: MenuGroup['items'] = [];
    if (f?.canEdit) {
      view.push({ label: t.home.rename, run: () => (renaming = { filter: f, name: f.name }) });
    } else if (!onFilter(g.kind)) {
      view.push(
        { label: t.home.rename, run: () => void editBuiltin(i) },
        { label: t.home.edit, run: () => void editBuiltin(i) },
      );
    }
    if (viewOf(g) !== null) {
      view.push({
        label: t.home.duplicate,
        run: () => void duplicate(i),
        disabled: gadgets.length >= MAX_GADGETS,
      });
    }
    if (f?.mine) view.push({ label: t.home.deleteView, run: () => (deleting = f) });
    const place: MenuGroup['items'] = [
      { label: t.home.moveUp, run: () => step(i, -1), disabled: i === 0 },
      { label: t.home.moveDown, run: () => step(i, 1), disabled: i === gadgets.length - 1 },
      { label: t.home.removeFromSpace, run: () => remove(i) },
    ];
    return [
      ...(view.length ? [{ label: t.home.menuView, items: view }] : []),
      { label: t.home.menuSpace, items: place },
    ];
  }
</script>

<div class="page" data-title={t.home.title}>
  <header class="space-head">
    <div class="heading">
      <h1 class="page-title">{t.home.title}</h1>
      <p class="intro muted">
        {session.me
          ? `${t.home.greeting(session.me.name.split(' ')[0] ?? '')} ${t.home.intro}`
          : t.home.intro}
      </p>
    </div>
    {#if dash}
      <div class="head-actions">
        {#if gadgets.length > 0}
          <button type="button" class="link-btn" disabled={saving} onclick={toggleAll}
            >{allCollapsed ? t.home.expandAll : t.home.collapseAll}</button
          >
        {/if}
        <NewViewMenu {gadgets} {filters} busy={saving} onadd={add} />
      </div>
    {/if}
  </header>

  {#if error}
    <p class="error-text" role="alert">{error}</p>
  {:else if !dash || choices === null}
    <p class="muted loading">{t.common.loading}</p>
  {:else if gadgets.length === 0}
    <EmptyState message={t.home.emptyDashboard} hint={t.home.emptyDashboardHint} />
  {:else}
    <div class="views">
      {#each gadgets as g, i (keyOf(g, i))}
        {@const q = shownQuery(g)}
        {@const order = q === null ? [] : queryOrder(q)}
        <ViewSection
          key={keyOf(g, i)}
          title={title(g)}
          query={q}
          href={hrefOf(g)}
          jql={viewOf(g) ?? ''}
          size={g.config.size || 10}
          collapsed={!!g.config.collapsed}
          sharedWith={sharedWith(g)}
          sharedBy={sharedBy(g)}
          columns={columnsOf(g)}
          {names}
          empty={empty(g)}
          showAssignee={g.kind !== 'assigned'}
          sort={order}
          onsort={(column) => sortGadget(g, column, order)}
          ontoggle={() => toggle(i)}
          menu={() => menu(g, i)}
        />
      {/each}
    </div>
  {/if}
</div>

<Modal open={renaming !== null} title={t.home.renameTitle} onclose={() => (renaming = null)}>
  {#if renaming}
    <form
      id="view-rename"
      onsubmit={(e) => {
        e.preventDefault();
        void rename();
      }}
    >
      <label class="field">
        <span>{t.view.name}</span>
        <input class="input" bind:value={renaming.name} required />
      </label>
    </form>
  {/if}
  {#snippet footer()}
    <button type="button" class="btn btn-quiet" onclick={() => (renaming = null)}
      >{t.common.cancel}</button
    >
    <button type="submit" form="view-rename" class="btn btn-primary">{t.common.save}</button>
  {/snippet}
</Modal>

<ConfirmDialog
  open={deleting !== null}
  title={deleting ? t.home.deleteTitle(deleting.name) : ''}
  confirm={t.home.deleteView}
  onconfirm={() => (deleting ? deleteView(deleting) : Promise.resolve())}
  onclose={() => (deleting = null)}
>
  <p>{t.home.deleteBody}</p>
</ConfirmDialog>

<style>
  .space-head {
    display: flex;
    flex-wrap: wrap;
    align-items: flex-end;
    gap: 16px 24px;
    margin-bottom: 28px;
  }

  .heading {
    flex: 1 1 20rem;
    min-width: 0;
  }

  .page-title {
    margin: 0;
  }

  .intro {
    margin: 6px 0 0;
  }

  .head-actions {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px 16px;
  }

  .views {
    display: flex;
    flex-direction: column;
    gap: 20px;
  }

  .loading {
    padding: 10px 0;
  }
</style>
