<script lang="ts">
  import { LOCALE_NAMES, LOCALES, type Locale } from '../../i18n/locales.js';
  import type { SetupMessages } from '../../i18n/setup/index.js';

  interface Props {
    m: SetupMessages;
    locale: Locale;
    onchoose: (l: Locale) => void;
    onnext: () => void;
  }
  const { m, locale, onchoose, onnext }: Props = $props();

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
  <fieldset class="languages">
    <legend class="sr-only">{m.language.legend}</legend>
    {#each LOCALES as l (l)}
      <label class="language" class:on={l === locale}>
        <input
          type="radio"
          name="locale"
          value={l}
          checked={l === locale}
          onchange={() => onchoose(l)}
          lang={l}
        />
        <span lang={l}>{LOCALE_NAMES[l]}</span>
      </label>
    {/each}
  </fieldset>
  <div class="setup-actions">
    <span></span>
    <button class="btn btn-primary">{m.next}</button>
  </div>
</form>

<style>
  .languages {
    border: none;
    margin: 0;
    padding: 0;
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(9.375rem, 1fr));
    gap: 8px;
  }

  .language {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 10px 12px;
    border: var(--frame) solid var(--rule-strong);
    border-radius: var(--radius-s);
    background: var(--paper-raised);
    cursor: pointer;
    font-size: 0.9375rem;
  }

  .language:hover {
    border-color: var(--ink-soft);
  }

  .language.on {
    border-color: var(--accent);
    box-shadow: inset 0 0 0 1px var(--accent);
  }

  .language input {
    accent-color: var(--accent);
    margin: 0;
  }

  .language:has(input:focus-visible) {
    outline: 2px solid var(--focus);
    outline-offset: 2px;
  }
</style>
