<script lang="ts">
  import { tick } from 'svelte';
  import { SvelteMap, SvelteSet } from 'svelte/reactivity';

  import { moveItem } from '../../core/catalogs.js';
  import {
    clickGadgetSort,
    copyOnEditPath,
    gadgetAtHash,
    gadgetQuery,
    MAX_GADGETS,
    onFilter,
    queryOrder,
    setCollapsed,
    sortedQuery,
    toInput,
    viewHash,
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
  import { listKeys, refocusLater } from '../../lib/actions.js';
  import { api, errorMessage } from '../../lib/api.js';
  import { router } from '../../lib/router.svelte.js';
  import { session } from '../../lib/session.svelte.js';
  import { notify } from '../../lib/ui.svelte.js';
  import type { MenuGroup } from '../board/CardMenu.svelte';
  import ConfirmDialog from '../ConfirmDialog.svelte';
  import NewViewMenu from '../dashboard/NewViewMenu.svelte';
  import ShareViewDialog from '../dashboard/ShareViewDialog.svelte';
  import ViewIndex from '../dashboard/ViewIndex.svelte';
  import ViewSection from '../dashboard/ViewSection.svelte';
  import EmptyState from '../EmptyState.svelte';
  import GettingStarted from '../GettingStarted.svelte';
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
  let sharing = $state<SavedFilter | null>(null);
  /** The section counts, by key, for the index. */
  const totals = new SvelteMap<string, number | null>();
  /** Folded views opened by a jump, by hash: for this visit only, the saved state stays. */
  const openNow = new SvelteSet<string>();
  /** The section a jump went to, kept aligned while the ones above it load. */
  let jumped: string | null = null;
  let viewsEl = $state<HTMLElement>();

  const gadgets = $derived(dash?.gadgets ?? []);
  const names = $derived(Object.fromEntries((choices ?? []).map((c) => [c.id, c.name])));
  const collapsedOf = (g: Gadget) => !!g.config.collapsed && !openNow.has(viewHash(g));
  const allCollapsed = $derived(gadgets.length > 0 && gadgets.every(collapsedOf));
  const ready = $derived(dash !== null && choices !== null && gadgets.length > 0);

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

  // The hash on arrival, then on Back and Forward, once the sections are drawn.
  $effect(() => {
    if (!ready) return;
    void tick().then(() => jumpToHash(location.hash));
    const onpop = () => jumpToHash(location.hash);
    window.addEventListener('popstate', onpop);
    return () => window.removeEventListener('popstate', onpop);
  });

  $effect(() => {
    const el = viewsEl;
    if (!el) return;
    const align = () => {
      if (jumped) document.getElementById(`view-section-${jumped}`)?.scrollIntoView();
    };
    const stop = () => (jumped = null);
    const grow = new ResizeObserver(align);
    grow.observe(el);
    const events = ['keydown', 'pointerdown', 'wheel', 'touchstart'] as const;
    for (const e of events) window.addEventListener(e, stop, { capture: true, passive: true });
    return () => {
      grow.disconnect();
      for (const e of events) window.removeEventListener(e, stop, { capture: true });
    };
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

  function jumpToHash(hash: string) {
    const i = gadgetAtHash(gadgets, hash);
    if (i >= 0) jump(i);
  }

  /** Scrolls to the section at i, opened if folded, and focuses its toggle. */
  function jump(i: number) {
    const g = gadgets[i];
    if (!g) return;
    const key = keyOf(g, i);
    if (g.config.collapsed) openNow.add(viewHash(g));
    void tick().then(() => {
      jumped = key;
      document.getElementById(`view-section-${key}`)?.scrollIntoView();
      document.getElementById(`view-toggle-${key}`)?.focus({ preventScroll: true });
    });
  }

  function jumpFromIndex(key: string) {
    const i = gadgets.findIndex((g, k) => keyOf(g, k) === key);
    const g = gadgets[i];
    if (!g) return;
    router.navigate(`${router.here}#${viewHash(g)}`);
    jump(i);
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
    if (openNow.delete(viewHash(g))) return;
    const keep = !dash?.customised;
    saveQuietly(
      setCollapsed(gadgets, (x) => x === g, !g.config.collapsed),
      keep ? () => focusToggle(i) : undefined,
    );
  }

  function toggleAll() {
    if (saving) return;
    const on = !allCollapsed;
    openNow.clear();
    saveQuietly(setCollapsed(gadgets, () => true, on));
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

  {#if session.me?.admin}<GettingStarted />{/if}

  {#if error}
    <p class="error-text" role="alert">{error}</p>
  {:else if !dash || choices === null}
    <p class="muted loading">{t.common.loading}</p>
  {:else if gadgets.length === 0}
    <EmptyState message={t.home.emptyDashboard} hint={t.home.emptyDashboardHint} />
  {:else}
    <div class="space-body">
      <ViewIndex
        entries={gadgets.map((g, i) => {
          const key = keyOf(g, i);
          return { key, href: `#${viewHash(g)}`, title: title(g), count: totals.get(key) ?? null };
        })}
        onjump={jumpFromIndex}
      />
      <!-- One list for every view: j and k walk on from one to the next. -->
      <div
        bind:this={viewsEl}
        class="views"
        use:listKeys={{
          rows: 'tbody tr',
          open: (row) => row.dataset.key && router.navigate(paths.issue(row.dataset.key)),
        }}
      >
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
            collapsed={collapsedOf(g)}
            sharedWith={sharedWith(g)}
            sharedBy={sharedBy(g)}
            columns={columnsOf(g)}
            {names}
            empty={empty(g)}
            showAssignee={g.kind !== 'assigned'}
            sort={order}
            onsort={(column) => sortGadget(g, column, order)}
            ontoggle={() => toggle(i)}
            onshare={filterOf(g)?.mine ? () => (sharing = filterOf(g) ?? null) : undefined}
            menu={() => menu(g, i)}
            ontotal={(n) => totals.set(keyOf(g, i), n)}
          />
        {/each}
      </div>
    </div>
  {/if}
</div>

<ShareViewDialog
  filter={sharing}
  onclose={() => (sharing = null)}
  onsaved={(f) => {
    sharing = f;
    filters = filters.map((x) => (x.id === f.id ? f : x));
  }}
/>

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

  .space-body {
    display: grid;
    grid-template-columns: 13.125rem minmax(0, 1fr);
    gap: 36px;
    align-items: start;
  }

  .views {
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 20px;
  }

  .loading {
    padding: 10px 0;
  }

  @media (max-width: 900px) {
    .space-body {
      /* The folded index's sticky bar, for the sections' scroll margin. */
      --view-index-height: calc(2.25rem + 16px);
      grid-template-columns: minmax(0, 1fr);
      gap: 12px;
    }
  }
</style>
