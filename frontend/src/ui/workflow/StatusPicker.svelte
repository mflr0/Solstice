<script lang="ts">
  import type { StatusCategory, StatusRef } from '../../core/types.js';
  import { CATEGORIES, searchStatuses, statusNamed } from '../../core/workflow.js';
  import { categoryLabels, t } from '../../i18n/t.js';
  import { autofocus, outside } from '../../lib/actions.js';
  import { api, errorMessage } from '../../lib/api.js';
  import Stamp from '../Stamp.svelte';

  /**
   * Adds a status to the workflow: a search in the shared catalogue, or a
   * new status (name and category) when none has that name.
   */
  interface Props {
    projectKey: string;
    catalogue: StatusRef[];
    taken: StatusRef[];
    onpick: (s: StatusRef) => void;
  }
  const { projectKey, catalogue, taken, onpick }: Props = $props();

  let open = $state(false);
  let q = $state('');
  let active = $state(0);
  let busy = $state(false);
  let error = $state('');
  const listId = `statuses-${Math.random().toString(36).slice(2, 8)}`;

  const found = $derived(searchStatuses(catalogue, taken, q));
  const canCreate = $derived(q.trim() !== '' && !statusNamed(catalogue, q));

  function toggle() {
    open = !open;
    q = '';
    active = 0;
    error = '';
  }

  function pick(s: StatusRef) {
    open = false;
    onpick(s);
  }

  async function create(category: StatusCategory) {
    busy = true;
    error = '';
    try {
      const s = await api.createStatus({ name: q.trim(), category, project: projectKey });
      pick(s);
    } catch (e) {
      error = errorMessage(e);
    } finally {
      busy = false;
    }
  }

  function keydown(e: KeyboardEvent) {
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      active = Math.min(found.length - 1, active + 1);
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      active = Math.max(0, active - 1);
    } else if (e.key === 'Enter') {
      e.preventDefault();
      const s = found[active];
      if (s) pick(s);
    }
  }
</script>

<div class="adder" use:outside={() => (open = false)}>
  <button type="button" class="link-btn" aria-expanded={open} onclick={toggle}
    >+ {t.workflows.addStatus}</button
  >
  {#if open}
    <div class="popover">
      <input
        class="input filter"
        bind:value={q}
        oninput={() => (active = 0)}
        onkeydown={keydown}
        placeholder={t.workflows.searchStatus}
        aria-label={t.workflows.searchStatus}
        aria-controls={listId}
        aria-activedescendant={found[active] ? `${listId}-${active}` : undefined}
        role="combobox"
        aria-expanded="true"
        aria-autocomplete="list"
        use:autofocus
      />
      <ul id={listId} role="listbox" aria-label={t.workflows.statuses}>
        {#each found as s, i (s.id)}
          <li id="{listId}-{i}" role="option" aria-selected={i === active}>
            <button
              type="button"
              class="item"
              class:active={i === active}
              tabindex="-1"
              onclick={() => pick(s)}
              onmouseenter={() => (active = i)}
            >
              <Stamp status={s} />
              <span class="muted cat">{categoryLabels[s.category]}</span>
            </button>
          </li>
        {/each}
      </ul>
      {#if found.length === 0 && !canCreate}
        <p class="none">{t.workflows.noMatch}</p>
      {/if}
      {#if canCreate}
        <div class="sep"></div>
        <fieldset class="create" disabled={busy}>
          <legend>{t.workflows.createStatus(q.trim())}</legend>
          <div class="cats" role="group" aria-label={t.workflows.createAs}>
            {#each CATEGORIES as c (c)}
              <button type="button" class="btn btn-small" onclick={() => void create(c)}
                >{categoryLabels[c]}</button
              >
            {/each}
          </div>
          <p class="hint">{t.workflows.sharedStatuses}</p>
        </fieldset>
      {/if}
      {#if error}<p class="error-text" role="alert">{error}</p>{/if}
    </div>
  {/if}
</div>

<style>
  .adder {
    position: relative;
    font-size: 13px;
  }

  .popover {
    left: 0;
    top: calc(100% + 6px);
    width: 300px;
    max-height: 380px;
  }

  .filter {
    margin: 4px 8px 6px;
    width: calc(100% - 16px);
    height: 30px;
  }

  ul {
    list-style: none;
    margin: 0;
    padding: 0;
  }

  .cat {
    margin-left: auto;
    font-size: 12px;
  }

  .none {
    margin: 4px 12px 8px;
    color: var(--muted);
    font-style: italic;
    font-size: 13px;
  }

  .create {
    border: 0;
    margin: 0;
    padding: 4px 12px 8px;
    display: grid;
    gap: 6px;
  }

  .create legend {
    padding: 0;
    margin-bottom: 6px;
    font: 500 12.5px/1.3 var(--font-body);
    color: var(--ink);
    overflow-wrap: anywhere;
  }

  .cats {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
  }

  .hint {
    margin: 0;
    font-size: 12px;
    color: var(--muted);
  }

  .error-text {
    margin: 0 12px 8px;
    font-size: 13px;
  }
</style>
