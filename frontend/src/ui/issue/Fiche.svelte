<script lang="ts">
  import { customFieldId } from '../../core/issue-view.js';
  import type { IssueDetail } from '../../core/types.js';
  import { t } from '../../i18n/t.js';
  import { session } from '../../lib/session.svelte.js';
  import Dropdown from '../Dropdown.svelte';
  import TypeMark from '../TypeMark.svelte';
  import FieldFrame from './FieldFrame.svelte';
  import type { FieldState } from './field-state.svelte.js';
  import IssueField from './IssueField.svelte';

  /**
   * The issue's record card: its type, then the fields the view layout puts
   * in the side zone, each on its own ruled line, editable in place for those
   * allowed to.
   */
  interface Props {
    issue: IssueDetail;
    fields: FieldState;
  }
  const { issue, fields }: Props = $props();

  const edit = $derived(issue.can.edit === true);
  const types = $derived(
    (fields.project?.issueTypes ?? []).filter((ty) => ty.subtask === issue.type.subtask),
  );
</script>

<aside class="fiche card" aria-label={t.issue.fiche}>
  <h2 class="head card-head title">{issue.key}</h2>
  <dl>
    <FieldFrame zone="side" label={t.issue.type}>
      {#if edit && types.length > 1}
        <Dropdown
          quiet
          label={t.issue.type}
          value={issue.type.id}
          options={types.map((ty) => ({ value: ty.id, label: session.typeName(ty) }))}
          onchange={(v) => void fields.onchange({ type: v })}
        >
          {#snippet icon(id)}
            <TypeMark type={types.find((ty) => ty.id === id) ?? issue.type} />
          {/snippet}
        </Dropdown>
      {:else}
        <TypeMark type={issue.type} withName />
      {/if}
    </FieldFrame>
    {#each fields.zones.side as field (field)}
      <IssueField {issue} {field} zone="side" {fields} />
    {/each}
    {#each fields.zones.unplaced as value (value.id)}
      <IssueField {issue} field={customFieldId(value.id)} zone="side" {fields} {value} />
    {/each}
  </dl>
</aside>

<style>
  .fiche {
    font-size: 0.84375rem;
  }

  h2.title {
    margin: 0;
  }

  /* Specific enough to beat every skin's own title-bar type; skins keep their margins. */
  aside.fiche.card > h2.title {
    justify-content: flex-start;
    font-size: 1.125rem;
    font-weight: 700;
    line-height: 1.2;
    letter-spacing: 0;
    overflow-wrap: anywhere;
  }

  dl {
    margin: 0;
  }

  dl :global(.wiki) {
    font-size: 0.84375rem;
  }
</style>
