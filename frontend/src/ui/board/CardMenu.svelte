<script lang="ts" module>
  interface MenuItem {
    label: string;
    run: () => void;
    disabled?: boolean;
  }

  export interface MenuGroup {
    label: string;
    items: MenuItem[];
  }
</script>

<script lang="ts">
  import { flushSync } from 'svelte';

  /**
   * A card's menu: the keyboard (and touch) way to do what dragging does.
   * It opens in the top layer, so neither the board's scrolling nor a
   * skin's tilted cards clip or shift it. The entries are built on opening.
   */
  interface Props {
    label: string;
    groups: () => MenuGroup[];
  }
  const { label, groups }: Props = $props();

  let button: HTMLButtonElement | undefined = $state();
  let menu: HTMLDivElement | undefined = $state();
  let open = $state(false);
  let shown = $state<MenuGroup[]>([]);
  const uid = $props.id();
  const id = `card-menu-${uid}`;

  /** Builds the entries and places the menu before the browser shows it. */
  function beforeToggle(e: ToggleEvent) {
    if (e.newState !== 'open' || !menu || !button) return;
    flushSync(() => {
      shown = groups();
      open = true;
    });
    place(button, menu);
  }

  /**
   * Under the button, right-aligned; above it when the menu would not fit
   * below. The menu is still hidden here, so its height is estimated from
   * its entries.
   */
  function place(anchor: HTMLElement, el: HTMLElement) {
    const r = anchor.getBoundingClientRect();
    const rows = shown.reduce((n, g) => n + g.items.length + 1, 0);
    const needed = rows * 32 + shown.length * 10;
    const below = innerHeight - r.bottom;
    el.style.right = `${Math.max(8, innerWidth - r.right)}px`;
    if (below < needed && r.top > below) {
      el.style.top = 'auto';
      el.style.bottom = `${innerHeight - r.top + 4}px`;
      el.style.maxHeight = `${r.top - 12}px`;
    } else {
      el.style.bottom = 'auto';
      el.style.top = `${r.bottom + 4}px`;
      el.style.maxHeight = `${below - 12}px`;
    }
  }

  function items(): HTMLButtonElement[] {
    return menu ? [...menu.querySelectorAll<HTMLButtonElement>('.item:not([disabled])')] : [];
  }

  function onToggle(e: ToggleEvent) {
    open = e.newState === 'open';
    if (open) {
      items()[0]?.focus();
      return;
    }
    // Back to the button, unless the user moved on to something else.
    const active = document.activeElement;
    if (!active || active === document.body || menu?.contains(active)) button?.focus();
  }

  function onKey(e: KeyboardEvent) {
    const list = items();
    const at = list.indexOf(document.activeElement as HTMLButtonElement);
    let next = -1;
    if (e.key === 'ArrowDown') next = (at + 1) % list.length;
    else if (e.key === 'ArrowUp') next = (at - 1 + list.length) % list.length;
    else if (e.key === 'Home') next = 0;
    else if (e.key === 'End') next = list.length - 1;
    else if (e.key === 'Tab') menu?.hidePopover();
    if (next >= 0) {
      e.preventDefault();
      list[next]?.focus();
    }
  }

  function choose(item: MenuItem) {
    menu?.hidePopover();
    button?.focus();
    item.run();
  }
</script>

<button
  bind:this={button}
  type="button"
  class="btn btn-quiet btn-small card-menu-toggle"
  aria-haspopup="menu"
  aria-expanded={open}
  aria-label={label}
  title={label}
  popovertarget={id}
  draggable="false"
>
  <svg viewBox="0 0 16 16" width="14" height="14" aria-hidden="true">
    <circle cx="3" cy="8" r="1.4" />
    <circle cx="8" cy="8" r="1.4" />
    <circle cx="13" cy="8" r="1.4" />
  </svg>
</button>
<div
  bind:this={menu}
  {id}
  popover="auto"
  class="popover card-menu"
  role="menu"
  tabindex="-1"
  aria-label={label}
  onbeforetoggle={beforeToggle}
  ontoggle={onToggle}
  onkeydown={onKey}
>
  {#each shown as group, g (group.label)}
    {#if g > 0}<div class="sep" role="separator"></div>{/if}
    <div role="group" aria-label={group.label}>
      <div class="group-label" aria-hidden="true">{group.label}</div>
      {#each group.items as item, i (i)}
        <button
          type="button"
          class="item"
          role="menuitem"
          disabled={item.disabled}
          onclick={() => choose(item)}>{item.label}</button
        >
      {/each}
    </div>
  {/each}
</div>

<style>
  .card-menu-toggle {
    padding: 0 4px;
    color: var(--muted);
  }

  .card-menu-toggle:hover,
  .card-menu-toggle[aria-expanded='true'] {
    color: var(--ink);
  }

  svg {
    fill: currentColor;
  }

  /* In the top layer: fixed to the window, placed by script. */
  .card-menu {
    position: fixed;
    left: auto;
    margin: 0;
    min-width: 12.5rem;
    color: var(--ink);
  }

  .item[disabled] {
    color: var(--muted);
    cursor: default;
  }

  .item[disabled]:hover {
    background: none;
  }
</style>
