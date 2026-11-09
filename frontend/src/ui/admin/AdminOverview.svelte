<script lang="ts">
  import { ADMIN_NAV, adminPath } from '../../core/areas.js';
  import type { Onboarding } from '../../core/admin-types.js';
  import { paths } from '../../core/route.js';
  import { t } from '../../i18n/t.js';
  import { adminApi } from '../../lib/admin-api.js';
  import { openCreate } from '../../lib/ui.svelte.js';
  import { ADMIN_ICONS } from './admin-icons.js';

  /**
   * The administration's front page: the setup steps a fresh install still
   * lacks, then every section with a line on what it holds.
   */

  let onboarding = $state<Onboarding | null>(null);

  $effect(() => {
    adminApi
      .onboarding()
      .then((o) => (onboarding = o))
      .catch(() => (onboarding = null));
  });

  type Step = { title: string; hint: string; action: string; primary?: boolean } & (
    { href: string } | { onclick: () => void }
  );

  const steps = $derived.by((): Step[] => {
    const o = onboarding;
    if (!o) return [];
    const a = t.adminArea;
    const out: Step[] = [];
    if (!o.projects) {
      out.push({
        title: a.noProject,
        hint: a.noProjectHint,
        action: a.createProject,
        href: `${adminPath('projects')}?new=1`,
        primary: true,
      });
    }
    if (!o.otherUsers) {
      out.push({
        title: a.noUsers,
        hint: a.noUsersHint,
        action: a.addUsers,
        href: adminPath('users'),
      });
    }
    if (!o.mail) {
      out.push({
        title: a.mailOff,
        hint: a.mailOffHint,
        action: a.setUpMail,
        href: adminPath('mail'),
      });
    }
    if (!o.boards) {
      out.push({
        title: a.noBoards,
        hint: a.noBoardsHint,
        action: a.createBoard,
        href: paths.boards(),
      });
    }
    // An issue needs a project: the dialog would have nothing to offer.
    if (!o.issues && o.projects) {
      out.push({
        title: a.noIssues,
        hint: a.noIssuesHint,
        action: a.createIssue,
        onclick: () => openCreate(),
      });
    }
    return out;
  });
</script>

<header class="page-head">
  <div>
    <p class="eyebrow">{t.appName}</p>
    <h1 class="page-title">{t.adminArea.title}</h1>
  </div>
</header>
<p class="lead-text">{t.adminArea.intro}</p>

{#if steps.length > 0}
  <section class="todo" aria-labelledby="admin-todo">
    <h2 class="section-title" id="admin-todo">{t.adminArea.todo}</h2>
    <ul class="todo-list">
      {#each steps as step (step.title)}
        <li class="card">
          <div>
            <strong>{step.title}</strong>
            <p class="muted small">{step.hint}</p>
          </div>
          {#if 'href' in step}
            <a class="btn" class:btn-primary={step.primary} href={step.href}>{step.action}</a>
          {:else}
            <button class="btn" type="button" onclick={step.onclick}>{step.action}</button>
          {/if}
        </li>
      {/each}
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
            <a
              class="admin-card"
              href={adminPath(item)}
              aria-labelledby="admin-card-{item}"
              aria-describedby="admin-card-{item}-text"
            >
              <svg class="admin-icon" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true">
                {#each ADMIN_ICONS[item as keyof typeof ADMIN_ICONS] as d, i (i)}<path {d} />{/each}
              </svg>
              <span class="admin-card-title" id="admin-card-{item}"
                >{t.adminArea.sections[item]}</span
              >
              <span class="admin-card-text muted small" id="admin-card-{item}-text"
                >{t.adminArea.descriptions[item]}</span
              >
            </a>
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
    gap: 8px;
  }

  .admin-card {
    display: grid;
    grid-template-columns: auto 1fr;
    align-items: center;
    gap: 2px 8px;
    padding: 9px 12px;
    background: var(--paper-raised);
    border: var(--frame) solid var(--rule);
    border-radius: var(--radius-m);
    text-decoration: none;
    transition:
      background-color 0.12s ease,
      border-color 0.12s ease,
      transform 0.12s ease;
  }

  .admin-card:hover {
    background: var(--paper-sunk);
    border-color: var(--accent);
    transform: translateY(-1px);
  }

  .admin-icon {
    flex: none;
    fill: none;
    stroke: currentColor;
    stroke-width: 1.4;
    stroke-linecap: round;
    stroke-linejoin: round;
  }

  .admin-card-title {
    font-weight: 600;
  }

  .admin-card-text {
    grid-column: 2;
  }

  @media (prefers-reduced-motion: reduce) {
    .admin-card {
      transition: none;
    }

    .admin-card:hover {
      transform: none;
    }
  }
</style>
