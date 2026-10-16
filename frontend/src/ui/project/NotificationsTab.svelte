<script lang="ts">
  import type { ExtraRecipient, ProjectNotifications } from '../../core/admin-types.js';
  import { extraSignature, hasExtra, toggleExtra } from '../../core/project-admin.js';
  import type { IdName } from '../../core/types.js';
  import { holderLabels, t } from '../../i18n/t.js';
  import { accountApi } from '../../lib/admin-api.js';
  import { api, errorMessage } from '../../lib/api.js';
  import { notify } from '../../lib/ui.svelte.js';

  /**
   * The project's e-mail notifications: on or off for the whole project, and
   * the groups and roles that also receive each kind of event. Saved
   * together with the bar at the bottom.
   */
  interface Props {
    projectKey: string;
    ondirty: (dirty: boolean) => void;
  }
  const { projectKey, ondirty }: Props = $props();

  interface Recipient {
    kind: 'group' | 'role';
    id: number;
    name: string;
  }

  let data = $state<ProjectNotifications | null>(null);
  let enabled = $state(true);
  let extra = $state<ExtraRecipient[]>([]);
  let saved = $state('');
  let recipients = $state<Recipient[]>([]);
  let error = $state('');
  let saveError = $state('');
  let saving = $state(false);

  const signature = (on: boolean, list: readonly ExtraRecipient[]) =>
    `${on ? 1 : 0}#${extraSignature(list)}`;
  const dirty = $derived(data !== null && signature(enabled, extra) !== saved);

  $effect(() => ondirty(dirty));
  $effect(() => () => ondirty(false));

  const eventLabel = (event: string) =>
    (t.account.eventLabels as Record<string, { label: string } | undefined>)[event]?.label ?? event;

  function adopt(n: ProjectNotifications) {
    data = n;
    enabled = n.enabled;
    extra = n.extra;
    saved = signature(n.enabled, n.extra);
  }

  async function load() {
    try {
      const [n, p] = await Promise.all([
        accountApi.projectNotifications(projectKey),
        api.permissions(projectKey),
      ]);
      const groups: IdName[] = p.groups;
      recipients = [
        ...p.roles.map((r) => ({ kind: 'role' as const, id: r.id, name: r.name })),
        ...groups.map((g) => ({ kind: 'group' as const, id: g.id, name: g.name })),
      ];
      adopt(n);
      error = '';
    } catch (e) {
      error = errorMessage(e);
    }
  }

  $effect(() => {
    void projectKey;
    void load();
  });

  function toggle(r: Recipient, event: string, on: boolean) {
    extra = toggleExtra(extra, { event, kind: r.kind, id: r.id, name: r.name }, on);
  }

  function discard() {
    if (!data) return;
    enabled = data.enabled;
    extra = data.extra;
    saveError = '';
  }

  async function save() {
    saving = true;
    saveError = '';
    try {
      adopt(await accountApi.saveProjectNotifications(projectKey, { enabled, extra }));
      notify(t.account.notificationsSaved);
    } catch (e) {
      saveError = errorMessage(e);
    } finally {
      saving = false;
    }
  }
</script>

<p class="lead-text">{t.projectNotify.intro}</p>

{#if error}
  <p class="error-text" role="alert">{error}</p>
{:else if !data}
  <p class="muted loading">{t.common.loading}</p>
{:else}
  <div class="settings-form">
    <label class="check master">
      <input class="checkbox" type="checkbox" bind:checked={enabled} />
      <span>
        <strong>{t.projectNotify.enabled}</strong>
        <span class="form-hint block">{t.projectNotify.enabledHint}</span>
      </span>
    </label>

    <h2 class="section-title">{t.projectNotify.extraTitle}</h2>
    {#if !enabled}<p class="form-hint">{t.projectNotify.offNote}</p>{:else}<p class="form-hint">
        {t.projectNotify.extraHint}
      </p>{/if}

    {#if recipients.length === 0}
      <p class="muted">{t.projectNotify.extraEmpty}</p>
    {:else}
      <div class="table-scroll">
        <table class="ledger matrix">
          <thead>
            <tr>
              <th scope="col">{t.projectNotify.recipient}</th>
              {#each data.events as event (event)}
                <th scope="col" class="c-event">{eventLabel(event)}</th>
              {/each}
            </tr>
          </thead>
          <tbody>
            {#each recipients as r (`${r.kind}${r.id}`)}
              <tr class:is-off={!enabled}>
                <th scope="row">
                  <span class="kind">{holderLabels[r.kind]}</span>
                  {r.name}
                </th>
                {#each data.events as event (event)}
                  <td class="c-event">
                    <input
                      class="checkbox"
                      type="checkbox"
                      disabled={!enabled}
                      checked={hasExtra(extra, event, r.kind, r.id)}
                      aria-label={t.projectNotify.cell(r.name, eventLabel(event))}
                      onchange={(e) => toggle(r, event, e.currentTarget.checked)}
                    />
                  </td>
                {/each}
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}

    {#if saveError}<p class="error-text" role="alert">{saveError}</p>{/if}
  </div>

  {#if dirty}
    <div class="savebar" role="region" aria-label={t.settings.unsaved}>
      <span>{t.settings.unsaved}</span>
      <button class="btn btn-quiet" disabled={saving} onclick={discard}>{t.settings.discard}</button
      >
      <button class="btn btn-primary" disabled={saving} onclick={() => void save()}
        >{saving ? t.common.saving : t.common.save}</button
      >
    </div>
  {/if}
{/if}

<style>
  .master {
    align-items: flex-start;
  }

  .block {
    display: block;
    margin: 2px 0 0;
  }

  .settings-form .section-title {
    margin-bottom: 0;
  }

  .settings-form .form-hint {
    margin: 0;
  }

  .matrix th {
    text-align: left;
    text-transform: none;
    letter-spacing: 0;
    font-weight: 500;
  }

  .matrix tbody th {
    white-space: nowrap;
    border-bottom: 1px solid var(--rule);
  }

  .kind {
    font: 500 11px/1 var(--font-body);
    color: var(--muted);
    margin-right: 4px;
  }

  .c-event {
    text-align: center;
    min-width: 84px;
  }

  .matrix tbody tr:hover td {
    background: none;
  }
</style>
