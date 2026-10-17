<script lang="ts">
  import { initials } from '../core/marks.js';
  import { hasPicture, pictureUrl } from '../lib/api.js';

  /** The inside of an avatar box: the person's picture, else their initials. The box is the caller's. */
  interface Props {
    user: { id: number; name: string };
  }
  const { user }: Props = $props();

  let picture = $state(false);

  $effect(() => {
    const id = user.id;
    picture = false;
    let current = true;
    void hasPicture(id).then((has) => {
      if (current) picture = has;
    });
    return () => {
      current = false;
    };
  });
</script>

{#if picture}<img
    src={pictureUrl(user.id)}
    alt=""
    onerror={() => (picture = false)}
  />{:else}{initials(user.name)}{/if}

<style>
  img {
    width: 100%;
    height: 100%;
    object-fit: cover;
    border-radius: inherit;
    display: block;
  }
</style>
