<script lang="ts">
  import type { ImportSource } from '../../core/admin-types.js';
  import type { SetupMessages } from '../../i18n/setup/index.js';
  import TimeZoneField from './TimeZoneField.svelte';

  /**
   * Where to import from: the Jira database, its time zone and, optionally,
   * its home folder (for the attachment files). Shared by the setup wizard
   * and the administration's import page.
   */
  interface Props {
    m: SetupMessages;
    form: ImportSource;
  }
  let { m, form = $bindable() }: Props = $props();
</script>

<label class="field">
  <span>{m.import.url}</span>
  <input
    class="input mono"
    bind:value={form.url}
    required
    autocomplete="off"
    spellcheck="false"
    placeholder="postgres://jira:…@db-jira:5432/jira"
    aria-describedby="source-url-hint"
  />
  <span class="hint" id="source-url-hint">{m.import.urlHint}</span>
</label>
<TimeZoneField label={m.import.tz} hint={m.import.tzHint} bind:value={form.tz} id="source-tz" />
<label class="field">
  <span>{m.import.home}</span>
  <input
    class="input mono"
    bind:value={form.jiraHome}
    autocomplete="off"
    spellcheck="false"
    placeholder="/var/atlassian/application-data/jira"
    aria-describedby="source-home-hint"
  />
  <span class="hint" id="source-home-hint">{m.import.homeHint}</span>
</label>
