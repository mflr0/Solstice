<script lang="ts">
  import { onBoard, scopeSwitch } from '../core/scope.js';
  import type { Project } from '../core/types.js';
  import { t } from '../i18n/t.js';
  import { menu, outside } from '../lib/actions.js';
  import { api, errorMessage } from '../lib/api.js';
  import { router } from '../lib/router.svelte.js';
  import { session } from '../lib/session.svelte.js';
  import { notify } from '../lib/ui.svelte.js';

  /** The top bar's project switcher: which project Work, Boards and Search cover. */
  let open = $state(false);
  let projects = $state<Project[] | null>(null);
  const current = $derived(session.me?.project ?? null);

  function toggle() {
    open = !open;
    if (open && !projects) {
      api
        .projects()
        .then((p) => (projects = p))
        .catch((e: unknown) => {
          open = false;
          notify(errorMessage(e), 'error');
        });
    }
  }

  async function choose(key: string | null) {
    open = false;
    if (key === session.scope) return;
    try {
      await session.setScope(key);
    } catch (e) {
      notify(errorMessage(e), 'error');
      return;
    }
    // The board pages follow the new scope; a search already made stays.
    const route = router.route;
    if (route.name !== 'boards' && !onBoard(route)) return;
    let to: string | null;
    try {
      to = scopeSwitch(route, await api.boards(), key);
    } catch (e) {
      notify(errorMessage(e), 'error');
      return;
    }
    // Stay put if the user went elsewhere while the boards loaded.
    if (to && router.route === route) router.navigate(to, { replace: true });
  }
</script>

<div class="scope" use:outside={() => (open = false)}>
  <button
    class="scope-button"
    aria-haspopup="menu"
    aria-expanded={open}
    aria-label={t.nav.scopeLabel(current?.name ?? t.nav.scopeAllProjects)}
    title={current?.name ?? t.nav.scopeAllProjects}
    onclick={toggle}
  >
    <span class="scope-key">{current?.key ?? t.nav.scopeAll}</span>
    <span class="caret" aria-hidden="true">▾</span>
  </button>
  {#if open && projects}
    <div class="popover scope-menu" role="menu" use:menu={() => (open = false)}>
      <button
        class="item"
        role="menuitemradio"
        aria-checked={current === null}
        onclick={() => void choose(null)}>{t.nav.scopeAllProjects}</button
      >
      {#if projects.length}<div class="sep"></div>{/if}
      {#each projects as p (p.id)}
        <button
          class="item"
          role="menuitemradio"
          aria-checked={current?.id === p.id}
          onclick={() => void choose(p.key)}
        >
          <span class="key">{p.key}</span>
          <span class="name">{p.name}</span>
        </button>
      {/each}
    </div>
  {/if}
</div>

<style>
  .scope {
    position: relative;
    flex: none;
  }

  .scope-button {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    height: 1.875rem;
    max-width: 12ch;
    padding: 0 8px;
    border: 1px solid var(--rule);
    border-radius: var(--radius-s);
    background: none;
    color: var(--bar-ink-soft);
    font: 500 0.78125rem/1 var(--font-mono);
    cursor: pointer;
  }

  .scope-button:hover,
  .scope-button[aria-expanded='true'] {
    color: var(--bar-ink);
  }

  .scope-key {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .caret {
    font-size: 0.625rem;
  }

  .scope-menu {
    left: 0;
    top: calc(100% + 8px);
    min-width: 16.25rem;
    max-height: calc(100dvh - 3.375rem - 18px);
  }

  .scope-menu .key {
    min-width: 6ch;
    font: 500 0.75rem/1 var(--font-mono);
    color: var(--muted);
  }

  .scope-menu .name {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
