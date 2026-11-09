<script lang="ts">
  import { flushSync } from 'svelte';

  import { BUILTIN_KINDS, inSpace, MAX_GADGETS, type Gadget } from '../../core/dashboard.js';
  import { paths } from '../../core/route.js';
  import type { SavedFilter } from '../../core/types.js';
  import { t } from '../../i18n/t.js';
  import Avatar from '../Avatar.svelte';

  /**
   * The New view button and its menu: a blank view, the ready-made views,
   * and the views shared with the user, each added at the end of the space.
   */
  interface Props {
    gadgets: Gadget[];
    filters: SavedFilter[];
    busy: boolean;
    onadd: (g: Gadget) => void;
  }
  const { gadgets, filters, busy, onadd }: Props = $props();

  let button: HTMLButtonElement | undefined = $state();
  let menu: HTMLDivElement | undefined = $state();
  let open = $state(false);
  const uid = $props.id();
  const id = `view-new-menu-${uid}`;

  const full = $derived(gadgets.length >= MAX_GADGETS);
  const shared = $derived(filters.filter((f) => !f.mine));

  function beforeToggle(e: ToggleEvent) {
    if (e.newState !== 'open' || !menu || !button) return;
    flushSync(() => (open = true));
    const r = button.getBoundingClientRect();
    // Still hidden, the menu has no width to measure: 22.5rem, narrowed to the window.
    const rem = parseFloat(getComputedStyle(document.documentElement).fontSize) || 16;
    const width = Math.min(22.5 * rem, innerWidth - 16);
    menu.style.width = `${width}px`;
    // Right-aligned to the button, but kept in the window when the button wraps to the left.
    menu.style.left = `${Math.max(8, Math.min(r.right - width, innerWidth - width - 8))}px`;
    menu.style.top = `${r.bottom + 4}px`;
    menu.style.maxHeight = `${Math.max(160, innerHeight - r.bottom - 12)}px`;
  }

  const items = () => (menu ? [...menu.querySelectorAll<HTMLElement>('[role="menuitem"]')] : []);

  function onToggle(e: ToggleEvent) {
    open = e.newState === 'open';
    if (open) {
      items()[0]?.focus();
      return;
    }
    const active = document.activeElement;
    if (!active || active === document.body || menu?.contains(active)) button?.focus();
  }

  function onKey(e: KeyboardEvent) {
    const list = items();
    const at = list.indexOf(document.activeElement as HTMLElement);
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

  function add(g: Gadget, disabled: boolean) {
    if (disabled) return;
    menu?.hidePopover();
    onadd(g);
  }

  const builtin = (kind: (typeof BUILTIN_KINDS)[number]): Gadget => ({
    id: 0,
    kind,
    filter: null,
    unavailable: false,
    config: {},
  });

  const onFilter = (f: SavedFilter): Gadget => ({
    id: 0,
    kind: 'results',
    filter: { id: f.id, name: f.name, query: f.query },
    unavailable: false,
    config: {},
  });
</script>

<button
  bind:this={button}
  id="view-new"
  type="button"
  class="btn"
  aria-haspopup="menu"
  aria-expanded={open}
  popovertarget={id}
>
  <svg viewBox="0 0 14 14" width="14" height="14" aria-hidden="true"
    ><path d="M7 2v10M2 7h10" /></svg
  >{t.home.newView}</button
>
<div
  bind:this={menu}
  {id}
  popover="auto"
  class="popover new-view-menu"
  role="menu"
  tabindex="-1"
  aria-label={t.home.newView}
  aria-describedby={full ? `${id}-full` : undefined}
  onbeforetoggle={beforeToggle}
  ontoggle={onToggle}
  onkeydown={onKey}
>
  <a class="item blank" role="menuitem" href={paths.search()} onclick={() => menu?.hidePopover()}>
    <span class="blank-mark" aria-hidden="true"
      ><svg viewBox="0 0 14 14" width="12" height="12"><path d="M7 2v10M2 7h10" /></svg></span
    >
    <span class="grow"
      ><span class="name">{t.home.blankView}</span><span class="hint">{t.home.blankViewHint}</span
      ></span
    >
  </a>
  {#if full}
    <p id="{id}-full" class="full">{t.home.full(MAX_GADGETS)}</p>
  {/if}

  <div class="sep" role="separator"></div>
  <div role="group" aria-labelledby="{id}-ready">
    <div id="{id}-ready" class="group-label">{t.home.readyMade}</div>
    {#each BUILTIN_KINDS as kind (kind)}
      {@const added = inSpace(gadgets, kind)}
      {@const disabled = added || full || busy}
      <button
        type="button"
        class="item"
        role="menuitem"
        aria-disabled={disabled}
        onclick={() => add(builtin(kind), disabled)}
      >
        <span class="grow">{t.home.kinds[kind]}</span>
        {#if added}<span class="state">{t.home.inYourSpace}</span>{/if}
      </button>
    {/each}
  </div>

  {#if shared.length > 0}
    <div class="sep" role="separator"></div>
    <div role="group" aria-labelledby="{id}-shared">
      <div id="{id}-shared" class="group-label">{t.home.sharedWithYou}</div>
      {#each shared as f (f.id)}
        {@const added = inSpace(gadgets, { filter: f.id })}
        {@const disabled = added || full || busy}
        <button
          type="button"
          class="item shared"
          role="menuitem"
          aria-disabled={disabled}
          onclick={() => add(onFilter(f), disabled)}
        >
          {#if f.owner}<Avatar user={f.owner} />{/if}
          <span class="grow"
            ><span class="name">{f.name}</span>{#if f.owner}<span class="hint">{f.owner.name}</span
              >{/if}</span
          >
          {#if added}
            <span class="state">{t.home.inYourSpace}</span>
          {:else}
            <span class="add btn btn-small">{t.home.add}</span>
          {/if}
        </button>
      {/each}
    </div>
  {/if}
</div>

<style>
  #view-new {
    display: inline-flex;
    align-items: center;
    gap: 8px;
  }

  svg {
    fill: none;
    stroke: currentColor;
    stroke-width: 1.6;
    stroke-linecap: round;
  }

  /* In the top layer: fixed to the window, placed by script. */
  .new-view-menu {
    position: fixed;
    margin: 0;
    max-height: none;
    padding: 4px;
  }

  .item {
    min-height: 2.25rem;
    border-radius: var(--radius-s);
  }

  .blank,
  .shared {
    min-height: 2.75rem;
    gap: 10px;
  }

  .blank {
    color: inherit;
  }

  .blank-mark {
    display: inline-grid;
    place-items: center;
    flex: none;
    width: 1.5rem;
    height: 1.5rem;
    border: 1px dashed var(--rule-strong);
    border-radius: 3px;
    color: var(--ink-soft);
  }

  .grow {
    flex: 1;
    min-width: 0;
  }

  .name {
    display: block;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .blank .name {
    font-weight: 500;
  }

  .hint {
    display: block;
    font-size: 0.75rem;
    color: var(--muted);
  }

  .state {
    flex: none;
    font-size: 0.75rem;
    color: var(--muted);
  }

  .add {
    flex: none;
  }

  .item[aria-disabled='true'] {
    cursor: default;
  }

  .item[aria-disabled='true']:hover {
    background: none;
  }

  .item[aria-disabled='true'] .add {
    opacity: 0.5;
  }

  .full {
    margin: 4px 12px 6px;
    font-size: 0.75rem;
    color: var(--muted);
  }

  .popover .group-label {
    padding-top: 8px;
    text-transform: uppercase;
    letter-spacing: 0.06em;
  }
</style>
