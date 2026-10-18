<script lang="ts">
  import { onDestroy } from 'svelte';
  import { parseNotes } from '../../core/release-notes.js';
  import type { StatusCategory } from '../../core/types.js';
  import { formatDateTime, formatDay, relative, t } from '../../i18n/t.js';
  import { ApiError, errorMessage } from '../../lib/api.js';
  import { session } from '../../lib/session.svelte.js';
  import { notify } from '../../lib/ui.svelte.js';
  import {
    inProgress,
    updateNotice,
    updatesApi,
    type AttemptStatus,
    type UpdateState,
  } from '../../lib/updates.svelte.js';
  import Modal from '../Modal.svelte';
  import NotFoundPage from '../pages/NotFoundPage.svelte';

  /**
   * Solstice's version, the latest release on GitHub, and the update: in
   * place on a Linux service (the root helper does it), as commands to run
   * elsewhere. While an update runs, the page follows it through the
   * restart and reloads on the new version.
   */
  let info = $state<UpdateState | null>(null);
  let error = $state('');
  let checking = $state(false);
  let saving = $state(false);
  let confirming = $state(false);
  let password = $state('');
  let applyError = $state('');
  let applying = $state(false);
  /** The server went away (restarting): the page waits for it to answer again. */
  let restarting = $state(false);
  /** The version this page was loaded from: another one answering means reload. */
  let loadedVersion: string | null = null;
  let timer: ReturnType<typeof setTimeout> | undefined;

  const STAMPS: Record<AttemptStatus, StatusCategory> = {
    pending: 'todo',
    running: 'doing',
    ok: 'done',
    rolled_back: 'todo',
    failed: 'todo',
  };

  const notes = $derived(info?.newer ? parseNotes(info.notes) : []);
  const attempt = $derived(info?.lastAttempt ?? null);
  const isDev = $derived(!!info && !/^\d/.test(info.current));

  function show(s: UpdateState) {
    loadedVersion ??= s.current;
    if (s.current !== loadedVersion) {
      // Another version answers: its interface is the one to use.
      location.reload();
      return;
    }
    info = s;
    updateNotice.set(s);
    clearTimeout(timer);
    if (inProgress(s.lastAttempt)) timer = setTimeout(() => void follow(), 2000);
  }

  async function load() {
    try {
      show(await updatesApi.state());
      error = '';
    } catch (e) {
      error = errorMessage(e);
    }
  }

  /** Polls the attempt; an unreachable server is restarting. */
  async function follow() {
    try {
      const s = await updatesApi.state();
      if (restarting) {
        location.reload();
        return;
      }
      show(s);
    } catch (e) {
      if (e instanceof ApiError && (e.status === 0 || e.status >= 500)) {
        restarting = true;
        void waitHealthy();
      } else {
        error = errorMessage(e);
      }
    }
  }

  /** Waits until /healthz answers, then reloads the page. */
  async function waitHealthy() {
    try {
      const res = await fetch('/healthz', { cache: 'no-store' });
      if (res.ok) {
        location.reload();
        return;
      }
    } catch {
      // Not up yet.
    }
    clearTimeout(timer);
    timer = setTimeout(() => void waitHealthy(), 2000);
  }

  async function check() {
    checking = true;
    try {
      show(await updatesApi.check());
      error = '';
    } catch (e) {
      error = errorMessage(e);
    } finally {
      checking = false;
    }
  }

  async function toggleChecks(on: boolean) {
    saving = true;
    try {
      show(await updatesApi.setChecks(on));
      notify(on ? t.updates.autoOn : t.updates.autoOff);
    } catch (e) {
      notify(errorMessage(e), 'error');
    } finally {
      saving = false;
    }
  }

  function openConfirm() {
    password = '';
    applyError = '';
    confirming = true;
  }

  async function apply(e: SubmitEvent) {
    e.preventDefault();
    if (!info?.latest) return;
    applying = true;
    applyError = '';
    try {
      const s = await updatesApi.apply(info.latest, password);
      confirming = false;
      password = '';
      notify(t.updates.requested);
      show(s);
    } catch (err) {
      applyError = errorMessage(err);
    } finally {
      applying = false;
    }
  }

  $effect(() => {
    document.title = `${t.updates.title} — ${t.appName}`;
    if (session.me?.admin) void load();
  });

  onDestroy(() => clearTimeout(timer));
</script>

{#if !session.me?.admin}
  <NotFoundPage message={t.updates.forbidden} />
{:else}
  <div class="page updates-page" data-title={t.updates.title}>
    <header class="page-head">
      <div>
        <p class="eyebrow">{t.updates.eyebrow}</p>
        <h1 class="page-title">{t.updates.title}</h1>
      </div>
      <button type="button" class="btn" disabled={checking || !info} onclick={() => void check()}
        >{checking ? t.updates.checking : t.updates.checkNow}</button
      >
    </header>

    {#if error}<p class="error-text" role="alert">{error}</p>{/if}

    {#if restarting}
      <p class="muted loading" role="status">{t.updates.restarting}</p>
    {/if}

    {#if !info}
      {#if !error}<p class="muted loading">{t.common.loading}</p>{/if}
    {:else}
      <dl class="facts versions">
        <div>
          <dt>{t.updates.current}</dt>
          <dd>
            <span class="key">{info.current}</span>
            {#if isDev}<span class="muted">({t.updates.dev})</span>{/if}
          </dd>
        </div>
        <div>
          <dt>{t.updates.latest}</dt>
          <dd>
            {#if info.latest}
              <span class="key">{info.latest}</span>
              {#if info.publishedAt}
                <span class="muted"
                  >· {t.updates.published(formatDay(info.publishedAt.slice(0, 10)))}</span
                >
              {/if}
              ·
              <a href={info.releaseUrl} rel="noopener noreferrer" target="_blank"
                >{t.updates.releasePage}</a
              >
            {:else}
              <span class="muted">{t.updates.none}</span>
            {/if}
          </dd>
        </div>
        <div>
          <dt>{t.updates.install}</dt>
          <dd>{t.updates.installs[info.install]}</dd>
        </div>
      </dl>

      <p class="check-line muted">
        {info.checkedAt ? t.updates.checked(relative(info.checkedAt)) : t.updates.neverChecked}
      </p>
      {#if info.checkError}
        <p class="error-text" role="alert">{t.updates.checkErrors[info.checkError]}</p>
      {/if}

      {#if info.newer && info.latest}
        <p class="lead available">{t.updates.available(info.latest)}</p>
      {:else if info.latest && isDev}
        <p class="lead">{t.updates.devNote}</p>
      {:else if info.latest}
        <p class="lead">{t.updates.upToDate}</p>
      {/if}

      {#if attempt}
        <section class="attempt" aria-live="polite">
          <h2 class="section-title">{t.updates.last}</h2>
          <p class="attempt-line">
            <span class="stamp {STAMPS[attempt.status]}" data-category={STAMPS[attempt.status]}
              >{t.updates.statuses[attempt.status]}</span
            >
            <span class="key">{t.updates.fromTo(attempt.from, attempt.to)}</span>
            <span class="muted">{formatDateTime(attempt.at)}</span>
          </p>
          <p
            class:error-text={attempt.status === 'failed'}
            class:muted={attempt.status !== 'failed'}
          >
            {t.updates.statusHints[attempt.status]}
          </p>
          {#if attempt.log}
            <details open={attempt.status === 'failed' || attempt.status === 'rolled_back'}>
              <summary>{t.updates.log}</summary>
              <pre class="log">{attempt.log}</pre>
            </details>
          {/if}
        </section>
      {/if}

      {#if info.newer && info.latest}
        {#if info.install === 'systemd'}
          <div class="apply">
            <p class="muted hint-text">{t.updates.applyHint}</p>
            <button
              type="button"
              class="btn btn-primary"
              disabled={!info.canApply}
              onclick={openConfirm}>{t.updates.apply(info.latest)}</button
            >
          </div>
        {:else}
          <section class="install">
            <h2 class="section-title">{t.updates.apply(info.latest)}</h2>
            {#if info.install === 'docker'}
              <p>{t.updates.docker}</p>
              <pre class="commands">{info.commands.join('\n')}</pre>
              <p class="muted hint-text">{t.updates.dockerHint}</p>
            {:else if info.download.includes('-linux-')}
              <p>{t.updates.manualLinux}</p>
              <pre class="commands">{info.commands.join('\n')}</pre>
              <p class="muted hint-text">{t.updates.manualLinuxHint}</p>
            {:else}
              <p>{t.updates.manualOther}</p>
              {#if info.commands.length}<pre class="commands">{info.commands.join('\n')}</pre>{/if}
            {/if}
            {#if info.install === 'manual' && info.download}
              <p>
                <a class="btn" href={info.download} rel="noopener noreferrer"
                  >{t.updates.download}</a
                >
              </p>
            {/if}
          </section>
        {/if}

        <section class="notes">
          <h2 class="section-title">{t.updates.notes}</h2>
          {#if notes.length}
            <div class="wiki release-notes">
              {#each notes as block, i (i)}
                {#if block.kind === 'heading'}
                  <h3>{block.text}</h3>
                {:else if block.kind === 'paragraph'}
                  <p>{block.text}</p>
                {:else if block.kind === 'list' && block.ordered}
                  <ol>
                    {#each block.items as item, j (j)}<li>{item}</li>{/each}
                  </ol>
                {:else if block.kind === 'list'}
                  <ul>
                    {#each block.items as item, j (j)}<li>{item}</li>{/each}
                  </ul>
                {:else}
                  <pre>{block.text}</pre>
                {/if}
              {/each}
            </div>
          {:else}
            <p class="muted">{t.updates.noNotes}</p>
          {/if}
        </section>
      {/if}

      <section class="checks">
        <label class="check">
          <input
            class="checkbox"
            type="checkbox"
            checked={info.checksEnabled}
            disabled={saving}
            onchange={(e) => void toggleChecks(e.currentTarget.checked)}
          />
          {t.updates.auto}
        </label>
        <p class="hint muted">{t.updates.autoHint}</p>
      </section>
    {/if}
  </div>

  <Modal
    open={confirming}
    title={info?.latest ? t.updates.apply(info.latest) : t.updates.title}
    onclose={() => !applying && (confirming = false)}
  >
    <form id="update-confirm" class="form" onsubmit={apply}>
      <p>{t.updates.confirmBody}</p>
      <label class="field">
        <span>{t.updates.password}</span>
        <input
          class="input"
          type="password"
          bind:value={password}
          required
          autocomplete="current-password"
        />
        <span class="hint">{t.updates.passwordHint}</span>
      </label>
      {#if applyError}<p class="error-text" role="alert">{applyError}</p>{/if}
    </form>
    {#snippet footer()}
      <button
        type="button"
        class="btn btn-quiet"
        disabled={applying}
        onclick={() => (confirming = false)}>{t.common.cancel}</button
      >
      <button class="btn btn-primary" type="submit" form="update-confirm" disabled={applying}
        >{t.updates.start}</button
      >
    {/snippet}
  </Modal>
{/if}

<style>
  .versions {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
    gap: 12px 28px;
    margin: 4px 0 8px;
  }

  .versions dt {
    font-size: 12.5px;
    color: var(--muted);
  }

  .versions dd {
    margin: 2px 0 0;
    overflow-wrap: anywhere;
  }

  .check-line {
    margin: 0 0 8px;
    font-size: 13px;
  }

  .lead {
    margin: 12px 0 4px;
  }

  .available {
    font-weight: 600;
  }

  section {
    margin-top: 26px;
  }

  .attempt-line {
    display: flex;
    flex-wrap: wrap;
    gap: 6px 12px;
    align-items: center;
    margin: 0 0 6px;
  }

  .hint-text {
    max-width: 70ch;
    font-size: 13px;
  }

  .apply {
    margin-top: 14px;
  }

  .checks {
    padding-top: 16px;
    border-top: 1px solid var(--rule);
  }

  pre.commands,
  pre.log {
    margin: 8px 0;
    padding: 10px 12px;
    overflow-x: auto;
    border: var(--frame) solid var(--rule);
    border-radius: var(--radius-s);
    background: var(--paper-sunk);
    color: var(--ink);
    font: 12.5px/1.5 var(--font-mono);
    white-space: pre;
  }

  pre.log {
    max-height: 320px;
    overflow-y: auto;
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }

  details summary {
    cursor: pointer;
    color: var(--ink-soft);
  }

  .release-notes {
    max-width: 75ch;
  }

  .check {
    display: flex;
    gap: 8px;
    align-items: center;
  }

  .checks .hint {
    margin: 6px 0 0;
    max-width: 70ch;
    font-size: 13px;
  }

  .form {
    display: grid;
    gap: 12px;
  }

  .form p {
    margin: 0;
  }
</style>
