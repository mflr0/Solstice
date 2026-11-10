<script lang="ts">
  import { refused, reportFailure } from '../../lib/ui.svelte.js';
  import { memberPayload } from '../../core/admin-forms.js';
  import type { Group } from '../../core/admin-types.js';
  import { groupKeyHolder, holderGroups } from '../../core/project-admin.js';
  import type { UserRef } from '../../core/types.js';
  import { holderLabels, t } from '../../i18n/t.js';
  import { adminApi, securityApi, type SecurityLevel } from '../../lib/admin-api.js';
  import { api, errorMessage } from '../../lib/api.js';
  import Dropdown from '../Dropdown.svelte';
  import Modal from '../Modal.svelte';
  import UserPicker from '../UserPicker.svelte';

  /**
   * Adding someone to a security level: a person, or a group (its members in the project),
   * the built-in ones first: everyone, the project lead, the reporter or the assignee of each issue.
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

  let kind = $state<'group' | 'user'>('group');
  let pick = $state('');
  let user = $state<UserRef | null>(null);
  let groups = $state<Group[]>([]);
  let tried = $state(false);
  let error = $state('');
  let busy = $state(false);

  const payload = $derived(
    kind === 'user' ? memberPayload('user', user?.id ?? null) : groupKeyHolder(pick),
  );
  const groupOptions = $derived(
    holderGroups(holderLabels, groups).map((g) => ({
      value: g.key,
      label: g.label,
      group: g.builtin ? t.builtInGroups.heading : t.builtInGroups.groups,
    })),
  );

  $effect(() => {
    if (!open) return;
    kind = 'group';
    pick = '';
    user = null;
    tried = false;
    error = '';
    void adminApi
      .groups()
      .then((g) => (groups = g))
      .catch((e: unknown) => (error = errorMessage(e)));
  });

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    tried = true;
    error = '';
    if (!payload) void refused(e.currentTarget as HTMLFormElement, t.securityAdmin.mustChoose);
    if (!level || !payload) return;
    busy = true;
    try {
      onsaved(await securityApi.addMember(project, level.id, payload));
      onclose();
    } catch (err) {
      error = reportFailure(err);
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
        value={kind}
        options={(['group', 'user'] as const).map((h) => ({ value: h, label: holderLabels[h] }))}
        onchange={(v) => {
          kind = v as 'group' | 'user';
          pick = '';
          user = null;
        }}
      />
    </div>
    {#if kind === 'group'}
      <div class="field">
        <span>{t.securityAdmin.chooseGroup}</span>
        <Dropdown
          label={t.securityAdmin.chooseGroup}
          value={pick}
          options={groupOptions}
          placeholder={t.securityAdmin.choose}
          invalid={tried && !payload}
          onchange={(v) => (pick = v)}
        />
      </div>
    {:else}
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
    {/if}
    {#if pick === 'builtin:everyone'}
      <p class="muted small">{t.securityAdmin.everyoneHint}</p>
    {/if}
    {#if tried && !payload}
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
