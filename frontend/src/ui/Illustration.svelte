<script lang="ts">
  /**
   * A small drawing above an empty or lost page: an empty box that skins
   * may show and paint (see src/themes/ANATOMY.md). Carnet shows nothing.
   */
  interface Props {
    kind: 'lost' | 'empty' | 'search';
  }
  const { kind }: Props = $props();
</script>

<div class="illustration" data-kind={kind} aria-hidden="true"></div>
