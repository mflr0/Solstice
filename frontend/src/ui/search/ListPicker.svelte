<script lang="ts">
  import {
    withCategories,
    withValues,
    type FilterState,
    type ListField,
  } from '../../core/filters.js';
  import { quote } from '../../core/jql.js';
  import type { StatusCategory } from '../../core/types.js';
  import { t } from '../../i18n/t.js';
  import { session } from '../../lib/session.svelte.js';
  import {
    DONE,
    hasMarks,
    labelOf,
    loadOptions,
    markOf,
    OPEN,
    sameCategories,
    specialValues,
    type Option,
  } from './sources.svelte.js';
  import ValuePicker from './ValuePicker.svelte';

  /**
   * The checklist of a list field's values, shared by the filter bar and the
   * quick filter row: each tick changes the filters at once.
   */
  interface Props {
    filters: FilterState;
    field: ListField;
    onchange: (next: FilterState) => void;
    onreset: () => void;
    /** A project the search is already limited to (project page). */
    project?: string | undefined;
    /** Issues per value (facetCounts), null while unknown. */
    counts?: Record<string, number> | null | undefined;
    /** The values offered, in place of the field's own source (a board's). */
    load?: ((q: string) => Promise<Option[]>) | undefined;
  }
  const { filters, field, onchange, onreset, project, counts = null, load }: Props = $props();

  /** Fields whose values may be typed in freely (anything the server will match by name). */
  const FREE: ListField[] = ['labels', 'fixVersion', 'component', 'epic'];

  const scope = () => (project ? [quote(project)] : (filters.values.project ?? []));

  /** Issues in some status categories, from the status counts. */
  function categoryCount(cats: StatusCategory[]): number | undefined {
    if (!counts) return undefined;
    return (session.meta?.statuses ?? [])
      .filter((st) => cats.includes(st.category))
      .reduce((n, st) => n + (counts[st.id] ?? 0), 0);
  }

  function shortcut(label: string, cats: StatusCategory[]) {
    const on = sameCategories(filters.categories, cats);
    return {
      label,
      checked: on,
      toggle: () => onchange(withCategories(filters, on ? [] : cats)),
      count: categoryCount(cats),
    };
  }

  const shortcuts = $derived(
    field === 'status'
      ? [shortcut(t.search.special.open, OPEN), shortcut(t.search.special.done, DONE)]
      : [],
  );
</script>

<ValuePicker
  label={t.search.fields[field]}
  selected={filters.values[field] ?? []}
  onchange={(v) => onchange(withValues(filters, field, v))}
  load={load ?? ((q) => loadOptions(field, q, scope()))}
  special={specialValues(field)}
  {shortcuts}
  labelOf={(v) => labelOf(field, v)}
  free={FREE.includes(field) ? (s) => (field === 'epic' ? s.toUpperCase() : quote(s)) : undefined}
  {onreset}
  {counts}
  markOf={hasMarks(field) ? (v) => markOf(field, v) : undefined}
/>
