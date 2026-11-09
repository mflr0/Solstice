<script lang="ts">
  import { adminPath } from '../core/areas.js';
  import type { Onboarding } from '../core/admin-types.js';
  import { onboardingSteps, showOnboarding, type OnboardingStep } from '../core/onboarding.js';
  import { paths } from '../core/route.js';
  import { t } from '../i18n/t.js';
  import { adminApi } from '../lib/admin-api.js';
  import { errorMessage } from '../lib/api.js';
  import { dialogs, notify, openCreate } from '../lib/ui.svelte.js';

  /**
   * The setup steps of a new instance, on an administrator's space, until
   * they are all done or hidden.
   */

  let onboarding = $state<Onboarding | null>(null);
  let hiding = $state(false);
  let creating = false;

  const list = $derived(onboarding ? onboardingSteps(onboarding) : null);

  function load() {
    adminApi.onboarding().then(
      (o) => (onboarding = o),
      () => (onboarding = null),
    );
  }

  $effect(load);

  // A first issue created from here ticks its step once the dialog closes.
  $effect(() => {
    if (dialogs.create) creating = true;
    else if (creating) {
      creating = false;
      load();
    }
  });

  async function hide() {
    hiding = true;
    try {
      onboarding = await adminApi.saveOnboarding(true);
    } catch (e) {
      notify(errorMessage(e), 'error');
    } finally {
      hiding = false;
    }
  }

  const a = $derived(t.adminArea);

  /** The step's action: a link, a button, or none while it can't be done yet. */
  function action(step: OnboardingStep): { label: string; href?: string; run?: () => void } | null {
    switch (step) {
      case 'projects':
        return { label: a.createProject, href: `${adminPath('projects')}?new=1` };
      case 'users':
        return { label: a.addUsers, href: adminPath('users') };
      case 'mail':
        return { label: a.setUpMail, href: adminPath('mail') };
      case 'boards':
        return { label: a.createBoard, href: paths.boards() };
      case 'issues':
        // An issue needs a project: the dialog would have nothing to offer.
        return onboarding?.projects ? { label: a.createIssue, run: () => openCreate() } : null;
    }
  }
</script>

{#if showOnboarding(onboarding) && list}
  {@const g = t.gettingStarted}
  <section class="getting-started card" aria-labelledby="getting-started-title">
    <div class="head">
      <h2 class="title" id="getting-started-title">{g.title}</h2>
      <span class="count">{g.progress(list.done, list.steps.length)}</span>
      <span class="gap"></span>
      <button type="button" class="link-btn" disabled={hiding} onclick={hide}>{g.hide}</button>
    </div>
    <ul class="steps">
      {#each list.steps as s (s.step)}
        {@const act = s.done ? null : action(s.step)}
        <li class:done={s.done}>
          <svg class="mark" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true">
            <circle cx="8" cy="8" r="6.5" />
            {#if s.done}<path d="M5 8.2 7 10.2 11 6" />{/if}
          </svg>
          <span class="label">
            {g.steps[s.step]}
            <span class="sr-only">— {s.done ? g.done : g.notDone}</span>
          </span>
          {#if act?.href}
            <a class="btn btn-small" href={act.href}>{act.label}</a>
          {:else if act?.run}
            <button type="button" class="btn btn-small" onclick={act.run}>{act.label}</button>
          {/if}
        </li>
      {/each}
    </ul>
  </section>
{/if}

<style>
  .getting-started {
    margin-bottom: 28px;
  }

  .head {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px 12px;
    padding: 12px 14px 8px;
  }

  .title {
    margin: 0;
    font-size: 1.0625rem;
    line-height: 1.2;
  }

  .count {
    padding: 1px 6px;
    border-radius: var(--radius-xs);
    background: var(--paper-sunk);
    color: var(--ink-soft);
    font: 0.75rem var(--font-mono);
  }

  .gap {
    flex: 1 1 40px;
  }

  .steps {
    list-style: none;
    margin: 0;
    padding: 0 14px 12px;
    display: grid;
    gap: 4px;
  }

  .steps li {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 6px 10px;
    min-height: 36px;
  }

  .label {
    flex: 1 1 12rem;
    min-width: 0;
    overflow-wrap: anywhere;
  }

  .done .label {
    color: var(--ink-soft);
    text-decoration: line-through;
  }

  .mark {
    flex: none;
    fill: none;
    stroke: var(--ink-soft);
    stroke-width: 1.4;
    stroke-linecap: round;
    stroke-linejoin: round;
  }

  .done .mark {
    stroke: var(--accent);
  }
</style>
