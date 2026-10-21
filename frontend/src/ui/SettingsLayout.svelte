<script lang="ts">
  import type { Snippet } from 'svelte';
  import { router } from '../lib/router.svelte.js';
  import './settings.css';

  /**
   * The frame of the administration and of the personal settings: a side
   * navigation grouped by theme, then the section. On a narrow screen the
   * navigation folds into a button naming the current section.
   */
  interface NavLink {
    href: string;
    label: string;
    current: boolean;
  }
  interface NavGroup {
    label: string | null;
    links: NavLink[];
  }
  interface Props {
    /** The area's name, above the navigation ("Administration"). */
    area: string;
    /** The section's title, for skins that draw a window title (data-title). */
    title: string;
    groups: NavGroup[];
    children: Snippet;
  }
  const { area, title, groups, children }: Props = $props();

  let open = $state(false);
  const current = $derived(groups.flatMap((g) => g.links).find((l) => l.current));
  const uid = $props.id();
  const navId = `side-${uid}`;

  // Another section opened: fold the navigation again (narrow screens).
  $effect(() => {
    void router.path;
    open = false;
  });
</script>

<div class="page settings-area" data-title={title}>
  <div class="settings-layout">
    <nav class="side-nav" aria-label={area}>
      <p class="side-title eyebrow">{area}</p>
      <button
        type="button"
        class="btn side-toggle"
        aria-expanded={open}
        aria-controls={navId}
        onclick={() => (open = !open)}
      >
        <span class="grow">{current?.label ?? area}</span>
        <span class="caret" aria-hidden="true">▾</span>
      </button>
      <div class="side-body" class:open id={navId}>
        {#each groups as g, i (i)}
          <div class="side-group">
            {#if g.label}<p class="side-group-label">{g.label}</p>{/if}
            <ul>
              {#each g.links as l (l.href)}
                <li>
                  <a class="side-link" href={l.href} aria-current={l.current ? 'page' : undefined}
                    >{l.label}</a
                  >
                </li>
              {/each}
            </ul>
          </div>
        {/each}
      </div>
    </nav>
    <div class="settings-main">{@render children()}</div>
  </div>
</div>

<style>
  .settings-layout {
    display: grid;
    grid-template-columns: 210px minmax(0, 1fr);
    gap: 36px;
    align-items: start;
  }

  .side-nav {
    position: sticky;
    top: 16px;
    font-size: 13.5px;
  }

  .side-title {
    margin: 4px 0 12px;
  }

  .side-toggle {
    display: none;
  }

  .side-group + .side-group {
    margin-top: 14px;
  }

  .side-group-label {
    margin: 0 0 4px;
    padding: 0 10px;
    font: 600 11.5px/1.3 var(--font-body);
    color: var(--muted);
  }

  .side-group ul {
    list-style: none;
    margin: 0;
    padding: 0;
  }

  .side-link {
    display: block;
    padding: 5px 10px;
    border-left: 2px solid transparent;
    border-radius: 0 var(--radius-s) var(--radius-s) 0;
    color: var(--ink-soft);
    text-decoration: none;
    line-height: 1.3;
  }

  .side-link:hover {
    color: var(--ink);
    background: var(--accent-wash);
  }

  .side-link[aria-current='page'] {
    color: var(--ink);
    font-weight: 600;
    border-left-color: var(--accent);
    background: var(--accent-wash);
  }

  .settings-main {
    min-width: 0;
  }

  @media (max-width: 860px) {
    .settings-layout {
      grid-template-columns: minmax(0, 1fr);
      gap: 18px;
    }

    .side-nav {
      position: relative;
      top: 0;
    }

    .side-title {
      display: none;
    }

    .side-toggle {
      display: flex;
      width: 100%;
      height: 36px;
      justify-content: space-between;
      text-align: left;
    }

    .side-body {
      display: none;
      margin-top: 6px;
      padding: 10px 6px;
      border: var(--frame) solid var(--rule-strong);
      border-radius: var(--radius-m);
      background: var(--paper-raised);
    }

    .side-body.open {
      display: block;
    }
  }
</style>
