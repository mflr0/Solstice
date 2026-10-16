<script lang="ts">
  import type { ProjectRole } from '../../core/admin-types.js';
  import { withMember } from '../../core/project-admin.js';
  import type { IdName, UserRef } from '../../core/types.js';
  import { holderLabels, t } from '../../i18n/t.js';
  import { projectAdminApi } from '../../lib/admin-api.js';
  import { api, errorMessage } from '../../lib/api.js';
  import { notify } from '../../lib/ui.svelte.js';
  import EmptyState from '../EmptyState.svelte';
  import UserPicker from '../UserPicker.svelte';
  import Who from '../Who.svelte';

  /**
   * Who holds each role in the project, people and groups. Each change is
   * saved as soon as it is made: the server replaces the role's members.
   */
  interface Props {
    projectKey: string;
  }
  const { projectKey }: Props = $props();

  let roles = $state<ProjectRole[] | null>(null);
  let groups = $state<IdName[]>([]);
  let error = $state('');
  let busy = $state<number | null>(null);
  let pickedGroup = $state<Record<number, string>>({});

  async function load() {
    try {
      const [r, p] = await Promise.all([
        projectAdminApi.roles(projectKey),
        api.permissions(projectKey),
      ]);
      roles = r;
      groups = p.groups;
      error = '';
    } catch (e) {
      error = errorMessage(e);
    }
  }

  $effect(() => {
    void projectKey;
    void load();
  });

  /** Adds or removes one person or group, then keeps the role the server answers. */
  async function change(
    role: ProjectRole,
    kind: 'user' | 'group',
    id: number,
    name: string,
    on: boolean,
  ) {
    busy = role.id;
    try {
      const next = await projectAdminApi.setRole(
        projectKey,
        role.id,
        withMember(role, kind, id, on),
      );
      roles = (roles ?? []).map((r) => (r.id === next.id ? next : r));
      notify(
        on ? t.projectPeople.added(name, role.name) : t.projectPeople.removed(name, role.name),
      );
    } catch (e) {
      notify(errorMessage(e), 'error');
    } finally {
      busy = null;
    }
  }

  function addPerson(role: ProjectRole, u: UserRef | null) {
    if (u) void change(role, 'user', u.id, u.name, true);
  }

  function addGroup(role: ProjectRole) {
    const g = groups.find((x) => String(x.id) === pickedGroup[role.id]);
    if (!g) return;
    pickedGroup[role.id] = '';
    void change(role, 'group', g.id, g.name, true);
  }

  const freeGroups = (role: ProjectRole) =>
    groups.filter((g) => !role.groups.some((x) => x.id === g.id));
</script>

<p class="lead-text">{t.projectPeople.intro}</p>

{#if error}
  <p class="error-text" role="alert">{error}</p>
{:else if !roles}
  <p class="muted loading">{t.common.loading}</p>
{:else if roles.length === 0}
  <EmptyState message={t.projectPeople.noRoles} />
{:else}
  <div class="roles">
    {#each roles as role (role.id)}
      {@const left = freeGroups(role)}
      <section class="role" aria-labelledby="role-{role.id}">
        <h2 class="section-title" id="role-{role.id}">{role.name}</h2>
        {#if role.description}<p class="muted small desc">{role.description}</p>{/if}
        <div class="members">
          {#each role.groups as g (`g${g.id}`)}
            <span class="chip">
              <span class="kind">{holderLabels.group}</span>
              {g.name}
              <button
                type="button"
                aria-label={t.projectPeople.removeMember(g.name, role.name)}
                disabled={busy === role.id}
                onclick={() => void change(role, 'group', g.id, g.name, false)}>×</button
              >
            </span>
          {/each}
          {#each role.users as u (`u${u.id}`)}
            <span class="chip">
              <Who user={u} initialsBox={false} />
              <button
                type="button"
                aria-label={t.projectPeople.removeMember(u.name, role.name)}
                disabled={busy === role.id}
                onclick={() => void change(role, 'user', u.id, u.name, false)}>×</button
              >
            </span>
          {:else}
            {#if role.groups.length === 0}<span class="nobody">{t.settings.noMembers}</span>{/if}
          {/each}
        </div>
        <div class="form-grid adders">
          <div class="field">
            <span>{t.projectPeople.personField}</span>
            <UserPicker
              field
              value={null}
              label={t.projectPeople.personField}
              noneLabel={t.projectPeople.choosePerson}
              allowNone={false}
              disabled={busy === role.id}
              search={(q) => api.users(q)}
              onpick={(u) => addPerson(role, u)}
            />
          </div>
          <div class="field">
            <span id="group-label-{role.id}">{t.projectPeople.groupField}</span>
            <div class="row">
              <select
                class="select grow"
                aria-labelledby="group-label-{role.id}"
                bind:value={pickedGroup[role.id]}
                disabled={busy === role.id || left.length === 0}
              >
                <option value="" disabled>
                  {left.length === 0 ? t.projectPeople.noGroupLeft : t.projectsAdmin.chooseGroup}
                </option>
                {#each left as g (g.id)}<option value={String(g.id)}>{g.name}</option>{/each}
              </select>
              <button
                type="button"
                class="btn"
                disabled={busy === role.id || !pickedGroup[role.id]}
                onclick={() => addGroup(role)}>{t.common.add}</button
              >
            </div>
          </div>
        </div>
      </section>
    {/each}
  </div>
{/if}

<style>
  .roles {
    display: grid;
    gap: 22px;
    max-width: 760px;
  }

  .role {
    display: grid;
    gap: 10px;
  }

  .role .section-title {
    margin: 0;
  }

  .desc {
    margin: 0;
  }

  .members {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    align-items: center;
  }

  .kind {
    font: 500 11px/1 var(--font-body);
    color: var(--muted);
  }

  .nobody {
    color: var(--muted);
    font-style: italic;
    font-size: 13px;
  }

  .adders {
    align-items: start;
  }
</style>
