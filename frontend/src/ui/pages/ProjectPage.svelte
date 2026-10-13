<script lang="ts">
  import { paths } from '../../core/route.js';
  import type { ProjectDetail, SearchMode } from '../../core/types.js';
  import { formatDay, t } from '../../i18n/fr.js';
  import { api, errorMessage } from '../../lib/api.js';
  import { router } from '../../lib/router.svelte.js';
  import { openCreate } from '../../lib/ui.svelte.js';
  import IssueQuery from '../IssueQuery.svelte';
  import SearchBox from '../SearchBox.svelte';
  import Who from '../Who.svelte';
  import NotFoundPage from './NotFoundPage.svelte';

  /** A project: its header, then its issues as a ledger with their own search. */
  interface Props {
    projectKey: string;
  }
  const { projectKey }: Props = $props();

  const DEFAULT_Q = 'est:ouvert';
  let detail = $state<ProjectDetail | null>(null);
  let error = $state('');
  let total = $state(0);
  let mode = $state<SearchMode>('simple');
  const q = $derived(router.param('q') ?? DEFAULT_Q);
  const page = $derived(Number(router.param('page') ?? '1') || 1);
  let draft = $derived(q);

  $effect(() => {
    const key = projectKey;
    detail = null;
    error = '';
    api
      .project(key)
      .then((d) => {
        if (key !== projectKey) return;
        detail = d;
        document.title = `${d.project.name} — ${t.appName}`;
        if (d.project.key !== key) router.navigate(paths.project(d.project.key), { replace: true });
      })
      .catch((e: unknown) => (error = errorMessage(e)));
  });

  function search(v: string) {
    router.setParams({ q: v === DEFAULT_Q ? null : v, page: null });
  }
</script>

{#if error}
  <NotFoundPage message={error} />
{:else if !detail}
  <div class="page"><p class="muted">{t.common.loading}</p></div>
{:else}
  {@const p = detail.project}
  <div class="page">
    <header class="page-head">
      <div>
        <p class="eyebrow">
          {t.projects.title}{#if p.category}&nbsp;· {p.category}{/if} · {t.projects.kinds[p.kind] ??
            p.kind}
        </p>
        <h1><span class="pkey">{p.key}</span> {p.name}</h1>
        {#if p.lead}<p class="lead">{t.projects.lead} : <Who user={p.lead} /></p>{/if}
      </div>
      <div class="row">
        {#if detail.can.configure}
          <a class="btn" href={paths.projectSettings(p.key)}>{t.projects.settings}</a>
        {/if}
        {#if detail.can.create}
          <button class="btn btn-primary" onclick={() => openCreate({ project: p.key })}
            >{t.nav.create}</button
          >
        {/if}
      </div>
    </header>

    {#if detail.versions.length || detail.components.length}
      <dl class="facts">
        {#if detail.versions.length}
          <div>
            <dt>{t.projects.versions}</dt>
            <dd>
              {#each detail.versions.filter((v) => !v.archived) as v (v.id)}
                <a
                  class="chip"
                  class:released={v.released}
                  href={paths.search(`projet:${p.key} version:"${v.name}"`)}
                  title={v.releaseDate ? formatDay(v.releaseDate) : ''}
                  >{v.name}{#if v.released}<span class="sr-only">
                      ({t.projects.released})</span
                    >{/if}</a
                >
              {/each}
            </dd>
          </div>
        {/if}
        {#if detail.components.length}
          <div>
            <dt>{t.projects.components}</dt>
            <dd>
              {#each detail.components.filter((c) => !c.archived) as c (c.id)}
                <a class="chip" href={paths.search(`projet:${p.key} composant:"${c.name}"`)}
                  >{c.name}</a
                >
              {/each}
            </dd>
          </div>
        {/if}
      </dl>
    {/if}

    <div class="tools">
      <div class="grow"><SearchBox bind:value={draft} onsearch={search} {mode} /></div>
      <div class="quick" role="group" aria-label={t.projects.filters}>
        {#each t.projects.quick as f (f.q)}
          <button class="btn btn-small" class:on={q === f.q} onclick={() => search(f.q)}
            >{f.label}</button
          >
        {/each}
      </div>
      <span class="count muted">{t.search.results(total)}</span>
    </div>

    <IssueQuery
      {q}
      project={p.key}
      {page}
      ontotal={(n) => (total = n)}
      onmode={(m) => (mode = m)}
      onpage={(n) => router.setParams({ page: String(n) }, { replace: false })}
    />
  </div>
{/if}

<style>
  .pkey {
    font: 500 0.62em/1 var(--mono);
    color: var(--muted);
    vertical-align: 0.2em;
    margin-right: 4px;
  }

  .lead {
    display: flex;
    gap: 6px;
    align-items: center;
    margin: 8px 0 0;
    font-size: 13px;
    color: var(--ink-soft);
  }

  .facts {
    display: flex;
    flex-wrap: wrap;
    gap: 10px 40px;
    margin: 0 0 18px;
  }

  .facts div {
    display: flex;
    gap: 10px;
    align-items: baseline;
  }

  dt {
    font: 500 12px/1 var(--sans);
    color: var(--muted);
  }

  dd {
    margin: 0;
    display: flex;
    flex-wrap: wrap;
    gap: 4px;
  }

  .chip {
    text-decoration: none;
  }

  .chip:hover {
    border-color: var(--ink-soft);
  }

  .chip.released {
    color: var(--muted);
    text-decoration: line-through;
    text-decoration-color: var(--rule-strong);
  }

  .tools {
    display: flex;
    flex-wrap: wrap;
    gap: 10px 16px;
    align-items: center;
    margin-bottom: 16px;
  }

  .grow {
    flex: 1;
    min-width: 280px;
  }

  .quick {
    display: flex;
    gap: 4px;
  }

  .count {
    font: 12.5px var(--mono);
  }
</style>
