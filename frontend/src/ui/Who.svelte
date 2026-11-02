<script lang="ts">
  import { paths } from '../core/route.js';
  import type { UserRef } from '../core/types.js';
  import { t } from '../i18n/t.js';
  import Avatar from './Avatar.svelte';
  import Highlight from './Highlight.svelte';

  /** A person: their picture, else initials, in a small square, then the name, linking to their page. */
  interface Props {
    user: UserRef | null;
    initialsBox?: boolean | undefined;
    empty?: string | undefined;
    /** False inside a link or a button, where a link is not valid. */
    link?: boolean | undefined;
    /** A search whose matches the name marks. */
    search?: string | undefined;
  }
  const {
    user,
    initialsBox = true,
    empty = t.common.unassigned,
    link = true,
    search = '',
  }: Props = $props();
</script>

{#if user}
  <svelte:element
    this={link ? 'a' : 'span'}
    class="who"
    class:inactive={!user.active}
    href={link ? paths.person(user.username) : undefined}
    title={user.active ? user.username : `${user.username} — ${t.common.inactive}`}
  >
    {#if initialsBox}<Avatar {user} />{/if}
    <span class="name"
      >{#if search}<Highlight text={user.name} {search} />{:else}{user.name}{/if}</span
    >
  </svelte:element>
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

  .who[href]:hover .name {
    text-decoration: underline;
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
