<script lang="ts">
  import { readLocation, toJql, writeLocation } from '../../core/filters-url.js';
  import { scopeSearch } from '../../core/scope.js';
  import type { SavedFilter } from '../../core/types.js';
  import { t } from '../../i18n/t.js';
  import { api, errorMessage } from '../../lib/api.js';
  import { router } from '../../lib/router.svelte.js';
  import { session } from '../../lib/session.svelte.js';
  import { notify } from '../../lib/ui.svelte.js';
  import Modal from '../Modal.svelte';
  import IssueSearch from '../search/IssueSearch.svelte';
  import SavedFilters from '../search/SavedFilters.svelte';

  /**
   * Search across every project, with the saved filters beside it. The
   * search lives in the address (?jql=…); ?filter=12 opens a saved filter,
   * and a change made to it shows as "modified" until it is saved.
   */
  let filters = $state<SavedFilter[] | null>(null);
  const loc = $derived(readLocation(router.search).loc);
  const current = $derived(
    loc.filter === null ? null : (filters?.find((f) => f.id === loc.filter) ?? null),
  );
  const gone = $derived(loc.filter !== null && filters !== null && current === null);
  /** The query when the address has none: the filter's, or everything. */
  const fallback = $derived.by(() => {
    if (loc.filter === null) return '';
    if (filters === null) return null;
    return current ? toJql(current.query) : '';
  });
  const jql = $derived(loc.jql ?? fallback ?? '');
  const modified = $derived(
    current !== null && loc.jql !== null && loc.jql.trim() !== toJql(current.query).trim(),
  );

  // A new search starts in the scoped project, in the address like any other.
  $effect(() => {
    const scoped = scopeSearch(loc, session.scope);
    if (scoped !== null) {
      router.navigate(`${router.path}${writeLocation({ ...loc, jql: scoped })}`, { replace: true });
    }
  });

  let dialog = $state<'save' | 'rename' | 'delete' | 'deleteMany' | null>(null);
  let name = $state('');
  let description = $state('');
  let busy = $state(false);
  let dialogError = $state('');
  /** The filters picked for deletion in manage mode, and how many of them aren't the user's. */
  let doomed = $state<SavedFilter[]>([]);
  let skipped = $state(0);

  $effect(() => {
    document.title = `${current?.name ?? t.search.title} — ${t.appName}`;
  });

  async function reload() {
    try {
      filters = await api.filters();
    } catch (e) {
      filters ??= [];
      notify(errorMessage(e), 'error');
    }
  }

  $effect(() => {
    void reload();
  });

  /** Opens the saved filter as saved (its own query). */
  function openFilter(id: number) {
    router.navigate(
      `${router.path}${writeLocation({ jql: null, mode: loc.mode, filter: id, page: 1 })}`,
    );
  }

  /** Back to the plain search, in the same mode; Back reopens the filter. */
  function closeFilter() {
    router.navigate(
      `${router.path}${writeLocation({ jql: null, mode: loc.mode, filter: null, page: 1 })}`,
    );
  }

  function ask(which: 'save' | 'rename' | 'delete') {
    dialogError = '';
    name = which === 'rename' && current ? current.name : '';
    description = '';
    dialog = which;
  }

  async function act(run: () => Promise<void>) {
    busy = true;
    dialogError = '';
    try {
      await run();
      dialog = null;
    } catch (e) {
      dialogError = errorMessage(e);
    } finally {
      busy = false;
    }
  }

  const saveNew = () =>
    act(async () => {
      const f = await api.createFilter({
        name: name.trim(),
        query: jql,
        ...(description.trim() ? { description: description.trim() } : {}),
      });
      await reload();
      notify(t.saved.created(f.name));
      openFilter(f.id);
    });

  const rename = () =>
    act(async () => {
      if (!current) return;
      await api.updateFilter(current.id, { name: name.trim() });
      await reload();
      notify(t.saved.updated(name.trim()));
    });

  const remove = () =>
    act(async () => {
      if (!current) return;
      const was = current;
      await api.deleteFilter(was.id);
      // Stay on the same search, no longer tied to the filter.
      router.navigate(
        `${router.path}${writeLocation({ jql, mode: loc.mode, filter: null, page: 1 })}`,
        { replace: true },
      );
      await reload();
      notify(t.saved.deleted(was.name));
    });

  async function saveChanges() {
    if (!current) return;
    busy = true;
    try {
      await api.updateFilter(current.id, { query: jql });
      await reload();
      notify(t.saved.updated(current.name));
      openFilter(current.id);
    } catch (e) {
      notify(errorMessage(e), 'error');
    } finally {
      busy = false;
    }
  }

  /** Runs one call per filter; the others still apply when some fail. */
  async function each(fs: SavedFilter[], run: (f: SavedFilter) => Promise<unknown>) {
    const results = await Promise.allSettled(fs.map(run));
    await reload();
    const failed = results.filter((r) => r.status === 'rejected').length;
    if (failed > 0) notify(t.saved.failedMany(failed), 'error');
    return results;
  }

  async function starMany(fs: SavedFilter[], on: boolean) {
    await each(fs, (f) => api.favouriteFilter(f.id, on));
  }

  function askDeleteMany(fs: SavedFilter[]) {
    doomed = fs.filter((f) => f.canEdit);
    skipped = fs.length - doomed.length;
    dialogError = '';
    dialog = 'deleteMany';
  }

  async function removeMany() {
    busy = true;
    try {
      const fs = doomed;
      // The reload below drops the deleted filter, and its query with it.
      const kept = jql;
      const results = await each(fs, (f) => api.deleteFilter(f.id));
      const gone = fs.filter((_, i) => results[i]!.status === 'fulfilled');
      if (gone.some((f) => f.id === loc.filter)) {
        router.navigate(
          `${router.path}${writeLocation({ jql: kept, mode: loc.mode, filter: null, page: 1 })}`,
          { replace: true },
        );
      }
      if (gone.length > 0) notify(t.saved.deletedMany(gone.length));
      dialog = null;
    } finally {
      busy = false;
    }
  }

  async function star(f: SavedFilter) {
    try {
      await api.favouriteFilter(f.id, !f.favourite);
      await reload();
    } catch (e) {
      notify(errorMessage(e), 'error');
    }
  }
</script>

<div class="page search-page" data-title={current?.name ?? t.search.title}>
  <header class="page-head">
    <div class="title-block">
      {#if current}<p class="eyebrow">{t.saved.current}</p>{/if}
      <h1 class="page-title">
        {current?.name ?? t.search.title}
        {#if modified}<span class="modified">{t.saved.modified}</span>{/if}
      </h1>
      {#if current?.description}<p class="description muted">{current.description}</p>{/if}
    </div>
    <div class="row head-actions">
      {#if current?.canEdit}
        {#if modified}
          <button class="btn btn-primary" disabled={busy} onclick={saveChanges}
            >{t.saved.save}</button
          >
        {/if}
        <button class="btn btn-quiet" onclick={() => ask('rename')}>{t.saved.rename}</button>
        <button class="btn btn-quiet btn-danger" onclick={() => ask('delete')}
          >{t.saved.delete}</button
        >
      {/if}
      <button class="btn" onclick={() => ask('save')}
        >{current ? t.saved.saveAs : t.saved.saveSearch}</button
      >
    </div>
  </header>

  <div class="search-layout">
    <SavedFilters
      {filters}
      current={loc.filter}
      onstar={star}
      onclose={closeFilter}
      onstarmany={starMany}
      ondeletemany={askDeleteMany}
    />
    <div class="search-main">
      {#if gone}<p class="error-text gone" role="alert">{t.saved.gone}</p>{/if}
      <IssueSearch {fallback} autofocus={loc.filter === null && loc.jql === null} selectable />
    </div>
  </div>
</div>

<Modal
  open={dialog === 'save' || dialog === 'rename'}
  title={dialog === 'rename' ? t.saved.renameTitle : t.saved.newTitle}
  onclose={() => (dialog = null)}
>
  <form
    id="filter-form"
    class="filter-form"
    onsubmit={(e) => {
      e.preventDefault();
      if (!name.trim() || busy) return;
      void (dialog === 'rename' ? rename() : saveNew());
    }}
  >
    <label class="field">
      <span>{t.saved.name}</span>
      <!-- svelte-ignore a11y_autofocus -->
      <input class="input" bind:value={name} maxlength="255" required autofocus />
    </label>
    {#if dialog === 'save'}
      <label class="field">
        <span>{t.saved.description}</span>
        <input class="input" bind:value={description} />
      </label>
      <code class="query">{jql || '—'}</code>
    {/if}
    {#if dialogError}<p class="error-text" role="alert">{dialogError}</p>{/if}
  </form>
  {#snippet footer()}
    <button class="btn" type="button" onclick={() => (dialog = null)}>{t.common.cancel}</button>
    <button class="btn btn-primary" type="submit" form="filter-form" disabled={busy || !name.trim()}
      >{t.saved.save}</button
    >
  {/snippet}
</Modal>

<Modal open={dialog === 'delete'} title={t.saved.deleteTitle} onclose={() => (dialog = null)}>
  <p class="confirm">{current ? t.saved.confirmDelete(current.name) : ''}</p>
  {#if dialogError}<p class="error-text" role="alert">{dialogError}</p>{/if}
  {#snippet footer()}
    <button class="btn" type="button" onclick={() => (dialog = null)}>{t.common.cancel}</button>
    <button class="btn btn-primary" type="button" disabled={busy} onclick={() => void remove()}
      >{t.saved.delete}</button
    >
  {/snippet}
</Modal>

<Modal
  open={dialog === 'deleteMany'}
  title={t.saved.deleteManyTitle(doomed.length)}
  onclose={() => (dialog = null)}
>
  <p class="confirm">{t.saved.confirmDeleteMany(doomed.length, skipped)}</p>
  {#snippet footer()}
    <button class="btn" type="button" onclick={() => (dialog = null)}>{t.common.cancel}</button>
    <button class="btn btn-primary" type="button" disabled={busy} onclick={() => void removeMany()}
      >{t.saved.deleteMany(doomed.length)}</button
    >
  {/snippet}
</Modal>

<style>
  .title-block {
    min-width: 0;
  }

  .modified {
    margin-left: 8px;
    font: italic 400 0.875rem/1 var(--font-body);
    color: var(--accent-text);
    vertical-align: 0.25em;
  }

  .description {
    margin: 6px 0 0;
    font-size: 0.84375rem;
  }

  .head-actions {
    justify-content: flex-end;
  }

  .search-layout {
    display: grid;
    grid-template-columns: 13.125rem minmax(0, 1fr);
    gap: 28px;
    align-items: start;
  }

  .search-layout > :global(.saved-filters) {
    position: sticky;
    top: calc(3.375rem + 20px);
  }

  .gone {
    margin: 0 0 12px;
  }

  .filter-form {
    display: grid;
    gap: 14px;
  }

  .query {
    display: block;
    padding: 8px 10px;
    border: 1px solid var(--rule);
    border-radius: var(--radius-xs);
    background: var(--paper-sunk);
    font: 0.78125rem/1.5 var(--font-mono);
    overflow-wrap: anywhere;
  }

  .confirm {
    margin: 0;
  }

  @media (max-width: 1000px) {
    .search-layout {
      grid-template-columns: minmax(0, 1fr);
      gap: 12px;
    }

    .search-layout > :global(.saved-filters) {
      position: static;
    }
  }
</style>
