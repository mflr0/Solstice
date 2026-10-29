<script lang="ts">
  import { ADMIN_NAV, adminPath } from '../../core/areas.js';
  import { t } from '../../i18n/t.js';
  import { adminApi } from '../../lib/admin-api.js';
  import { api } from '../../lib/api.js';

  /**
   * The administration's front page: what still needs doing on a fresh
   * install (a first project, e-mail), then every section with a line on
   * what it holds.
   */

  let projects = $state<number | null>(null);
  let mailOn = $state<boolean | null>(null);

  $effect(() => {
    api
      .projects()
      .then((p) => (projects = p.length))
      .catch(() => (projects = null));
    adminApi
      .mail()
      .then((m) => (mailOn = m.enabled))
      .catch(() => (mailOn = null));
  });
</script>

<header class="page-head">
  <div>
    <p class="eyebrow">{t.appName}</p>
    <h1 class="page-title">{t.adminArea.title}</h1>
  </div>
</header>
<p class="lead-text">{t.adminArea.intro}</p>

{#if projects === 0 || mailOn === false}
  <section class="todo" aria-labelledby="admin-todo">
    <h2 class="section-title" id="admin-todo">{t.adminArea.todo}</h2>
    <ul class="todo-list">
      {#if projects === 0}
        <li class="card">
          <div>
            <strong>{t.adminArea.noProject}</strong>
            <p class="muted small">{t.adminArea.noProjectHint}</p>
          </div>
          <a class="btn btn-primary" href={`${adminPath('projects')}?new=1`}
            >{t.adminArea.createProject}</a
          >
        </li>
      {/if}
      {#if mailOn === false}
        <li class="card">
          <div>
            <strong>{t.adminArea.mailOff}</strong>
            <p class="muted small">{t.adminArea.mailOffHint}</p>
          </div>
          <a class="btn" href={adminPath('mail')}>{t.adminArea.setUpMail}</a>
        </li>
      {/if}
    </ul>
  </section>
{/if}

<div class="groups">
  {#each ADMIN_NAV as g (g.group)}
    <section class="group" aria-labelledby="group-{g.group}">
      <h2 class="section-title" id="group-{g.group}">{t.adminArea.groups[g.group]}</h2>
      <ul>
        {#each g.items.filter((i) => i !== 'overview') as item (item)}
          <li>
            <a href={adminPath(item)}>{t.adminArea.sections[item]}</a>
            <span class="muted small">{t.adminArea.descriptions[item]}</span>
          </li>
        {/each}
      </ul>
    </section>
  {/each}
</div>

<style>
  .todo-list {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    gap: 10px;
  }

  .todo-list li {
    display: flex;
    flex-wrap: wrap;
    gap: 10px 18px;
    align-items: center;
    justify-content: space-between;
    padding: 12px 14px;
  }

  .todo-list p {
    margin: 2px 0 0;
  }

  .groups {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(16.25rem, 1fr));
    gap: 0 32px;
  }

  .group ul {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    gap: 9px;
  }

  .group li {
    display: grid;
    gap: 1px;
  }

  .group a {
    font-weight: 600;
    text-decoration: none;
  }

  .group a:hover {
    text-decoration: underline;
  }
</style>
