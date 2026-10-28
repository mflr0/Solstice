<script lang="ts">
  import {
    holderNeedsId,
    LEVEL_HOLDERS,
    memberPayload,
    type LevelHolder,
  } from '../../core/admin-forms.js';
  import type { Group, RoleRow } from '../../core/admin-types.js';
  import type { UserRef } from '../../core/types.js';
  import { holderLabels, t } from '../../i18n/t.js';
  import { adminApi, securityApi, type SecurityLevel } from '../../lib/admin-api.js';
  import { api, errorMessage } from '../../lib/api.js';
  import Dropdown from '../Dropdown.svelte';
  import Modal from '../Modal.svelte';
  import UserPicker from '../UserPicker.svelte';

  /**
   * Adding someone to a security level: a group, a project role, a person,
   * everyone, or the project lead, the reporter or the assignee of each issue.
   */
  interface Props {
    open: boolean;
    project: string;
    /** The level the member joins. */
    level: SecurityLevel | null;
    onclose: () => void;
    onsaved: (level: SecurityLevel) => void;
  }
  const { open, project, level, onclose, onsaved }: Props = $props();

  let holder = $state<LevelHolder>('group');
  let pick = $state('');
  let user = $state<UserRef | null>(null);
  let groups = $state<Group[]>([]);
  let roles = $state<RoleRow[]>([]);
  let tried = $state(false);
  let error = $state('');
  let busy = $state(false);

  const chosen = $derived(holder === 'user' ? (user?.id ?? null) : pick ? Number(pick) : null);
  const payload = $derived(memberPayload(holder, chosen));

  $effect(() => {
    if (!open) return;
    holder = 'group';
    pick = '';
    user = null;
    tried = false;
    error = '';
    void Promise.all([adminApi.groups(), adminApi.roles()])
      .then(([g, r]) => {
        groups = g;
        roles = r;
      })
      .catch((e: unknown) => (error = errorMessage(e)));
  });

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    tried = true;
    error = '';
    if (!level || !payload) return;
    busy = true;
    try {
      onsaved(await securityApi.addMember(project, level.id, payload));
      onclose();
    } catch (err) {
      error = errorMessage(err);
    } finally {
      busy = false;
    }
  }
</script>

<Modal
  {open}
  title={level ? t.securityAdmin.addTitle(level.name) : t.securityAdmin.addMember}
  onclose={() => !busy && onclose()}
>
  <form id="security-member-form" class="member-form" onsubmit={submit} novalidate>
    <div class="field">
      <span>{t.securityAdmin.kind}</span>
      <Dropdown
        label={t.securityAdmin.kind}
        value={holder}
        options={LEVEL_HOLDERS.map((h) => ({ value: h, label: holderLabels[h] }))}
        onchange={(v) => {
          holder = v as LevelHolder;
          pick = '';
          user = null;
        }}
      />
    </div>
    {#if holder === 'group' || holder === 'role'}
      <div class="field">
        <span>{holder === 'group' ? t.securityAdmin.chooseGroup : t.securityAdmin.chooseRole}</span>
        <Dropdown
          label={holder === 'group' ? t.securityAdmin.chooseGroup : t.securityAdmin.chooseRole}
          value={pick}
          options={(holder === 'group' ? groups : roles).map((x) => ({
            value: String(x.id),
            label: x.name,
          }))}
          placeholder={t.securityAdmin.choose}
          invalid={tried && !payload}
          onchange={(v) => (pick = v)}
        />
      </div>
    {:else if holder === 'user'}
      <div class="field">
        <span>{t.securityAdmin.chooseUser}</span>
        <UserPicker
          field
          value={user}
          label={t.securityAdmin.chooseUser}
          allowNone={false}
          search={(q) => api.users(q)}
          onpick={(u) => (user = u)}
        />
      </div>
    {:else if holder === 'everyone'}
      <p class="muted small">{t.securityAdmin.everyoneHint}</p>
    {/if}
    {#if tried && !payload && holderNeedsId(holder)}
      <p class="field-error">{t.securityAdmin.mustChoose}</p>
    {/if}
    {#if error}<p class="error-text" role="alert">{error}</p>{/if}
  </form>
  {#snippet footer()}
    <button type="button" class="btn btn-quiet" disabled={busy} onclick={onclose}
      >{t.common.cancel}</button
    >
    <button class="btn btn-primary" type="submit" form="security-member-form" disabled={busy}
      >{busy ? t.common.saving : t.common.add}</button
    >
  {/snippet}
</Modal>

<style>
  .member-form {
    display: grid;
    gap: 14px;
  }

  .member-form p {
    margin: 0;
  }
</style>
