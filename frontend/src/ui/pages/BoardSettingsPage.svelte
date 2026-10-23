<script lang="ts">
  import type { BoardDetail, BoardPatch, SwimlaneStrategy } from '../../core/board-types.js';
  import { searchHref } from '../../core/filters-url.js';
  import { paths } from '../../core/route.js';
  import type { StatusRef } from '../../core/types.js';
  import { t } from '../../i18n/t.js';
  import { api, errorMessage } from '../../lib/api.js';
  import { router } from '../../lib/router.svelte.js';
  import { session } from '../../lib/session.svelte.js';
  import { notify } from '../../lib/ui.svelte.js';
  import BoardHeader from '../board/BoardHeader.svelte';
  import Modal from '../Modal.svelte';
  import Stamp from '../Stamp.svelte';
  import NotFoundPage from './NotFoundPage.svelte';

  /**
   * A board's settings, one section at a time: each section saves on its
   * own, so the server's verdict (invalid JQL, a status in two columns…)
   * shows right where the change was made.
   */
  interface Props {
    boardId: number;
  }
  const { boardId }: Props = $props();

  type Part = 'general' | 'columns' | 'quick' | 'lanes' | 'estimate';

  interface ColumnDraft {
    id?: number;
    name: string;
    min: string;
    max: string;
    statuses: StatusRef[];
  }
  interface QuickDraft {
    id?: number;
    name: string;
    query: string;
    description: string;
  }
  interface LaneDraft {
    id?: number;
    name: string;
    query: string;
    isDefault: boolean;
  }
  interface Drafts {
    general: { name: string; subQuery: string; doneCutoff: string };
    columns: ColumnDraft[];
    quick: QuickDraft[];
    lanes: { strategy: SwimlaneStrategy; list: LaneDraft[] };
    estimate: string;
  }

  let detail = $state<BoardDetail | null>(null);
  let error = $state('');
  let draft = $state<Drafts | null>(null);
  let errors = $state<Partial<Record<Part, string>>>({});
  let saving = $state<Part | null>(null);
  let pool = $state<StatusRef[]>([]);
  let deleting = $state(false);
  let deleteBusy = $state(false);

  function fromDetail(d: BoardDetail): Drafts {
    return {
      general: {
        name: d.board.name,
        subQuery: d.board.subQuery ?? '',
        doneCutoff: d.board.doneCutoff ?? '',
      },
      columns: d.columns.map((c) => ({
        id: c.id,
        name: c.name,
        min: c.min === null ? '' : String(c.min),
        max: c.max === null ? '' : String(c.max),
        statuses: [...c.statuses],
      })),
      quick: d.quickFilters.map((q) => ({
        id: q.id,
        name: q.name,
        query: q.query ?? '',
        description: q.description ?? '',
      })),
      lanes: {
        strategy: d.board.swimlanes,
        list: d.swimlanes.map((l) => ({
          id: l.id,
          name: l.name,
          query: l.query ?? '',
          isDefault: !!l.default,
        })),
      },
      estimate: d.board.estimateField ?? '',
    };
  }

  $effect(() => {
    api
      .board(boardId)
      .then((d) => {
        detail = d;
        draft = fromDetail(d);
        document.title = `${t.boardSettings.title} — ${d.board.name} — ${t.appName}`;
        void loadPool(d);
      })
      .catch((e: unknown) => (error = errorMessage(e)));
  });

  /** The statuses the board's projects use, to offer those without a column. */
  async function loadPool(d: BoardDetail) {
    // eslint-disable-next-line svelte/prefer-svelte-reactivity -- a throwaway value, not state
    const seen = new Map<string, StatusRef>();
    try {
      const lists = await Promise.all(d.projects.map((p) => api.boardProjectStatuses(p.key)));
      for (const l of lists)
        for (const w of l.workflows) for (const s of w.statuses) seen.set(s.id, s);
    } catch {
      for (const s of session.meta?.statuses ?? []) seen.set(s.id, s);
    }
    pool = [...seen.values()];
  }

  const saved = $derived(detail ? fromDetail(detail) : null);
  const dirty = $derived.by(() => {
    const out: Partial<Record<Part, boolean>> = {};
    if (!draft || !saved) return out;
    for (const part of ['general', 'columns', 'quick', 'lanes', 'estimate'] as const) {
      out[part] = JSON.stringify(draft[part]) !== JSON.stringify(saved[part]);
    }
    return out;
  });

  const mapped = $derived(new Set(draft?.columns.flatMap((c) => c.statuses.map((s) => s.id))));
  const unmapped = $derived(pool.filter((s) => !mapped.has(s.id)));

  function count(v: string): number | null {
    const n = Number(v);
    return v.trim() === '' || !Number.isFinite(n) ? null : Math.max(0, Math.round(n));
  }

  function patchFor(part: Part, d: Drafts, kanban: boolean): BoardPatch {
    switch (part) {
      case 'general':
        return {
          name: d.general.name,
          ...(kanban ? { subQuery: d.general.subQuery, doneCutoff: d.general.doneCutoff } : {}),
        };
      case 'columns':
        return {
          columns: d.columns.map((c) => ({
            ...(c.id ? { id: c.id } : {}),
            name: c.name,
            min: count(c.min),
            max: count(c.max),
            statuses: c.statuses.map((s) => s.id),
          })),
        };
      case 'quick':
        return {
          quickFilters: d.quick.map((q) => ({
            ...(q.id ? { id: q.id } : {}),
            name: q.name,
            query: q.query,
            ...(q.description.trim() ? { description: q.description } : {}),
          })),
        };
      case 'lanes':
        return {
          swimlanes: d.lanes.strategy,
          ...(d.lanes.strategy === 'custom'
            ? {
                lanes: d.lanes.list.map((l) => ({
                  ...(l.id ? { id: l.id } : {}),
                  name: l.name,
                  ...(l.query.trim() ? { query: l.query } : {}),
                  default: l.isDefault,
                })),
              }
            : {}),
        };
      case 'estimate':
        return { estimate: d.estimate };
    }
  }

  async function save(part: Part, e?: SubmitEvent) {
    e?.preventDefault();
    if (!detail || !draft || saving) return;
    saving = part;
    errors[part] = '';
    try {
      const next = await api.saveBoard(
        detail.board.id,
        patchFor(part, draft, detail.board.kind === 'kanban'),
      );
      detail = next;
      // Only this section takes the server's version: the others keep their edits.
      const fresh = fromDetail(next);
      if (part === 'general') draft.general = fresh.general;
      else if (part === 'columns') draft.columns = fresh.columns;
      else if (part === 'quick') draft.quick = fresh.quick;
      else if (part === 'lanes') draft.lanes = fresh.lanes;
      else draft.estimate = fresh.estimate;
      notify(t.boardSettings.saved);
    } catch (err) {
      errors[part] = errorMessage(err);
    } finally {
      saving = null;
    }
  }

  function discard(part: Part) {
    if (!draft || !saved) return;
    errors[part] = '';
    if (part === 'general') draft.general = saved.general;
    else if (part === 'columns') draft.columns = saved.columns;
    else if (part === 'quick') draft.quick = saved.quick;
    else if (part === 'lanes') draft.lanes = saved.lanes;
    else draft.estimate = saved.estimate;
  }

  /** Swaps an item with its neighbour. */
  function shift<T>(list: T[], i: number, step: -1 | 1) {
    const j = i + step;
    const a = list[i];
    const b = list[j];
    if (a === undefined || b === undefined) return;
    list[i] = b;
    list[j] = a;
  }

  /** The row an error message names (« name », “name”, „name“…), to mark it. */
  function named(part: Part, name: string): boolean {
    const message = errors[part];
    if (!name || !message) return false;
    return [`« ${name} »`, `«${name}»`, `“${name}”`, `„${name}“`].some((q) => message.includes(q));
  }

  async function remove() {
    if (!detail || deleteBusy) return;
    deleteBusy = true;
    try {
      await api.deleteBoard(detail.board.id);
      notify(t.boardSettings.deleted(detail.board.name));
      deleting = false;
      router.navigate(paths.boards());
    } catch (err) {
      notify(errorMessage(err), 'error');
    } finally {
      deleteBusy = false;
    }
  }
</script>

{#snippet bar(part: Part)}
  <div class="bar">
    {#if errors[part]}<p class="error-text" role="alert">{errors[part]}</p>{/if}
    <span class="grow"></span>
    {#if dirty[part]}
      <span class="muted unsaved">{t.boardSettings.unsaved}</span>
      <button type="button" class="link-btn" onclick={() => discard(part)}
        >{t.boardSettings.discard}</button
      >
    {/if}
    <button type="submit" class="btn btn-primary" disabled={!dirty[part] || saving !== null}
      >{saving === part ? t.common.saving : t.boardSettings.save}</button
    >
  </div>
{/snippet}

{#if error}
  <NotFoundPage message={error} />
{:else if !detail || !draft}
  <div class="page"><p class="muted loading">{t.common.loading}</p></div>
{:else if !detail.can.manage}
  <div class="page">
    <BoardHeader {detail} view="settings" />
    <p class="empty">{t.boardSettings.forbidden}</p>
  </div>
{:else}
  {@const d = detail}
  {@const f = draft}
  {@const kanban = d.board.kind === 'kanban'}
  <div class="page" data-title={`${t.boardSettings.title} — ${d.board.name}`}>
    <BoardHeader detail={d} view="settings" />
    <p class="eyebrow">{t.boardSettings.title}</p>

    <!-- General -->
    <form class="part" onsubmit={(e) => save('general', e)} aria-labelledby="part-general">
      <h2 class="section-title" id="part-general">{t.boardSettings.general}</h2>
      <div class="fields">
        <label class="field">
          <span>{t.boardSettings.name}</span>
          <input class="input" bind:value={f.general.name} required maxlength="255" />
        </label>
        <div class="field">
          <span class="field-label">{t.boardSettings.filter}</span>
          <code class="query">{d.board.query}</code>
          <span class="hint"
            >{t.boardSettings.filterHint}
            <a href={searchHref(d.board.query)}>{t.boardSettings.seeIssues}</a></span
          >
        </div>
        {#if kanban}
          <label class="field">
            <span>{t.boardSettings.subQuery}</span>
            <input
              class="input mono"
              bind:value={f.general.subQuery}
              spellcheck="false"
              aria-invalid={errors.general?.startsWith(t.boardSettings.subQuery) || undefined}
            />
            <span class="hint">{t.boardSettings.subQueryHint}</span>
          </label>
          <label class="field narrow">
            <span>{t.boardSettings.doneCutoff}</span>
            <input class="input mono" bind:value={f.general.doneCutoff} placeholder="2w" />
            <span class="hint">{t.boardSettings.doneCutoffHint}</span>
          </label>
        {/if}
      </div>
      {@render bar('general')}
    </form>

    <!-- Columns -->
    <form class="part" onsubmit={(e) => save('columns', e)} aria-labelledby="part-columns">
      <h2 class="section-title" id="part-columns">
        {t.boardSettings.columns} <span class="count">{f.columns.length}</span>
      </h2>
      <p class="hint intro">{t.boardSettings.columnsHint}</p>
      <ol class="columns">
        {#each f.columns as column, i (column.id ?? `new-${i}`)}
          <li class="card column-edit">
            <div class="card-head">
              <span class="sr-only">{i + 1}.</span>
              <input
                class="input name"
                bind:value={column.name}
                required
                maxlength="255"
                aria-label={t.boardSettings.columnName}
              />
            </div>
            <div class="column-body">
              <div class="limits">
                <label class="field">
                  <span>{t.boardSettings.min}</span>
                  <input class="input" type="number" min="0" bind:value={column.min} />
                </label>
                <label class="field">
                  <span>{t.boardSettings.max}</span>
                  <input class="input" type="number" min="0" bind:value={column.max} />
                </label>
              </div>
              <div class="field">
                <span class="field-label">{t.boardSettings.statuses}</span>
                {#if column.statuses.length}
                  <ul class="statuses">
                    {#each column.statuses as s (s.id)}
                      <li>
                        <Stamp status={s} />
                        <button
                          type="button"
                          class="btn btn-quiet btn-small icon"
                          aria-label={t.boardSettings.remove(s.name)}
                          title={t.boardSettings.remove(s.name)}
                          onclick={() =>
                            (column.statuses = column.statuses.filter((x) => x.id !== s.id))}
                          >×</button
                        >
                      </li>
                    {/each}
                  </ul>
                {:else}
                  <span class="hint">{t.boardSettings.noStatus}</span>
                {/if}
              </div>
              <div class="tools">
                <button
                  type="button"
                  class="btn btn-small"
                  disabled={i === 0}
                  aria-label={t.boardSettings.moveLeft(column.name)}
                  title={t.boardSettings.moveLeft(column.name)}
                  onclick={() => shift(f.columns, i, -1)}>←</button
                >
                <button
                  type="button"
                  class="btn btn-small"
                  disabled={i === f.columns.length - 1}
                  aria-label={t.boardSettings.moveRight(column.name)}
                  title={t.boardSettings.moveRight(column.name)}
                  onclick={() => shift(f.columns, i, 1)}>→</button
                >
                <button
                  type="button"
                  class="btn btn-small btn-danger"
                  disabled={f.columns.length === 1}
                  onclick={() => f.columns.splice(i, 1)}
                  aria-label={t.boardSettings.remove(column.name)}>{t.common.delete}</button
                >
              </div>
            </div>
          </li>
        {/each}
        <li class="add-column">
          <button
            type="button"
            class="btn"
            onclick={() =>
              f.columns.push({ name: t.boardSettings.newColumn, min: '', max: '', statuses: [] })}
            >{t.boardSettings.addColumn}</button
          >
        </li>
      </ol>

      <div class="unmapped">
        <h3 class="sub">{t.boardSettings.unmapped}</h3>
        {#if unmapped.length}
          <p class="hint">{t.boardSettings.unmappedHint}</p>
          <ul class="statuses">
            {#each unmapped as s (s.id)}
              <li>
                <Stamp status={s} />
                <select
                  class="select put"
                  aria-label={t.boardSettings.putIn(s.name)}
                  value=""
                  onchange={(e) => {
                    const target = f.columns[Number(e.currentTarget.value)];
                    if (target) target.statuses.push(s);
                  }}
                >
                  <option value="" disabled>{t.boardSettings.chooseColumn}</option>
                  {#each f.columns as c, i (c.id ?? `new-${i}`)}
                    <option value={String(i)}>{c.name}</option>
                  {/each}
                </select>
              </li>
            {/each}
          </ul>
        {:else}
          <p class="hint">{t.boardSettings.allMapped}</p>
        {/if}
      </div>
      {@render bar('columns')}
    </form>

    <!-- Quick filters -->
    <form class="part" onsubmit={(e) => save('quick', e)} aria-labelledby="part-quick">
      <h2 class="section-title" id="part-quick">
        {t.boardSettings.quickFilters} <span class="count">{f.quick.length}</span>
      </h2>
      <p class="hint intro">{t.boardSettings.quickFiltersHint}</p>
      {#if f.quick.length === 0}<p class="empty">{t.boardSettings.noQuickFilters}</p>{/if}
      <ol class="rows">
        {#each f.quick as q, i (q.id ?? `new-${i}`)}
          <li class="row-edit" class:invalid={named('quick', q.name)}>
            <label class="field">
              <span>{t.boardSettings.name}</span>
              <input class="input" bind:value={q.name} required maxlength="255" />
            </label>
            <label class="field wide">
              <span>{t.boardSettings.query}</span>
              <input
                class="input mono"
                bind:value={q.query}
                spellcheck="false"
                aria-invalid={named('quick', q.name) || undefined}
              />
            </label>
            <label class="field">
              <span>{t.boardSettings.description}</span>
              <input class="input" bind:value={q.description} maxlength="255" />
            </label>
            {@render order(f.quick, i, q.name)}
          </li>
        {/each}
      </ol>
      <button
        type="button"
        class="btn"
        onclick={() =>
          f.quick.push({ name: t.boardSettings.newQuickFilter, query: '', description: '' })}
        >{t.boardSettings.addQuickFilter}</button
      >
      {@render bar('quick')}
    </form>

    <!-- Swimlanes -->
    <form class="part" onsubmit={(e) => save('lanes', e)} aria-labelledby="part-lanes">
      <h2 class="section-title" id="part-lanes">{t.boardSettings.swimlanes}</h2>
      <fieldset class="choices">
        <legend class="sr-only">{t.boardSettings.swimlanes}</legend>
        {#each ['none', 'custom', 'assignee', 'epic', 'parent'] as const as s (s)}
          <label class="choice">
            <input type="radio" name="strategy" value={s} bind:group={f.lanes.strategy} />
            <span>
              <strong>{t.boardSettings.strategies[s]}</strong>
              <span class="hint">{t.boardSettings.strategyHints[s]}</span>
            </span>
          </label>
        {/each}
      </fieldset>
      {#if f.lanes.strategy === 'custom'}
        <h3 class="sub">{t.boardSettings.lanes}</h3>
        <p class="hint">{t.boardSettings.defaultLaneHint}</p>
        {#if f.lanes.list.length === 0}<p class="empty">{t.boardSettings.noLanes}</p>{/if}
        <ol class="rows">
          {#each f.lanes.list as lane, i (lane.id ?? `new-${i}`)}
            <li class="row-edit" class:invalid={named('lanes', lane.name)}>
              <label class="field">
                <span>{t.boardSettings.name}</span>
                <input class="input" bind:value={lane.name} required maxlength="255" />
              </label>
              <label class="field wide">
                <span>{t.boardSettings.query}</span>
                <input
                  class="input mono"
                  bind:value={lane.query}
                  spellcheck="false"
                  disabled={lane.isDefault}
                  aria-invalid={named('lanes', lane.name) || undefined}
                />
              </label>
              <label class="check">
                <input
                  type="radio"
                  name="default-lane"
                  checked={lane.isDefault}
                  onchange={() => f.lanes.list.forEach((l, j) => (l.isDefault = j === i))}
                />
                {t.boardSettings.defaultLane}
              </label>
              {@render order(f.lanes.list, i, lane.name)}
            </li>
          {/each}
        </ol>
        <button
          type="button"
          class="btn"
          onclick={() =>
            f.lanes.list.push({ name: t.boardSettings.newLane, query: '', isDefault: false })}
          >{t.boardSettings.addLane}</button
        >
      {/if}
      {@render bar('lanes')}
    </form>

    <!-- Estimation -->
    <form class="part" onsubmit={(e) => save('estimate', e)} aria-labelledby="part-estimate">
      <h2 class="section-title" id="part-estimate">{t.boardSettings.estimation}</h2>
      <fieldset class="choices">
        <legend class="sr-only">{t.boardSettings.estimation}</legend>
        <label class="choice">
          <input type="radio" name="estimate" value="" bind:group={f.estimate} />
          <span><strong>{t.boardSettings.estimateNone}</strong></span>
        </label>
        <label class="choice">
          <input
            type="radio"
            name="estimate"
            value="timeoriginalestimate"
            bind:group={f.estimate}
          />
          <span><strong>{t.boardSettings.estimateTime}</strong></span>
        </label>
        <div class="choice">
          <input
            type="radio"
            name="estimate"
            id="estimate-field"
            checked={f.estimate.startsWith('customfield_')}
            disabled={d.estimateFields.length === 0}
            onchange={() => (f.estimate = d.estimateFields[0]?.id ?? '')}
          />
          <span class="field-choice">
            <label for="estimate-field"><strong>{t.boardSettings.estimateField}</strong></label>
            {#if d.estimateFields.length}
              <select
                class="select"
                aria-label={t.boardSettings.estimateField}
                value={f.estimate.startsWith('customfield_') ? f.estimate : ''}
                onchange={(e) => (f.estimate = e.currentTarget.value)}
              >
                <option value="" disabled>{t.boardSettings.chooseField}</option>
                {#each d.estimateFields as field (field.id)}
                  <option value={field.id}>{field.name}</option>
                {/each}
              </select>
            {:else}
              <span class="hint">{t.boardSettings.noFields}</span>
            {/if}
          </span>
        </div>
      </fieldset>
      {@render bar('estimate')}
    </form>

    <!-- Deleting -->
    <section class="part" aria-labelledby="part-danger">
      <h2 class="section-title" id="part-danger">{t.boardSettings.danger}</h2>
      <div class="bar">
        <p class="hint">{t.boardSettings.dangerHint}</p>
        <span class="grow"></span>
        <button type="button" class="btn btn-danger" onclick={() => (deleting = true)}
          >{t.boardSettings.danger}</button
        >
      </div>
    </section>
  </div>

  <Modal open={deleting} title={t.boardSettings.deleteTitle} onclose={() => (deleting = false)}>
    <p>{t.boardSettings.deleteConfirm(d.board.name)}</p>
    {#snippet footer()}
      <button type="button" class="btn btn-quiet" onclick={() => (deleting = false)}
        >{t.common.cancel}</button
      >
      <button
        type="button"
        class="btn btn-primary"
        disabled={deleteBusy}
        onclick={() => void remove()}>{t.boardSettings.danger}</button
      >
    {/snippet}
  </Modal>
{/if}

{#snippet order(list: { name: string }[], i: number, name: string)}
  <div class="tools">
    <button
      type="button"
      class="btn btn-small"
      disabled={i === 0}
      aria-label={t.boardSettings.moveUp(name)}
      title={t.boardSettings.moveUp(name)}
      onclick={() => shift(list, i, -1)}>↑</button
    >
    <button
      type="button"
      class="btn btn-small"
      disabled={i === list.length - 1}
      aria-label={t.boardSettings.moveDown(name)}
      title={t.boardSettings.moveDown(name)}
      onclick={() => shift(list, i, 1)}>↓</button
    >
    <button
      type="button"
      class="btn btn-small btn-danger"
      aria-label={t.boardSettings.remove(name)}
      onclick={() => list.splice(i, 1)}>{t.common.delete}</button
    >
  </div>
{/snippet}

<style>
  .part {
    margin-bottom: 8px;
  }

  .fields {
    display: grid;
    gap: 14px;
    max-width: 720px;
  }

  .narrow {
    max-width: 320px;
  }

  .mono {
    font-family: var(--font-mono);
    font-size: 13px;
  }

  .query {
    display: block;
    padding: 6px 8px;
    background: var(--paper-sunk);
    border: 1px solid var(--rule);
    font-size: 12.5px;
    overflow-wrap: anywhere;
  }

  .hint {
    font-size: 12.5px;
    color: var(--muted);
    margin: 0;
  }

  .intro {
    margin: -2px 0 12px;
  }

  .sub {
    margin: 18px 0 6px;
    font: 600 13px/1.2 var(--font-body);
  }

  .bar {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px 12px;
    margin-top: 16px;
  }

  .bar .error-text {
    margin: 0;
    flex-basis: 100%;
  }

  .unsaved {
    font-size: 12.5px;
  }

  /* Columns, side by side like on the board. */
  .columns {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
    gap: 12px;
    margin: 0;
    padding: 0;
    list-style: none;
  }

  .column-edit {
    display: grid;
    align-content: start;
    min-width: 0;
  }

  .column-edit .card-head {
    padding: 8px;
  }

  .column-edit .name {
    font-weight: 600;
  }

  .column-body {
    display: grid;
    gap: 12px;
    padding: 10px;
  }

  .limits {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 8px;
  }

  .statuses {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    margin: 0;
    padding: 0;
    list-style: none;
  }

  .statuses li {
    display: inline-flex;
    align-items: center;
    gap: 2px;
  }

  .icon {
    padding: 0 6px;
    font-size: 15px;
  }

  .tools {
    display: flex;
    gap: 4px;
    align-items: center;
  }

  .tools .btn-danger {
    margin-left: auto;
  }

  .add-column {
    display: grid;
    place-items: center;
    min-height: 120px;
    border: 1px dashed var(--rule-strong);
    border-radius: var(--radius-m);
  }

  .unmapped {
    margin-top: 6px;
  }

  .unmapped .statuses {
    gap: 8px 14px;
    margin-top: 8px;
  }

  .unmapped .statuses li {
    gap: 6px;
  }

  .put {
    width: auto;
    padding: 2px 6px;
    font-size: 12.5px;
  }

  /* Quick filters and lanes: one row each. */
  .rows {
    display: grid;
    gap: 10px;
    margin: 0 0 12px;
    padding: 0;
    list-style: none;
  }

  .row-edit {
    display: grid;
    grid-template-columns: minmax(140px, 1fr) minmax(220px, 2fr) minmax(140px, 1fr) auto;
    gap: 8px;
    align-items: end;
    padding-bottom: 10px;
    border-bottom: 1px solid var(--rule);
  }

  .row-edit.invalid .mono {
    border-color: var(--accent);
  }

  .check {
    display: flex;
    gap: 6px;
    align-items: center;
    padding-bottom: 6px;
    white-space: nowrap;
    font-size: 13px;
  }

  .check input,
  .choice input {
    accent-color: var(--accent);
  }

  .choices {
    display: grid;
    gap: 8px;
    margin: 0;
    padding: 0;
    border: none;
  }

  .choice {
    display: flex;
    gap: 10px;
    align-items: baseline;
  }

  .choice > span {
    display: grid;
    gap: 2px;
  }

  .choice > .field-choice {
    display: flex;
    flex-wrap: wrap;
    gap: 8px 12px;
    align-items: baseline;
  }

  .field-choice .select {
    width: auto;
    min-width: 200px;
  }

  @media (max-width: 760px) {
    .row-edit {
      grid-template-columns: 1fr;
    }

    .row-edit .tools {
      justify-content: flex-start;
    }
  }
</style>
