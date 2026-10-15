<script lang="ts">
  import type { SavedFilter } from '../../core/types.js';
  import { t } from '../../i18n/fr.js';

  /**
   * The filters one can open: favourites, one's own, those shared with one.
   * A column beside the results; a list to unfold on narrow screens.
   */
  interface Props {
    filters: SavedFilter[] | null;
    current: number | null;
    onstar: (f: SavedFilter) => void;
  }
  const { filters, current, onstar }: Props = $props();

  let open = $state(false);
  const id = `saved-${Math.random().toString(36).slice(2, 8)}`;

  const groups = $derived.by(() => {
    const list = filters ?? [];
    return [
      { title: t.saved.favourites, items: list.filter((f) => f.favourite) },
      { title: t.saved.mine, items: list.filter((f) => !f.favourite && f.mine) },
      { title: t.saved.shared, items: list.filter((f) => !f.favourite && !f.mine) },
    ].filter((g) => g.items.length > 0);
  });
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
    <h2 class="saved-title" id="{id}-title">{t.saved.title}</h2>
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
                onclick={() => (open = false)}
              >
                <span class="name">{f.name}</span>
                {#if !f.mine && f.owner}<span class="owner">{t.saved.by(f.owner.name)}</span>{/if}
              </a>
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

  .saved-title {
    margin: 0 0 8px;
    font: var(--title-weight) 15px/1.2 var(--font-title);
  }

  .group-label {
    margin: 14px 0 4px;
    font: 500 11.5px/1 var(--font-body);
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
    width: 24px;
    height: 26px;
    border: none;
    background: none;
    padding: 0;
    color: var(--muted);
    font-size: 14px;
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
    min-width: 0;
    padding: 4px 6px 4px 0;
    text-decoration: none;
    font-size: 13.5px;
    line-height: 1.3;
  }

  .saved-item a:hover .name {
    text-decoration: underline;
  }

  .saved-item a[aria-current='page'] .name {
    font-weight: 600;
  }

  .owner {
    font-size: 11.5px;
    color: var(--muted);
  }

  .small {
    font-size: 13px;
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
