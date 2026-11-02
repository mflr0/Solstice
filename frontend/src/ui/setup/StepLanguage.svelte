<script lang="ts">
  import { localeOptions, parseLocale, type Locale } from '../../i18n/locales.js';
  import type { SetupMessages } from '../../i18n/setup/index.js';
  import LanguageDropdown from '../LanguageDropdown.svelte';

  interface Props {
    m: SetupMessages;
    locale: Locale;
    onchoose: (l: Locale) => void;
    onnext: () => void;
  }
  const { m, locale, onchoose, onnext }: Props = $props();

  const options = $derived(localeOptions(locale));

  function submit(e: SubmitEvent) {
    e.preventDefault();
    onnext();
  }
</script>

<form onsubmit={submit}>
  <div>
    <h1 class="page-title" id="setup-step-title" tabindex="-1">{m.language.title}</h1>
    <p class="lead">{m.language.lead}</p>
  </div>
  <div class="field">
    <LanguageDropdown
      {options}
      value={locale}
      label={m.language.legend}
      messages={m.language}
      onchange={(v) => {
        const l = parseLocale(v);
        if (l) onchoose(l);
      }}
    />
  </div>
  <div class="setup-actions">
    <span></span>
    <button class="btn btn-primary">{m.next}</button>
  </div>
</form>

<style>
  .field {
    max-width: 20rem;
  }
</style>
