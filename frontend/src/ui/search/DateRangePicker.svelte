<script lang="ts">
  import {
    datePresets,
    dayRange,
    matchPreset,
    rangeDays,
    rangeSet,
    type DateField,
    type DateRange,
  } from '../../core/filters.js';
  import { t } from '../../i18n/t.js';

  /** A date filter: shortcuts (« 7 derniers jours »…) or two calendar days. */
  interface Props {
    field: DateField;
    label: string;
    range: DateRange | undefined;
    onchange: (range: DateRange | undefined) => void;
  }
  const { field, label, range, onchange }: Props = $props();

  const presets = $derived(datePresets(field));
  const current = $derived(range ? matchPreset(field, range) : null);
  const days = $derived(range ? rangeDays(range) : { from: '', to: '' });
  const uid = $props.id();
  const name = `dr-${uid}`;

  function setDay(which: 'from' | 'to', value: string) {
    const next = { ...days, [which]: value };
    const r = dayRange(next.from, next.to);
    onchange(rangeSet(r) ? r : undefined);
  }
</script>

<div class="date-picker">
  <fieldset class="date-presets">
    <legend class="sr-only">{label}</legend>
    {#each presets as p (p.id)}
      <label class="item date-preset" class:active={current === p.id}>
        <input
          type="radio"
          class="checkbox"
          {name}
          checked={current === p.id}
          onchange={() => onchange(p.range)}
        />
        <span>{t.search.datePresets[p.id] ?? p.id}</span>
      </label>
    {/each}
  </fieldset>
  <div class="custom">
    <span class="group-label">{t.search.customRange}</span>
    <div class="days">
      <label class="field">
        <span>{t.search.from}</span>
        <input
          class="input"
          type="date"
          value={days.from}
          onchange={(e) => setDay('from', e.currentTarget.value)}
        />
      </label>
      <label class="field">
        <span>{t.search.to}</span>
        <input
          class="input"
          type="date"
          value={days.to}
          min={days.from || undefined}
          onchange={(e) => setDay('to', e.currentTarget.value)}
        />
      </label>
    </div>
  </div>
  {#if range}
    <div class="picker-foot">
      <button type="button" class="link-btn" onclick={() => onchange(undefined)}
        >{t.search.reset}</button
      >
    </div>
  {/if}
</div>

<style>
  .date-picker {
    overflow: auto;
  }

  .date-presets {
    border: none;
    margin: 0;
    padding: 6px 0 4px;
  }

  .date-preset {
    cursor: pointer;
  }

  .date-preset input {
    margin: 0;
  }

  .custom {
    padding: 4px 0 10px;
    border-top: 1px solid var(--rule);
  }

  .custom .group-label {
    display: block;
    padding: 8px 12px 6px;
    font: 500 0.71875rem/1 var(--font-body);
    color: var(--muted);
  }

  .days {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 8px;
    padding: 0 12px;
  }

  .days .input {
    height: 1.875rem;
    font-size: 0.8125rem;
    padding: 4px 6px;
  }

  .picker-foot {
    display: flex;
    justify-content: flex-end;
    padding: 6px 12px 8px;
    border-top: 1px solid var(--rule);
  }
</style>
