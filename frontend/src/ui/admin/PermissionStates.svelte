<script lang="ts">
  import {
    DEFAULT_STATES,
    PERMISSIONS,
    type DefaultsForm,
    type DefaultState,
  } from '../../core/admin-forms.js';
  import { permissionLabels, t } from '../../i18n/t.js';

  /** Each permission not set, allowed or denied: a holder's global rules. */
  interface Props {
    value: DefaultsForm;
    /** Who the rules are for, in each control's accessible name. */
    holder: string;
    /** Keeps the radio names and ids apart from other lists on the page. */
    prefix: string;
  }
  let { value = $bindable(), holder, prefix }: Props = $props();

  const marks: Record<DefaultState, string> = { unset: '/', allow: '✓', deny: '✗' };
  const stateLabels = $derived<Record<DefaultState, string>>({
    unset: t.adminArea.groupsPage.notSet,
    allow: t.settings.states.allow,
    deny: t.settings.states.deny,
  });
</script>

<ul class="defaults">
  {#each PERMISSIONS as p (p)}
    {@const perm = permissionLabels[p].label}
    <li>
      <span class="perm" id="{prefix}-perm-{p}" title={permissionLabels[p].help}>{perm}</span>
      <span class="states" role="radiogroup" aria-labelledby="{prefix}-perm-{p}">
        {#each DEFAULT_STATES as st (st)}
          <label class="state {st}" class:on={value[p] === st}>
            <input
              type="radio"
              class="sr-only"
              name="{prefix}-{p}"
              value={st}
              bind:group={value[p]}
              aria-label={t.adminArea.groupsPage.defaultName(holder, perm, stateLabels[st])}
            />
            <span aria-hidden="true">{marks[st]}</span>
            {stateLabels[st]}
          </label>
        {/each}
      </span>
    </li>
  {/each}
</ul>

<style>
  .defaults {
    display: grid;
    gap: 4px;
    margin: 0;
    padding: 0;
    list-style: none;
  }

  .defaults li {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    justify-content: space-between;
    gap: 4px 12px;
  }

  .perm {
    min-width: 0;
    overflow-wrap: anywhere;
  }

  /* Each state has its own glyph and word, so color is never the only cue. */
  .states {
    display: inline-flex;
    flex-wrap: wrap;
    gap: 2px;
  }

  /* Positioned so the hidden radio scrolls with the dialog's body when focused. */
  .state {
    position: relative;
    display: inline-flex;
    align-items: center;
    gap: 4px;
    padding: 2px 8px;
    border: 1px solid var(--rule);
    border-radius: 4px;
    color: var(--muted);
    font-size: 0.8125rem;
    cursor: pointer;
  }

  .state:hover {
    background: var(--accent-wash);
    color: var(--ink);
  }

  .state:has(input:focus-visible) {
    outline: 2px solid var(--focus);
    outline-offset: 1px;
  }

  .state.on {
    font-weight: 700;
  }

  .state.unset.on {
    background: var(--paper-sunk);
    color: var(--ink-soft);
  }

  .state.allow.on {
    border-color: var(--stamp-done);
    background: var(--stamp-done);
    color: var(--paper);
  }

  .state.deny.on {
    border-color: var(--tone-bug);
    background: var(--tone-bug);
    color: var(--paper);
  }
</style>
