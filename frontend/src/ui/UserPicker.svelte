<script lang="ts">
  import type { UserRef } from '../core/types.js';
  import { t } from '../i18n/t.js';
  import { autofocus, outside } from '../lib/actions.js';
  import Who from './Who.svelte';

  /**
   * Choose a person: shows the current one, opens a searchable list. The
   * list comes from `search` (e.g. who can be assigned in this project).
   */
  interface Props {
    value: UserRef | null;
    label: string;
    search: (q: string) => Promise<UserRef[]>;
    onpick: (user: UserRef | null) => void;
    allowNone?: boolean | undefined;
    disabled?: boolean | undefined;
    noneLabel?: string | undefined;
    /** Draw it as a form field (a frame, like an input) rather than inline text. */
    field?: boolean | undefined;
  }
  const {
    value,
    label,
    search,
    onpick,
    allowNone = true,
    disabled = false,
    noneLabel = t.common.unassigned,
    field = false,
  }: Props = $props();

  let open = $state(false);
  let q = $state('');
  let results = $state<UserRef[]>([]);
  let active = $state(0);
  let timer: ReturnType<typeof setTimeout> | undefined;
  let seq = 0;
  const uid = $props.id();
  const listId = `users-${uid}`;

  /** The rows of the list: an optional "nobody" entry, then people. */
  const rows = $derived<(UserRef | null)[]>(allowNone ? [null, ...results] : results);

  async function load() {
    const mine = ++seq;
    try {
      const found = await search(q.trim());
      if (mine === seq) {
        results = found;
        active = 0;
      }
    } catch {
      if (mine === seq) results = [];
    }
  }

  function toggle() {
    if (disabled) return;
    open = !open;
    if (open) {
      q = '';
      void load();
    }
  }

  function typed() {
    clearTimeout(timer);
    timer = setTimeout(() => void load(), 180);
  }

  function pick(u: UserRef | null) {
    open = false;
    if ((u?.id ?? null) !== (value?.id ?? null)) onpick(u);
  }

  function keydown(e: KeyboardEvent) {
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      active = Math.min(rows.length - 1, active + 1);
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      active = Math.max(0, active - 1);
    } else if (e.key === 'Enter') {
      e.preventDefault();
      const row = rows[active];
      if (row !== undefined) pick(row);
    }
  }
</script>

<div class="picker" class:framed={field} use:outside={() => (open = false)}>
  <button
    type="button"
    class="current"
    class:select={field}
    aria-haspopup="listbox"
    aria-expanded={open}
    aria-label={label}
    {disabled}
    onclick={toggle}
  >
    {#if value}
      <Who user={value} link={false} />
    {:else}
      <span class="nobody">{noneLabel}</span>
    {/if}
    {#if !disabled && !field}<span class="caret" aria-hidden="true">▾</span>{/if}
  </button>
  {#if open}
    <div class="popover">
      <input
        class="input filter"
        bind:value={q}
        oninput={typed}
        onkeydown={keydown}
        placeholder={t.common.filter}
        aria-label={label}
        aria-controls={listId}
        use:autofocus
      />
      <ul id={listId} role="listbox" aria-label={label}>
        {#each rows as u, i (u?.id ?? 0)}
          <li role="option" aria-selected={i === active}>
            <button
              type="button"
              class="item"
              class:active={i === active}
              onclick={() => pick(u)}
              onmouseenter={() => (active = i)}
            >
              {#if u}
                <span class="name">{u.name}</span><span class="user muted">{u.username}</span>
              {:else}
                <span class="nobody">{noneLabel}</span>
              {/if}
            </button>
          </li>
        {/each}
      </ul>
    </div>
  {/if}
</div>

<style>
  .picker {
    position: relative;
    min-width: 0;
    width: 100%;
  }

  .current {
    display: flex;
    align-items: center;
    gap: 6px;
    width: 100%;
    text-align: left;
    cursor: pointer;
  }

  .current:not(.select) {
    min-height: 26px;
    border: 1px solid transparent;
    background: none;
    padding: 2px 4px;
    margin: -3px -5px;
  }

  .current:not(.select):hover:not([disabled]) {
    border-color: var(--rule-strong);
    background: var(--paper);
  }

  /* The avatar never makes the box taller than the skin's select. */
  .framed .current :global(.who) {
    height: 1lh;
  }

  .framed .current :global(.who .avatar.avatar) {
    width: auto;
    height: 100%;
    max-height: 20px;
    aspect-ratio: 1;
  }

  .framed .popover {
    left: 0;
  }

  .current[disabled] {
    cursor: default;
  }

  .caret {
    margin-left: auto;
    color: var(--muted);
    font-size: 11px;
  }

  .nobody {
    color: var(--muted);
    font-style: italic;
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

  ul {
    list-style: none;
    margin: 0;
    padding: 0;
  }

  .item .user {
    margin-left: auto;
    font: 11.5px var(--font-mono);
  }
</style>
