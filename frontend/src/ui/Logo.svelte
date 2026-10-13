<script lang="ts">
  /** The sun on the horizon line, as on a solstice morning. */
  interface Props {
    size?: number | undefined;
  }
  const { size = 22 }: Props = $props();
</script>

<svg width={size} height={size} viewBox="0 0 24 24" aria-hidden="true" class="logo">
  <path d="M6.5 15.5a5.5 5.5 0 0 1 11 0Z" class="sun" />
  <path d="M2.5 15.5h19" />
  <path d="M7 19h10" class="thin" />
</svg>

<style>
  .logo path {
    fill: none;
    stroke: currentColor;
    stroke-width: 1.6;
    stroke-linecap: round;
  }

  .logo .sun {
    fill: var(--accent);
    stroke: none;
  }

  .logo .thin {
    stroke-width: 1.1;
    opacity: 0.55;
  }
</style>
