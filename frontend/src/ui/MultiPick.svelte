<script lang="ts">
  import { t } from '../i18n/t.js';
  import { autofocus, outside } from '../lib/actions.js';
  import { errorMessage } from '../lib/api.js';

  /**
   * Several values from a list (components, versions), narrowed by a filter.
   * The choice is applied when the list closes, as one change. With
   * `oncreate`, a name no option has can be created and selected.
   */
  interface Option {
    id: number;
    name: string;
    note?: string | undefined;
  }
  interface Props {
    options: Option[];
    selected: number[];
    label: string;
    onchange: (ids: number[]) => void;
    disabled?: boolean | undefined;
    /** Creates a value with that name and resolves to its id, once `options` has it. */
    oncreate?: ((name: string) => Promise<number>) | undefined;
  }
  const { options, selected, label, onchange, disabled = false, oncreate }: Props = $props();

  let open = $state(false);
  let draft = $state<number[]>([]);
  let q = $state('');
  let creating = $state(false);
  let error = $state('');
  const shown = $derived.by(() => {
    const needle = q.trim().toLowerCase();
    return needle ? options.filter((o) => o.name.toLowerCase().includes(needle)) : options;
  });
  const createName = $derived.by(() => {
    const name = q.trim();
    if (!oncreate || !name) return '';
    const lower = name.toLowerCase();
    return options.some((o) => o.name.toLowerCase() === lower) ? '' : name;
  });
  const names = $derived(
    selected
      .map((id) => options.find((o) => o.id === id)?.name)
      .filter((n): n is string => n !== undefined),
  );

  function toggleOpen() {
    if (disabled) return;
    if (open) close();
    else {
      draft = [...selected];
      q = '';
      error = '';
      open = true;
    }
  }

  function close() {
    if (!open) return;
    open = false;
    const before = [...selected].sort((a, b) => a - b).join();
    const after = [...draft].sort((a, b) => a - b).join();
    if (before !== after) onchange(draft);
  }

  function flip(id: number) {
    draft = draft.includes(id) ? draft.filter((x) => x !== id) : [...draft, id];
  }

  async function create() {
    if (!oncreate || !createName || creating) return;
    creating = true;
    error = '';
    try {
      const id = await oncreate(createName);
      if (!draft.includes(id)) draft = [...draft, id];
      q = '';
    } catch (e) {
      error = errorMessage(e);
    } finally {
      creating = false;
    }
  }

  function keydown(e: KeyboardEvent) {
    if (e.key !== 'Enter') return;
    e.preventDefault();
    const first = shown[0];
    if (first) flip(first.id);
    else void create();
  }
</script>

<div class="multi" use:outside={close}>
  <button
    type="button"
    class="current"
    aria-haspopup="true"
    aria-expanded={open}
    aria-label={label}
    {disabled}
    onclick={toggleOpen}
  >
    {#if names.length}
      <span class="values">{names.join(', ')}</span>
    {:else}
      <span class="none">{t.common.none}</span>
    {/if}
    {#if !disabled}<span class="caret" aria-hidden="true">▾</span>{/if}
  </button>
  {#if open}
    <div class="popover">
      <input
        class="input filter"
        bind:value={q}
        onkeydown={keydown}
        placeholder={t.common.filter}
        aria-label={t.common.filter}
        use:autofocus
      />
      <div role="menu" aria-label={label}>
        {#each shown as o (o.id)}
          <button
            type="button"
            class="item"
            role="menuitemcheckbox"
            aria-checked={draft.includes(o.id)}
            onclick={() => flip(o.id)}
          >
            <span>{o.name}</span>{#if o.note}<span class="muted note">{o.note}</span>{/if}
          </button>
        {:else}
          {#if !createName}<p class="empty pad">{t.common.none}</p>{/if}
        {/each}
        {#if createName}
          <button
            type="button"
            class="item create"
            role="menuitem"
            disabled={creating}
            onclick={() => void create()}
          >
            {t.common.createNamed(createName)}
          </button>
        {/if}
      </div>
      {#if error}<p class="error-text pad" role="alert">{error}</p>{/if}
    </div>
  {/if}
</div>

<style>
  .multi {
    position: relative;
    min-width: 0;
    width: 100%;
  }

  .current {
    display: flex;
    align-items: center;
    gap: 6px;
    width: 100%;
    min-height: 26px;
    border: 1px solid transparent;
    background: none;
    padding: 2px 4px;
    margin: -3px -5px;
    text-align: left;
    cursor: pointer;
  }

  .current:hover:not([disabled]) {
    border-color: var(--rule-strong);
    background: var(--paper);
  }

  .current[disabled] {
    cursor: default;
  }

  .values {
    overflow-wrap: anywhere;
  }

  .none {
    color: var(--muted);
    font-style: italic;
  }

  .caret {
    margin-left: auto;
    color: var(--muted);
    font-size: 11px;
  }

  .popover {
    left: -6px;
    top: calc(100% + 4px);
    width: 260px;
  }

  .filter {
    margin: 4px 8px 6px;
    width: calc(100% - 16px);
    height: 28px;
  }

  .create {
    overflow-wrap: anywhere;
  }

  .note {
    font-size: 12px;
  }

  .pad {
    margin: 0 12px;
  }
</style>
