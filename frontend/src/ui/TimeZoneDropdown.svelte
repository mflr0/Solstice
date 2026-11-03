<script lang="ts">
  import { zoneChoices } from '../core/setup.js';
  import Dropdown, { type DropdownMessages, type DropdownOption } from './Dropdown.svelte';

  /**
   * A time zone chooser: the searchable Dropdown, each zone behind its UTC
   * offset badge. Build the zones with zoneChoices() (src/core/setup.ts). A
   * value no zone has stays listed, so it is shown and kept.
   */
  interface Props {
    zones: ReturnType<typeof zoneChoices>;
    value: string;
    label: string;
    onchange: (value: string) => void;
    /** An option before the zones, e.g. the empty value "The server's". */
    first?: DropdownOption | undefined;
    id?: string | undefined;
    invalid?: boolean | undefined;
    required?: boolean | undefined;
    onblur?: (() => void) | undefined;
    messages?: DropdownMessages | undefined;
  }
  const { zones, value, label, onchange, first, id, invalid, required, onblur, messages }: Props =
    $props();

  const offsets = $derived(new Map(zones.map((z) => [z.zone, z.offset])));

  const options = $derived.by(() => {
    const out: DropdownOption[] = first ? [first] : [];
    if (value && value !== first?.value && !offsets.has(value)) out.push({ value, label: value });
    for (const z of zones) out.push({ value: z.zone, label: z.zone, keywords: z.keywords });
    return out;
  });
</script>

<Dropdown {options} {value} {label} {onchange} {id} {invalid} {required} {onblur} {messages} search>
  {#snippet icon(v)}
    {#if offsets.has(v)}<span class="chip offset">{offsets.get(v)}</span>{/if}
  {/snippet}
</Dropdown>

<style>
  /* The same width for every offset, so the names line up. */
  .offset {
    justify-content: center;
    min-width: 5.5rem;
    line-height: 1.25;
    font-variant-numeric: tabular-nums;
  }
</style>
