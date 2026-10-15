<script lang="ts">
  import { matches } from '../../core/admin-forms.js';
  import type { CatalogItem } from '../../core/admin-types.js';
  import {
    CATALOG_KINDS,
    moveItem,
    moveTargets,
    usageOf,
    type CatalogSection,
  } from '../../core/catalogs.js';
  import { formatNumber } from '../../i18n/format.js';
  import { t } from '../../i18n/t.js';
  import { adminApi } from '../../lib/admin-api.js';
  import { api, ApiError, errorMessage } from '../../lib/api.js';
  import { session } from '../../lib/session.svelte.js';
  import { notify } from '../../lib/ui.svelte.js';
  import ConfirmDialog from '../ConfirmDialog.svelte';
  import EmptyState from '../EmptyState.svelte';
  import PriorityMark from '../PriorityMark.svelte';
  import TypeMark from '../TypeMark.svelte';
  import CatalogDialog from './CatalogDialog.svelte';

  /**
   * One shared catalogue (issue types, priorities, resolutions or link
   * types): its items in the order every list of choices shows them, how
   * much each is used, and their creation, change and removal. A used item
   * goes only once its issues (or links) are moved to another.
   */
  interface Props {
    kind: CatalogSection;
  }
  const { kind }: Props = $props();

  const apiKind = $derived(CATALOG_KINDS[kind]);
  const m = $derived(t.catalogAdmin.kinds[kind]);

  let items = $state<CatalogItem[] | null>(null);
  let error = $state('');
  let unavailable = $state(false);
  let q = $state('');
  let dialog = $state<{ item: CatalogItem | null } | null>(null);
  let removing = $state<CatalogItem | null>(null);
  let moveTo = $state('');
  let ordering = $state(false);
  let dragged = $state<number | null>(null);
  let over = $state<number | null>(null);

  const list = $derived(
    (items ?? []).filter((i) => matches(q, i.name, i.description, i.outward, i.inward)),
  );
  const canOrder = $derived(q.trim() === '' && !ordering);
  const targets = $derived(items && removing ? moveTargets(kind, items, removing) : []);
  const removingUsage = $derived(removing ? usageOf(removing) : 0);

  async function load() {
    try {
      items = await adminApi.catalog(apiKind);
      error = '';
    } catch (e) {
      if (e instanceof ApiError && e.status === 404) unavailable = true;
      else error = errorMessage(e);
    }
  }

  $effect(() => {
    void load();
  });

  /** The rest of the interface reads the catalogues from the session. */
  async function refreshMeta() {
    try {
      session.meta = await api.meta();
    } catch {
      // The next page load brings them.
    }
  }

  async function reorder(next: CatalogItem[], focus?: string) {
    if (!items) return;
    const before = items;
    items = next;
    ordering = true;
    try {
      items = await adminApi.orderCatalog(
        apiKind,
        next.map((i) => i.id),
      );
      void refreshMeta();
    } catch (e) {
      items = before;
      notify(errorMessage(e), 'error');
    } finally {
      ordering = false;
      if (focus) queueMicrotask(() => document.getElementById(focus)?.focus());
    }
  }

  function step(item: CatalogItem, by: -1 | 1) {
    if (!items || !canOrder) return;
    const from = items.findIndex((i) => i.id === item.id);
    const to = from + by;
    if (to < 0 || to >= items.length) return;
    const atEdge = to === 0 || to === items.length - 1;
    // Keep the keyboard on the item: on the same arrow, or the other one at an end.
    const dir = atEdge ? (by < 0 ? 'down' : 'up') : by < 0 ? 'up' : 'down';
    void reorder(moveItem(items, from, to), `order-${dir}-${item.id}`);
  }

  function drop(target: number) {
    if (dragged !== null && items && canOrder && dragged !== target) {
      void reorder(moveItem(items, dragged, target));
    }
    dragged = null;
    over = null;
  }

  function saved(item: CatalogItem, created: boolean) {
    notify(created ? m.created(item.name) : t.catalogAdmin.saved(item.name));
    void load();
    void refreshMeta();
  }

  function askRemove(item: CatalogItem) {
    removing = item;
    moveTo = '';
  }

  async function remove() {
    const item = removing;
    if (!item) return;
    const used = usageOf(item) > 0;
    if (used && !moveTo) throw new Error(t.catalogAdmin.chooseTarget);
    const target = used ? targets.find((x) => String(x.id) === moveTo)?.id : undefined;
    await adminApi.deleteCatalogItem(apiKind, item.id, target);
    notify(t.catalogAdmin.deleted(item.name));
    await load();
    void refreshMeta();
  }

  const usageText = (i: CatalogItem) => {
    if (kind === 'linkTypes') return t.catalogAdmin.links(i.links ?? 0);
    const issues = t.catalogAdmin.issues(i.issues ?? 0);
    return kind === 'issueTypes' ? t.catalogAdmin.inProjects(issues, i.projects ?? 0) : issues;
  };
</script>

<header class="page-head">
  <div>
    <p class="eyebrow">{t.adminArea.groups.issues}</p>
    <h1 class="page-title">
      {t.adminArea.sections[kind]}
      {#if items}<span class="count muted">{formatNumber(items.length)}</span>{/if}
    </h1>
  </div>
  {#if !unavailable}
    <button class="btn btn-primary" onclick={() => (dialog = { item: null })}>{m.newItem}</button>
  {/if}
</header>
<p class="lead-text">{m.intro}</p>

{#if unavailable}
  <p class="empty">{t.adminArea.unavailable}</p>
{:else if error}
  <p class="error-text" role="alert">{error}</p>
{:else if !items}
  <p class="muted loading">{t.common.loading}</p>
{:else if items.length === 0}
  <EmptyState message={m.empty} hint={m.emptyHint}>
    <button class="btn btn-primary" onclick={() => (dialog = { item: null })}>{m.newItem}</button>
  </EmptyState>
{:else}
  <div class="list-tools">
    <input
      class="input search"
      type="search"
      bind:value={q}
      placeholder={t.catalogAdmin.search}
      aria-label={t.catalogAdmin.search}
    />
    <span class="muted small"
      >{canOrder || ordering ? t.catalogAdmin.orderHint : t.catalogAdmin.orderLocked}</span
    >
  </div>

  {#if list.length === 0}
    <EmptyState kind="search" message={t.adminArea.noMatch}>
      <button class="btn" onclick={() => (q = '')}>{t.adminArea.clearSearch}</button>
    </EmptyState>
  {:else}
    <div class="table-scroll">
      <table class="ledger catalog">
        <thead>
          <tr>
            <th class="c-order"><span class="sr-only">{t.catalogAdmin.order}</span></th>
            <th>{t.catalogAdmin.name}</th>
            {#if kind === 'linkTypes'}<th class="c-extra">{t.catalogAdmin.verbs}</th>{/if}
            <th class="num c-usage">{t.catalogAdmin.usage}</th>
            <th><span class="sr-only">{t.common.actions}</span></th>
          </tr>
        </thead>
        <tbody>
          {#each list as item, i (item.id)}
            <tr
              class:is-dragged={dragged === i}
              class:is-over={over === i && dragged !== null && dragged !== i}
              draggable={canOrder}
              ondragstart={(e) => {
                dragged = i;
                e.dataTransfer?.setData('text/plain', String(item.id));
              }}
              ondragover={(e) => {
                if (dragged === null) return;
                e.preventDefault();
                over = i;
              }}
              ondragleave={() => over === i && (over = null)}
              ondrop={(e) => {
                e.preventDefault();
                drop(i);
              }}
              ondragend={() => ((dragged = null), (over = null))}
            >
              <td class="c-order">
                <span class="grip" aria-hidden="true">⋮⋮</span>
                <button
                  type="button"
                  class="btn btn-quiet btn-small arrow"
                  id="order-up-{item.id}"
                  aria-label={t.catalogAdmin.moveUp(item.name)}
                  disabled={!canOrder || i === 0}
                  onclick={() => step(item, -1)}>↑</button
                >
                <button
                  type="button"
                  class="btn btn-quiet btn-small arrow"
                  id="order-down-{item.id}"
                  aria-label={t.catalogAdmin.moveDown(item.name)}
                  disabled={!canOrder || i === list.length - 1}
                  onclick={() => step(item, 1)}>↓</button
                >
              </td>
              <td>
                <span class="item-name">
                  {#if kind === 'issueTypes'}
                    <TypeMark
                      type={{ id: String(item.id), name: item.name, subtask: !!item.subtask }}
                      withName
                    />
                    {#if item.subtask}<span class="chip">{t.catalogAdmin.subtask}</span>{/if}
                  {:else if kind === 'priorities'}
                    <PriorityMark
                      priority={{
                        id: String(item.id),
                        name: item.name,
                        ...(item.color ? { color: item.color } : {}),
                      }}
                      withName
                    />
                    {#if item.color}<span class="muted mono">{item.color}</span>{/if}
                    {#if item.default}<span class="stamp done">{t.catalogAdmin.isDefault}</span
                      >{/if}
                  {:else}
                    <strong>{item.name}</strong>
                  {/if}
                </span>
                {#if item.description}<span class="muted small desc">{item.description}</span>{/if}
              </td>
              {#if kind === 'linkTypes'}
                <td class="c-extra">
                  <span class="verb">{item.outward}</span>
                  <span class="muted small">/</span>
                  <span class="verb">{item.inward}</span>
                </td>
              {/if}
              <td class="num c-usage" class:muted={usageOf(item) === 0}>
                {usageOf(item) === 0 && !(item.projects ?? 0)
                  ? t.catalogAdmin.unused
                  : usageText(item)}
              </td>
              <td class="row-actions">
                <button class="link-btn" onclick={() => (dialog = { item })}>{t.common.edit}</button
                >
                <button class="link-btn" onclick={() => askRemove(item)}>{t.common.delete}</button>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
{/if}

{#if items}
  <CatalogDialog
    section={kind}
    open={dialog !== null}
    item={dialog?.item ?? null}
    {items}
    onclose={() => (dialog = null)}
    onsaved={saved}
  />
{/if}

<ConfirmDialog
  open={removing !== null}
  title={m.deleteTitle(removing?.name ?? '')}
  confirm={removingUsage > 0 ? t.catalogAdmin.moveAndDelete : t.common.delete}
  onclose={() => (removing = null)}
  onconfirm={remove}
>
  {#if removing && removingUsage > 0}
    <p>{m.deleteUsed(removingUsage)}</p>
    {#if targets.length === 0}
      <p class="error-text">{m.noTarget}</p>
    {:else}
      <label class="field">
        <span>{m.moveLabel}</span>
        <select class="select" bind:value={moveTo}>
          <option value="" disabled>{t.catalogAdmin.choose}</option>
          {#each targets as x (x.id)}<option value={String(x.id)}>{x.name}</option>{/each}
        </select>
      </label>
    {/if}
  {:else}
    <p>{m.deleteUnused}</p>
  {/if}
</ConfirmDialog>

<style>
  .count {
    margin-left: 6px;
    font: 500 14px/1 var(--font-mono);
  }

  .c-order {
    width: 92px;
    white-space: nowrap;
  }

  .grip {
    display: inline-block;
    width: 14px;
    color: var(--muted);
    font-size: 11px;
    letter-spacing: -2px;
    cursor: grab;
  }

  tr:not([draggable='true']) .grip {
    visibility: hidden;
  }

  .arrow {
    width: 24px;
    padding: 0;
    justify-content: center;
  }

  .item-name {
    display: inline-flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 6px 10px;
  }

  .desc {
    display: block;
    margin-top: 2px;
    max-width: 60ch;
  }

  .verb {
    font-style: italic;
  }

  .c-usage {
    white-space: nowrap;
  }

  tr.is-dragged td {
    opacity: 0.45;
  }

  tr.is-over td {
    box-shadow: inset 0 2px 0 var(--accent);
  }

  @media (max-width: 600px) {
    .c-order {
      width: auto;
    }

    .grip {
      display: none;
    }

    .c-usage {
      display: none;
    }
  }
</style>
