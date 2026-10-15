<script lang="ts">
  import { paths } from '../../core/route.js';
  import type { Project } from '../../core/types.js';
  import { formatLocale } from '../../i18n/format.js';
  import { t } from '../../i18n/t.js';
  import { api, errorMessage } from '../../lib/api.js';
  import Illustration from '../Illustration.svelte';
  import Who from '../Who.svelte';

  /** Every project one may see, grouped by category. */
  let projects = $state<Project[] | null>(null);
  let error = $state('');

  $effect(() => {
    document.title = `${t.projects.title} — ${t.appName}`;
    api
      .projects()
      .then((p) => (projects = p))
      .catch((e: unknown) => (error = errorMessage(e)));
  });

  const groups = $derived.by(() => {
    const out: [string, Project[]][] = [];
    for (const p of projects ?? []) {
      const c = p.category ?? t.projects.uncategorised;
      const group = out.find(([name]) => name === c);
      if (group) group[1].push(p);
      else out.push([c, [p]]);
    }
    return out.sort(([a], [b]) => a.localeCompare(b, formatLocale()));
  });
</script>

<div class="page" data-title={t.projects.title}>
  <header class="page-head">
    <h1 class="page-title">{t.projects.title}</h1>
  </header>

  {#if error}
    <p class="error-text" role="alert">{error}</p>
  {:else if !projects}
    <p class="muted loading">{t.common.loading}</p>
  {:else if projects.length === 0}
    <Illustration kind="empty" />
    <p class="empty">{t.projects.empty}</p>
  {:else}
    {#each groups as [category, list] (category)}
      <section>
        <h2 class="section-title">{category} <span class="count">{list.length}</span></h2>
        <table class="ledger">
          <thead>
            <tr>
              <th class="c-key">{t.projects.key}</th>
              <th class="c-name">{t.projects.name}</th>
              <th class="c-lead">{t.projects.lead}</th>
              <th class="num">{t.projects.open}</th>
            </tr>
          </thead>
          <tbody>
            {#each list as p (p.id)}
              <tr>
                <td class="c-key"><a class="key" href={paths.project(p.key)}>{p.key}</a></td>
                <td>
                  <a class="name" href={paths.project(p.key)}>{p.name}</a>
                  <span class="muted kind">{t.projects.kinds[p.kind] ?? p.kind}</span>
                </td>
                <td><Who user={p.lead} empty="—" /></td>
                <td class="num">
                  <a href={paths.search(`projet:${p.key} est:ouvert`)}>{p.open}</a>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </section>
    {/each}
  {/if}
</div>

<style>
  .c-key {
    width: 90px;
  }

  .c-name {
    width: 55%;
  }

  .c-lead {
    width: 28%;
  }

  .name {
    font: var(--title-weight) 15.5px/1.3 var(--font-title);
    text-decoration: none;
  }

  .name:hover {
    text-decoration: underline;
  }

  .kind {
    margin-left: 8px;
    font-size: 12px;
  }

  td.num a {
    text-decoration: none;
  }
</style>
