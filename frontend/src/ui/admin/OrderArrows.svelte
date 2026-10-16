<script lang="ts">
  import { t } from '../../i18n/t.js';

  /**
   * The two arrows that move a row up or down: the keyboard's way to
   * reorder a list (dragging is the mouse's). The ids let the page put the
   * focus back on the arrows after the list was saved.
   */
  interface Props {
    /** Unique among the page's rows. */
    id: string | number;
    /** The row's name, for the arrows' labels. */
    name: string;
    first: boolean;
    last: boolean;
    disabled?: boolean | undefined;
    onmove: (by: -1 | 1) => void;
  }
  const { id, name, first, last, disabled = false, onmove }: Props = $props();
</script>

<button
  type="button"
  class="btn btn-quiet btn-small arrow"
  id="order-up-{id}"
  aria-label={t.catalogAdmin.moveUp(name)}
  disabled={disabled || first}
  onclick={() => onmove(-1)}>↑</button
>
<button
  type="button"
  class="btn btn-quiet btn-small arrow"
  id="order-down-{id}"
  aria-label={t.catalogAdmin.moveDown(name)}
  disabled={disabled || last}
  onclick={() => onmove(1)}>↓</button
>

<style>
  .arrow {
    width: 24px;
    padding: 0;
    justify-content: center;
  }
</style>
