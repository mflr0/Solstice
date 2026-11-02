<script lang="ts">
  import { parseLocale } from '../i18n/locales.js';
  import Dropdown, { type DropdownMessages, type DropdownOption } from './Dropdown.svelte';

  /**
   * A language chooser: the searchable Dropdown, each language behind its
   * code badge. Build the options with localeOptions() (src/i18n/locales.ts);
   * an option that is not a language (Follow the browser) has no badge.
   */
  interface Props {
    options: DropdownOption[];
    value: string;
    label: string;
    onchange: (value: string) => void;
    id?: string | undefined;
    disabled?: boolean | undefined;
    messages?: DropdownMessages | undefined;
  }
  const { options, value, label, onchange, id, disabled, messages }: Props = $props();
</script>

<Dropdown {options} {value} {label} {onchange} {id} {disabled} {messages} search>
  {#snippet icon(v)}
    {#if parseLocale(v)}<span class="chip code">{v.toUpperCase()}</span>{/if}
  {/snippet}
</Dropdown>

<style>
  /* The same width for every code, so the names line up. */
  .code {
    justify-content: center;
    min-width: 2.25rem;
    line-height: 1.25;
  }
</style>
