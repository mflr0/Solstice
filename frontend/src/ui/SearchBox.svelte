<script lang="ts">
  import type { SearchMode } from '../core/types.js';
  import { jqlExamples, searchExamples, t } from '../i18n/fr.js';
  import { outside } from '../lib/actions.js';

  /** The search bar with its syntax help; examples insert themselves. */
  interface Props {
    value: string;
    onsearch: (q: string) => void;
    placeholder?: string | undefined;
    autofocus?: boolean | undefined;
    /** How the last search was read; "jql" shows a small tag. */
    mode?: SearchMode | undefined;
  }
  let {
    value = $bindable(),
    onsearch,
    placeholder = t.search.placeholder,
    autofocus = false,
    mode = 'simple',
  }: Props = $props();

  let helpOpen = $state(false);
  let input: HTMLInputElement | undefined = $state();
  let timer: ReturnType<typeof setTimeout> | undefined;

  $effect(() => {
    if (autofocus) input?.focus();
  });

  function typed() {
    clearTimeout(timer);
    timer = setTimeout(() => onsearch(value.trim()), 350);
  }

  function submit(e: SubmitEvent) {
    e.preventDefault();
    clearTimeout(timer);
    onsearch(value.trim());
  }

  /** JQL cannot be mixed with the simple syntax: an example replaces the query. */
  function replace(q: string) {
    value = q;
    helpOpen = false;
    onsearch(value);
    input?.focus();
  }

  function insert(q: string) {
    value = value.trim() ? `${value.trim()} ${q}` : q;
    helpOpen = false;
    onsearch(value);
    input?.focus();
  }
</script>

<form class="search" role="search" onsubmit={submit}>
  <label class="sr-only" for="search-input">{t.search.title}</label>
  <span class="glass" aria-hidden="true">⌕</span>
  <input
    id="search-input"
    bind:this={input}
    bind:value
    oninput={typed}
    class="input"
    type="search"
    {placeholder}
    autocomplete="off"
    spellcheck="false"
    class:jql={mode === 'jql'}
  />
  {#if mode === 'jql'}
    <span class="mode" title={t.search.jqlModeTitle}>{t.search.jqlMode}</span>
  {/if}
  <div class="help-wrap" use:outside={() => (helpOpen = false)}>
    <button
      type="button"
      class="btn btn-quiet btn-small"
      aria-expanded={helpOpen}
      aria-controls="search-help"
      onclick={() => (helpOpen = !helpOpen)}>{t.search.help}</button
    >
    {#if helpOpen}
      <div class="popover help" id="search-help">
        <p class="intro"><strong>{t.search.helpTitle}.</strong> {t.search.helpIntro}</p>
        <ul>
          {#each searchExamples as ex (ex.q)}
            <li>
              <button type="button" class="item" onclick={() => insert(ex.q)}>
                <code>{ex.q}</code><span class="muted">{ex.label}</span>
              </button>
            </li>
          {/each}
        </ul>
        <div class="sep"></div>
        <p class="intro"><strong>{t.search.jqlTitle}.</strong> {t.search.jqlIntro}</p>
        <ul>
          {#each jqlExamples as ex (ex.q)}
            <li>
              <button type="button" class="item jql-ex" onclick={() => replace(ex.q)}>
                <code>{ex.q}</code><span class="muted">{ex.label}</span>
              </button>
            </li>
          {/each}
        </ul>
      </div>
    {/if}
  </div>
</form>

<style>
  .search {
    position: relative;
    display: flex;
    align-items: center;
    gap: 6px;
  }

  .glass {
    position: absolute;
    left: 9px;
    top: 50%;
    transform: translateY(-52%);
    color: var(--muted);
    font-size: 17px;
    pointer-events: none;
  }

  .input {
    padding-left: 28px;
    height: 34px;
    font-family: var(--mono);
    font-size: 13.5px;
  }

  .input.jql {
    padding-right: 48px;
  }

  .mode {
    position: absolute;
    right: 84px;
    top: 50%;
    transform: translateY(-50%);
    font: 600 10px/1 var(--mono);
    letter-spacing: 0.08em;
    color: var(--stamp-doing);
    border: 1px solid currentColor;
    padding: 3px 4px 2px;
    pointer-events: none;
  }

  .help-wrap {
    position: relative;
  }

  .help {
    right: 0;
    top: calc(100% + 6px);
    width: min(440px, 90vw);
    max-height: min(72vh, 680px);
    padding: 10px 0 6px;
  }

  .intro {
    margin: 0 14px 8px;
    font-size: 13px;
    color: var(--ink-soft);
  }

  ul {
    list-style: none;
    margin: 0;
    padding: 0;
  }

  .item {
    justify-content: space-between;
  }

  code {
    font-size: 12.5px;
    color: var(--ink);
  }

  .item .muted {
    font-size: 12.5px;
    text-align: right;
  }

  .jql-ex {
    display: grid;
    gap: 2px;
  }

  .jql-ex .muted {
    text-align: left;
  }
</style>
