<script lang="ts">
  import Picture from './Picture.svelte';

  /** A person's small avatar box: their picture, else the initials the server draws. */
  interface Props {
    user: { id: number };
  }
  const { user }: Props = $props();
</script>

<span class="initials avatar" aria-hidden="true"><Picture {user} /></span>

<style>
  .initials {
    display: inline-grid;
    place-items: center;
    width: 1.25rem;
    height: 1.25rem;
    flex: none;
    border: 1px solid var(--rule-strong);
    border-radius: var(--avatar-radius);
    background: var(--paper-raised);
    color: var(--ink-soft);
    font: 500 0.59375rem/1 var(--font-mono);
    letter-spacing: 0.02em;
    overflow: hidden;
  }
</style>
