<script lang="ts">
  import { untrack } from 'svelte';
  import { sameValue, toggle } from '../../core/filters.js';
  import { matches } from '../../core/text.js';
  import { t } from '../../i18n/t.js';
  import { autofocus } from '../../lib/actions.js';
  import Highlight from '../Highlight.svelte';
  import type { Option } from './sources.svelte.js';

  /**
   * A searchable checklist: shortcuts first (Moi, Non assigné, Ouverts…),
   * then the values. Each tick applies at once. The list is driven from the
   * search field: arrows move, Enter ticks.
   */
  interface Shortcut {
    label: string;
    checked: boolean;
    toggle: () => void;
    /** Issues it would show, when counts are known. */
    count?: number | undefined;
  }
  interface Props {
    label: string;
    /** Chosen values (JQL). */
    selected: string[];
    onchange: (values: string[]) => void;
    /** Values matching what is typed. */
    load: (q: string) => Promise<Option[]>;
    /** Values with a meaning of their own, ticked like the others. */
    special?: Option[] | undefined;
    /** Toggles that are not values (status categories). */
    shortcuts?: Shortcut[] | undefined;
    labelOf: (value: string) => string;
    /** Typed text may be added as a value (labels, versions…). */
    free?: ((text: string) => string) | undefined;
    /** Clears the filter, shortcuts included. */
    onreset: () => void;
    /** Issues per value key (Option.facets), for the search without this filter. */
    counts?: Record<string, number> | null | undefined;
  }
  const {
    label,
    selected,
    onchange,
    load,
    special = [],
    shortcuts = [],
    labelOf,
    free,
    onreset,
    counts = null,
  }: Props = $props();

  interface Row {
    key: string;
    label: string;
    hint?: string | undefined;
    group?: string | undefined;
    checked: boolean;
    act: () => void;
    count?: number | undefined;
    /** The typed text offered as a value: shown as is, not as a match. */
    typed?: boolean | undefined;
  }

  let q = $state('');
  let options = $state<Option[]>([]);
  let loading = $state(true);
  let active = $state(0);
  let seq = 0;
  let timer: ReturnType<typeof setTimeout> | undefined;
  const uid = $props.id();
  const listId = `vp-${uid}`;

  const isChosen = (v: string) => selected.some((s) => sameValue(s, v));
  const pick = (v: string) => onchange(toggle(selected, v));
  /**
   * The count of an option: the sum over its keys, when counts are known. An
   * empty bucket ("") the server did not report is unknown, not zero.
   */
  function countOf(o: Option): number | undefined {
    if (!counts || !o.facets) return undefined;
    if (o.facets.length === 1 && o.facets[0] === '' && !('' in counts)) return undefined;
    return o.facets.reduce((n, k) => n + (counts[k] ?? 0), 0);
  }

  const rows = $derived.by<Row[]>(() => {
    const out: Row[] = [];
    const group = special.length || shortcuts.length ? t.search.shortcuts : undefined;
    for (const [i, s] of shortcuts.entries()) {
      if (matches(q, s.label)) {
        out.push({
          key: `s${i}`,
          label: s.label,
          group,
          checked: s.checked,
          act: s.toggle,
          count: s.count,
        });
      }
    }
    for (const o of special) {
      if (matches(q, o.label)) {
        out.push({
          key: `v${o.value}`,
          label: o.label,
          group,
          checked: isChosen(o.value),
          act: () => pick(o.value),
          count: countOf(o),
        });
      }
    }
    // Chosen values the list does not show (yet): still there to untick.
    const listed = [...special, ...options];
    for (const v of selected) {
      if (listed.some((o) => sameValue(o.value, v))) continue;
      out.push({ key: `c${v}`, label: labelOf(v), checked: true, act: () => pick(v) });
    }
    for (const o of options) {
      out.push({
        key: `o${o.value}`,
        label: o.label,
        hint: o.hint,
        group: o.group,
        checked: isChosen(o.value),
        act: () => pick(o.value),
        count: countOf(o),
      });
    }
    const typed = q.trim();
    if (free && typed && !out.some((r) => r.label.toLowerCase() === typed.toLowerCase())) {
      const v = free(typed);
      out.push({
        key: `f${v}`,
        label: t.search.addValue(typed),
        typed: true,
        checked: false,
        act: () => pick(v),
      });
    }
    return out;
  });

  async function fetchOptions() {
    const mine = ++seq;
    loading = true;
    try {
      const found = await load(q);
      if (mine !== seq) return;
      options = found;
    } catch {
      if (mine === seq) options = [];
    } finally {
      if (mine === seq) loading = false;
    }
  }

  $effect(() => {
    untrack(() => void fetchOptions());
    return () => clearTimeout(timer);
  });

  function typed() {
    active = 0;
    clearTimeout(timer);
    timer = setTimeout(() => void fetchOptions(), 160);
  }

  function keydown(e: KeyboardEvent) {
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      active = Math.min(rows.length - 1, active + 1);
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      active = Math.max(0, active - 1);
    } else if (e.key === 'Home' && !q) {
      e.preventDefault();
      active = 0;
    } else if (e.key === 'End' && !q) {
      e.preventDefault();
      active = rows.length - 1;
    } else if (e.key === 'Enter') {
      e.preventDefault();
      rows[active]?.act();
    }
  }

  $effect(() => {
    if (active < 0 || active >= rows.length) return;
    document.getElementById(`${listId}-${active}`)?.scrollIntoView({ block: 'nearest' });
  });

  const anyChosen = $derived(selected.length > 0 || shortcuts.some((s) => s.checked));
</script>

<div class="value-picker">
  <input
    class="input filter-search"
    type="text"
    bind:value={q}
    oninput={typed}
    onkeydown={keydown}
    placeholder={t.search.filterPlaceholder}
    aria-label={t.search.filterValues(label)}
    role="combobox"
    aria-expanded="true"
    aria-controls={listId}
    aria-autocomplete="list"
    aria-activedescendant={rows[active] ? `${listId}-${active}` : undefined}
    autocomplete="off"
    spellcheck="false"
    use:autofocus
  />
  <ul
    class="filter-options"
    id={listId}
    role="listbox"
    aria-multiselectable="true"
    aria-label={label}
  >
    {#each rows as row, i (row.key)}
      {#if row.group && row.group !== rows[i - 1]?.group}
        <li class="group-label" role="presentation">{row.group}</li>
      {/if}
      <!-- Keyboard use goes through the search field (aria-activedescendant);
           a pointer press leaves the focus there, so typing goes on. -->
      <!-- svelte-ignore a11y_click_events_have_key_events -->
      <li
        id="{listId}-{i}"
        class="item filter-option"
        class:active={i === active}
        role="option"
        aria-selected={row.checked}
        onclick={row.act}
        onpointerdown={(e) => e.preventDefault()}
        onmouseenter={() => (active = i)}
      >
        <span class="box" aria-hidden="true">{row.checked ? '✓' : ''}</span>
        <span class="label"
          >{#if row.typed}{row.label}{:else}<Highlight text={row.label} search={q} />{/if}</span
        >
        {#if row.hint}<span class="hint">{row.hint}</span>{/if}
        {#if row.count !== undefined}<span class="count" class:zero={row.count === 0}
            >{row.count}</span
          >{/if}
      </li>
    {/each}
    {#if rows.length === 0}
      <li class="none" role="presentation">
        {loading ? t.search.loadingValues : t.search.noValue}
      </li>
    {/if}
  </ul>
  {#if anyChosen}
    <div class="picker-foot">
      <button type="button" class="link-btn" onclick={onreset}>{t.search.reset}</button>
    </div>
  {/if}
</div>

<style>
  .value-picker {
    display: flex;
    flex-direction: column;
    min-height: 0;
  }

  .filter-search {
    margin: 8px 8px 4px;
    width: calc(100% - 16px);
    height: 1.875rem;
    flex: none;
  }

  .filter-options {
    list-style: none;
    margin: 0;
    padding: 2px 0 4px;
    overflow: auto;
    min-height: 0;
  }

  .filter-option {
    gap: 8px;
  }

  .filter-option .box {
    display: inline-grid;
    place-items: center;
    width: 15px;
    height: 15px;
    flex: none;
    border: var(--frame) solid var(--rule-strong);
    border-radius: var(--radius-xs);
    background-color: var(--paper);
    color: var(--accent-text);
    font-size: 0.6875rem;
    line-height: 1;
  }

  .filter-option[aria-selected='true'] .box {
    border-color: var(--accent);
  }

  .label {
    min-width: 0;
    overflow-wrap: anywhere;
  }

  .count {
    margin-left: auto;
    padding-left: 10px;
    min-width: 3ch;
    text-align: right;
    font: 0.71875rem var(--font-mono);
    font-variant-numeric: tabular-nums;
    color: var(--ink-soft);
  }

  .hint + .count {
    margin-left: 0;
  }

  .count.zero {
    color: var(--muted);
  }

  .hint {
    margin-left: auto;
    padding-left: 8px;
    font: 0.71875rem var(--font-mono);
    color: var(--muted);
    white-space: nowrap;
  }

  .none {
    padding: 8px 12px;
    color: var(--muted);
    font-size: 0.8125rem;
  }

  .picker-foot {
    display: flex;
    justify-content: flex-end;
    padding: 6px 12px 8px;
    border-top: 1px solid var(--rule);
    flex: none;
  }

  @media (max-width: 600px) {
    .filter-option {
      padding-top: 9px;
      padding-bottom: 9px;
    }
  }
</style>
