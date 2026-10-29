<script lang="ts">
  import { onMount, tick } from 'svelte';
  import { setupSteps, type SetupStep } from '../../core/setup.js';
  import { LOCALE_STORAGE_KEY, type Locale } from '../../i18n/locales.js';
  import { setupCatalogues } from '../../i18n/setup/index.js';
  import {
    configureSetupApi,
    setupApi,
    setupErrorMessage,
    type ConfigSave,
    type SetupFinished,
    type SetupStatus,
  } from '../../lib/setup-api.js';
  import { initAppearance } from '../../lib/ui.svelte.js';
  import Logo from '../Logo.svelte';
  import SourceLinks from '../SourceLinks.svelte';
  import { insecurePage } from './insecure.js';
  import StepCode from './StepCode.svelte';
  import StepDatabase from './StepDatabase.svelte';
  import StepDone from './StepDone.svelte';
  import StepInstance from './StepInstance.svelte';
  import StepLanguage from './StepLanguage.svelte';
  import StepStart from './StepStart.svelte';

  /** The first-run wizard: its own page, its own texts, before any account exists. */
  interface Props {
    initialLocale: Locale;
  }
  const { initialLocale }: Props = $props();

  // The prop only seeds it: the language step changes it in place.
  // svelte-ignore state_referenced_locally
  let locale = $state<Locale>(initialLocale);
  const m = $derived(setupCatalogues[locale]);
  let status = $state<SetupStatus | null>(null);
  let loadError = $state('');
  let step = $state<SetupStep>('language');
  /** Whether the database step belongs to this run (decided when the code is given). */
  let needDatabase = $state(true);
  /** The last word on the configuration file, for the summary. */
  let lastConfig = $state<ConfigSave | null>(null);
  let finished = $state<SetupFinished | null>(null);
  /** The database held an instance already: nothing more to set. */
  let existing = $state(false);

  const steps = $derived(setupSteps(needDatabase));
  const index = $derived(Math.max(0, steps.indexOf(step)));
  /** The passwords typed here cross the network readable: say so. */
  const insecure = $derived(insecurePage(location.protocol, location.hostname));
  const num = $derived.by(() => {
    const nf = new Intl.NumberFormat(locale);
    return (n: number) => nf.format(n);
  });

  $effect(() => {
    configureSetupApi(locale, m.errors.network, m.errors.generic);
    document.title = `${m.steps[step]} — ${m.documentTitle}`;
    document.documentElement.lang = locale;
    if (locale === 'hi') void import('../../i18n/devanagari.css');
  });

  onMount(() => {
    const stop = initAppearance();
    void load();
    return stop;
  });

  async function load() {
    loadError = '';
    try {
      status = await setupApi.status();
      needDatabase = !status.database?.configured;
      if (status.unlocked) resume(status);
    } catch (e) {
      loadError = setupErrorMessage(e, m.errors.generic);
    }
  }

  /** Back in an unlocked browser (a reload): go on where it makes sense. */
  function resume(s: SetupStatus) {
    needDatabase = !s.database?.configured;
    lastConfig = s.config ?? null;
    step = s.import ? 'start' : s.database?.configured ? 'instance' : 'database';
  }

  async function go(next: SetupStep) {
    step = next;
    await tick();
    document.getElementById('setup-step-title')?.focus();
  }

  function chooseLocale(l: Locale) {
    locale = l;
    try {
      localStorage.setItem(LOCALE_STORAGE_KEY, l);
    } catch {
      // Private browsing: the choice lasts for this page.
    }
  }

  async function unlocked() {
    status = await setupApi.status();
    needDatabase = !status.database?.configured;
    void go(needDatabase ? 'database' : 'instance');
  }

  async function databaseSaved(config: ConfigSave, wasExisting: boolean) {
    lastConfig = config;
    status = await setupApi.status();
    if (wasExisting) {
      existing = true;
      void go('done');
    } else {
      void go('instance');
    }
  }

  function instanceSaved(config: ConfigSave) {
    lastConfig = config;
    void go('start');
  }

  function done(result: SetupFinished) {
    finished = result;
    if (result.config) lastConfig = result.config;
    void go('done');
  }

  /** The code, once given, is not asked again. */
  function afterLanguage(): SetupStep {
    if (!status?.unlocked) return 'code';
    return needDatabase ? 'database' : 'instance';
  }

  function back() {
    const prev = steps[steps.indexOf(step) - 1];
    if (prev) void go(prev === 'code' && status?.unlocked ? 'language' : prev);
  }
</script>

<div class="setup">
  <main class="card setup-sheet">
    <div class="setup-inner">
      <header class="setup-head">
        <div class="brand">
          <Logo size={28} />
          <span class="wordmark">{m.appName}</span>
        </div>
        <span class="eyebrow">{m.eyebrow}</span>
      </header>

      {#if insecure}
        <p class="setup-warn insecure" role="alert">{m.insecureWarning}</p>
      {/if}

      {#if loadError}
        <p class="error-text" role="alert">{loadError}</p>
        <div class="setup-actions">
          <span></span>
          <button class="btn btn-primary" type="button" onclick={load}>{m.retry}</button>
        </div>
      {:else if !status}
        <p class="muted loading setup-loading">{m.loading}</p>
      {:else if !status.setup && step !== 'done'}
        <h1 class="page-title" id="setup-step-title" tabindex="-1">{m.installed.title}</h1>
        <p class="lead">{m.installed.lead}</p>
        <div class="setup-actions">
          <span></span>
          <a class="btn btn-primary" href="/login">{m.installed.signIn}</a>
        </div>
      {:else}
        <nav class="setup-steps" aria-label={m.stepsLabel}>
          <ol>
            {#each steps as s, i (s)}
              <li
                class:is-done={i < index}
                class:is-current={i === index}
                aria-current={i === index ? 'step' : undefined}
              >
                <span class="num" aria-hidden="true">{i < index ? '✓' : i + 1}</span>
                <span class="label">{m.steps[s]}</span>
                {#if i < index}<span class="sr-only">({m.stepDone})</span>{/if}
              </li>
            {/each}
          </ol>
        </nav>

        <p class="counter">{m.stepOf(index + 1, steps.length)}</p>
        {#if step === 'language'}
          <StepLanguage {m} {locale} onchoose={chooseLocale} onnext={() => go(afterLanguage())} />
        {:else if step === 'code'}
          <StepCode {m} onunlocked={unlocked} onback={back} />
        {:else if step === 'database'}
          <StepDatabase {m} onsaved={databaseSaved} onback={back} />
        {:else if step === 'instance'}
          <StepInstance {m} {status} onsaved={instanceSaved} onback={back} />
        {:else if step === 'start'}
          <StepStart {m} {num} {status} ondone={done} onback={back} />
        {:else}
          <StepDone {m} {status} {finished} {existing} config={lastConfig} />
        {/if}
      {/if}

      <footer class="setup-foot">
        <SourceLinks source={m.sourceCode} licenses={m.licenses} />
        {#if status?.version}<span>{m.version(status.version)}</span>{/if}
      </footer>
    </div>
  </main>
</div>

<style>
  .setup {
    min-height: 100vh;
    max-width: 48.75rem;
    margin: 0 auto;
    padding: 32px 20px 40px;
  }

  .setup-sheet {
    box-shadow: var(--shadow-pop);
  }

  /* The padding lives inside, so skins that reshape .card keep it. */
  .setup-inner {
    padding: 22px 30px 18px;
  }

  .setup-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    padding-bottom: 14px;
    border-bottom: 1px solid var(--rule);
  }

  .brand {
    display: flex;
    align-items: center;
    gap: 10px;
  }

  .wordmark {
    font: var(--title-weight) 1.625rem/1 var(--font-title);
    letter-spacing: -0.015em;
  }

  .setup-head .eyebrow {
    margin: 0;
  }

  .setup-warn.insecure {
    margin: 14px 0 0;
  }

  .setup-loading {
    padding: 40px 0;
    text-align: center;
  }

  /* The steps: a numbered row, like the tabs of a logbook. */
  .setup-steps {
    padding: 12px 0 14px;
    margin-bottom: 18px;
    border-bottom: 1px solid var(--rule);
  }

  .setup-steps ol {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-wrap: wrap;
    gap: 6px 18px;
  }

  .setup-steps li {
    display: flex;
    align-items: center;
    gap: 7px;
    color: var(--muted);
    font-size: 0.8125rem;
  }

  .setup-steps .num {
    display: inline-grid;
    place-items: center;
    min-width: 1.375rem;
    height: 1.375rem;
    padding: 0 4px;
    border: var(--frame) solid var(--rule-strong);
    border-radius: var(--radius-xs);
    font: 500 0.71875rem/1 var(--font-mono);
  }

  .setup-steps li.is-done .num {
    background: var(--paper-sunk);
    color: var(--ink-soft);
  }

  .setup-steps li.is-current {
    color: var(--ink);
    font-weight: 600;
  }

  .setup-steps li.is-current .num {
    border-color: var(--accent);
    color: var(--accent-text);
  }

  .counter {
    margin: 0 0 6px;
    font: 500 0.6875rem/1.2 var(--font-body);
    text-transform: uppercase;
    letter-spacing: 0.12em;
    color: var(--muted);
  }

  .setup-foot {
    margin-top: 18px;
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    justify-content: space-between;
    gap: 4px 12px;
    font: 0.71875rem var(--font-mono);
    color: var(--muted);
  }

  @media (max-width: 600px) {
    .setup {
      padding: 12px 10px 24px;
    }

    .setup-inner {
      padding: 16px 16px 14px;
    }

    .setup-steps li:not(.is-current) .label {
      position: absolute;
      width: 1px;
      height: 1px;
      overflow: hidden;
      clip: rect(0 0 0 0);
      white-space: nowrap;
    }
  }
</style>
