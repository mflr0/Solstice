<script lang="ts" module>
  /** One choice of a Dropdown. */
  export interface DropdownOption {
    value: string;
    label: string;
    /** The heading this option sits under; consecutive options share one. */
    group?: string | undefined;
    /** The language of the label, when it differs from the page's. */
    lang?: string | undefined;
    /** More words the search finds it by, not shown (a code, another language's name). */
    keywords?: string[] | undefined;
    /** A line under the name in the list, e.g. what "Follow the browser" follows now. */
    hint?: string | undefined;
    /** Listed but not chosen. */
    disabled?: boolean | undefined;
  }

  /** The search field's texts, for a page without the application's catalogue (the setup wizard). */
  export interface DropdownMessages {
    filter: string;
    filterOptions: string;
    noMatch: string;
  }
</script>

<script lang="ts">
  import type { Snippet } from 'svelte';
  import { matches } from '../core/text.js';
  import { t } from '../i18n/t.js';
  import { layer } from '../lib/actions.js';
  import Highlight from './Highlight.svelte';

  /**
   * One value from a short list, like a native select but drawn by the skin,
   * with an optional icon per option. The button keeps the focus and points
   * at the highlighted option; the list opens in the top layer, so neither a
   * dialog's scrolling nor the window's edge clips it.
   */
  interface Props {
    options: DropdownOption[];
    value: string;
    label: string;
    onchange: (value: string) => void;
    disabled?: boolean | undefined;
    /** Quiet, inline look (the issue's sidebar) instead of a form field. */
    quiet?: boolean | undefined;
    /** The icon drawn before an option's name, given its value. */
    icon?: Snippet<[string]> | undefined;
    /** Shown when the value matches no option; it can't be chosen. */
    placeholder?: string | undefined;
    id?: string | undefined;
    /** Extra classes on the button, for global styles (a skin, app.css). */
    class?: string | undefined;
    invalid?: boolean | undefined;
    required?: boolean | undefined;
    /** Takes the focus once mounted and enabled, also after its dialog has opened. */
    autofocus?: boolean | undefined;
    onblur?: (() => void) | undefined;
    /** A field at the top of the open list filters the options; it takes the focus. */
    search?: boolean | undefined;
    /** The search field's texts; the application's catalogue by default. */
    messages?: DropdownMessages | undefined;
  }
  const {
    options,
    value,
    label,
    onchange,
    disabled = false,
    quiet = false,
    icon,
    placeholder,
    id,
    class: className = '',
    invalid = false,
    required = false,
    autofocus = false,
    onblur,
    search = false,
    messages,
  }: Props = $props();

  const words = $derived(messages ?? t.common);

  const uid = $props.id();
  const listId = `dd-${uid}`;
  const optionId = (i: number) => `dd-${uid}-${i}`;

  let button: HTMLButtonElement | undefined = $state();
  let list: HTMLElement | undefined = $state();
  let field: HTMLInputElement | undefined = $state();
  let open = $state(false);
  let active = $state(-1);

  $effect(() => {
    if (autofocus && !disabled) queueMicrotask(() => button?.focus());
  });
  let query = $state('');

  const selected = $derived(options.findIndex((o) => o.value === value));
  const current = $derived(options[selected]);

  /** The indexes of the options listed: those the search matches. */
  const shown = $derived(
    options.flatMap((o, i) =>
      !search || matches(query, o.label, ...(o.keywords ?? [])) ? [i] : [],
    ),
  );

  /** Runs of consecutive options under the same heading, keeping their indexes. */
  const sections = $derived.by(() => {
    const out: { group: string | undefined; items: { o: DropdownOption; i: number }[] }[] = [];
    shown.forEach((i) => {
      const o = options[i]!;
      const last = out[out.length - 1];
      if (last && last.group === o.group) last.items.push({ o, i });
      else out.push({ group: o.group, items: [{ o, i }] });
    });
    return out;
  });

  let typed = '';
  let typedAt = 0;

  function show(at = selected) {
    if (disabled || !options.length) return;
    query = '';
    active = at >= 0 ? at : 0;
    open = true;
  }

  /** The option `by` places from the highlighted one among those listed, without wrapping. */
  function step(by: number): number {
    if (!shown.length) return -1;
    const at = shown.indexOf(active);
    return shown[Math.max(0, Math.min(shown.length - 1, at + by))]!;
  }

  function filter() {
    if (!shown.includes(active)) active = shown[0] ?? -1;
  }

  function hide() {
    open = false;
  }

  function choose(i: number) {
    const o = options[i];
    if (o?.disabled) return;
    hide();
    button?.focus();
    if (o && o.value !== value) onchange(o.value);
  }

  /** The next option whose name starts with what was typed in the last half second. */
  function typeahead(key: string): number {
    const now = Date.now();
    typed = now - typedAt > 500 ? key : typed + key;
    typedAt = now;
    const needle = typed.toLowerCase();
    // The same letter again steps through the options starting with it.
    const repeat = [...needle].every((c) => c === needle[0]);
    const from = open ? active : selected;
    const start = repeat || from < 0 ? from + 1 : from;
    const search = repeat ? needle[0]! : needle;
    for (let k = 0; k < options.length; k++) {
      const i = (start + k + options.length) % options.length;
      if (options[i]!.label.toLowerCase().startsWith(search)) return i;
    }
    return -1;
  }

  function keydown(e: KeyboardEvent) {
    const last = options.length - 1;
    const printable = e.key.length === 1 && !e.ctrlKey && !e.metaKey && !e.altKey;
    const typing = printable && (e.key !== ' ' || Date.now() - typedAt < 500);
    if (search && printable && e.key !== ' ' && !open) {
      // The letter starts the search.
      e.preventDefault();
      show();
      query = e.key;
      filter();
      return;
    }
    if (typing) {
      e.preventDefault();
      const i = typeahead(e.key);
      if (i < 0) return;
      if (open) active = i;
      else show(i);
      return;
    }
    if (!open) {
      if (['ArrowDown', 'ArrowUp', 'Enter', ' '].includes(e.key)) show();
      else if (e.key === 'Home') show(0);
      else if (e.key === 'End') show(last);
      else return;
      e.preventDefault();
      return;
    }
    switch (e.key) {
      case 'ArrowDown':
        active = Math.min(last, active + 1);
        break;
      case 'ArrowUp':
        if (e.altKey) choose(active);
        else active = Math.max(0, active - 1);
        break;
      case 'Home':
        active = 0;
        break;
      case 'End':
        active = last;
        break;
      case 'PageDown':
        active = Math.min(last, active + 10);
        break;
      case 'PageUp':
        active = Math.max(0, active - 10);
        break;
      case 'Enter':
      case ' ':
        choose(active);
        break;
      case 'Escape':
        // Only the list closes, not the dialog or menu around it.
        e.stopPropagation();
        hide();
        break;
      case 'Tab':
        choose(active);
        return;
      default:
        return;
    }
    e.preventDefault();
  }

  /** The search field's keys: the caret keeps Home, End and the letters. */
  function fieldKeydown(e: KeyboardEvent) {
    switch (e.key) {
      case 'ArrowDown':
        active = step(1);
        break;
      case 'ArrowUp':
        if (e.altKey) choose(active);
        else active = step(-1);
        break;
      case 'PageDown':
        active = step(10);
        break;
      case 'PageUp':
        active = step(-10);
        break;
      case 'Enter':
        choose(active);
        break;
      case 'Escape':
        e.stopPropagation();
        hide();
        button?.focus();
        break;
      case 'Tab':
        choose(active);
        return;
      default:
        return;
    }
    e.preventDefault();
  }

  /** Whether the focus went from one part of the dropdown to another. */
  const inside = (e: FocusEvent) =>
    e.relatedTarget instanceof Node &&
    (e.relatedTarget === button || !!list?.contains(e.relatedTarget));

  // A native listener: Svelte's delegated one would stop Escape only once
  // the handlers around (a menu, a dialog) had already seen it.
  $effect(() => {
    const el = button;
    if (!el) return;
    el.addEventListener('keydown', keydown);
    return () => el.removeEventListener('keydown', keydown);
  });

  $effect(() => {
    const el = field;
    if (!el) return;
    el.addEventListener('keydown', fieldKeydown);
    el.focus();
    return () => el.removeEventListener('keydown', fieldKeydown);
  });

  $effect(() => {
    if (!open || !list) return;
    list.querySelector(`#${CSS.escape(optionId(active))}`)?.scrollIntoView?.({ block: 'nearest' });
  });
</script>

<button
  bind:this={button}
  {id}
  type="button"
  class="dropdown {className}"
  class:select={!quiet}
  class:quiet
  role="combobox"
  aria-label={label}
  aria-haspopup="listbox"
  aria-invalid={invalid || undefined}
  aria-required={required || undefined}
  aria-expanded={open}
  aria-controls={open ? listId : undefined}
  aria-activedescendant={open && !search && active >= 0 ? optionId(active) : undefined}
  {disabled}
  onclick={() => (open ? hide() : show())}
  onblur={(e) => {
    if (search && inside(e)) return;
    hide();
    onblur?.();
  }}
>
  {#if current}
    {#if icon}<span class="icon" aria-hidden="true">{@render icon(current.value)}</span>{/if}
    <span class="name" lang={current.lang}>{current.label}</span>
  {:else}
    <span class="name none">{placeholder ?? t.common.none}</span>
  {/if}
</button>
{#if open && search}
  <!-- The field keeps the focus and handles the keys; a press elsewhere here only must not take it. -->
  <div
    bind:this={list}
    class="popover dropdown-list searchable"
    popover="manual"
    use:layer={{ anchor: button, matchWidth: true, onoutside: hide }}
    onmousedown={(e) => {
      if (e.target !== field) e.preventDefault();
    }}
  >
    <input
      bind:this={field}
      value={query}
      class="input filter"
      type="text"
      role="combobox"
      autocomplete="off"
      spellcheck="false"
      aria-label={words.filterOptions}
      aria-autocomplete="list"
      aria-expanded="true"
      aria-controls={listId}
      aria-activedescendant={active >= 0 ? optionId(active) : undefined}
      placeholder={words.filter}
      oninput={(e) => {
        query = e.currentTarget.value;
        filter();
      }}
      onblur={(e) => {
        if (inside(e)) return;
        hide();
        onblur?.();
      }}
    />
    <ul id={listId} role="listbox" aria-label={label}>
      {@render rows()}
    </ul>
    <p class="no-match" role="status">{shown.length ? '' : words.noMatch}</p>
  </div>
{:else if open}
  <!-- The button keeps the focus and handles the keys; a press here only must not take it. -->
  <ul
    bind:this={list}
    id={listId}
    class="popover dropdown-list"
    role="listbox"
    aria-label={label}
    popover="manual"
    use:layer={{ anchor: button, matchWidth: true, onoutside: hide }}
    onmousedown={(e) => e.preventDefault()}
  >
    {@render rows()}
  </ul>
{/if}

{#snippet rows()}
  {#each sections as sec, k (sec.items[0]!.o.value)}
    {#if sec.group === undefined}
      {#each sec.items as { o, i } (o.value)}
        {@render item(o, i)}
      {/each}
    {:else}
      <li role="group" aria-labelledby="{listId}-g{k}">
        <span id="{listId}-g{k}" class="group-label" role="presentation">{sec.group}</span>
        <ul class="group" role="none">
          {#each sec.items as { o, i } (o.value)}
            {@render item(o, i)}
          {/each}
        </ul>
      </li>
    {/if}
  {/each}
{/snippet}

{#snippet item(o: DropdownOption, i: number)}
  <!-- svelte-ignore a11y_click_events_have_key_events -->
  <li
    id={optionId(i)}
    class="item"
    class:active={i === active}
    role="option"
    aria-selected={i === selected}
    aria-disabled={o.disabled || undefined}
    onpointermove={() => (active = i)}
    onclick={() => choose(i)}
  >
    {#if icon}<span class="icon" aria-hidden="true">{@render icon(o.value)}</span>{/if}
    {#if o.hint}
      <span class="text">
        {@render name(o)}
        <span class="option-hint">{o.hint}</span>
      </span>
    {:else}
      {@render name(o)}
    {/if}
  </li>
{/snippet}

{#snippet name(o: DropdownOption)}
  <span class="name" lang={o.lang}
    >{#if search}<Highlight text={o.label} search={query} />{:else}{o.label}{/if}</span
  >
{/snippet}

<style>
  .dropdown {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    min-width: 0;
    max-width: 100%;
    text-align: left;
    cursor: pointer;
  }

  .dropdown.select {
    display: flex;
  }

  .dropdown[disabled] {
    cursor: default;
  }

  .quiet {
    border: 1px solid transparent;
    background: transparent;
    padding: 1px 18px 1px 4px;
    margin-left: -5px;
    background-image:
      linear-gradient(45deg, transparent 50%, var(--muted) 50%),
      linear-gradient(135deg, var(--muted) 50%, transparent 50%);
    background-position:
      right 7px center,
      right 3px center;
    background-size: 4px 4px;
    background-repeat: no-repeat;
  }

  .quiet:hover:not([disabled]),
  .quiet[aria-expanded='true'] {
    border-color: var(--rule-strong);
    background-color: var(--paper);
  }

  .name {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .none {
    color: var(--muted);
    font-style: italic;
  }

  .icon {
    display: inline-flex;
    flex: none;
  }

  /* In the top layer: fixed to the window, placed by script. */
  .dropdown-list {
    list-style: none;
    max-width: calc(100vw - 16px);
    color: var(--ink);
  }

  .dropdown-list .item {
    white-space: nowrap;
  }

  .dropdown-list .item[aria-disabled='true'] {
    color: var(--muted);
    cursor: not-allowed;
  }

  .dropdown-list .group {
    list-style: none;
    margin: 0;
    padding: 0;
  }

  .dropdown-list .group-label {
    display: block;
  }

  .dropdown-list .item .name {
    flex: 1;
  }

  .dropdown-list .text {
    display: grid;
    flex: 1;
    min-width: 0;
  }

  .option-hint {
    color: var(--muted);
    font-size: 0.75rem;
    white-space: normal;
  }

  /* The field stays at the top while the options scroll under it. */
  .dropdown-list.searchable {
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }

  .searchable > ul {
    flex: 1 1 auto;
    min-height: 0;
    overflow: auto;
    list-style: none;
    margin: 0;
    padding: 0;
  }

  .filter {
    flex: none;
    margin: 4px 8px 6px;
    width: calc(100% - 16px);
    height: 1.75rem;
  }

  .no-match {
    margin: 0;
    padding: 6px 12px;
    color: var(--muted);
    font-size: 0.84375rem;
    font-style: italic;
  }

  /* Kept in the page while empty, so that a screen reader announces the line. */
  .no-match:empty {
    padding: 0;
  }
</style>
