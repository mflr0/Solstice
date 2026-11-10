<script lang="ts">
  import type { Snippet } from 'svelte';
  import type { LayoutZone } from '../../core/admin-types.js';
  import { t } from '../../i18n/t.js';

  /**
   * A field of the issue view, framed for its zone: a titled section in the
   * main column, a ruled line of the side card (inside its `<dl>`).
   */
  interface Props {
    zone: LayoutZone;
    label: string;
    /** The id of the field's control, for a `<label for>`. */
    labelFor?: string | undefined;
    /** Shows the "Required" hint. */
    required?: boolean | undefined;
    actions?: Snippet | undefined;
    children: Snippet;
  }
  const { zone, label, labelFor, required = false, actions, children }: Props = $props();
</script>

{#snippet name()}
  {#if labelFor}<label for={labelFor}>{label}</label>{:else}{label}{/if}
  {#if required}<span class="req">{t.issue.required}</span>{/if}
{/snippet}

{#if zone === 'main'}
  <section class="field" aria-label={label}>
    <h2 class="section-title">
      <span>{@render name()}</span>
      {#if actions}<span class="actions">{@render actions()}</span>{/if}
    </h2>
    {@render children()}
  </section>
{:else}
  <div class="line card-row">
    <dt>{@render name()}</dt>
    <dd>
      {#if actions}<span class="side-actions">{@render actions()}</span>{/if}
      {@render children()}
    </dd>
  </div>
{/if}

<style>
  /* Ruled like an index card, in pale blue ink. */
  .line {
    display: grid;
    grid-template-columns: 7rem minmax(0, 1fr);
    gap: 10px;
    align-items: start;
    padding: 7px 14px 6px;
    border-bottom: 1px solid color-mix(in srgb, var(--rule) 70%, transparent);
    min-height: 2.125rem;
  }

  .line:last-child {
    border-bottom: none;
  }

  dt {
    font-size: 0.75rem;
    color: var(--muted);
    padding-top: 2px;
  }

  dd {
    margin: 0;
    min-width: 0;
  }

  .req {
    display: block;
    font-size: 0.6875rem;
    color: var(--accent-text);
  }

  h2 .req {
    display: inline;
    margin-left: 8px;
    font-weight: 400;
  }

  .side-actions {
    float: right;
    margin-left: 6px;
  }
</style>
