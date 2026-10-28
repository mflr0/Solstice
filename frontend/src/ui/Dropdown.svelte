<script lang="ts" module>
  /** One choice of a Dropdown. */
  export interface DropdownOption {
    value: string;
    label: string;
    /** The heading this option sits under; consecutive options share one. */
    group?: string | undefined;
    /** The language of the label, when it differs from the page's. */
    lang?: string | undefined;
  }
</script>

<script lang="ts">
  import type { Snippet } from 'svelte';
  import { t } from '../i18n/t.js';

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
    onblur?: (() => void) | undefined;
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
    onblur,
  }: Props = $props();

  const uid = $props.id();
  const listId = `dd-${uid}`;
  const optionId = (i: number) => `dd-${uid}-${i}`;

  let button: HTMLButtonElement | undefined = $state();
  let list: HTMLUListElement | undefined = $state();
  let open = $state(false);
  let active = $state(-1);

  const selected = $derived(options.findIndex((o) => o.value === value));
  const current = $derived(options[selected]);

  /** Runs of consecutive options under the same heading, keeping their indexes. */
  const sections = $derived.by(() => {
    const out: { group: string | undefined; items: { o: DropdownOption; i: number }[] }[] = [];
    options.forEach((o, i) => {
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
    active = at >= 0 ? at : 0;
    open = true;
  }

  function hide() {
    open = false;
  }

  function choose(i: number) {
    const o = options[i];
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

  // A native listener: Svelte's delegated one would stop Escape only once
  // the handlers around (a menu, a dialog) had already seen it.
  $effect(() => {
    const el = button;
    if (!el) return;
    el.addEventListener('keydown', keydown);
    return () => el.removeEventListener('keydown', keydown);
  });

  $effect(() => {
    if (!open || !list) return;
    list.querySelector(`#${CSS.escape(optionId(active))}`)?.scrollIntoView?.({ block: 'nearest' });
  });

  /** Shows the list in the top layer, under the button or above it, inside the window. */
  function layer(el: HTMLUListElement) {
    el.showPopover?.();
    const place = () => {
      if (!button) return;
      const r = button.getBoundingClientRect();
      const width = document.documentElement.clientWidth;
      const height = document.documentElement.clientHeight;
      el.style.minWidth = `${r.width}px`;
      el.style.maxHeight = '';
      el.style.left = `${Math.max(8, Math.min(r.left, width - 8 - el.offsetWidth))}px`;
      const below = height - r.bottom - 12;
      const above = r.top - 12;
      if (el.offsetHeight > below && above > below) {
        el.style.top = '';
        el.style.bottom = `${height - r.top + 4}px`;
        el.style.maxHeight = `${above}px`;
      } else {
        el.style.bottom = '';
        el.style.top = `${r.bottom + 4}px`;
        el.style.maxHeight = `${below}px`;
      }
    };
    place();
    const onPointer = (e: PointerEvent) => {
      const target = e.target as Node;
      if (!el.contains(target) && !button?.contains(target)) hide();
    };
    const onScroll = (e: Event) => {
      if (e.target !== el) place();
    };
    document.addEventListener('pointerdown', onPointer, true);
    addEventListener('scroll', onScroll, { capture: true, passive: true });
    addEventListener('resize', place);
    return {
      destroy() {
        document.removeEventListener('pointerdown', onPointer, true);
        removeEventListener('scroll', onScroll, { capture: true });
        removeEventListener('resize', place);
      },
    };
  }
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
  aria-activedescendant={open && active >= 0 ? optionId(active) : undefined}
  {disabled}
  onclick={() => (open ? hide() : show())}
  onblur={() => {
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
{#if open}
  <!-- The button keeps the focus and handles the keys; a press here only must not take it. -->
  <ul
    bind:this={list}
    id={listId}
    class="popover dropdown-list"
    role="listbox"
    aria-label={label}
    popover="manual"
    use:layer
    onmousedown={(e) => e.preventDefault()}
  >
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
  </ul>
{/if}

{#snippet item(o: DropdownOption, i: number)}
  <!-- svelte-ignore a11y_click_events_have_key_events -->
  <li
    id={optionId(i)}
    class="item"
    class:active={i === active}
    role="option"
    aria-selected={i === selected}
    onpointermove={() => (active = i)}
    onclick={() => choose(i)}
  >
    {#if icon}<span class="icon" aria-hidden="true">{@render icon(o.value)}</span>{/if}
    <span class="name" lang={o.lang}>{o.label}</span>
  </li>
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
    position: fixed;
    inset: auto;
    margin: 0;
    list-style: none;
    max-width: calc(100vw - 16px);
    color: var(--ink);
  }

  .dropdown-list .item {
    white-space: nowrap;
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
</style>
