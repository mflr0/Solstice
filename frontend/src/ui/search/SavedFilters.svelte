<script lang="ts">
  import type { SavedFilter } from '../../core/types.js';
  import { t } from '../../i18n/t.js';

  /**
   * The filters one can open: favourites, one's own, those shared with one.
   * A column beside the results; a list to unfold on narrow screens.
   */
  interface Props {
    filters: SavedFilter[] | null;
    current: number | null;
    onstar: (f: SavedFilter) => void;
    /** Leaves the current filter for the plain search. */
    onclose: () => void;
    /** Stars (or unstars) the given filters, from manage mode. */
    onstarmany: (fs: SavedFilter[], on: boolean) => Promise<void>;
    /** Asks to delete the given filters, from manage mode. */
    ondeletemany: (fs: SavedFilter[]) => void;
  }
  const { filters, current, onstar, onclose, onstarmany, ondeletemany }: Props = $props();

  function close(e: MouseEvent) {
    // Other clicks (new tab…) still follow the link.
    if (e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
    e.preventDefault();
    open = false;
    onclose();
  }

  let open = $state(false);
  let managing = $state(false);
  let picked = $state<number[]>([]);
  let busy = $state(false);
  const uid = $props.id();
  const id = `saved-${uid}`;

  const groups = $derived.by(() => {
    const list = filters ?? [];
    return [
      { title: t.saved.favourites, items: list.filter((f) => f.favourite) },
      { title: t.saved.mine, items: list.filter((f) => !f.favourite && f.mine) },
      { title: t.saved.shared, items: list.filter((f) => !f.favourite && !f.mine) },
    ].filter((g) => g.items.length > 0);
  });

  // Filters gone from the list (deleted, unshared) drop out of the selection.
  const selection = $derived((filters ?? []).filter((f) => picked.includes(f.id)));
  const allPicked = $derived(
    filters !== null && filters.length > 0 && selection.length === filters.length,
  );
  const somePicked = $derived(selection.length > 0 && !allPicked);
  const toStar = $derived(selection.filter((f) => !f.favourite));
  const toUnstar = $derived(selection.filter((f) => f.favourite));
  const deletable = $derived(selection.filter((f) => f.canEdit));

  function pick(id: number, on: boolean) {
    picked = on ? [...picked, id] : picked.filter((p) => p !== id);
  }

  function pickAll(on: boolean) {
    picked = on ? (filters ?? []).map((f) => f.id) : [];
  }

  function done() {
    managing = false;
    picked = [];
  }

  async function starMany(fs: SavedFilter[], on: boolean) {
    busy = true;
    try {
      await onstarmany(fs, on);
    } finally {
      busy = false;
    }
  }
</script>

<aside class="saved-filters" aria-labelledby="{id}-title">
  <button
    type="button"
    class="btn btn-small saved-toggle"
    aria-expanded={open}
    aria-controls="{id}-body"
    onclick={() => (open = !open)}
  >
    {t.saved.toggle(filters?.length ?? 0)}
    <span class="caret" aria-hidden="true">{open ? '▴' : '▾'}</span>
  </button>
  <div class="saved-body" class:open id="{id}-body">
    <div class="saved-head">
      <h2 class="saved-title" id="{id}-title">{t.saved.title}</h2>
      {#if !managing && filters?.length}
        <button
          type="button"
          class="btn btn-small btn-quiet manage"
          onclick={() => (managing = true)}>{t.saved.manage}</button
        >
      {/if}
    </div>
    {#if managing && filters?.length}
      <div class="manage-bar" role="group" aria-label={t.saved.actions}>
        <label class="pick-all">
          <input
            type="checkbox"
            checked={allPicked}
            indeterminate={somePicked}
            onchange={(e) => pickAll(e.currentTarget.checked)}
          />
          {t.saved.selectAll}
        </label>
        <div class="manage-actions">
          <button
            type="button"
            class="btn btn-small"
            disabled={busy || toStar.length === 0}
            onclick={() => void starMany(toStar, true)}
            >{t.saved.favouriteMany(toStar.length)}</button
          >
          <button
            type="button"
            class="btn btn-small"
            disabled={busy || toUnstar.length === 0}
            onclick={() => void starMany(toUnstar, false)}
            >{t.saved.unfavouriteMany(toUnstar.length)}</button
          >
          <button
            type="button"
            class="btn btn-small btn-danger"
            disabled={busy || deletable.length === 0}
            onclick={() => ondeletemany(selection)}>{t.saved.deleteMany(deletable.length)}</button
          >
          <button type="button" class="btn btn-small btn-quiet" onclick={done}
            >{t.saved.manageDone}</button
          >
        </div>
      </div>
    {/if}
    {#if filters === null}
      <p class="muted small loading">{t.common.loading}</p>
    {:else if groups.length === 0}
      <p class="muted small">{t.saved.none}</p>
    {:else}
      {#each groups as g (g.title)}
        <h3 class="group-label">{g.title}</h3>
        <ul>
          {#each g.items as f (f.id)}
            <li class="saved-item" class:is-current={f.id === current}>
              {#if managing}
                <label class="pick">
                  <input
                    type="checkbox"
                    aria-label={t.saved.select(f.name)}
                    checked={picked.includes(f.id)}
                    onchange={(e) => pick(f.id, e.currentTarget.checked)}
                  />
                  <span class="label">
                    <span class="name">{f.name}</span>
                    {#if !f.mine && f.owner}<span class="owner">{t.saved.by(f.owner.name)}</span
                      >{/if}
                  </span>
                </label>
              {:else}
                <button
                  type="button"
                  class="star"
                  aria-pressed={f.favourite}
                  aria-label={f.favourite ? t.saved.unstar(f.name) : t.saved.star(f.name)}
                  title={f.favourite ? t.saved.unstar(f.name) : t.saved.star(f.name)}
                  onclick={() => onstar(f)}
                  ><span aria-hidden="true">{f.favourite ? '★' : '☆'}</span></button
                >
                <a
                  href="/issues?filter={f.id}"
                  aria-current={f.id === current ? 'page' : undefined}
                  title={f.description ?? undefined}
                  onclick={f.id === current ? close : () => (open = false)}
                >
                  <span class="name">{f.name}</span>
                  {#if !f.mine && f.owner}<span class="owner">{t.saved.by(f.owner.name)}</span>{/if}
                </a>
                {#if f.id === current}
                  <button
                    type="button"
                    class="close"
                    aria-label={t.saved.close(f.name)}
                    title={t.saved.close(f.name)}
                    onclick={close}><span aria-hidden="true">×</span></button
                  >
                {/if}
              {/if}
            </li>
          {/each}
        </ul>
      {/each}
    {/if}
  </div>
</aside>

<style>
  .saved-toggle {
    display: none;
  }

  .saved-head {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: 8px;
    margin: 0 0 8px;
  }

  .saved-title {
    margin: 0;
    font: var(--title-weight) 0.9375rem/1.2 var(--font-title);
  }

  .manage-bar {
    display: grid;
    gap: 6px;
    margin-bottom: 4px;
  }

  .pick-all {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 0.8125rem;
    cursor: pointer;
  }

  .manage-actions {
    display: flex;
    flex-wrap: wrap;
    gap: 4px;
  }

  .pick {
    display: flex;
    flex: 1;
    min-width: 0;
    align-items: flex-start;
    gap: 6px;
    padding: 4px 6px 4px 2px;
    font-size: 0.84375rem;
    line-height: 1.3;
    cursor: pointer;
  }

  .pick input {
    flex: none;
    margin: 2px 0 0;
  }

  .pick .label {
    display: grid;
    min-width: 0;
  }

  .group-label {
    margin: 14px 0 4px;
    font: 500 0.71875rem/1 var(--font-body);
    color: var(--muted);
  }

  ul {
    list-style: none;
    margin: 0;
    padding: 0;
  }

  .saved-item {
    display: flex;
    align-items: flex-start;
    gap: 4px;
    border-radius: var(--radius-s);
  }

  .saved-item.is-current {
    background-color: var(--paper-sunk);
  }

  .star {
    flex: none;
    width: 1.5rem;
    height: 1.625rem;
    border: none;
    background: none;
    padding: 0;
    color: var(--muted);
    font-size: 0.875rem;
    cursor: pointer;
  }

  .star[aria-pressed='true'] {
    color: var(--accent-text);
  }

  .star:hover {
    color: var(--ink);
  }

  .saved-item a {
    display: grid;
    flex: 1;
    min-width: 0;
    padding: 4px 6px 4px 0;
    text-decoration: none;
    font-size: 0.84375rem;
    line-height: 1.3;
  }

  /* The × takes the padding's place, so the name keeps its room. */
  .saved-item.is-current a {
    padding-right: 0;
  }

  .saved-item a:hover .name {
    text-decoration: underline;
  }

  .saved-item a[aria-current='page'] .name {
    font-weight: 600;
  }

  .close {
    flex: none;
    width: 1.25rem;
    height: 1.625rem;
    border: none;
    background: none;
    padding: 0;
    color: var(--muted);
    font-size: 1rem;
    line-height: 1;
    cursor: pointer;
  }

  .close:hover {
    color: var(--ink);
  }

  .owner {
    font-size: 0.71875rem;
    color: var(--muted);
  }

  .small {
    font-size: 0.8125rem;
  }

  /* Narrow screens: a button unfolds the list above the search. */
  @media (max-width: 1000px) {
    .saved-toggle {
      display: inline-flex;
    }

    .saved-body {
      display: none;
      margin-top: 8px;
      padding: 10px 12px;
      border: var(--frame) solid var(--rule);
      border-radius: var(--radius-m);
      background-color: var(--paper-raised);
    }

    .saved-body.open {
      display: block;
    }

    .saved-title {
      display: none;
    }

    .group-label:first-of-type {
      margin-top: 0;
    }
  }
</style>
