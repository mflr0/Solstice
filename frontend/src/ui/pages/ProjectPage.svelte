<script lang="ts">
  import { projectViewJql, searchHref, writeLocation } from '../../core/filters-url.js';
  import { quote } from '../../core/jql.js';
  import { paths } from '../../core/route.js';
  import type { ProjectDetail } from '../../core/types.js';
  import { formatDay, t } from '../../i18n/t.js';
  import { api, errorMessage } from '../../lib/api.js';
  import { debounce, live } from '../../lib/live.svelte.js';
  import { router } from '../../lib/router.svelte.js';
  import { notify, openCreate } from '../../lib/ui.svelte.js';
  import { ICONS } from '../editor/icons.js';
  import EmptyState from '../EmptyState.svelte';
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
  /** Whether the project holds any issue, whatever the search; null until known. */
  let hasIssues = $state<boolean | null>(null);

  /** This page, its list filtered by a field's value (a version, a component). */
  function filterHref(key: string, field: string, value: string): string {
    const jql = `${field} = ${quote(value)}`;
    return paths.project(key) + writeLocation({ jql, mode: null, filter: null, page: 1 });
  }

  /** Plain http has no navigator.clipboard: copy from a throwaway field instead. */
  function copyBySelection(text: string): boolean {
    const focused = document.activeElement;
    const field = document.createElement('textarea');
    field.value = text;
    field.style.position = 'fixed';
    field.style.opacity = '0';
    document.body.append(field);
    field.select();
    const copied = document.execCommand('copy');
    field.remove();
    if (focused instanceof HTMLElement) focused.focus();
    return copied;
  }

  async function copyLink(key: string) {
    const url = `${location.origin}${paths.project(key)}`;
    try {
      await navigator.clipboard.writeText(url);
    } catch {
      if (!copyBySelection(url)) {
        notify(t.projects.copyLinkFailed(url), 'error');
        return;
      }
    }
    notify(t.projects.linkCopied);
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

  $effect(() => {
    const key = projectKey;
    hasIssues = null;
    const probe = () =>
      api
        .issues({ project: key, size: 1 })
        .then((r) => {
          if (key === projectKey) hasIssues = r.total > 0;
        })
        // The search says what went wrong.
        .catch(() => {
          if (key === projectKey) hasIssues = true;
        });
    void probe();
    return live.onChange(
      debounce(() => {
        if (hasIssues === false) void probe();
      }, 700),
    );
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
        <button
          type="button"
          class="btn btn-icon"
          aria-label={t.projects.copyLink}
          title={t.projects.copyLink}
          onclick={() => void copyLink(p.key)}
        >
          <svg viewBox="0 0 16 16" width="16" height="16" aria-hidden="true">
            {#each ICONS.link as d, i (i)}<path {d} />{/each}
          </svg>
        </button>
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

    {#if hasIssues === null}
      <p class="muted loading">{t.common.loading}</p>
    {:else if !hasIssues}
      <EmptyState message={t.projects.noIssues} hint={t.projects.noIssuesHint}>
        {#if detail.can.create}
          <button class="btn btn-primary" onclick={() => openCreate({ project: p.key })}
            >{t.nav.create}</button
          >
        {/if}
        <a class="btn" href={paths.boards(p.key)}>{t.boards.title}</a>
      </EmptyState>
    {:else}
      <IssueSearch
        project={p.key}
        fallback={DEFAULT_JQL}
        selectable
        saveAs={(q) => {
          const jql = projectViewJql(p.key, q);
          return jql === null ? null : searchHref(jql);
        }}
      />
    {/if}
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
