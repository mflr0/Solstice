<script lang="ts">
  import { t } from '../i18n/t.js';
  import Avatar from './Avatar.svelte';
  import Highlight from './Highlight.svelte';

  /**
   * A person in a list that offers people: avatar, name, then the muted
   * username. It draws only the inside of the row; the row is the caller's.
   */
  interface Props {
    user: { id: number; active?: boolean | undefined };
    name: string;
    /** Left out for rows such as « Moi », which name nobody by username. */
    username?: string | undefined;
    /** A search whose matches the name and username mark. */
    search?: string | undefined;
  }
  const { user, name, username, search = '' }: Props = $props();
  const inactive = $derived(user.active === false);
  const gone = $derived(username ? ` · ${t.common.inactive}` : t.common.inactive);
</script>

<span class="face"><Avatar {user} /></span><span class="name" class:inactive
  ><Highlight text={name} {search} /></span
>
{#if username || inactive}
  <span class="user muted"
    >{#if username}<Highlight text={username} {search} />{/if}{inactive ? gone : ''}</span
  >
{/if}

<style>
  /* The avatar fits the line, so the row keeps the height it has without one. */
  .face {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 1.25rem;
    height: 1lh;
    flex: none;
  }

  .face :global(.avatar.avatar) {
    width: auto;
    height: 100%;
    max-height: 1.25rem;
    aspect-ratio: 1;
  }

  .name {
    min-width: 0;
    overflow-wrap: anywhere;
  }

  .inactive {
    color: var(--muted);
    text-decoration: line-through;
    text-decoration-color: var(--rule-strong);
  }

  .user {
    flex: none;
    margin-left: auto;
    padding-left: 4px;
    font: 0.71875rem var(--font-mono);
    white-space: nowrap;
  }
</style>
