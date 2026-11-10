<script lang="ts">
  import { tick } from 'svelte';
  import type { LayoutZone } from '../../core/admin-types.js';
  import type { IssueDetail } from '../../core/types.js';
  import { t } from '../../i18n/t.js';
  import { refocus } from '../../lib/actions.js';
  import WikiEditor from '../WikiEditor.svelte';
  import WikiText from '../WikiText.svelte';
  import FieldFrame from './FieldFrame.svelte';

  /** A wiki text field of the issue view (description, environment, a text area), edited in place. */
  interface Props {
    issue: IssueDetail;
    zone: LayoutZone;
    label: string;
    source: string | null;
    editable: boolean;
    required?: boolean | undefined;
    /** What an empty field says. */
    empty?: string | undefined;
    /** The button that starts an empty field. */
    add?: string | undefined;
    /** Saves the text, null for none; false when refused. */
    onsave: (value: string | null) => Promise<boolean>;
  }
  const {
    issue,
    zone,
    label,
    source,
    editable,
    required = false,
    empty = '—',
    add = t.common.add,
    onsave,
  }: Props = $props();

  let editing = $state(false);
  let draft = $state('');
  let saving = $state(false);
  let button = $state<HTMLElement>();

  async function stop() {
    editing = false;
    await tick();
    refocus(button);
  }

  async function save() {
    saving = true;
    if (await onsave(draft.trim() ? draft : null)) await stop();
    saving = false;
  }
</script>

{#snippet start()}
  <button
    bind:this={button}
    class="btn btn-small"
    onclick={() => {
      draft = source ?? '';
      editing = true;
    }}>{source ? t.common.edit : add}</button
  >
{/snippet}

<FieldFrame {zone} {label} {required} actions={editable && !editing ? start : undefined}>
  {#if editing}
    <WikiEditor
      bind:value={draft}
      {label}
      rows={zone === 'main' ? 12 : 6}
      attachments={issue.attachments}
      autofocus
      onsubmit={() => void save()}
      oncancel={() => void stop()}
      uploadTo={issue.can.attach ? issue.key : undefined}
    />
    <div class="row end">
      <button class="btn btn-quiet" onclick={() => void stop()}>{t.common.cancel}</button>
      <button class="btn btn-primary" disabled={saving} onclick={() => void save()}
        >{saving ? t.common.saving : t.common.save}</button
      >
    </div>
  {:else if source}
    <WikiText {source} attachments={issue.attachments} />
  {:else if zone === 'main'}
    <p class="empty">{empty}</p>
  {:else}
    <span class="muted">{empty}</span>
  {/if}
</FieldFrame>

<style>
  .end {
    justify-content: flex-end;
    margin-top: 8px;
  }
</style>
