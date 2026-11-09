<script lang="ts">
  import type { Snippet } from 'svelte';
  import { t } from '../../i18n/t.js';
  import { outside } from '../../lib/actions.js';

  /**
   * A filter's button and the panel it opens: a popover under the button,
   * a sheet at the bottom of the screen on a phone. The button reads
   * "Status: In progress, To do"; a × next to it clears the filter.
   */
  interface Props {
    label: string;
    /** The button's and the panel's accessible name, when the label alone is too short. */
    name?: string | undefined;
    /** A sign drawn before the label ("+"), hidden from screen readers. */
    lead?: string | undefined;
    /** The chosen values, as they read on the button. */
    summary?: string | undefined;
    active?: boolean | undefined;
    open: boolean;
    onclear?: (() => void) | undefined;
    /** Wider panel (dates, the field picker). */
    wide?: boolean | undefined;
    /** The panel stays where it opened while the button moves (chips added before it). */
    pinned?: boolean | undefined;
    children: Snippet;
  }
  let {
    label,
    name,
    lead,
    summary = '',
    active = false,
    open = $bindable(),
    onclear,
    wide = false,
    pinned = false,
    children,
  }: Props = $props();

  let button: HTMLButtonElement | undefined = $state();
  let wrap: HTMLDivElement | undefined = $state();
  const uid = $props.id();
  const id = `fp-${uid}`;

  function close(refocus: boolean) {
    if (!open) return;
    open = false;
    if (refocus) button?.focus();
  }

  /** Shifts the panel left when it would pass the right edge of the window. */
  function fit(el: HTMLElement) {
    // The phone's sheet is fixed and already spans the screen.
    if (getComputedStyle(el).position === 'fixed') return;
    // Pinned, the panel is placed in the wrap's positioned ancestor, under the button.
    let left = 0;
    if (pinned && button) {
      left = button.offsetLeft;
      el.style.left = `${left}px`;
      el.style.top = `${button.offsetTop + button.offsetHeight + 4}px`;
    }
    const r = el.getBoundingClientRect();
    const edge = document.documentElement.clientWidth - 8;
    if (r.right > edge) el.style.left = `${left + Math.max(8 - r.left, edge - r.right)}px`;
  }
</script>

<div
  class="filter-wrap"
  class:pinned
  bind:this={wrap}
  use:outside={() => close(false)}
  onfocusout={(e) => {
    // Tabbing out of the panel closes it; a click inside keeps it open.
    const to = e.relatedTarget;
    if (to instanceof Node && !wrap?.contains(to)) close(false);
  }}
>
  <button
    bind:this={button}
    type="button"
    class="btn btn-small filter-button"
    class:is-active={active}
    class:with-clear={active && !!onclear}
    aria-expanded={open}
    aria-controls={open ? id : undefined}
    aria-label={name}
    onclick={() => (open ? close(false) : (open = true))}
  >
    {#if lead}<span class="lead" aria-hidden="true">{lead}</span>{/if}
    <span class="filter-name"
      >{label}{#if summary}<span class="sep">{t.common.labelSep}</span>{/if}</span
    >
    {#if summary}<span class="filter-value">{summary}</span>{/if}
    {#if !lead}<span class="caret" aria-hidden="true">▾</span>{/if}
  </button>
  {#if active && onclear}
    <button
      type="button"
      class="btn btn-small filter-clear"
      aria-label={t.search.clear(label)}
      title={t.search.clear(label)}
      onclick={onclear}>×</button
    >
  {/if}
  {#if open}
    <!-- A phone's sheet sits on a veil; tapping the veil closes it. -->
    <!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
    <div class="filter-veil" onclick={() => close(true)}></div>
    <div
      class="popover filter-popover"
      class:wide
      {id}
      role="dialog"
      aria-label={name ?? label}
      tabindex="-1"
      use:fit
    >
      <div class="sheet-head">
        <strong>{name ?? label}</strong>
        <button type="button" class="btn btn-quiet btn-small" onclick={() => close(true)}
          >{t.search.close}</button
        >
      </div>
      {@render children()}
    </div>
  {/if}
</div>

<style>
  .filter-wrap {
    position: relative;
    display: inline-flex;
    min-width: 0;
    max-width: 100%;
  }

  .filter-wrap.pinned {
    position: static;
  }

  .filter-button {
    max-width: 100%;
    min-width: 0;
    gap: 4px;
  }

  .filter-button.is-active {
    border-color: var(--ink-soft);
    background-color: var(--paper-sunk);
    color: var(--ink);
  }

  .filter-button.with-clear {
    border-top-right-radius: 0;
    border-bottom-right-radius: 0;
  }

  .filter-name {
    flex: none;
  }

  .filter-value {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    font-weight: 400;
    max-width: 22ch;
  }

  .lead {
    font-size: 0.9375rem;
    line-height: 1;
  }

  .caret {
    color: var(--muted);
    font-size: 0.625rem;
  }

  .filter-clear {
    margin-left: calc(-1 * var(--frame));
    padding: 0 7px;
    border-top-left-radius: 0;
    border-bottom-left-radius: 0;
    border-color: var(--ink-soft);
    background-color: var(--paper-sunk);
    font-size: 0.875rem;
  }

  .filter-clear:hover {
    color: var(--accent-text);
  }

  .filter-popover {
    left: 0;
    top: calc(100% + 4px);
    width: 18.125rem;
    max-height: min(27.5rem, 70vh);
    padding: 0;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }

  .filter-popover.wide {
    width: 20rem;
  }

  .filter-popover:focus {
    outline: none;
  }

  .sheet-head,
  .filter-veil {
    display: none;
  }

  /* A phone: the panel is a sheet along the bottom of the screen. */
  @media (max-width: 600px) {
    .filter-veil {
      display: block;
      position: fixed;
      inset: 0;
      z-index: 58;
      background: var(--backdrop);
    }

    .filter-popover,
    .filter-popover.wide {
      position: fixed;
      left: 0;
      right: 0;
      top: auto;
      bottom: 0;
      z-index: 59;
      width: auto;
      max-height: 78vh;
      border-bottom: none;
      border-radius: var(--radius-l) var(--radius-l) 0 0;
    }

    .sheet-head {
      display: flex;
      align-items: center;
      justify-content: space-between;
      padding: 10px 12px 6px 16px;
      border-bottom: 1px solid var(--rule);
    }

    .filter-value {
      max-width: 14ch;
    }
  }
</style>
