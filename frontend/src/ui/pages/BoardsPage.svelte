<script lang="ts">
  import type { BoardKind, BoardSummary } from '../../core/board-types.js';
  import { paths } from '../../core/route.js';
  import type { Project } from '../../core/types.js';
  import { t } from '../../i18n/t.js';
  import { api, ApiError, errorMessage } from '../../lib/api.js';
  import { router } from '../../lib/router.svelte.js';
  import { notify } from '../../lib/ui.svelte.js';
  import Illustration from '../Illustration.svelte';
  import Modal from '../Modal.svelte';
  import Who from '../Who.svelte';

  /** Every board one may see, optionally for one project (?project=KEY). */
  let boards = $state<BoardSummary[] | null>(null);
  let error = $state('');
  const project = $derived(router.param('project')?.toUpperCase() ?? '');

  $effect(() => {
    document.title = `${t.boards.title} — ${t.appName}`;
    api
      .boards()
      .then((b) => (boards = b))
      .catch((e: unknown) => (error = errorMessage(e)));
  });

  /** Projects the boards cover, for the filter. */
  const projectKeys = $derived(
    [...new Set((boards ?? []).flatMap((b) => b.projects.map((p) => p.key)))].sort(),
  );
  const shown = $derived(
    (boards ?? []).filter((b) => !project || b.projects.some((p) => p.key === project)),
  );

  let creating = $state(false);
  let projects = $state<Project[] | null>(null);
  let form = $state<{ name: string; kind: BoardKind; project: string }>({
    name: '',
    kind: 'kanban',
    project: '',
  });
  let canCreate = $state<boolean | null>(null);
  let formError = $state('');
  let busy = $state(false);

  function openCreate() {
    creating = true;
    formError = '';
    form = { name: '', kind: 'kanban', project: project || form.project };
    if (!projects) {
      api
        .projects()
        .then((p) => {
          projects = p;
          if (!form.project) form.project = p[0]?.key ?? '';
        })
        .catch((e: unknown) => (formError = errorMessage(e)));
    }
  }

  // Only a project's administrators may give it a board: say so before they try.
  $effect(() => {
    const key = form.project;
    canCreate = null;
    if (!creating || !key) return;
    api
      .project(key)
      .then((d) => {
        if (form.project === key) canCreate = !!d.can.configure;
      })
      .catch(() => {
        if (form.project === key) canCreate = null;
      });
  });

  async function create(e: SubmitEvent) {
    e.preventDefault();
    if (busy || !form.project) return;
    busy = true;
    formError = '';
    try {
      const name = form.name.trim();
      const { id } = await api.createBoard({
        kind: form.kind,
        project: form.project,
        ...(name ? { name } : {}),
      });
      notify(t.boards.created);
      creating = false;
      router.navigate(paths.board(id));
    } catch (err) {
      formError =
        err instanceof ApiError && err.status === 403 ? t.boards.notAdmin : errorMessage(err);
    } finally {
      busy = false;
    }
  }
</script>

<div class="page" data-title={t.boards.title}>
  <header class="page-head">
    <h1 class="page-title">{t.boards.title}</h1>
    <button class="btn btn-primary" onclick={openCreate}>{t.boards.create}</button>
  </header>

  {#if error}
    <p class="error-text" role="alert">{error}</p>
  {:else if !boards}
    <p class="muted loading">{t.common.loading}</p>
  {:else}
    {#if projectKeys.length > 1 || project}
      <label class="field filter">
        <span>{t.boards.project}</span>
        <select
          class="select"
          value={project}
          onchange={(e) => router.setParams({ project: e.currentTarget.value || null })}
        >
          <option value="">{t.boards.allProjects}</option>
          {#each projectKeys as key (key)}
            <option value={key}>{key}</option>
          {/each}
          {#if project && !projectKeys.includes(project)}
            <option value={project}>{project}</option>
          {/if}
        </select>
      </label>
    {/if}

    {#if shown.length === 0}
      <Illustration kind="empty" />
      <p class="empty">{project ? t.boards.emptyProject(project) : t.boards.empty}</p>
    {:else}
      <h2 class="section-title">
        {project || t.boards.allProjects} <span class="count">{t.boards.count(shown.length)}</span>
      </h2>
      <div class="scroll">
        <table class="ledger">
          <thead>
            <tr>
              <th class="c-name">{t.boards.name}</th>
              <th>{t.boards.kind}</th>
              <th>{t.boards.projects}</th>
              <th>{t.boards.owner}</th>
              <th>{t.boards.activeSprint}</th>
            </tr>
          </thead>
          <tbody>
            {#each shown as b (b.id)}
              <tr
                onclick={(e) =>
                  !(e.target as Element).closest('a') && router.navigate(paths.board(b.id))}
              >
                <td class="c-name"><a class="name" href={paths.board(b.id)}>{b.name}</a></td>
                <td>{t.boards.kinds[b.kind]}</td>
                <td class="keys">
                  {#each b.projects as p (p.id)}
                    <a class="key" href={paths.project(p.key)} title={p.name}>{p.key}</a>
                  {/each}
                </td>
                <td><Who user={b.owner} empty="—" /></td>
                <td>
                  {#if b.activeSprint}{b.activeSprint.name}{:else}<span class="muted">—</span>{/if}
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}
  {/if}
</div>

<Modal open={creating} title={t.boards.createTitle} onclose={() => (creating = false)}>
  <form id="new-board" class="form" onsubmit={create}>
    <p class="help">{t.boards.createHelp}</p>
    <label class="field">
      <span>{t.boards.project}</span>
      {#if projects && projects.length === 0}
        <span class="muted">{t.boards.noProject}</span>
      {:else}
        <select class="select" bind:value={form.project} required>
          {#each projects ?? [] as p (p.id)}
            <option value={p.key}>{p.key} — {p.name}</option>
          {/each}
        </select>
      {/if}
    </label>
    <fieldset class="kinds">
      <legend class="field-label">{t.boards.kind}</legend>
      {#each ['kanban', 'scrum'] as const as kind (kind)}
        <label class="kind">
          <input type="radio" name="kind" value={kind} bind:group={form.kind} />
          <span>
            <strong>{t.boards.kinds[kind]}</strong>
            <span class="hint">{t.boards.kindHints[kind]}</span>
          </span>
        </label>
      {/each}
    </fieldset>
    <label class="field">
      <span>{t.boards.name}</span>
      <input class="input" bind:value={form.name} maxlength="255" autocomplete="off" />
      <span class="hint"
        >{t.boards.createNameHint(`${form.project || 'DEMO'} — ${t.boards.kinds[form.kind]}`)}</span
      >
    </label>
    {#if canCreate === false}<p class="error-text">{t.boards.notAdmin}</p>{/if}
    {#if formError}<p class="error-text" role="alert">{formError}</p>{/if}
  </form>
  {#snippet footer()}
    <button type="button" class="btn btn-quiet" onclick={() => (creating = false)}
      >{t.common.cancel}</button
    >
    <button
      type="submit"
      class="btn btn-primary"
      form="new-board"
      disabled={busy || !form.project || canCreate === false}
      >{busy ? t.boards.creating : t.boards.createSubmit}</button
    >
  {/snippet}
</Modal>

<style>
  .filter {
    max-width: 260px;
    margin-bottom: 4px;
  }

  .scroll {
    overflow-x: auto;
  }

  tbody tr {
    cursor: pointer;
  }

  .c-name {
    width: 40%;
  }

  .name {
    font: var(--title-weight) 15px/1.3 var(--font-title);
    text-decoration: none;
  }

  .name:hover {
    text-decoration: underline;
  }

  .keys {
    white-space: nowrap;
  }

  .keys .key + .key {
    margin-left: 8px;
  }

  .form {
    display: grid;
    gap: 14px;
  }

  .help {
    margin: 0;
    color: var(--ink-soft);
  }

  .kinds {
    display: grid;
    gap: 8px;
    margin: 0;
    padding: 0;
    border: none;
  }

  .kinds legend {
    padding: 0;
    margin-bottom: 6px;
  }

  .kind {
    display: flex;
    gap: 10px;
    align-items: baseline;
    cursor: pointer;
  }

  .kind input {
    accent-color: var(--accent);
  }

  .kind > span {
    display: grid;
    gap: 2px;
  }

  .kind .hint {
    font-size: 12.5px;
    color: var(--muted);
  }
</style>
