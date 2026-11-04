<script lang="ts">
  /**
   * The mark: the sun on the horizon line. Skins with their own mark hide
   * its parts and paint .logo themselves (see backend/internal/themes/ANATOMY.md).
   */
  interface Props {
    size?: number | undefined;
  }
  const { size = 22 }: Props = $props();
</script>

<svg width={size} height={size} viewBox="0 0 24 24" aria-hidden="true" class="logo">
  <path d="M6.5 15.5a5.5 5.5 0 0 1 11 0Z" class="logo-sun" />
  <path d="M2.5 15.5h19" class="logo-line" />
  <path d="M7 19h10" class="logo-line logo-faint" />
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
</style>
