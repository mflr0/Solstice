<script lang="ts">
  import { zoneChoices } from '../../core/setup.js';
  import type { DropdownMessages } from '../Dropdown.svelte';
  import TimeZoneDropdown from '../TimeZoneDropdown.svelte';

  /** A time zone, picked among the browser's IANA names, or typed when it can't list them. */
  interface Props {
    label: string;
    hint: string;
    value: string;
    id: string;
    messages: DropdownMessages;
  }
  let { label, hint, value = $bindable(), id, messages }: Props = $props();

  const zones = zoneChoices();
</script>

{#if zones.length}
  <div class="field">
    <label class="field-label" for={id}>{label}</label>
    <TimeZoneDropdown
      {id}
      {zones}
      {value}
      {label}
      {messages}
      required
      onchange={(v) => (value = v)}
    />
    <span class="hint">{hint}</span>
  </div>
{:else}
  <label class="field">
    <span>{label}</span>
    <input
      {id}
      class="input"
      bind:value
      required
      autocomplete="off"
      spellcheck="false"
      aria-describedby={`${id}-hint`}
    />
    <span class="hint" id={`${id}-hint`}>{hint}</span>
  </label>
{/if}
