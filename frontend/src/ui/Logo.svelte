<script lang="ts">
  import { skinInEffect } from '../lib/ui.svelte.js';

  /**
   * The mark: the sun on the horizon line (Carnet and skins that keep it),
   * the sun with a planet on its orbit (Orbite), a sun drawn in blue pencil
   * (Crayonné). Skins may hide its parts and paint .logo themselves.
   */
  interface Props {
    size?: number | undefined;
  }
  const { size = 22 }: Props = $props();
  const skin = skinInEffect();
</script>

<svg width={size} height={size} viewBox="0 0 24 24" aria-hidden="true" class="logo">
  {#if skin() === 'orbite'}
    <ellipse
      cx="12"
      cy="12.5"
      rx="10.2"
      ry="4.3"
      transform="rotate(-18 12 12.5)"
      class="logo-orbit"
    />
    <circle cx="12" cy="12.5" r="4.2" class="logo-sun" />
    <circle cx="21.1" cy="9.3" r="1.6" class="logo-planet" />
  {:else if skin() === 'crayon'}
    <path
      class="logo-sketch"
      d="M12.4 7.6c2.8-.1 4.9 2 4.9 4.6 0 2.7-2.2 4.9-5 4.9s-5-2.1-4.9-4.8c.1-2.5 2.1-4.5 4.8-4.6 1-.1 1.7.1 2.3.5"
    />
    <path
      class="logo-ray"
      d="M12.1 2.8v2.1M12 19.4v2M2.9 12.3h2.2M19.1 12h2.2M5.6 5.7l1.5 1.5M17.1 17.3l1.6 1.4M18.6 5.5l-1.5 1.6M5.5 18.6 7 17.1"
    />
  {:else}
    <path d="M6.5 15.5a5.5 5.5 0 0 1 11 0Z" class="logo-sun" />
    <path d="M2.5 15.5h19" class="logo-line" />
    <path d="M7 19h10" class="logo-line logo-faint" />
  {/if}
</svg>

<style>
  .logo path {
    fill: none;
    stroke: currentColor;
    stroke-width: 1.6;
    stroke-linecap: round;
  }

  .logo .logo-sun {
    fill: var(--sun);
    stroke: none;
  }

  .logo .logo-faint {
    stroke-width: 1.1;
    opacity: 0.55;
  }

  .logo .logo-orbit {
    fill: none;
    stroke: currentColor;
    stroke-width: 1.1;
    opacity: 0.55;
  }

  .logo .logo-planet {
    fill: currentColor;
  }

  .logo .logo-sketch {
    stroke: var(--accent);
    stroke-width: 1.5;
    stroke-linejoin: round;
  }

  .logo .logo-ray {
    stroke: var(--accent);
    stroke-width: 1.3;
  }
</style>
