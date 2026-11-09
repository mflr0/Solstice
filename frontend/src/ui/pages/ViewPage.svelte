<script lang="ts">
  import { readLocation, toJql, writeLocation } from '../../core/filters-url.js';
  import { paths } from '../../core/route.js';
  import { scopeSearch } from '../../core/scope.js';
  import { storeColumns } from '../../core/search-columns.js';
  import type { SavedFilter } from '../../core/types.js';
  import { t } from '../../i18n/t.js';
  import { api, errorMessage } from '../../lib/api.js';
  import { router } from '../../lib/router.svelte.js';
  import { session } from '../../lib/session.svelte.js';
  import { notify } from '../../lib/ui.svelte.js';
  import IssueSearch from '../search/IssueSearch.svelte';

  interface Props {
    /** The saved filter shown; null for an unsaved view. */
    viewId: number | null;
  }
  let { viewId }: Props = $props();

  /**
   * A view: a search kept in the address, with what is around it. A saved
   * one (/views/12) shows its changes (name, query, columns) as unsaved
   * until they are saved or discarded; /views/new is one not saved yet. A
   * filter the user can't edit takes their own columns.
   */
  let filter = $state<SavedFilter | null | undefined>(undefined);
  const loc = $derived({ ...readLocation(router.search).loc, filter: viewId });
  const current = $derived(filter ?? null);
  const gone = $derived(viewId !== null && filter === null);
  /** The query when the address has none: the filter's, or everything. */
  const fallback = $derived.by(() => {
    if (viewId === null) return '';
    if (filter === undefined) return null;
    return current ? toJql(current.query) : '';
  });
  const jql = $derived(loc.jql ?? fallback ?? '');
  let columnEdit = $state<string[] | null>(null);
  const columnsModified = $derived(
    !!current?.canEdit &&
      columnEdit !== null &&
      columnEdit.join() !== (current.columns?.join() ?? null),
  );
  let nameDraft = $state<string | null>(null);
  const name = $derived(nameDraft ?? current?.name ?? '');
  const nameModified = $derived(
    !!current?.canEdit && name.trim() !== '' && name.trim() !== current.name,
  );
  const queryModified = $derived(
    current !== null && loc.jql !== null && loc.jql.trim() !== toJql(current.query).trim(),
  );
  const modified = $derived(queryModified || columnsModified || nameModified);
  const title = $derived(current?.name ?? (viewId === null ? t.view.untitled : t.view.title));
  let shownColumns = $state<string[]>([]);
  let busy = $state(false);

  function setColumns(next: string[]) {
    if (!current) return;
    columnEdit = next;
    if (!current.canEdit) storeColumns(next);
  }

  // A new search starts in the scoped project, in the address like any other.
  $effect(() => {
    const scoped = scopeSearch(loc, session.scope);
    if (scoped !== null) {
      router.navigate(`${router.path}${writeLocation({ ...loc, filter: null, jql: scoped })}`, {
        replace: true,
      });
    }
  });

  $effect(() => {
    document.title = `${title} — ${t.appName}`;
  });

  async function load(id: number) {
    try {
      filter = (await api.filters()).find((f) => f.id === id) ?? null;
    } catch (e) {
      filter ??= null;
      notify(errorMessage(e), 'error');
    }
  }

  $effect(() => {
    if (viewId !== null) void load(viewId);
  });

  /** Back to the view as saved, in the same mode. */
  function discard() {
    nameDraft = null;
    columnEdit = null;
    if (viewId === null) return;
    router.navigate(
      `${paths.view(viewId)}${writeLocation({ jql: null, mode: loc.mode, filter: null, page: 1 })}`,
    );
  }

  async function run(work: () => Promise<void>) {
    if (busy) return;
    busy = true;
    try {
      await work();
    } catch (e) {
      notify(errorMessage(e), 'error');
    } finally {
      busy = false;
    }
  }

  const save = () =>
    run(async () => {
      if (!current?.canEdit) return;
      const id = current.id;
      const named = nameModified ? name.trim() : current.name;
      await api.updateFilter(id, {
        query: jql,
        ...(nameModified ? { name: named } : {}),
        ...(columnsModified ? { columns: columnEdit } : {}),
      });
      await load(id);
      nameDraft = null;
      columnEdit = null;
      notify(t.view.updated(named));
      router.navigate(
        `${paths.view(id)}${writeLocation({ jql: null, mode: loc.mode, filter: null, page: 1 })}`,
        { replace: true },
      );
    });

  const saveNew = () =>
    run(async () => {
      const f = await api.createFilter({
        name: name.trim() || t.view.untitled,
        query: jql,
        columns: shownColumns,
      });
      notify(t.view.created(f.name));
      router.navigate(paths.view(f.id));
    });

  function nameKey(e: KeyboardEvent) {
    if (e.key === 'Enter') {
      e.preventDefault();
      if (modified) void save();
    } else if (e.key === 'Escape' && nameDraft !== null) {
      e.preventDefault();
      nameDraft = null;
    }
  }
</script>

<div class="page view-page" data-title={title}>
  <div class="view-head">
    <a class="back" href={paths.home()}>
      <svg width="12" height="12" viewBox="0 0 12 12" aria-hidden="true"
        ><path d="M7.5 3L4.5 6l3 3" fill="none" stroke="currentColor" stroke-width="1.5" /></svg
      >
      {t.nav.work}
    </a>
    {#if gone}
      <h1 class="view-title">{t.view.title}</h1>
    {:else}
      <div class="title-block">
        {#if viewId === null || current?.canEdit}
          <h1 class="view-title">
            <label class="name-field">
              <span class="sr-only">{t.view.name}</span>
              <input
                class="name-input"
                value={name}
                placeholder={t.view.untitled}
                maxlength="255"
                oninput={(e) => (nameDraft = e.currentTarget.value)}
                onkeydown={nameKey}
              />
            </label>
          </h1>
        {:else}
          <h1 class="view-title">{current?.name ?? ''}</h1>
        {/if}
        {#if modified}<span class="unsaved">{t.view.unsaved}</span>{/if}
      </div>
      <div class="actions">
        {#if modified}
          <button type="button" class="link-btn discard" disabled={busy} onclick={discard}
            >{t.view.discard}</button
          >
        {/if}
        {#if viewId === null || current}
          <button type="button" class="btn" disabled={busy} onclick={() => void saveNew()}
            >{t.view.saveAsNew}</button
          >
        {/if}
        {#if current?.canEdit}
          <button
            type="button"
            class="btn btn-primary"
            disabled={busy || !modified}
            onclick={() => void save()}>{t.view.save}</button
          >
        {/if}
      </div>
    {/if}
  </div>
  {#if current?.description}<p class="description muted">{current.description}</p>{/if}

  {#if gone}
    <p class="empty-state" role="status">{t.view.gone}</p>
  {:else}
    <IssueSearch
      {fallback}
      autofocus={!router.arrived && viewId === null && loc.jql === null}
      selectable
      columns={current ? (columnEdit ?? current.columns) : null}
      oncolumns={current ? setColumns : undefined}
      columnsNote={current?.canEdit ? t.search.columns.withView : t.search.columns.inBrowser}
      onshown={(c) => (shownColumns = c)}
    />
  {/if}
</div>

<style>
  .view-head {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 12px 16px;
    margin-bottom: 16px;
  }

  .back {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    font-size: 0.8125rem;
    color: var(--ink-soft);
    text-decoration: none;
  }

  .back:hover {
    color: var(--accent-text);
  }

  .title-block {
    display: flex;
    align-items: baseline;
    gap: 10px;
    flex: 1 1 20rem;
    min-width: 0;
  }

  .view-title {
    flex: 1;
    min-width: 0;
    margin: 0;
    overflow-wrap: anywhere;
  }

  .name-field {
    display: flex;
  }

  .name-input {
    flex: 1;
    min-width: 0;
    padding: 2px 0;
    border: 0;
    border-bottom: 1px dashed var(--rule-strong);
    background: transparent;
    color: var(--ink);
    font: inherit;
  }

  .name-input::placeholder {
    color: var(--muted);
  }

  .unsaved {
    font-size: 0.75rem;
    color: var(--accent-text);
    white-space: nowrap;
  }

  .actions {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px;
  }

  .discard {
    padding: 0 12px;
  }

  .description {
    margin: -8px 0 16px;
    font-size: 0.84375rem;
  }

  .empty-state {
    margin: 32px 0;
    color: var(--ink-soft);
  }
</style>
