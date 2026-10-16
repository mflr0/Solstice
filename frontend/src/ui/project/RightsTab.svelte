<script lang="ts">
  import type { Grant, Holder, Permission, ProjectPermissions, UserRef } from '../../core/types.js';
  import { grantLabel, holderLabels, permissionLabels, t } from '../../i18n/t.js';
  import { outside } from '../../lib/actions.js';
  import { api, errorMessage } from '../../lib/api.js';
  import { notify } from '../../lib/ui.svelte.js';
  import UserPicker from '../UserPicker.svelte';

  /**
   * Who holds each permission in the project, and the roles' members. The
   * grants are edited together and saved with the bar at the bottom.
   */
  interface Props {
    projectKey: string;
    ondirty: (dirty: boolean) => void;
  }
  const { projectKey, ondirty }: Props = $props();

  let data = $state<ProjectPermissions | null>(null);
  let grants = $state<Grant[]>([]);
  let saved = $state('');
  let error = $state('');
  let saving = $state(false);
  let adding = $state<Permission | null>(null);
  let kind = $state<Holder>('group');
  let pick = $state('');

  const signature = (list: Grant[]) =>
    list
      .map((g) => `${g.permission}:${g.holder}:${g.id ?? ''}`)
      .sort()
      .join('|');
  const dirty = $derived(signature(grants) !== saved);

  $effect(() => ondirty(dirty));
  $effect(() => () => ondirty(false));

  $effect(() => {
    const key = projectKey;
    api
      .permissions(key)
      .then((d) => {
        data = d;
        grants = d.grants;
        saved = signature(d.grants);
      })
      .catch((e: unknown) => (error = errorMessage(e)));
  });

  const kinds: Holder[] = ['everyone', 'group', 'role', 'user', 'lead', 'reporter', 'assignee'];

  function holdersOf(p: Permission): Grant[] {
    return grants.filter((g) => g.permission === p);
  }

  function remove(g: Grant) {
    grants = grants.filter((x) => x !== g);
  }

  function add(p: Permission, g: Omit<Grant, 'permission'>) {
    const grant: Grant = { permission: p, ...g };
    if (!grants.some((x) => signature([x]) === signature([grant]))) grants = [...grants, grant];
    adding = null;
    pick = '';
  }

  function addPicked(p: Permission) {
    if (!data) return;
    if (kind === 'group' || kind === 'role') {
      const list = kind === 'group' ? data.groups : data.roles;
      const found = list.find((x) => String(x.id) === pick);
      if (found) add(p, { holder: kind, id: found.id, name: found.name });
    } else if (kind !== 'user') {
      add(p, { holder: kind });
    }
  }

  async function save() {
    saving = true;
    try {
      const d = await api.savePermissions(
        projectKey,
        grants.map(({ permission, holder, id }) =>
          id ? { permission, holder, id } : { permission, holder },
        ),
      );
      data = d;
      grants = d.grants;
      saved = signature(d.grants);
      notify(t.settings.saved);
    } catch (e) {
      notify(errorMessage(e), 'error');
    } finally {
      saving = false;
    }
  }
</script>

{#if error}
  <p class="error-text" role="alert">{error}</p>
{:else if !data}
  <p class="muted loading">{t.common.loading}</p>
{:else}
  <h2 class="section-title">{t.settings.permissions}</h2>
  <div class="table-scroll">
    <table class="ledger matrix">
      <tbody>
        {#each data.permissions as p (p)}
          <tr>
            <th scope="row">
              <span class="label">{permissionLabels[p].label}</span>
              <span class="help">{permissionLabels[p].help}</span>
            </th>
            <td>
              <div class="holders">
                {#each holdersOf(p) as g (signature([g]))}
                  <span class="chip holder {g.holder}">
                    {#if g.holder === 'group' || g.holder === 'role' || g.holder === 'user'}<span
                        class="kind">{holderLabels[g.holder]}</span
                      >{/if}
                    {grantLabel(g)}
                    <button
                      type="button"
                      aria-label={`${t.common.remove} ${grantLabel(g)}`}
                      onclick={() => remove(g)}>×</button
                    >
                  </span>
                {:else}
                  <span class="nobody">{t.settings.nobody}</span>
                {/each}
                <span class="adder" use:outside={() => adding === p && (adding = null)}>
                  <button
                    type="button"
                    class="link-btn"
                    aria-expanded={adding === p}
                    onclick={() => {
                      adding = adding === p ? null : p;
                      kind = 'group';
                      pick = '';
                    }}>+ {t.settings.addHolder}</button
                  >
                  {#if adding === p}
                    <div class="popover add">
                      <label class="field">
                        <span>{t.settings.chooseKind}</span>
                        <select class="select" bind:value={kind}>
                          {#each kinds as k (k)}<option value={k}>{holderLabels[k]}</option>{/each}
                        </select>
                      </label>
                      {#if kind === 'group' || kind === 'role'}
                        <label class="field">
                          <span
                            >{kind === 'group'
                              ? t.settings.chooseGroup
                              : t.settings.chooseRole}</span
                          >
                          <select class="select" bind:value={pick}>
                            <option value="" disabled></option>
                            {#each kind === 'group' ? data.groups : data.roles as x (x.id)}
                              <option value={String(x.id)}>{x.name}</option>
                            {/each}
                          </select>
                        </label>
                      {:else if kind === 'user'}
                        <div class="field">
                          <span class="field-label">{t.settings.chooseUser}</span>
                          <div class="boxed">
                            <UserPicker
                              value={null}
                              label={t.settings.chooseUser}
                              allowNone={false}
                              noneLabel="…"
                              search={(q) => api.users(q)}
                              onpick={(u: UserRef | null) =>
                                u && add(p, { holder: 'user', id: u.id, name: u.name })}
                            />
                          </div>
                        </div>
                      {/if}
                      {#if kind !== 'user'}
                        <button
                          class="btn btn-small btn-primary"
                          disabled={(kind === 'group' || kind === 'role') && !pick}
                          onclick={() => addPicked(p)}>{t.common.add}</button
                        >
                      {/if}
                    </div>
                  {/if}
                </span>
              </div>
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>

  <h2 class="section-title">{t.settings.roles}</h2>
  <dl class="roles">
    {#each data.roles as r (r.id)}
      <div>
        <dt>{r.name}</dt>
        <dd>
          {#each r.groups as g (g.id)}<span class="chip">{holderLabels.group} {g.name}</span>{/each}
          {#each r.users as u (u.id)}<span class="chip">{u.name}</span>{/each}
          {#if !r.groups.length && !r.users.length}<span class="nobody">{t.settings.noMembers}</span
            >{/if}
        </dd>
      </div>
    {/each}
  </dl>

  {#if dirty}
    <div class="savebar" role="region" aria-label={t.settings.unsaved}>
      <span>{t.settings.unsaved}</span>
      <button class="btn btn-quiet" onclick={() => data && ((grants = data.grants), undefined)}
        >{t.settings.discard}</button
      >
      <button class="btn btn-primary" disabled={saving} onclick={() => void save()}
        >{saving ? t.common.saving : t.common.save}</button
      >
    </div>
  {/if}
{/if}

<style>
  .matrix th {
    width: 280px;
    text-align: left;
    text-transform: none;
    letter-spacing: 0;
    vertical-align: top;
    padding: 9px 16px 9px 0;
    border-bottom: 1px solid var(--rule);
  }

  .label {
    display: block;
    font: 600 14px/1.3 var(--font-body);
    color: var(--ink);
  }

  .help {
    display: block;
    font: 400 12.5px/1.35 var(--font-body);
    color: var(--muted);
  }

  .matrix td {
    vertical-align: top;
    padding-top: 10px;
  }

  .matrix tbody tr:hover td {
    background: none;
  }

  .holders {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    align-items: center;
  }

  .holder {
    font-family: var(--font-body);
    font-size: 12.5px;
  }

  .holder .kind {
    font: 500 11px/1 var(--font-body);
    color: var(--muted);
  }

  .holder.everyone {
    border-color: var(--ink-soft);
    font-weight: 600;
  }

  .nobody {
    color: var(--muted);
    font-style: italic;
    font-size: 13px;
  }

  .adder {
    position: relative;
    font-size: 12.5px;
  }

  .add {
    left: 0;
    top: calc(100% + 6px);
    width: 280px;
    padding: 12px;
    display: grid;
    gap: 10px;
    justify-items: start;
  }

  .add .field {
    width: 100%;
  }

  .boxed {
    border: 1px solid var(--rule-strong);
    padding: 6px 8px;
  }

  .roles {
    margin: 0;
    display: grid;
    gap: 6px;
  }

  .roles div {
    display: grid;
    grid-template-columns: 200px 1fr;
    gap: 10px;
    padding: 6px 0;
    border-bottom: 1px solid var(--rule);
  }

  .roles dt {
    font-weight: 600;
  }

  .roles dd {
    margin: 0;
    display: flex;
    flex-wrap: wrap;
    gap: 4px;
  }

  /* A phone: a role's name sits over its members, and the grid scrolls in its box. */
  @media (max-width: 600px) {
    .roles div {
      grid-template-columns: minmax(0, 1fr);
      gap: 4px;
    }

    .matrix th {
      width: 220px;
    }
  }
</style>
