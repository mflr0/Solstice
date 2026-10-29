<script lang="ts">
  import { onDestroy } from 'svelte';
  import type { ExtraRecipient, ProjectNotifications } from '../../core/admin-types.js';
  import { hasExtra, notificationsChanged, toggleExtra } from '../../core/project-admin.js';
  import type { IdName } from '../../core/types.js';
  import { holderLabels, t } from '../../i18n/t.js';
  import { accountApi } from '../../lib/admin-api.js';
  import { errorMessage } from '../../lib/api.js';
  import { notify } from '../../lib/ui.svelte.js';

  /**
   * The Notifications tab: whether this project sends email at all, and the
   * groups and roles that also receive an event's email in it. Changes wait
   * for « Save ».
   */
  interface Props {
    projectKey: string;
    /** What can be chosen: the instance's groups and the project's roles. */
    groups: IdName[];
    roles: IdName[];
    /** Tells the page whether there are unsaved changes (it asks before leaving). */
    ondirty: (dirty: boolean) => void;
  }
  const { projectKey, groups, roles, ondirty }: Props = $props();

  let saved = $state<ProjectNotifications | null>(null);
  let enabled = $state(true);
  let extra = $state<ExtraRecipient[]>([]);
  let loadError = $state('');
  let saving = $state(false);

  const dirty = $derived(saved ? notificationsChanged(saved, enabled, extra) : false);
  const rows = $derived([
    ...groups.map((g) => ({ kind: 'group' as const, ...g })),
    ...roles.map((r) => ({ kind: 'role' as const, id: r.id, name: r.name })),
  ]);
  const eventLabel = (e: string) =>
    (t.account.eventLabels as Record<string, { label: string }>)[e]?.label ?? e;

  $effect(() => {
    ondirty(dirty);
  });
  onDestroy(() => ondirty(false));

  // Reloading or closing the tab with unsaved changes asks first.
  $effect(() => {
    if (!dirty) return;
    const hold = (e: BeforeUnloadEvent) => e.preventDefault();
    window.addEventListener('beforeunload', hold);
    return () => window.removeEventListener('beforeunload', hold);
  });

  function adopt(n: ProjectNotifications) {
    saved = n;
    enabled = n.enabled;
    extra = n.extra;
  }

  $effect(() => {
    accountApi
      .projectNotifications(projectKey)
      .then(adopt)
      .catch((e: unknown) => (loadError = errorMessage(e)));
  });

  function discard() {
    if (saved) adopt(saved);
  }

  async function save() {
    saving = true;
    try {
      adopt(await accountApi.saveProjectNotifications(projectKey, { enabled, extra }));
      notify(t.settings.saved);
    } catch (e) {
      notify(errorMessage(e), 'error');
    } finally {
      saving = false;
    }
  }
</script>

{#if loadError}
  <p class="error-text" role="alert">{loadError}</p>
{:else if !saved}
  <p class="muted loading">{t.common.loading}</p>
{:else}
  <h2 class="section-title">{t.projectConfig.notifications.title}</h2>
  <p class="lead-text">{t.projectConfig.notifications.intro}</p>

  <div class="settings-form">
    <label class="check">
      <input class="checkbox" type="checkbox" bind:checked={enabled} />
      <span>
        {t.projectConfig.notifications.enabled}
        <span class="muted small">{t.projectConfig.notifications.enabledHint}</span>
      </span>
    </label>
  </div>

  <h2 class="section-title">{t.projectConfig.notifications.extraTitle}</h2>
  <p class="lead-text">{t.projectConfig.notifications.extraIntro}</p>

  {#if rows.length === 0}
    <p class="muted">{t.projectConfig.notifications.none}</p>
  {:else}
    <div class="table-scroll">
      <table class="ledger">
        <thead>
          <tr>
            <th scope="col">{t.projectConfig.notifications.recipient}</th>
            {#each saved.events as e (e)}
              <th scope="col" class="cell">{eventLabel(e)}</th>
            {/each}
          </tr>
        </thead>
        <tbody>
          {#each rows as r (`${r.kind}:${r.id}`)}
            <tr>
              <th scope="row" class="who">
                <span class="muted small">{holderLabels[r.kind]}</span>
                {r.name}
              </th>
              {#each saved.events as e (e)}
                <td class="cell">
                  <input
                    class="checkbox"
                    type="checkbox"
                    aria-label={t.projectConfig.notifications.cell(r.name, eventLabel(e))}
                    checked={hasExtra(extra, e, r.kind, r.id)}
                    onchange={(ev) =>
                      (extra = toggleExtra(
                        extra,
                        { event: e, kind: r.kind, id: r.id, name: r.name },
                        ev.currentTarget.checked,
                      ))}
                  />
                </td>
              {/each}
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}

  {#if dirty}
    <div class="savebar" role="region" aria-label={t.settings.unsaved}>
      <span>{t.settings.unsaved}</span>
      <button type="button" class="btn btn-quiet" disabled={saving} onclick={discard}
        >{t.settings.discard}</button
      >
      <button type="button" class="btn btn-primary" disabled={saving} onclick={() => void save()}
        >{saving ? t.common.saving : t.common.save}</button
      >
    </div>
  {/if}
{/if}

<style>
  th.who {
    text-align: left;
    text-transform: none;
    letter-spacing: 0;
    font-weight: 600;
  }

  th.who .muted {
    margin-right: 4px;
    font-weight: 500;
  }

  .cell {
    text-align: center;
  }

  th.cell {
    white-space: normal;
    min-width: 5.625rem;
  }
</style>
