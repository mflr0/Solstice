<script lang="ts">
  import { arrowAfterMove, moveItem } from '../../core/catalogs.js';
  import {
    gadgetQuery,
    MAX_GADGETS,
    toInput,
    type Dashboard,
    type Gadget,
  } from '../../core/dashboard.js';
  import { localParts } from '../../core/dates.js';
  import { paths } from '../../core/route.js';
  import { scopeGadgetQuery } from '../../core/scope.js';
  import { NATIVE_COLUMNS, type ColumnChoice } from '../../core/search-columns.js';
  import { renderDay, t } from '../../i18n/t.js';
  import { api, errorMessage } from '../../lib/api.js';
  import { session } from '../../lib/session.svelte.js';
  import { notify } from '../../lib/ui.svelte.js';
  import OrderArrows from '../admin/OrderArrows.svelte';
  import CountGadget from '../dashboard/CountGadget.svelte';
  import GadgetDialog from '../dashboard/GadgetDialog.svelte';
  import EmptyState from '../EmptyState.svelte';
  import IssueQuery from '../IssueQuery.svelte';

  /**
   * The user's dashboard: gadgets on saved filters (their results, or their
   * issues counted by a field) and built-in lists of their own work, which
   * they add, configure, order and remove under "Customise".
   */

  let dash = $state<Dashboard | null>(null);
  let error = $state('');
  let editing = $state(false);
  let dialog = $state<{ gadget: Gadget | null } | null>(null);
  let saving = $state(false);
  let dragged = $state<number | null>(null);
  let over = $state<number | null>(null);
  let choices = $state<ColumnChoice[] | null>(null);
  let totals = $state<Record<string, number>>({});
  const today = renderDay(localParts(new Date().toISOString()));

  const gadgets = $derived(dash?.gadgets ?? []);
  const names = $derived(Object.fromEntries((choices ?? []).map((c) => [c.id, c.name])));

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
  });

  /** A key per gadget, the defaults having no id yet. */
  const keyOf = (g: Gadget, i: number) => (g.id ? `g${g.id}` : `d${i}`);

  function title(g: Gadget): string {
    switch (g.kind) {
      case 'results':
        return g.filter?.name ?? t.home.unavailableTitle;
      case 'count': {
        const field = g.config.field ? t.search.fields[g.config.field] : '';
        return g.filter ? t.home.countBy(g.filter.name, field) : t.home.unavailableTitle;
      }
      default:
        return t.home.kinds[g.kind];
    }
  }

  function empty(g: Gadget): string {
    switch (g.kind) {
      case 'assigned':
        return t.home.emptyAssigned;
      case 'reported':
        return t.home.emptyReported;
      case 'recent':
        return t.home.emptyRecent;
      default:
        return t.home.emptyFilter;
    }
  }

  /** The chosen columns the search still knows (a custom field may be gone). */
  function columnsOf(g: Gadget): string[] | undefined {
    const cols = g.config.columns;
    if (!cols?.length) return undefined;
    const known = choices ?? [];
    return known.length
      ? cols.filter((c) => known.some((x) => x.id === c))
      : cols.filter((c) => NATIVE_COLUMNS.has(c));
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

  function saveQuietly(next: Gadget[], focus?: string) {
    save(next).then(
      () => {
        if (focus) queueMicrotask(() => document.getElementById(focus)?.focus());
      },
      (e: unknown) => notify(errorMessage(e), 'error'),
    );
  }

  function step(i: number, by: -1 | 1) {
    const to = i + by;
    if (saving || to < 0 || to >= gadgets.length) return;
    const dir = arrowAfterMove(by, to, gadgets.length);
    const moved = moveItem(gadgets, i, to);
    // Saved, a default gadget gets an id: the arrows' ids follow it.
    saveQuietly(moved, dash?.customised ? `order-${dir}-${keyOf(gadgets[i]!, to)}` : undefined);
  }

  function drop(target: number) {
    if (dragged !== null && !saving && dragged !== target) {
      saveQuietly(moveItem(gadgets, dragged, target));
    }
    dragged = null;
    over = null;
  }

  function saved(g: Gadget): Promise<void> {
    const edited = dialog?.gadget;
    const next = edited ? gadgets.map((x) => (x === edited ? g : x)) : [...gadgets, g];
    return save(next);
  }
</script>

<div class="page" data-title={t.home.title}>
  <header class="page-head">
    <div>
      <p class="eyebrow">{today}</p>
      <h1 class="page-title">
        {session.me ? t.home.greeting(session.me.name.split(' ')[0] ?? '') : t.home.title}
      </h1>
    </div>
    {#if dash}
      <div class="head-actions">
        {#if editing && gadgets.length > 0}
          <button
            class="btn"
            disabled={saving || gadgets.length >= MAX_GADGETS}
            title={gadgets.length >= MAX_GADGETS ? t.home.full : undefined}
            onclick={() => (dialog = { gadget: null })}>{t.home.addGadget}</button
          >
        {/if}
        <button
          class="btn"
          class:btn-primary={editing}
          aria-pressed={editing}
          onclick={() => (editing = !editing)}>{editing ? t.home.done : t.home.customise}</button
        >
      </div>
    {/if}
  </header>

  {#if error}
    <p class="error-text" role="alert">{error}</p>
  {:else if !dash}
    <p class="muted loading">{t.common.loading}</p>
  {:else if gadgets.length === 0}
    <EmptyState message={t.home.emptyDashboard} hint={t.home.emptyDashboardHint}>
      <button class="btn btn-primary" onclick={() => (dialog = { gadget: null })}
        >{t.home.addGadget}</button
      >
    </EmptyState>
  {:else}
    {#if editing}<p class="muted small order-hint">{t.home.orderHint}</p>{/if}
    {#each gadgets as g, i (keyOf(g, i))}
      {@const q = scopeGadgetQuery(gadgetQuery(g), g.kind, session.scope)}
      <section
        class="gadget"
        class:editing
        class:is-dragged={dragged === i}
        class:is-over={over === i && dragged !== null && dragged !== i}
        draggable={editing && !saving}
        aria-label={title(g)}
        ondragstart={(e) => {
          dragged = i;
          e.dataTransfer?.setData('text/plain', keyOf(g, i));
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
        <h2 class="section-title">
          {#if editing}<span class="grip" aria-hidden="true">⋮⋮</span>{/if}
          <span class="name">{title(g)}</span>
          {#if g.kind !== 'count' && q !== null}
            <span class="count">{totals[keyOf(g, i)] ?? 0}</span>
          {/if}
          <span class="actions">
            {#if editing}
              <OrderArrows
                id={keyOf(g, i)}
                name={title(g)}
                first={i === 0}
                last={i === gadgets.length - 1}
                disabled={saving}
                onmove={(by) => step(i, by)}
              />
              <button class="link-btn" disabled={saving} onclick={() => (dialog = { gadget: g })}
                >{t.home.configure}</button
              >
              <button
                class="link-btn"
                disabled={saving}
                onclick={() => saveQuietly(gadgets.filter((x) => x !== g))}
                >{t.common.remove}</button
              >
            {:else if q !== null}
              <a class="link-btn" href={paths.search(q)}>{t.common.seeAll}</a>
            {/if}
          </span>
        </h2>
        {#if q === null}
          <p class="unavailable">{t.home.unavailable}</p>
        {:else if g.kind === 'count' && g.config.field}
          <CountGadget query={q} field={g.config.field} />
        {:else if choices !== null}
          <IssueQuery
            {q}
            size={g.config.size || 10}
            empty={empty(g)}
            showAssignee={g.kind !== 'assigned'}
            columns={columnsOf(g)}
            {names}
            ontotal={(n) => (totals[keyOf(g, i)] = n)}
          />
        {/if}
      </section>
    {/each}
  {/if}
</div>

{#if choices}
  <GadgetDialog
    open={dialog !== null}
    gadget={dialog?.gadget ?? null}
    {choices}
    onclose={() => (dialog = null)}
    onsave={saved}
  />
{/if}

<style>
  .head-actions {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
  }

  .order-hint {
    margin: 0 0 12px;
  }

  .section-title {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    gap: 6px;
  }

  .section-title .name {
    min-width: 0;
    overflow-wrap: anywhere;
  }

  .section-title .actions {
    display: inline-flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px;
  }

  .grip {
    color: var(--muted);
    font-size: 0.6875rem;
    letter-spacing: -2px;
    cursor: grab;
  }

  .gadget.editing {
    padding: 4px 8px 8px;
    margin-inline: -8px;
    border: 1px dashed var(--rule);
  }

  .gadget.is-dragged {
    opacity: 0.45;
  }

  .gadget.is-over {
    box-shadow: inset 0 2px 0 var(--accent);
  }

  .unavailable {
    margin: 0;
    padding: 8px 12px;
    border-left: 3px solid var(--rule);
    color: var(--muted);
  }

  .loading {
    padding: 10px 0;
  }

  /*
   * The lists stack, so they share their column widths and line up; a list
   * without the assignee or due column keeps the room it would take. Each
   * width holds the column's usual widest content (a nine-character key,
   * « IN PROGRESS », two triangles, avatar and name, the longest date). They
   * are minimums: the summary claims the whole row, and a plain width gives
   * way to it. Narrower screens need that room for the summary.
   */
  @media (min-width: 1000px) {
    section :global(th.c-key) {
      min-width: 5.625rem;
    }

    section :global(th.c-status) {
      min-width: 8rem;
    }

    section :global(th.c-priority) {
      min-width: 2.5rem;
    }

    section :global(th.c-assignee) {
      min-width: 10rem;
    }

    section :global(th.c-due) {
      min-width: 8.75rem;
    }

    section :global(th.c-updated) {
      min-width: 9.75rem;
    }

    section :global(.ledger:not(:has(.c-assignee)) th.c-priority) {
      min-width: calc(2.5rem + 10rem);
    }

    section :global(.ledger:not(:has(.c-due)) th.c-updated) {
      min-width: calc(9.75rem + 8.75rem);
    }
  }

  @media (max-width: 600px) {
    .grip {
      display: none;
    }
  }
</style>
