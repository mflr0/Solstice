<script lang="ts">
  /** A time zone, typed or picked among the browser's IANA names. */
  interface Props {
    label: string;
    hint: string;
    value: string;
    id: string;
  }
  let { label, hint, value = $bindable(), id }: Props = $props();

  const zones: string[] = (() => {
    try {
      return Intl.supportedValuesOf('timeZone');
    } catch {
      return [];
    }
  })();
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
