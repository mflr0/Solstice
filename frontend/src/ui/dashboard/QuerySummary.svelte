<script lang="ts">
  import {
    DATE_FIELDS,
    jqlToFilters,
    LIST_FIELDS,
    matchPreset,
    rangeSet,
    type FilterState,
  } from '../../core/filters.js';
  import type { StatusCategory } from '../../core/types.js';
  import { formatList } from '../../i18n/format.js';
  import { categoryLabels, t } from '../../i18n/t.js';
  import { labelOf } from '../search/sources.svelte.js';

  /** A view's filters on one line ("Assignee Me · Status Open"); its JQL when they can't show it. */
  interface Props {
    jql: string;
  }
  const { jql }: Props = $props();

  interface Chip {
    field: string;
    value: string;
  }

  const chips = $derived.by(() => {
    const read = jqlToFilters(jql);
    return read.ok ? chipsOf(read.state) : null;
  });

  const sameSet = (a: StatusCategory[], b: StatusCategory[]) =>
    a.length === b.length && a.every((c) => b.includes(c));

  function categories(cats: StatusCategory[]): string {
    if (sameSet(cats, ['todo', 'doing'])) return t.search.special.open;
    if (sameSet(cats, ['done'])) return t.search.special.done;
    return formatList(cats.map((c) => categoryLabels[c]));
  }

  function summarize(names: string[]): string {
    if (names.length <= 2) return names.join(', ');
    return `${names.slice(0, 2).join(', ')} ${t.search.andMore(names.length - 2)}`;
  }

  function chipsOf(s: FilterState): Chip[] {
    const out: Chip[] = [];
    for (const f of LIST_FIELDS) {
      const values = s.values[f] ?? [];
      if (f === 'status' && s.categories.length > 0 && s.categories.length < 3) {
        out.push({ field: t.search.fields.status, value: categories(s.categories) });
      }
      if (!values.length) continue;
      const role = f === 'people' ? s.peopleRole : undefined;
      out.push({
        field: t.search.fields[f],
        value: summarize(values.map((v) => labelOf(f, v, role))),
      });
    }
    for (const f of DATE_FIELDS) {
      const r = s.dates[f];
      if (!r || !rangeSet(r)) continue;
      const preset = matchPreset(f, r);
      const value = preset
        ? (t.search.datePresets[preset] ?? preset)
        : r.empty
          ? t.search.noDate
          : [r.from, r.to].flatMap((b) => (b ? [`${b.op} ${b.value}`] : [])).join(' ');
      out.push({ field: t.search.fields[f], value });
    }
    if (s.text.trim()) out.push({ field: t.search.text, value: `“${s.text.trim()}”` });
    return out;
  }
</script>

{#if chips === null}
  <span class="jql">{jql}</span>
{:else if chips.length === 0}
  <span>{t.home.allIssues}</span>
{:else}
  {#each chips as c, i (i)}
    {#if i > 0}<span class="dot" aria-hidden="true">·</span>{/if}
    <span class="part"><span class="what">{c.field}</span> {c.value}</span>
  {/each}
{/if}

<style>
  .part {
    white-space: nowrap;
  }

  .what {
    color: var(--muted);
  }

  .dot {
    color: var(--rule-strong);
  }

  .jql {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-family: var(--font-mono);
  }
</style>
