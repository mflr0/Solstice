<script lang="ts">
  import { initials } from '../core/marks.js';
  import { paths } from '../core/route.js';
  import type { UserRef } from '../core/types.js';
  import { t } from '../i18n/t.js';

  /** A person: initials in a small square, then the name, linking to their page. */
  interface Props {
    user: UserRef | null;
    initialsBox?: boolean | undefined;
    empty?: string | undefined;
  }
  const { user, initialsBox = true, empty = t.common.unassigned }: Props = $props();
</script>

{#if user}
  <a
    class="who"
    class:inactive={!user.active}
    href={paths.person(user.username)}
    title={user.active ? user.username : `${user.username} — ${t.common.inactive}`}
  >
    {#if initialsBox}<span class="initials avatar" aria-hidden="true">{initials(user.name)}</span
      >{/if}
    <span class="name">{user.name}</span>
  </a>
{:else}
  <span class="nobody">{empty}</span>
{/if}

<style>
  .who {
    display: inline-flex;
    align-items: center;
    gap: 7px;
    text-decoration: none;
    max-width: 100%;
    min-width: 0;
  }

  .who:hover .name {
    text-decoration: underline;
  }

  .initials {
    display: inline-grid;
    place-items: center;
    width: 20px;
    height: 20px;
    flex: none;
    border: 1px solid var(--rule-strong);
    border-radius: var(--avatar-radius);
    background: var(--paper-raised);
    color: var(--ink-soft);
    font: 500 9.5px/1 var(--font-mono);
    letter-spacing: 0.02em;
  }

  .name {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .inactive .name {
    color: var(--muted);
    text-decoration: line-through;
    text-decoration-color: var(--rule-strong);
  }

  .nobody {
    color: var(--muted);
    font-style: italic;
  }
</style>
