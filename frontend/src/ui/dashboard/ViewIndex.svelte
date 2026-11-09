<script lang="ts">
  import { t } from '../../i18n/t.js';

  /**
   * The views of the user's space in page order, sticky beside them: the one
   * in sight stands out, a click jumps to another. On a narrow screen the list
   * folds into a button naming the view in sight.
   */
  interface Entry {
    /** The section's key: its element is view-section-<key>. */
    key: string;
    href: string;
    title: string;
    count: number | null;
  }
  interface Props {
    entries: Entry[];
    onjump: (key: string) => void;
  }
  const { entries, onjump }: Props = $props();

  let open = $state(false);
  let current = $state<string | null>(null);
  let nav: HTMLElement;
  let toggle: HTMLButtonElement;
  const uid = $props.id();
  const listId = `view-index-${uid}`;
  const keys = $derived(entries.map((e) => e.key).join(' '));
  const shown = $derived(entries.find((e) => e.key === current));

  // The topmost section in sight, below the bars: an observer rebuilt when they change size.
  $effect(() => {
    const order = keys.split(' ');
    // eslint-disable-next-line svelte/prefer-svelte-reactivity -- the observer's own bookkeeping
    const seen = new Set<string>();
    let observer: IntersectionObserver | undefined;
    const build = () => {
      observer?.disconnect();
      seen.clear();
      const bar = parseFloat(
        getComputedStyle(document.documentElement).getPropertyValue('--topbar-height'),
      );
      const folded = getComputedStyle(toggle).display !== 'none';
      const top = (bar || 0) + (folded ? toggle.offsetHeight : 0);
      observer = new IntersectionObserver(
        (changes) => {
          for (const c of changes) {
            const key = c.target.id.slice('view-section-'.length);
            if (c.isIntersecting) seen.add(key);
            else seen.delete(key);
          }
          current = order.find((k) => seen.has(k)) ?? current;
        },
        { rootMargin: `-${Math.round(top)}px 0px 0px 0px` },
      );
      for (const k of order) {
        const el = document.getElementById(`view-section-${k}`);
        if (el) observer.observe(el);
      }
    };
    build();
    window.addEventListener('resize', build);
    return () => {
      observer?.disconnect();
      window.removeEventListener('resize', build);
    };
  });

  function pick(e: MouseEvent, key: string) {
    if (e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
    e.preventDefault();
    open = false;
    onjump(key);
  }

  /** The list folds when the focus leaves the index, or on Escape. */
  function leave(e: FocusEvent) {
    if (!nav.contains(e.relatedTarget as Node | null)) open = false;
  }

  function escape(e: KeyboardEvent) {
    if (e.key === 'Escape' && open) {
      open = false;
      toggle.focus();
    }
  }
</script>

<nav class="view-index" aria-label={t.home.viewIndex} bind:this={nav}>
  <p class="index-title eyebrow">{t.home.viewIndex}</p>
  <button
    type="button"
    class="btn index-toggle"
    aria-expanded={open}
    aria-controls={listId}
    bind:this={toggle}
    onclick={() => (open = !open)}
    onfocusout={leave}
    onkeydown={escape}
  >
    <span class="grow">{shown?.title ?? t.home.jumpToView}</span>
    {#if shown && shown.count !== null}<span class="count">{shown.count}</span>{/if}
    <span class="caret" aria-hidden="true">▾</span>
  </button>
  <ul id={listId} class="index-list" class:open>
    {#each entries as e (e.key)}
      <li>
        <a
          class="index-link"
          href={e.href}
          aria-current={e.key === current ? 'true' : undefined}
          onclick={(ev) => pick(ev, e.key)}
          onfocusout={leave}
          onkeydown={escape}
        >
          <span class="grow">{e.title}</span>
          {#if e.count !== null}<span class="count">{e.count}</span>{/if}
        </a>
      </li>
    {/each}
  </ul>
</nav>

<style>
  .view-index {
    position: sticky;
    top: calc(var(--topbar-height, 0px) + 16px);
    /* Its own scroller when taller than the room between the bars. The padding keeps focus rings. */
    max-height: calc(100dvh - var(--topbar-height, 0px) - var(--statusbar-height, 0px) - 32px);
    overflow-y: auto;
    margin: -4px;
    padding: 4px;
    scroll-padding-block: 8px;
    font-size: 0.84375rem;
  }

  .index-title {
    margin: 4px 0 12px;
  }

  .index-toggle {
    display: none;
  }

  .index-list {
    list-style: none;
    margin: 0;
    padding: 0;
  }

  .index-link {
    display: flex;
    align-items: baseline;
    gap: 8px;
    padding: 5px 10px;
    border-left: 2px solid transparent;
    border-radius: 0 var(--radius-s) var(--radius-s) 0;
    color: var(--ink-soft);
    text-decoration: none;
    line-height: 1.3;
  }

  .index-link:hover {
    color: var(--ink);
    background: var(--accent-wash);
  }

  .index-link[aria-current='true'] {
    color: var(--ink);
    font-weight: 600;
    border-left-color: var(--accent);
    background: var(--accent-wash);
  }

  .grow {
    flex: 1 1 auto;
    min-width: 0;
    overflow-wrap: anywhere;
  }

  .count {
    flex: none;
    padding: 1px 6px;
    border-radius: var(--radius-xs);
    background: var(--paper-sunk);
    color: var(--ink-soft);
    font: 400 0.75rem var(--font-mono);
  }

  @media (max-width: 900px) {
    .view-index {
      top: var(--topbar-height, 0px);
      z-index: 5;
      max-height: none;
      overflow: visible;
      margin: 0;
      padding: 8px 0;
      background: var(--paper);
    }

    .index-title {
      display: none;
    }

    .index-toggle {
      display: flex;
      align-items: center;
      gap: 8px;
      width: 100%;
      height: 2.25rem;
      text-align: left;
    }

    .index-toggle .grow {
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .index-list {
      display: none;
      position: absolute;
      left: 0;
      right: 0;
      top: calc(100% - 2px);
      max-height: calc(100dvh - var(--topbar-height, 0px) - var(--statusbar-height, 0px) - 4.5rem);
      overflow-y: auto;
      padding: 10px 6px;
      border: var(--frame) solid var(--rule-strong);
      border-radius: var(--radius-m);
      background: var(--paper-raised);
      box-shadow: var(--shadow-pop);
    }

    .index-list.open {
      display: block;
    }
  }
</style>
