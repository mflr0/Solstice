<script lang="ts">
  import { paths } from '../../core/route.js';
  import { t } from '../../i18n/t.js';
  import { api } from '../../lib/api.js';
  import { router } from '../../lib/router.svelte.js';
  import { notify } from '../../lib/ui.svelte.js';
  import ConfirmDialog from '../ConfirmDialog.svelte';

  /** "Clone": asks for a summary and what to bring along, then opens the new issue. */
  interface Props {
    open: boolean;
    issueKey: string;
    onclose: () => void;
  }
  const { open, issueKey, onclose }: Props = $props();

  let summary = $state('');
  let includeSubtasks = $state(false);
  let includeLinks = $state(false);
  let includeAttachments = $state(false);

  // Each opening starts from a clean form.
  $effect(() => {
    if (!open) return;
    summary = '';
    includeSubtasks = false;
    includeLinks = false;
    includeAttachments = false;
  });

  async function clone() {
    const { key, warnings } = await api.cloneIssue(issueKey, {
      summary: summary.trim() || undefined,
      includeSubtasks,
      includeLinks,
      includeAttachments,
    });
    notify(t.create.created(key));
    for (const w of warnings) notify(w, 'error');
    router.navigate(paths.issue(key));
  }
</script>

<ConfirmDialog
  {open}
  title={t.issue.cloneTitle(issueKey)}
  confirm={t.issue.cloneSubmit}
  danger={false}
  onconfirm={clone}
  {onclose}
>
  <label class="field">
    <span>{t.create.summary}</span>
    <input class="input" bind:value={summary} maxlength="255" />
    <span class="hint">{t.issue.cloneSummaryHint}</span>
  </label>
  <label class="check">
    <input class="checkbox" type="checkbox" bind:checked={includeSubtasks} />
    <span>{t.issue.cloneSubtasks}</span>
  </label>
  <label class="check">
    <input class="checkbox" type="checkbox" bind:checked={includeLinks} />
    <span>{t.issue.cloneLinks}</span>
  </label>
  <label class="check">
    <input class="checkbox" type="checkbox" bind:checked={includeAttachments} />
    <span>{t.issue.cloneAttachments}</span>
  </label>
</ConfirmDialog>

<style>
  .check {
    display: flex;
    gap: 8px;
    align-items: center;
  }
</style>
