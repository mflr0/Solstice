<script lang="ts">
  import { localParts } from '../../core/dates.js';
  import { paths } from '../../core/route.js';
  import { renderDay, t } from '../../i18n/fr.js';
  import { session } from '../../lib/session.svelte.js';
  import IssueQuery from '../IssueQuery.svelte';

  /** "Mon travail": what is on my plate, what I am waiting for, what moved. */
  const sections = [
    { q: '@moi est:ouvert tri:priorité', title: t.home.assigned, empty: t.home.emptyAssigned },
    { q: 'par:@moi est:ouvert', title: t.home.reported, empty: t.home.emptyReported },
    { q: 'tri:maj', title: t.home.recent, empty: t.home.emptyRecent },
  ];
  let totals = $state([0, 0, 0]);
  const today = renderDay(localParts(new Date().toISOString()));

  $effect(() => {
    document.title = `${t.home.title} — ${t.appName}`;
  });
</script>

<div class="page" data-title={t.home.title}>
  <header class="page-head">
    <div>
      <p class="eyebrow">{today}</p>
      <h1 class="page-title">
        {session.me ? t.home.greeting(session.me.name.split(' ')[0] ?? '') : t.home.title}
      </h1>
    </div>
  </header>

  {#each sections as s, i (s.q)}
    <section>
      <h2 class="section-title">
        {s.title}
        <span class="count">{totals[i]}</span>
        <span class="actions"
          ><a class="link-btn" href={paths.search(s.q)}>{t.common.seeAll}</a></span
        >
      </h2>
      <IssueQuery
        q={s.q}
        size={i === 2 ? 12 : 15}
        empty={s.empty}
        showAssignee={i !== 0}
        ontotal={(n) => (totals[i] = n)}
      />
    </section>
  {/each}
</div>
