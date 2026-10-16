<script lang="ts">
  import type { Group, GroupDetail } from '../../core/admin-types.js';
  import { groupInUse, isBlocked, restrictedUsage, usageLines } from '../../core/people-admin.js';
  import { t } from '../../i18n/t.js';
  import { adminApi } from '../../lib/admin-api.js';
  import { ApiError } from '../../lib/api.js';
  import ConfirmDialog from '../ConfirmDialog.svelte';
  import Modal from '../Modal.svelte';
  import TypedConfirmDialog from '../TypedConfirmDialog.svelte';

  /**
   * Deleting a group, after showing where it is used. Comments or work logs
   * restricted to it forbid the deletion (a notice, no button); grants, role
   * and security level memberships go with it, so a group that has some
   * wants its name typed; an unused group asks a plain confirmation.
   */
  interface Props {
    /** The group to delete; null when the dialog is closed. */
    group: Group | null;
    /** Its detail (members, usage), read before opening. */
    detail: GroupDetail | null;
    ondeleted: (name: string) => void;
    onclose: () => void;
  }
  const { group, detail, ondeleted, onclose }: Props = $props();

  const usage = $derived(detail?.usage ?? null);
  const blocked = $derived(usage !== null && isBlocked(usage));
  const typed = $derived(usage !== null && !blocked && groupInUse(usage));
  const plain = $derived(usage !== null && !blocked && !typed);
  const lines = $derived(usage ? usageLines(usage) : []);

  async function remove() {
    if (!group) return;
    try {
      await adminApi.deleteGroup(group.id);
    } catch (e) {
      // Restricted since the dialog opened: say what keeps it.
      const left = e instanceof ApiError && e.status === 409 ? restrictedUsage(e.data) : null;
      if (left)
        throw new Error(t.groupsAdmin.restricted(left.comments, left.worklogs), { cause: e });
      throw e;
    }
    ondeleted(group.name);
  }
</script>

{#snippet uses(only: 'all' | 'restricted')}
  <ul class="uses">
    {#each lines.filter((l) => only === 'all' || l.kind === 'comments' || l.kind === 'worklogs') as l (l.kind)}
      <li>{t.groupsAdmin.usage[l.kind](l.count)}</li>
    {/each}
  </ul>
{/snippet}

<Modal
  open={group !== null && blocked}
  title={t.groupsAdmin.blockedTitle(group?.name ?? '')}
  {onclose}
>
  <div class="body">
    <p>{t.groupsAdmin.blockedBody}</p>
    {@render uses('restricted')}
  </div>
  {#snippet footer()}
    <button type="button" class="btn" onclick={onclose}>{t.common.close}</button>
  {/snippet}
</Modal>

<TypedConfirmDialog
  open={group !== null && typed}
  title={t.groupsAdmin.deleteTitle(group?.name ?? '')}
  phrase={group?.name ?? ''}
  label={t.groupsAdmin.typeToConfirm(group?.name ?? '')}
  confirm={t.common.delete}
  {onclose}
  onconfirm={remove}
>
  <p>{t.groupsAdmin.deleteUses}</p>
  {@render uses('all')}
  <p>{t.groupsAdmin.deleteMembers(group?.members ?? 0)}</p>
</TypedConfirmDialog>

<ConfirmDialog
  open={group !== null && plain}
  title={t.groupsAdmin.deleteTitle(group?.name ?? '')}
  confirm={t.common.delete}
  {onclose}
  onconfirm={remove}
>
  <p>{t.groupsAdmin.deleteUnused}</p>
  <p>{t.groupsAdmin.deleteMembers(group?.members ?? 0)}</p>
</ConfirmDialog>

<style>
  .body {
    display: grid;
    gap: 8px;
  }

  .body p {
    margin: 0;
  }

  .uses {
    margin: 0;
    padding-left: 18px;
    display: grid;
    gap: 2px;
  }
</style>
