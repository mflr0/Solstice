<script lang="ts">
  import { matches } from '../../core/admin-forms.js';
  import type { StatusRow, StatusUsage } from '../../core/admin-types.js';
  import { arrowAfterMove, moveItem, statusUnused } from '../../core/catalogs.js';
  import { paths } from '../../core/route.js';
  import { formatNumber } from '../../i18n/format.js';
  import { t } from '../../i18n/t.js';
  import { adminApi } from '../../lib/admin-api.js';
  import { api, errorMessage } from '../../lib/api.js';
  import { session } from '../../lib/session.svelte.js';
  import { notify } from '../../lib/ui.svelte.js';
  import ConfirmDialog from '../ConfirmDialog.svelte';
  import EmptyState from '../EmptyState.svelte';
  import Modal from '../Modal.svelte';
  import OrderArrows from './OrderArrows.svelte';
  import StatusDialog from './StatusDialog.svelte';

  /**
   * The statuses every project shares: their category, how much each is
   * used, their order in lists, and their creation, change and removal. A
   * status in use is not deleted: the page says where it is used instead.
   */

  let items = $state<StatusRow[] | null>(null);
  let error = $state('');
  let q = $state('');
  let dialog = $state<{ item: StatusRow | null } | null>(null);
  let removing = $state<StatusRow | null>(null);
  let where = $state<StatusRow | null>(null);
  let usage = $state<StatusUsage | null>(null);
  let usageError = $state('');
  let ordering = $state(false);
  let dragged = $state<number | null>(null);
  let over = $state<number | null>(null);

  const list = $derived((items ?? []).filter((s) => matches(q, s.name, t.categories[s.category])));
  const canOrder = $derived(q.trim() === '' && !ordering);

  async function load() {
    try {
      items = await adminApi.statuses();
      error = '';
    } catch (e) {
      error = errorMessage(e);
    }
  }

  $effect(() => {
    void load();
  });

  /** The rest of the interface reads the statuses from the session. */
  async function refreshMeta() {
    try {
      session.meta = await api.meta();
    } catch {
      // The next page load brings them.
    }
  }

  async function reorder(next: StatusRow[], focus?: string) {
    if (!items) return;
    const before = items;
    items = next;
    ordering = true;
    try {
      await adminApi.orderCatalog(
        'statuses',
        next.map((s) => s.id),
      );
      items = await adminApi.statuses();
      void refreshMeta();
    } catch (e) {
      items = before;
      notify(errorMessage(e), 'error');
    } finally {
      ordering = false;
      if (focus) queueMicrotask(() => document.getElementById(focus)?.focus());
    }
  }

  function step(item: StatusRow, by: -1 | 1) {
    if (!items || !canOrder) return;
    const from = items.findIndex((s) => s.id === item.id);
    const to = from + by;
    if (to < 0 || to >= items.length) return;
    const dir = arrowAfterMove(by, to, items.length);
    void reorder(moveItem(items, from, to), `order-${dir}-${item.id}`);
  }

  function drop(target: number) {
    if (dragged !== null && items && canOrder && dragged !== target) {
      void reorder(moveItem(items, dragged, target));
    }
    dragged = null;
    over = null;
  }

  function saved(name: string, created: boolean) {
    notify(created ? t.statusesAdmin.created(name) : t.catalogAdmin.saved(name));
    void load();
    void refreshMeta();
  }

  async function remove() {
    const item = removing;
    if (!item) return;
    await adminApi.deleteStatus(item.id);
    notify(t.catalogAdmin.deleted(item.name));
    await load();
    void refreshMeta();
  }

  async function showWhere(item: StatusRow) {
    where = item;
    usage = null;
    usageError = '';
    try {
      usage = await adminApi.statusUsage(item.id);
    } catch (e) {
      usageError = errorMessage(e);
    }
  }

  const usageText = (s: StatusRow) =>
    [
      s.usage.issues > 0 ? t.catalogAdmin.issues(s.usage.issues) : '',
      s.usage.workflows > 0 ? t.statusesAdmin.workflows(s.usage.workflows) : '',
      s.usage.boardColumns > 0 ? t.statusesAdmin.columns(s.usage.boardColumns) : '',
    ]
      .filter(Boolean)
      .join(' · ');
</script>

<header class="page-head">
  <div>
    <p class="eyebrow">{t.adminArea.groups.issues}</p>
    <h1 class="page-title">
      {t.adminArea.sections.statuses}
      {#if items}<span class="count muted">{formatNumber(items.length)}</span>{/if}
    </h1>
  </div>
  <button class="btn btn-primary" onclick={() => (dialog = { item: null })}
    >{t.statusesAdmin.newItem}</button
  >
</header>
<p class="lead-text">{t.statusesAdmin.intro}</p>

{#if error}
  <p class="error-text" role="alert">{error}</p>
{:else if !items}
  <p class="muted loading">{t.common.loading}</p>
{:else if items.length === 0}
  <EmptyState message={t.statusesAdmin.empty} hint={t.statusesAdmin.emptyHint}>
    <button class="btn btn-primary" onclick={() => (dialog = { item: null })}
      >{t.statusesAdmin.newItem}</button
    >
  </EmptyState>
{:else}
  <div class="list-tools">
    <input
      class="input search"
      type="search"
      bind:value={q}
      placeholder={t.statusesAdmin.search}
      aria-label={t.statusesAdmin.search}
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
            <th class="c-category">{t.statusesAdmin.category}</th>
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
                e.dataTransfer?.setData('text/plain', item.id);
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
                <OrderArrows
                  id={item.id}
                  name={item.name}
                  first={i === 0}
                  last={i === list.length - 1}
                  disabled={!canOrder}
                  onmove={(by) => step(item, by)}
                />
              </td>
              <td><strong>{item.name}</strong></td>
              <td class="c-category">
                <span class="stamp {item.category}" data-category={item.category}
                  >{t.categories[item.category]}</span
                >
              </td>
              <td class="num c-usage" class:muted={statusUnused(item)}>
                {statusUnused(item) ? t.catalogAdmin.unused : usageText(item)}
              </td>
              <td class="row-actions">
                <button class="link-btn" onclick={() => (dialog = { item })}>{t.common.edit}</button
                >
                {#if statusUnused(item)}
                  <button class="link-btn" onclick={() => (removing = item)}
                    >{t.common.delete}</button
                  >
                {:else}
                  <button class="link-btn" onclick={() => void showWhere(item)}
                    >{t.statusesAdmin.whereUsed}</button
                  >
                {/if}
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
{/if}

{#if items}
  <StatusDialog
    open={dialog !== null}
    status={dialog?.item ?? null}
    statuses={items}
    onclose={() => (dialog = null)}
    onsaved={saved}
  />
{/if}

<ConfirmDialog
  open={removing !== null}
  title={t.statusesAdmin.deleteTitle(removing?.name ?? '')}
  confirm={t.common.delete}
  onclose={() => (removing = null)}
  onconfirm={remove}
>
  <p>{t.statusesAdmin.deleteUnused}</p>
</ConfirmDialog>

<Modal
  open={where !== null}
  title={t.statusesAdmin.whereTitle(where?.name ?? '')}
  onclose={() => (where = null)}
>
  <div class="where">
    <p>{t.statusesAdmin.whereIntro}</p>
    {#if usageError}
      <p class="error-text" role="alert">{usageError}</p>
    {:else if !usage}
      <p class="muted loading">{t.common.loading}</p>
    {:else}
      {#if usage.workflows.length > 0}
        <section>
          <h3 class="sub-title">{t.statusesAdmin.whereWorkflows}</h3>
          <ul>
            {#each usage.workflows as w (w.id)}
              <li>
                <a href="{paths.projectSettings(w.project.key)}?tab=workflows"
                  >{t.statusesAdmin.workflowIn(w.name, w.project.key)}</a
                >
              </li>
            {/each}
          </ul>
        </section>
      {/if}
      {#if usage.boards.length > 0}
        <section>
          <h3 class="sub-title">{t.statusesAdmin.whereBoards}</h3>
          <ul>
            {#each usage.boards as b (`${b.id}/${b.column}`)}
              <li>
                <a href={paths.boardSettings(b.id)}
                  >{t.statusesAdmin.boardColumn(b.name, b.column)}</a
                >
              </li>
            {/each}
          </ul>
        </section>
      {/if}
      {#if usage.projects.length > 0}
        <section>
          <h3 class="sub-title">{t.statusesAdmin.whereIssues}</h3>
          <ul>
            {#each usage.projects as p (p.key)}
              <li>
                <a href={paths.project(p.key)}>{p.key}</a>
                <span class="muted">{p.name}</span>
                {t.catalogAdmin.issues(p.issues)}
              </li>
            {/each}
          </ul>
          <p class="muted small">{t.statusesAdmin.whereIssuesHint}</p>
        </section>
      {/if}
    {/if}
  </div>
  {#snippet footer()}
    <button type="button" class="btn" onclick={() => (where = null)}>{t.common.close}</button>
  {/snippet}
</Modal>

<style>
  .count {
    margin-left: 6px;
    font: 500 0.875rem/1 var(--font-mono);
  }

  .c-order {
    width: 5.75rem;
    white-space: nowrap;
  }

  .grip {
    display: inline-block;
    width: 14px;
    color: var(--muted);
    font-size: 0.6875rem;
    letter-spacing: -2px;
    cursor: grab;
  }

  tr:not([draggable='true']) .grip {
    visibility: hidden;
  }

  .c-usage {
    white-space: nowrap;
  }

  .where {
    display: grid;
    gap: 14px;
  }

  .where p,
  .where ul {
    margin: 0;
  }

  .where ul {
    padding-left: 18px;
    display: grid;
    gap: 4px;
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
