<script lang="ts">
  import { timeZones } from '../../core/setup.js';

  /** A time zone, typed or picked among the browser's IANA names. */
  interface Props {
    label: string;
    hint: string;
    value: string;
    id: string;
  }
  let { label, hint, value = $bindable(), id }: Props = $props();

  const zones = timeZones();
</script>

<label class="field">
  <span>{label}</span>
  <input
    class="input"
    bind:value
    list={`${id}-zones`}
    required
    autocomplete="off"
    spellcheck="false"
    aria-describedby={`${id}-hint`}
  />
  <span class="hint" id={`${id}-hint`}>{hint}</span>
</label>
<datalist id={`${id}-zones`}>
  {#each zones as z (z)}<option value={z}></option>{/each}
</datalist>
