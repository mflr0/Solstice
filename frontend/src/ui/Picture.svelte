<script lang="ts">
  import { pictureUrl } from '../lib/api.js';

  /** The inside of an avatar box: the person's picture, else the initials the server draws. The box is the caller's. */
  interface Props {
    user: { id: number };
  }
  const { user }: Props = $props();
</script>

<img src={pictureUrl(user.id)} alt="" />

<style>
  img {
    width: 100%;
    height: 100%;
    object-fit: cover;
    border-radius: inherit;
    display: block;
  }
</style>
