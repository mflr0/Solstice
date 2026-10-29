<script lang="ts">
  import { writeLocation } from '../../core/filters-url.js';
  import { quote } from '../../core/jql.js';
  import { paths } from '../../core/route.js';
  import type { ProjectDetail } from '../../core/types.js';
  import { formatDay, t } from '../../i18n/t.js';
  import { api, errorMessage } from '../../lib/api.js';
  import { router } from '../../lib/router.svelte.js';
  import { openCreate } from '../../lib/ui.svelte.js';
  import IssueSearch from '../search/IssueSearch.svelte';
  import Who from '../Who.svelte';
  import NotFoundPage from './NotFoundPage.svelte';

  /** A project: its header, then its issues as a ledger with their own search. */
  interface Props {
    projectKey: string;
  }
  const { projectKey }: Props = $props();

  /** Without a search in the address, the open issues. */
  const DEFAULT_JQL = 'statusCategory != Done';
  let detail = $state<ProjectDetail | null>(null);
  let error = $state('');

  /** This page, its list filtered by a field's value (a version, a component). */
  function filterHref(key: string, field: string, value: string): string {
    const jql = `${field} = ${quote(value)}`;
    return paths.project(key) + writeLocation({ jql, mode: null, filter: null, page: 1 });
  }

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
</script>

{#if error}
  <NotFoundPage message={error} />
{:else if !detail}
  <div class="page"><p class="muted loading">{t.common.loading}</p></div>
{:else}
  {@const p = detail.project}
  <div class="page" data-title={p.name}>
    <header class="page-head">
      <div>
        <p class="eyebrow">
          {t.projects.title}{#if p.category}&nbsp;· {p.category}{/if} · {t.projects.kinds[p.kind] ??
            p.kind}
        </p>
        <h1 class="page-title"><span class="pkey">{p.key}</span> {p.name}</h1>
        {#if p.lead}<p class="lead">
            {t.projects.lead}{t.common.labelSep}<Who user={p.lead} />
          </p>{/if}
      </div>
      <div class="row">
        <a class="btn" href={paths.boards(p.key)}>{t.boards.title}</a>
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
                  href={filterHref(p.key, 'fixVersion', v.name)}
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
                <a class="chip" href={filterHref(p.key, 'component', c.name)}>{c.name}</a>
              {/each}
            </dd>
          </div>
        {/if}
      </dl>
    {/if}

    <IssueSearch project={p.key} fallback={DEFAULT_JQL} />
  </div>
{/if}

<style>
  .pkey {
    font: 500 0.62em/1 var(--font-mono);
    color: var(--muted);
    vertical-align: 0.2em;
    margin-right: 4px;
  }

  .lead {
    display: flex;
    gap: 6px;
    align-items: center;
    margin: 8px 0 0;
    font-size: 0.8125rem;
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
    font: 500 0.75rem/1 var(--font-body);
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
</style>
